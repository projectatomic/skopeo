package iplist

import (
	"bufio"
	"io"
	"net"
)

func ParseCIDRListReader(r io.Reader) (ret []Range, err error) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		err = func() (err error) {
			_, in, err := net.ParseCIDR(s.Text())
			if err != nil {
				return
			}
			ret = append(ret, Range{
				First: in.IP,
				Last:  IPNetLast(in),
			})
			return
		}()
		if err != nil {
			return
		}
	}
	return
}

// Returns the last, inclusive IP in a net.IPNet.
func IPNetLast(in *net.IPNet) (last net.IP) {
	n := len(in.IP)
	if n != len(in.Mask) {
		panic("wat")
	}
	last = make(net.IP, n)
	for i := 0; i < n; i++ {
		last[i] = in.IP[i] | ^in.Mask[i]
	}
	return
}
