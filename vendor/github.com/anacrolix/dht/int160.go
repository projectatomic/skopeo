package dht

import (
	"math"
	"math/big"
)

type int160 struct {
	bits [20]uint8
}

func (me *int160) AsByteArray() [20]byte {
	return me.bits
}

func (me *int160) ByteString() string {
	return string(me.bits[:])
}

func (me *int160) BitLen() int {
	var a big.Int
	a.SetBytes(me.bits[:])
	return a.BitLen()
}

func (me *int160) SetBytes(b []byte) {
	n := copy(me.bits[:], b)
	if n != 20 {
		panic(n)
	}
}

func (me int160) Bytes() []byte {
	return me.bits[:]
}

func (l int160) Cmp(r int160) int {
	for i := range l.bits {
		if l.bits[i] < r.bits[i] {
			return -1
		} else if l.bits[i] > r.bits[i] {
			return 1
		}
	}
	return 0
}

func (me *int160) SetMax() {
	for i := range me.bits {
		me.bits[i] = math.MaxUint8
	}
}

func (me *int160) Xor(a, b *int160) {
	for i := range me.bits {
		me.bits[i] = a.bits[i] ^ b.bits[i]
	}
}

func (me *int160) IsZero() bool {
	for _, b := range me.bits {
		if b != 0 {
			return false
		}
	}
	return true
}

func int160FromBytes(b []byte) (ret int160) {
	ret.SetBytes(b)
	return
}

func int160FromByteArray(b [20]byte) (ret int160) {
	ret.SetBytes(b[:])
	return
}

func int160FromByteString(s string) (ret int160) {
	ret.SetBytes([]byte(s))
	return
}

func distance(a, b int160) (ret int160) {
	ret.Xor(&a, &b)
	return
}
