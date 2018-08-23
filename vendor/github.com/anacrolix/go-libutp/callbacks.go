package utp

/*
#include "utp.h"
*/
import "C"
import (
	"log"
	"reflect"
	"strings"
	"sync/atomic"
	"unsafe"
)

func (a *C.utp_callback_arguments) bufBytes() []byte {
	return *(*[]byte)(unsafe.Pointer(&reflect.SliceHeader{
		uintptr(unsafe.Pointer(a.buf)),
		int(a.len),
		int(a.len),
	}))
}

func (a *C.utp_callback_arguments) state() C.int {
	return *(*C.int)(unsafe.Pointer(&a.anon0))
}

func (a *C.utp_callback_arguments) error_code() C.int {
	return *(*C.int)(unsafe.Pointer(&a.anon0))
}

func (a *C.utp_callback_arguments) address() *C.struct_sockaddr {
	return *(**C.struct_sockaddr)(unsafe.Pointer(&a.anon0[0]))
}

func (a *C.utp_callback_arguments) addressLen() C.socklen_t {
	return *(*C.socklen_t)(unsafe.Pointer(&a.anon1[0]))
}

var sends int64

//export sendtoCallback
func sendtoCallback(a *C.utp_callback_arguments) (ret C.uint64) {
	s := getSocketForLibContext(a.context)
	b := a.bufBytes()
	addr := structSockaddrToUDPAddr(a.address())
	newSends := atomic.AddInt64(&sends, 1)
	if logCallbacks {
		Logger.Printf("sending %d bytes, %d packets", len(b), newSends)
	}
	expMap.Add("socket PacketConn writes", 1)
	n, err := s.pc.WriteTo(b, addr)
	c := s.conns[a.socket]
	if err != nil {
		expMap.Add("socket PacketConn write errors", 1)
		if c != nil && c.userOnError != nil {
			go c.userOnError(err)
		} else if c != nil &&
			(strings.Contains(err.Error(), "can't assign requested address") ||
				strings.Contains(err.Error(), "invalid argument")) {
			// Should be an bad argument or network configuration problem we
			// can't recover from.
			c.onError(err)
		} else if c != nil && strings.Contains(err.Error(), "operation not permitted") {
			// Rate-limited. Probably Linux. The implementation might try
			// again later.
		} else {
			Logger.Printf("error sending packet: %s", err)
		}
		return
	}
	if n != len(b) {
		expMap.Add("socket PacketConn short writes", 1)
		Logger.Printf("expected to send %d bytes but only sent %d", len(b), n)
	}
	return
}

//export errorCallback
func errorCallback(a *C.utp_callback_arguments) C.uint64 {
	err := errorForCode(a.error_code())
	if logCallbacks {
		log.Printf("error callback: socket %p: %s", a.socket, err)
	}
	libContextToSocket[a.context].conns[a.socket].onError(err)
	return 0
}

//export logCallback
func logCallback(a *C.utp_callback_arguments) C.uint64 {
	Logger.Printf("libutp: %s", C.GoString((*C.char)(unsafe.Pointer(a.buf))))
	return 0
}

//export stateChangeCallback
func stateChangeCallback(a *C.utp_callback_arguments) C.uint64 {
	s := libContextToSocket[a.context]
	c := s.conns[a.socket]
	if logCallbacks {
		Logger.Printf("state changed: conn %p: %s", c, libStateName(a.state()))
	}
	switch a.state() {
	case C.UTP_STATE_CONNECT:
		c.setConnected()
		// A dialled connection will not tell the remote it's ready until it
		// writes. If the dialer has no intention of writing, this will stall
		// everything. We do an empty write to get things rolling again. This
		// circumstance occurs when c1 in the RacyRead nettest is the dialer.
		C.utp_write(a.socket, nil, 0)
	case C.UTP_STATE_WRITABLE:
		c.cond.Broadcast()
	case C.UTP_STATE_EOF:
		c.setGotEOF()
	case C.UTP_STATE_DESTROYING:
		c.onDestroyed()
		s.onLibSocketDestroyed(a.socket)
	default:
		panic(a.state)
	}
	return 0
}

//export readCallback
func readCallback(a *C.utp_callback_arguments) C.uint64 {
	s := libContextToSocket[a.context]
	c := s.conns[a.socket]
	b := a.bufBytes()
	if logCallbacks {
		log.Printf("read callback: conn %p: %d bytes", c, len(b))
	}
	if len(b) == 0 {
		panic("that will break the read drain invariant")
	}
	c.readBuf.Write(b)
	c.cond.Broadcast()
	return 0
}

//export acceptCallback
func acceptCallback(a *C.utp_callback_arguments) C.uint64 {
	if logCallbacks {
		log.Printf("accept callback: %#v", *a)
	}
	s := getSocketForLibContext(a.context)
	c := s.newConn(a.socket)
	c.setRemoteAddr()
	c.inited = true
	s.pushBacklog(c)
	return 0
}

//export getReadBufferSizeCallback
func getReadBufferSizeCallback(a *C.utp_callback_arguments) (ret C.uint64) {
	s := libContextToSocket[a.context]
	c := s.conns[a.socket]
	if c == nil {
		// socket hasn't been added to the Socket.conns yet. The read buffer
		// starts out empty, and the default implementation for this callback
		// returns 0, so we'll return that.
		return 0
	}
	ret = C.uint64(c.readBuf.Len())
	return
}

//export firewallCallback
func firewallCallback(a *C.utp_callback_arguments) C.uint64 {
	s := getSocketForLibContext(a.context)
	if s.block {
		return 1
	} else {
		return 0
	}
}
