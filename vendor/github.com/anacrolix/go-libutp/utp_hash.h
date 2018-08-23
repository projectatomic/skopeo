/*
 * Copyright (c) 2010-2013 BitTorrent, Inc.
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
 * THE SOFTWARE.
 */

#ifndef __UTP_HASH_H__
#define __UTP_HASH_H__

#include <string.h> // memset
#include <stdlib.h>	// malloc

#include "utp_types.h"
#include "utp_templates.h"

// TODO: make utp_link_t a template parameter to HashTable
typedef uint32 utp_link_t;

#ifdef _MSC_VER
// Silence the warning about the C99-compliant zero-length array at the end of the structure
#pragma warning (disable: 4200)
#endif

typedef uint32 (*utp_hash_compute_t)(const void *keyp, size_t keysize);
typedef uint (*utp_hash_equal_t)(const void *key_a, const void *key_b, size_t keysize);

// In memory the HashTable is laid out as follows:
//  ---------------------------- low
//  | hash table data members  |
//  ----------------------------  _
//  |  indices                 |  ^
//  |  .                       |  |  utp_link_t indices into the key-values.
//  |  .                       |  .
//  ----------------------------  -  <----- bep
//  |  keys and values         |     each key-value pair has size total_size
//  |  .                       |
//  |  .                       |
//  ---------------------------- high
//
// The code depends on the ability of the compiler to pad the length
// of the hash table data members structure to
// a length divisible by 32-bits with no remainder.
//
// Since the number of hash buckets (indices) should be odd, the code
// asserts this and adds one to the hash bucket count to ensure that the
// following key-value pairs array starts on a 32-bit boundary.
//
// The key-value pairs array should start on a 32-bit boundary, otherwise
// processors like the ARM will silently mangle 32-bit data in these structures
// (e.g., turning 0xABCD into 0XCDAB when moving a value from memory to register
// when the memory address is 16 bits offset from a 32-bit boundary),
// also, the value will be stored at an address two bytes lower than the address
// value would ordinarily indicate.
//
// The key-value pair is of type T. The first field in T must
// be the key, i.e., the first K bytes of T contains the key.
// total_size = sizeof(T) and thus sizeof(T) >= sizeof(K)
//
// N is the number of buckets.
//
struct utp_hash_t {
	utp_link_t N;
	byte K;
	byte E;
	size_t count;
	utp_hash_compute_t hash_compute;
	utp_hash_equal_t hash_equal;
	utp_link_t allocated;
	utp_link_t used;
	utp_link_t free;
	utp_link_t inits[0];
};

#ifdef _MSC_VER
#pragma warning (default: 4200)
#endif

struct utp_hash_iterator_t {
	utp_link_t bucket;
	utp_link_t elem;

	utp_hash_iterator_t() : bucket(0xffffffff), elem(0xffffffff) {}
};

uint utp_hash_mem(const void *keyp, size_t keysize);
uint utp_hash_comp(const void *key_a, const void *key_b, size_t keysize);

utp_hash_t *utp_hash_create(int N, int key_size, int total_size, int initial, utp_hash_compute_t hashfun = utp_hash_mem, utp_hash_equal_t eqfun = NULL);
void *utp_hash_lookup(utp_hash_t *hash, const void *key);
void *utp_hash_add(utp_hash_t **hashp, const void *key);
void *utp_hash_del(utp_hash_t *hash, const void *key);

void *utp_hash_iterate(utp_hash_t *hash, utp_hash_iterator_t *iter);
void utp_hash_free_mem(utp_hash_t *hash);

/*
	This HashTable requires that T have at least sizeof(K)+sizeof(utp_link_t) bytes.
	Usually done like this:

	struct K {
		int whatever;
	};

	struct T {
		K wtf;
		utp_link_t link; // also wtf
	};
*/

template<typename K, typename T> class utpHashTable {
	utp_hash_t *hash;
public:
	static uint compare(const void *k1, const void *k2, size_t ks) {
		return *((K*)k1) == *((K*)k2);
	}
	static uint32 compute_hash(const void *k, size_t ks) {
		return ((K*)k)->compute_hash();
	}
	void Init() { hash = NULL; }
	bool Allocated() { return (hash != NULL); }
	void Free() { utp_hash_free_mem(hash); hash = NULL; }
	void Create(int N, int initial) { hash = utp_hash_create(N, sizeof(K), sizeof(T), initial, &compute_hash, &compare); }
	T *Lookup(const K &key) { return (T*)utp_hash_lookup(hash, &key); }
	T *Add(const K &key) { return (T*)utp_hash_add(&hash, &key); }
	T *Delete(const K &key) { return (T*)utp_hash_del(hash, &key); }
	T *Iterate(utp_hash_iterator_t &iterator) { return (T*)utp_hash_iterate(hash, &iterator); }
	size_t GetCount() { return hash->count; }
};

#endif //__UTP_HASH_H__
