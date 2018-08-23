package iplist

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/edsrzf/mmap-go"
)

// The packed format is an 8 byte integer of the number of ranges. Then 20
// bytes per range, consisting of 4 byte packed IP being the lower bound IP of
// the range, then 4 bytes of the upper, inclusive bound, 8 bytes for the
// offset of the description from the end of the packed ranges, and 4 bytes
// for the length of the description. After these packed ranges, are the
// concatenated descriptions.

const (
	packedRangesOffset = 8
	packedRangeLen     = 44
)

func (ipl *IPList) WritePacked(w io.Writer) (err error) {
	descOffsets := make(map[string]int64, len(ipl.ranges))
	descs := make([]string, 0, len(ipl.ranges))
	var nextOffset int64
	// This is a little monadic, no?
	write := func(b []byte, expectedLen int) {
		if err != nil {
			return
		}
		var n int
		n, err = w.Write(b)
		if err != nil {
			return
		}
		if n != expectedLen {
			panic(n)
		}
	}
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(len(ipl.ranges)))
	write(b[:], 8)
	for _, r := range ipl.ranges {
		write(r.First.To16(), 16)
		write(r.Last.To16(), 16)
		descOff, ok := descOffsets[r.Description]
		if !ok {
			descOff = nextOffset
			descOffsets[r.Description] = descOff
			descs = append(descs, r.Description)
			nextOffset += int64(len(r.Description))
		}
		binary.LittleEndian.PutUint64(b[:], uint64(descOff))
		write(b[:], 8)
		binary.LittleEndian.PutUint32(b[:], uint32(len(r.Description)))
		write(b[:4], 4)
	}
	for _, d := range descs {
		write([]byte(d), len(d))
	}
	return
}

func NewFromPacked(b []byte) PackedIPList {
	ret := PackedIPList(b)
	minLen := packedRangesOffset + ret.len()*packedRangeLen
	if len(b) < minLen {
		panic(fmt.Sprintf("packed len %d < %d", len(b), minLen))
	}
	return ret
}

type PackedIPList []byte

var _ Ranger = PackedIPList{}

func (pil PackedIPList) len() int {
	return int(binary.LittleEndian.Uint64(pil[:8]))
}

func (pil PackedIPList) NumRanges() int {
	return pil.len()
}

func (pil PackedIPList) getFirst(i int) net.IP {
	off := packedRangesOffset + packedRangeLen*i
	return net.IP(pil[off : off+16])
}

func (pil PackedIPList) getRange(i int) (ret Range) {
	rOff := packedRangesOffset + packedRangeLen*i
	last := pil[rOff+16 : rOff+32]
	descOff := int(binary.LittleEndian.Uint64(pil[rOff+32:]))
	descLen := int(binary.LittleEndian.Uint32(pil[rOff+40:]))
	descOff += packedRangesOffset + packedRangeLen*pil.len()
	ret = Range{
		pil.getFirst(i),
		net.IP(last),
		string(pil[descOff : descOff+descLen]),
	}
	return
}

func (pil PackedIPList) Lookup(ip net.IP) (r Range, ok bool) {
	ip16 := ip.To16()
	if ip16 == nil {
		panic(ip)
	}
	return lookup(pil.getFirst, pil.getRange, pil.len(), ip16)
}

type closerFunc func() error

func (me closerFunc) Close() error {
	return me()
}

func MMapPackedFile(filename string) (
	ret interface {
		Ranger
		io.Closer
	},
	err error,
) {
	f, err := os.Open(filename)
	if err != nil {
		return
	}
	defer f.Close()
	mm, err := mmap.Map(f, mmap.RDONLY, 0)
	if err != nil {
		return
	}
	ret = struct {
		Ranger
		io.Closer
	}{NewFromPacked(mm), closerFunc(mm.Unmap)}
	return
}
