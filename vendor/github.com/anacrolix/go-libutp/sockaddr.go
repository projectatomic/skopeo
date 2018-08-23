package utp

/*
#include "utp.h"
*/
import "C"
import (
	"net"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/anacrolix/missinggo/inproc"
)

func toSockaddrInet(ip net.IP, port int, zone string) (*C.struct_sockaddr, C.socklen_t) {
	if ip4 := ip.To4(); ip4 != nil && zone == "" {
		rsa := syscall.RawSockaddrInet4{
			// Len:    syscall.SizeofSockaddrInet4,
			Family: syscall.AF_INET,
			Port:   uint16(port),
		}
		if n := copy(rsa.Addr[:], ip4); n != 4 {
			panic(n)
		}
		return (*C.struct_sockaddr)(unsafe.Pointer(&rsa)), C.socklen_t(unsafe.Sizeof(rsa))
	}
	rsa := syscall.RawSockaddrInet6{
		// Len:      syscall.SizeofSockaddrInet6,
		Family:   syscall.AF_INET6,
		Scope_id: zoneToScopeId(zone),
		Port:     uint16(port),
	}
	if ip != nil {
		if n := copy(rsa.Addr[:], ip); n != 16 {
			panic(n)
		}
	}
	return (*C.struct_sockaddr)(unsafe.Pointer(&rsa)), C.socklen_t(unsafe.Sizeof(rsa))
}

func zoneToScopeId(zone string) uint32 {
	if zone == "" {
		return 0
	}
	if ifi, err := net.InterfaceByName(zone); err == nil {
		return uint32(ifi.Index)
	}
	ui64, _ := strconv.ParseUint(zone, 10, 32)
	return uint32(ui64)
}

func structSockaddrToUDPAddr(sa *C.struct_sockaddr) *net.UDPAddr {
	meh, err := anyToSockaddr((*syscall.RawSockaddrAny)(unsafe.Pointer(sa)))
	if err != nil {
		panic(err)
	}
	return sockaddrToUDP(meh).(*net.UDPAddr)
}

func anyToSockaddr(rsa *syscall.RawSockaddrAny) (syscall.Sockaddr, error) {
	// log.Printf("anyToSockaddr %#v", rsa)
	switch rsa.Addr.Family {

	case syscall.AF_INET:
		pp := (*syscall.RawSockaddrInet4)(unsafe.Pointer(rsa))
		sa := new(syscall.SockaddrInet4)
		// p := (*[2]byte)(unsafe.Pointer(&pp.Port))
		// sa.Port = int(p[0])<<8 + int(p[1])
		// I don't know why the port isn't reversed when it comes from utp.
		sa.Port = int(pp.Port)
		for i := 0; i < len(sa.Addr); i++ {
			sa.Addr[i] = pp.Addr[i]
		}
		return sa, nil

	case syscall.AF_INET6:
		pp := (*syscall.RawSockaddrInet6)(unsafe.Pointer(rsa))
		sa := new(syscall.SockaddrInet6)
		// p := (*[2]byte)(unsafe.Pointer(&pp.Port))
		// sa.Port = int(p[0])<<8 + int(p[1])
		// I don't know why the port isn't reversed when it comes from utp.
		sa.Port = int(pp.Port)
		sa.ZoneId = pp.Scope_id
		for i := 0; i < len(sa.Addr); i++ {
			sa.Addr[i] = pp.Addr[i]
		}
		return sa, nil
	}
	return nil, syscall.EAFNOSUPPORT
}

func sockaddrToUDP(sa syscall.Sockaddr) net.Addr {
	switch sa := sa.(type) {
	case *syscall.SockaddrInet4:
		return &net.UDPAddr{IP: sa.Addr[0:], Port: sa.Port}
	case *syscall.SockaddrInet6:
		return &net.UDPAddr{IP: sa.Addr[0:], Port: sa.Port /*Zone: zoneToString(int(sa.ZoneId))*/}
	}
	return nil
}

func netAddrToLibSockaddr(na net.Addr) (*C.struct_sockaddr, C.socklen_t) {
	switch v := na.(type) {
	case *net.UDPAddr:
		return toSockaddrInet(v.IP, v.Port, v.Zone)
	case inproc.Addr:
		rsa := syscall.RawSockaddrInet6{
			Port: uint16(v.Port),
		}
		return (*C.struct_sockaddr)(unsafe.Pointer(&rsa)), C.socklen_t(unsafe.Sizeof(rsa))
	default:
		panic(na)
	}
}
