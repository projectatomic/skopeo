package utp

/*
#include "utp.h"
*/
import "C"
import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime/pprof"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	ErrConnClosed    = errors.New("closed")
	errConnDestroyed = errors.New("destroyed")
)

type Conn struct {
	s          *Socket
	us         *C.utp_socket
	cond       sync.Cond
	readBuf    bytes.Buffer
	gotEOF     bool
	gotConnect bool
	// Set on state changed to UTP_STATE_DESTROYING. Not valid to refer to the
	// socket after getting this.
	destroyed bool
	// Conn.Close was called.
	closed bool
	// Corresponds to utp_socket.state != CS_UNITIALIZED. This requires the
	// utp_socket was obtained from the accept callback, or has had
	// utp_connect called on it. We can't call utp_close until it's true.
	inited bool

	err error

	writeDeadline      time.Time
	writeDeadlineTimer *time.Timer
	readDeadline       time.Time
	readDeadlineTimer  *time.Timer

	numBytesRead    int64
	numBytesWritten int64

	localAddr  net.Addr
	remoteAddr net.Addr

	// Called for non-fatal errors, such as packet write errors.
	userOnError func(error)
}

func (c *Conn) onError(err error) {
	c.err = err
	c.cond.Broadcast()
}

func (c *Conn) setConnected() {
	c.gotConnect = true
	c.cond.Broadcast()
}

func (c *Conn) waitForConnect(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		<-ctx.Done()
		c.cond.Broadcast()
	}()
	for {
		if c.closed {
			return ErrConnClosed
		}
		if c.err != nil {
			return c.err
		}
		if c.gotConnect {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		c.cond.Wait()
	}
}

func (c *Conn) Close() error {
	mu.Lock()
	defer mu.Unlock()
	c.close()
	return nil
}

func (c *Conn) close() {
	if c.inited && !c.destroyed && !c.closed {
		C.utp_close(c.us)
	}
	if !c.inited {
		// We'll never receive a destroy message, so we should remove it now.
		delete(c.s.conns, c.us)
	}
	c.closed = true
	c.cond.Broadcast()
}

func (c *Conn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *Conn) readNoWait(b []byte) (n int, err error) {
	n, _ = c.readBuf.Read(b)
	if n != 0 && c.readBuf.Len() == 0 {
		// Can we call this if the utp_socket is closed, destroyed or errored?
		if c.us != nil {
			C.utp_read_drained(c.us)
			// C.utp_issue_deferred_acks(C.utp_get_context(c.s))
		}
	}
	if c.readBuf.Len() != 0 {
		return
	}
	err = func() error {
		switch {
		case c.gotEOF:
			return io.EOF
		case c.err != nil:
			return c.err
		case c.destroyed:
			return errConnDestroyed
		case c.closed:
			return errors.New("closed")
		case !c.readDeadline.IsZero() && !time.Now().Before(c.readDeadline):
			return errDeadlineExceeded{}
		default:
			return nil
		}
	}()
	return
}

func (c *Conn) Read(b []byte) (int, error) {
	mu.Lock()
	defer mu.Unlock()
	for {
		n, err := c.readNoWait(b)
		c.numBytesRead += int64(n)
		// log.Printf("read %d bytes", c.numBytesRead)
		if n != 0 || len(b) == 0 || err != nil {
			// log.Printf("conn %p: read %d bytes: %s", c, n, err)
			return n, err
		}
		c.cond.Wait()
	}
}

func (c *Conn) writeNoWait(b []byte) (n int, err error) {
	err = func() error {
		switch {
		case c.err != nil:
			return c.err
		case c.closed:
			return ErrConnClosed
		case c.destroyed:
			return errConnDestroyed
		case !c.writeDeadline.IsZero() && !time.Now().Before(c.writeDeadline):
			return errDeadlineExceeded{}
		default:
			return nil
		}
	}()
	if err != nil {
		return
	}
	pprof.Do(context.Background(), pprof.Labels("cgo", "utp_write"), func(context.Context) {
		n = int(C.utp_write(c.us, unsafe.Pointer(&b[0]), C.size_t(len(b))))
	})
	if n < 0 {
		panic(n)
	}
	return
}

