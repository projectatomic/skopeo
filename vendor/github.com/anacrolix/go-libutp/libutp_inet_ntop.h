#ifndef LIBUTP_INET_NTOP_H
#define LIBUTP_INET_NTOP_H

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

// About us linking the system inet_pton and inet_ntop symbols:
// 1) These symbols are usually defined on POSIX systems
// 2) They are not defined on Windows versions earlier than Vista
// Defined in:
// ut_utils/src/sockaddr.cpp
// libutp/win32_inet_ntop.obj
// 
// When we drop support for XP we can just #include <ws2tcpip.h>, and use the system functions
// For now, we will always use our functions on windows, on all builds
// The reason is: we would like the debug build to behave as much as the release build as possible
// It is much better to catch a problem in the debug build, than to link the system version
// in debug, and our version int he wild.

#if defined(_WIN32_WINNT)
#if _WIN32_WINNT >= 0x600 // Win32, post-XP
#include <ws2tcpip.h> // for inet_ntop, inet_pton
#define INET_NTOP inet_ntop
#define INET_PTON inet_pton
#else
#define INET_NTOP libutp::inet_ntop // Win32, pre-XP: Use ours
#define INET_PTON libutp::inet_pton
#endif
#else // not WIN32
#include <arpa/inet.h> // for inet_ntop, inet_pton
#define INET_NTOP inet_ntop
#define INET_PTON inet_pton
#endif

//######################################################################
//######################################################################
namespace libutp {


//######################################################################
const char *inet_ntop(int af, const void *src, char *dest, size_t length);

//######################################################################
int inet_pton(int af, const char* src, void* dest);


} //namespace libutp

#endif // LIBUTP_INET_NTOP_H