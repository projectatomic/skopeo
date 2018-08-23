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

#ifndef __UTP_H__
#define __UTP_H__

#ifdef __cplusplus
extern "C" {
#endif

#ifdef _WIN32
#include <stdint.h>
#endif

#include <stdarg.h>
#include "utp_types.h"

typedef struct UTPSocket					utp_socket;
typedef struct struct_utp_context			utp_context;

enum {
	UTP_UDP_DONTFRAG = 2,	// Used to be a #define as UDP_IP_DONTFRAG
};

enum {
	// socket has reveived syn-ack (notification only for outgoing connection completion)
	// this implies writability
	UTP_STATE_CONNECT = 1,

	// socket is able to send more data
	UTP_STATE_WRITABLE = 2,

	// connection closed
	UTP_STATE_EOF = 3,

	// socket is being destroyed, meaning all data has been sent if possible.
	// it is not valid to refer to the socket after this state change occurs
	UTP_STATE_DESTROYING = 4,
};

extern const char *utp_state_names[];

// Errors codes that can be passed to UTP_ON_ERROR callback
enum {
	UTP_ECONNREFUSED = 0,
	UTP_ECONNRESET,
	UTP_ETIMEDOUT,
};

extern const char *utp_error_code_names[];

enum {
	// callback names
	UTP_ON_FIREWALL = 0,
	UTP_ON_ACCEPT,
	UTP_ON_CONNECT,
	UTP_ON_ERROR,
	UTP_ON_READ,
	UTP_ON_OVERHEAD_STATISTICS,
	UTP_ON_STATE_CHANGE,
	UTP_GET_READ_BUFFER_SIZE,
	UTP_ON_DELAY_SAMPLE,
	UTP_GET_UDP_MTU,
	UTP_GET_UDP_OVERHEAD,
	UTP_GET_MILLISECONDS,
	UTP_GET_MICROSECONDS,
	UTP_GET_RANDOM,
	UTP_LOG,
	UTP_SENDTO,

	// context and socket options that may be set/queried
    UTP_LOG_NORMAL,
    UTP_LOG_MTU,
    UTP_LOG_DEBUG,
	UTP_SNDBUF,
	UTP_RCVBUF,
	UTP_TARGET_DELAY,

	UTP_ARRAY_SIZE,	// must be last
};

extern const char *utp_callback_names[];

typedef struct {
	utp_context *context;
	utp_socket *socket;
	size_t len;
	uint32 flags;
	int callback_type;
	const byte *buf;

	union {
		const struct sockaddr *address;
		int send;
		int sample_ms;
		int error_code;
		int state;
	};

	union {
		socklen_t address_len;
		int type;
	};
} utp_callback_arguments;

typedef uint64 utp_callback_t(utp_callback_arguments *);

// Returned by utp_get_context_stats()
typedef struct {
	uint32 _nraw_recv[5];	// total packets recieved less than 300/600/1200/MTU bytes fpr all connections (context-wide)
	uint32 _nraw_send[5];	// total packets sent     less than 300/600/1200/MTU bytes for all connections (context-wide)
} utp_context_stats;

// Returned by utp_get_stats()
typedef struct {
	uint64 nbytes_recv;	// total bytes received
	uint64 nbytes_xmit;	// total bytes transmitted
	uint32 rexmit;		// retransmit counter
	uint32 fastrexmit;	// fast retransmit counter
	uint32 nxmit;		// transmit counter
	uint32 nrecv;		// receive counter (total)
	uint32 nduprecv;	// duplicate receive counter
	uint32 mtu_guess;	// Best guess at MTU
} utp_socket_stats;

#define UTP_IOV_MAX 1024

// For utp_writev, to writes data from multiple buffers
struct utp_iovec {
	void *iov_base;
	size_t iov_len;
};

// Public Functions
utp_context*	utp_init						(int version);
void			utp_destroy						(utp_context *ctx);
void			utp_set_callback				(utp_context *ctx, int callback_name, utp_callback_t *proc);
void*			utp_context_set_userdata		(utp_context *ctx, void *userdata);
void*			utp_context_get_userdata		(utp_context *ctx);
int				utp_context_set_option			(utp_context *ctx, int opt, int val);
int				utp_context_get_option			(utp_context *ctx, int opt);
int				utp_process_udp					(utp_context *ctx, const byte *buf, size_t len, const struct sockaddr *to, socklen_t tolen);
int				utp_process_icmp_error			(utp_context *ctx, const byte *buffer, size_t len, const struct sockaddr *to, socklen_t tolen);
int				utp_process_icmp_fragmentation	(utp_context *ctx, const byte *buffer, size_t len, const struct sockaddr *to, socklen_t tolen, uint16 next_hop_mtu);
void			utp_check_timeouts				(utp_context *ctx);
void			utp_issue_deferred_acks			(utp_context *ctx);
utp_context_stats* utp_get_context_stats		(utp_context *ctx);
utp_socket*		utp_create_socket				(utp_context *ctx);
void*			utp_set_userdata				(utp_socket *s, void *userdata);
void*			utp_get_userdata				(utp_socket *s);
int				utp_setsockopt					(utp_socket *s, int opt, int val);
int				utp_getsockopt					(utp_socket *s, int opt);
int				utp_connect						(utp_socket *s, const struct sockaddr *to, socklen_t tolen);
ssize_t			utp_write						(utp_socket *s, void *buf, size_t count);
ssize_t			utp_writev						(utp_socket *s, struct utp_iovec *iovec, size_t num_iovecs);
int				utp_getpeername					(utp_socket *s, struct sockaddr *addr, socklen_t *addrlen);
void			utp_read_drained				(utp_socket *s);
int				utp_get_delays					(utp_socket *s, uint32 *ours, uint32 *theirs, uint32 *age);
utp_socket_stats* utp_get_stats					(utp_socket *s);
utp_context*	utp_get_context					(utp_socket *s);
void			utp_shutdown					(utp_socket *s, int how);
void			utp_close						(utp_socket *s);

#ifdef __cplusplus
}
#endif

#endif //__UTP_H__
