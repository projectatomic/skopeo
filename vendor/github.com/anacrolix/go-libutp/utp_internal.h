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

#ifndef __UTP_INTERNAL_H__
#define __UTP_INTERNAL_H__

#include <stdarg.h>
#include <string.h>
#include <assert.h>
#include <stdio.h>

#include "utp.h"
#include "utp_callbacks.h"
#include "utp_templates.h"
#include "utp_hash.h"
#include "utp_hash.h"
#include "utp_packedsockaddr.h"

/* These originally lived in utp_config.h */
#define CCONTROL_TARGET (100 * 1000) // us

enum bandwidth_type_t {
	payload_bandwidth, connect_overhead,
	close_overhead, ack_overhead,
	header_overhead, retransmit_overhead
};

#ifdef WIN32
	#ifdef _MSC_VER
		#include "libutp_inet_ntop.h"
	#endif

	// newer versions of MSVC define these in errno.h
	#ifndef ECONNRESET
		#define ECONNRESET WSAECONNRESET
		#define EMSGSIZE WSAEMSGSIZE
		#define ECONNREFUSED WSAECONNREFUSED
		#define ETIMEDOUT WSAETIMEDOUT
	#endif
#endif

struct PACKED_ATTRIBUTE RST_Info {
	PackedSockAddr addr;
	uint32 connid;
	uint16 ack_nr;
	uint64 timestamp;
};

// It's really important that we don't have duplicate keys in the hash table.
// If we do, we'll eventually crash. if we try to remove the second instance
// of the key, we'll accidentally remove the first instead. then later,
// checkTimeouts will try to access the second one's already freed memory.
void UTP_FreeAll(struct UTPSocketHT *utp_sockets);

struct UTPSocketKey {
	PackedSockAddr addr;
	uint32 recv_id;		 // "conn_seed", "conn_id"

	UTPSocketKey(const PackedSockAddr& _addr, uint32 _recv_id) {
		memset(this, 0, sizeof(*this));
		addr = _addr;
		recv_id = _recv_id;
	}

	bool operator == (const UTPSocketKey &other) const {
		return recv_id == other.recv_id && addr == other.addr;
	}

	uint32 compute_hash() const {
		return recv_id ^ addr.compute_hash();
	}
};

struct UTPSocketKeyData {
	UTPSocketKey key;
	UTPSocket *socket;
	utp_link_t link;
};

#define UTP_SOCKET_BUCKETS 79
#define UTP_SOCKET_INIT    15

struct UTPSocketHT : utpHashTable<UTPSocketKey, UTPSocketKeyData> {
	UTPSocketHT() {
		const int buckets = UTP_SOCKET_BUCKETS;
		const int initial = UTP_SOCKET_INIT;
		this->Create(buckets, initial);
	}
	~UTPSocketHT() {
		UTP_FreeAll(this);
		this->Free();
	}
};

struct struct_utp_context {
	void *userdata;
	utp_callback_t* callbacks[UTP_ARRAY_SIZE];

	uint64 current_ms;
	utp_context_stats context_stats;
	UTPSocket *last_utp_socket;
	Array<UTPSocket*> ack_sockets;
	Array<RST_Info> rst_info;
	UTPSocketHT *utp_sockets;
	size_t target_delay;
	size_t opt_sndbuf;
	size_t opt_rcvbuf;
	uint64 last_check;

	struct_utp_context();
	~struct_utp_context();

	void log(int level, utp_socket *socket, char const *fmt, ...);
	void log_unchecked(utp_socket *socket, char const *fmt, ...);
	bool would_log(int level);

	bool log_normal:1;	// log normal events?
	bool log_mtu:1;		// log MTU related events?
	bool log_debug:1;	// log debugging events? (Must also compile with UTP_DEBUG_LOGGING defined)
};

#endif //__UTP_INTERNAL_H__
