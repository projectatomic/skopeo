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

#ifndef __UTP_TYPES_H__
#define __UTP_TYPES_H__

// Allow libutp consumers or prerequisites to override PACKED_ATTRIBUTE
#ifndef PACKED_ATTRIBUTE
#if defined BROKEN_GCC_STRUCTURE_PACKING && defined __GNUC__
	// Used for gcc tool chains accepting but not supporting pragma pack
	// See http://gcc.gnu.org/onlinedocs/gcc/Type-Attributes.html
	#define PACKED_ATTRIBUTE __attribute__((__packed__))
#else
	#define PACKED_ATTRIBUTE
#endif // defined BROKEN_GCC_STRUCTURE_PACKING && defined __GNUC__
#endif // ndef PACKED_ATTRIBUTE

#ifdef __GNUC__
	#define ALIGNED_ATTRIBUTE(x)  __attribute__((aligned (x)))
#else
	#define ALIGNED_ATTRIBUTE(x)
#endif

// hash.cpp needs socket definitions, which is why this networking specific
// code is inclued in utypes.h
#ifdef WIN32
	#define _CRT_SECURE_NO_DEPRECATE
	#define WIN32_LEAN_AND_MEAN
	#include <windows.h>
	#include <winsock2.h>
	#include <ws2tcpip.h>
	#define IP_OPT_DONTFRAG IP_DONTFRAGMENT
	#define SHUT_RD SD_RECEIVE
	#define SHUT_WR SD_SEND
	#define SHUT_RDWR SD_BOTH
#else
	#include <netinet/in.h>
	#include <arpa/inet.h>
	#include <unistd.h>
	#include <sys/socket.h>

	#ifdef IP_DONTFRAG
		#define IP_OPT_DONTFRAG IP_DONTFRAG
	#elif defined IP_DONTFRAGMENT
		#define IP_OPT_DONTFRAG IP_DONTFRAGMENT
	#else
		//#warning "I don't know how to set DF bit on this system"
	#endif
#endif

#ifdef _MSC_VER
	#include <BaseTsd.h>
	typedef SSIZE_T ssize_t;
#endif

#ifdef POSIX
	typedef struct sockaddr_storage SOCKADDR_STORAGE;
#endif

#ifdef WIN32
	#define I64u "%I64u"
#else
	#define I64u "%Lu"
#endif

// standard types
typedef unsigned char byte;
typedef unsigned char uint8;
typedef signed char int8;
typedef unsigned short uint16;
typedef signed short int16;
typedef unsigned int uint;
typedef unsigned int uint32;
typedef signed int int32;

#ifdef _MSC_VER
typedef unsigned __int64 uint64;
typedef signed __int64 int64;
#else
typedef unsigned long long uint64;
typedef long long int64;
#endif

/* compile-time assert */
#ifndef CASSERT
#define CASSERT( exp, name ) typedef int is_not_##name [ (exp ) ? 1 : -1 ];
#endif

CASSERT(8 == sizeof(uint64), sizeof_uint64_is_8)
CASSERT(8 == sizeof(int64), sizeof_int64_is_8)

#ifndef INT64_MAX
#define INT64_MAX 0x7fffffffffffffffLL
#endif

// always ANSI
typedef const char * cstr;
typedef char * str;

#ifndef __cplusplus
typedef uint8 bool;
#endif

#endif //__UTP_TYPES_H__
