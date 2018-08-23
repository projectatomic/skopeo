// Package iplist handles the P2P Plaintext Format described by
// https://en.wikipedia.org/wiki/PeerGuardian#P2P_plaintext_format.
package iplist

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
)

// An abstraction of IP list implementations.
type Ranger interface {
	// Return a Range containing the IP.
	Lookup(net.IP) (r Range, ok bool)
	// If your ranges hurt, use this.
	NumRanges() int
}

type IPList struct {
	ranges []Range
}

type Range struct {
	First, Last net.IP
	Description string
}

func (r Range) String() string {
	return fmt.Sprintf("%s-%s: %s", r.First, r.Last, r.Description)
}

// Create a new IP list. The given ranges must already sorted by the lower
// bound IP in each range. Behaviour is undefined for lists of overlapping
// ranges.
func New(initSorted []Range) *IPList {
	return &IPList{
		ranges: initSorted,
	}
}

func (ipl *IPList) NumRanges() int {
	if ipl == nil {
		return 0
	}
	return len(ipl.ranges)
}

// Return the range the given IP is in. ok if false if no range is found.
func (ipl *IPList) Lookup(ip net.IP) (r Range, ok bool) {
	if ipl == nil {
		return
	}
	// TODO: Perhaps all addresses should be converted to IPv6, if the future
	// of IP is to always be backwards compatible. But this will cost 4x the
	// memory for IPv4 addresses?
	v4 := ip.To4()
	if v4 != nil {
		r, ok = ipl.lookup(v4)
		if ok {
			return
		}
	}
	v6 := ip.To16()
	if v6 != nil {
		return ipl.lookup(v6)
	}
	if v4 == nil && v6 == nil {
		r = Range{
			Description: "bad IP",
		}
		ok = true
	}
	return
}

// Return a range that contains ip, or nil.
func lookup(
	first func(i int) net.IP,
	full func(i int) Range,
	n int,
	ip net.IP,
) (
	r Range, ok bool,
) {
	// Find the index of the first range for which the following range exceeds
	// it.
	i := sort.Search(n, func(i int) bool {
		if i+1 >= n {
			return true
		}
		return bytes.Compare(ip, first(i+1)) < 0
	})
	if i == n {
		return
	}
	r = full(i)
	ok = bytes.Compare(r.First, ip) <= 0 && bytes.Compare(ip, r.Last) <= 0
	return
}

// Return the range the given IP is in. Returns nil if no range is found.
func (ipl *IPList) lookup(ip net.IP) (Range, bool) {
	return lookup(func(i int) net.IP {
		return ipl.ranges[i].First
	}, func(i int) Range {
		return ipl.ranges[i]
	}, len(ipl.ranges), ip)
}

func minifyIP(ip *net.IP) {
	v4 := ip.To4()
	if v4 != nil {
		*ip = append(make([]byte, 0, 4), v4...)
	}
}

// Parse a line of the PeerGuardian Text Lists (P2P) Format. Returns !ok but
// no error if a line doesn't contain a range but isn't erroneous, such as
// comment and blank lines.
func ParseBlocklistP2PLine(l []byte) (r Range, ok bool, err error) {
	l = bytes.TrimSpace(l)
	if len(l) == 0 || bytes.HasPrefix(l, []byte("#")) {
		return
	}
	// TODO: Check this when IPv6 blocklists are available.
	colon := bytes.LastIndexAny(l, ":")
	if colon == -1 {
		err = errors.New("missing colon")
		return
	}
	hyphen := bytes.IndexByte(l[colon+1:], '-')
	if hyphen == -1 {
		err = errors.New("missing hyphen")
		return
	}
	hyphen += colon + 1
	r.Description = string(l[:colon])
	r.First = net.ParseIP(string(l[colon+1 : hyphen]))
	minifyIP(&r.First)
	r.Last = net.ParseIP(string(l[hyphen+1:]))
	minifyIP(&r.Last)
	if r.First == nil || r.Last == nil || len(r.First) != len(r.Last) {
		err = errors.New("bad IP range")
		return
	}
	ok = true
	return
}

// Creates an IPList from a line-delimited P2P Plaintext file.
func NewFromReader(f io.Reader) (ret *IPList, err error) {
	var ranges []Range
	// There's a lot of similar descriptions, so we maintain a pool and reuse
	// them to reduce memory overhead.
	uniqStrs := make(map[string]string)
	scanner := bufio.NewScanner(f)
	lineNum := 1
	for scanner.Scan() {
		r, ok, lineErr := ParseBlocklistP2PLine(scanner.Bytes())
		if lineErr != nil {
			err = fmt.Errorf("error parsing line %d: %s", lineNum, lineErr)
			return
		}
		lineNum++
		if !ok {
			continue
		}
		if s, ok := uniqStrs[r.Description]; ok {
			r.Description = s
		} else {
			uniqStrs[r.Description] = r.Description
		}
		ranges = append(ranges, r)
	}
	err = scanner.Err()
	if err != nil {
		return
	}
	ret = New(ranges)
	return
}