func (c *Conn) Write(b []byte) (n int, err error) {
	mu.Lock()
	defer mu.Unlock()
	for len(b) != 0 {
		var n1 int
		n1, err = c.writeNoWait(b)
		b = b[n1:]
		n += n1
		if err != nil {
			break
		}
		if n1 != 0 {
			continue
		}
		c.cond.Wait()
	}
	c.numBytesWritten += int64(n)
	// log.Printf("wrote %d bytes", c.numBytesWritten)
	return
}

func (c *Conn) setRemoteAddr() {
	var rsa syscall.RawSockaddrAny
	var addrlen C.socklen_t = C.socklen_t(unsafe.Sizeof(rsa))
	C.utp_getpeername(c.us, (*C.struct_sockaddr)(unsafe.Pointer(&rsa)), &addrlen)
	sa, err := anyToSockaddr(&rsa)
	if err != nil {
		panic(err)
	}
	c.remoteAddr = sockaddrToUDP(sa)
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

func (c *Conn) SetDeadline(t time.Time) error {
	mu.Lock()
	defer mu.Unlock()
	c.readDeadline = t
	c.writeDeadline = t
	if t.IsZero() {
		c.readDeadlineTimer.Stop()
		c.writeDeadlineTimer.Stop()
	} else {
		d := t.Sub(time.Now())
		c.readDeadlineTimer.Reset(d)
		c.writeDeadlineTimer.Reset(d)
	}
	c.cond.Broadcast()
	return nil
}
func (c *Conn) SetReadDeadline(t time.Time) error {
	mu.Lock()
	defer mu.Unlock()
	c.readDeadline = t
	if t.IsZero() {
		c.readDeadlineTimer.Stop()
	} else {
		d := t.Sub(time.Now())
		c.readDeadlineTimer.Reset(d)
	}
	c.cond.Broadcast()
	return nil
}
func (c *Conn) SetWriteDeadline(t time.Time) error {
	mu.Lock()
	defer mu.Unlock()
	c.writeDeadline = t
	if t.IsZero() {
		c.writeDeadlineTimer.Stop()
	} else {
		d := t.Sub(time.Now())
		c.writeDeadlineTimer.Reset(d)
	}
	c.cond.Broadcast()
	return nil
}

func (c *Conn) setGotEOF() {
	c.gotEOF = true
	c.cond.Broadcast()
}

func (c *Conn) onDestroyed() {
	c.destroyed = true
	c.us = nil
	c.cond.Broadcast()
}

func (c *Conn) WriteBufferLen() int {
	mu.Lock()
	defer mu.Unlock()
	return int(C.utp_getsockopt(c.us, C.UTP_SNDBUF))
}

func (c *Conn) SetWriteBufferLen(len int) {
	mu.Lock()
	defer mu.Unlock()
	i := C.utp_setsockopt(c.us, C.UTP_SNDBUF, C.int(len))
	if i != 0 {
		panic(i)
	}
}

// Connect an unconnected Conn (obtained through Socket.NewConn).
func (c *Conn) Connect(ctx context.Context, network, addr string) error {
	if network == "" {
		network = c.localAddr.Network()
	}
	ua, err := resolveAddr(network, addr)
	if err != nil {
		return fmt.Errorf("error resolving address: %v", err)
	}
	sa, sl := netAddrToLibSockaddr(ua)
	mu.Lock()
	defer mu.Unlock()
	if c.s.closed {
		return errSocketClosed
	}
	if n := C.utp_connect(c.us, sa, sl); n != 0 {
		panic(n)
	}
	c.inited = true
	c.setRemoteAddr()
	err = c.waitForConnect(ctx)
	if err != nil {
		c.close()
		return err
	}
	return nil
}

func (c *Conn) OnError(f func(error)) {
	mu.Lock()
	c.userOnError = f
	mu.Unlock()
}
