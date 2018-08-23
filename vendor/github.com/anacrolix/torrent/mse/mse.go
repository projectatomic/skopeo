// https://wiki.vuze.com/w/Message_Stream_Encryption

package mse

import (
	"bytes"
	"crypto/rand"
	"crypto/rc4"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"expvar"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"math/big"
	"strconv"
	"sync"

	"github.com/anacrolix/missinggo/perf"
	"github.com/bradfitz/iter"
)

const (
	maxPadLen = 512

	CryptoMethodPlaintext CryptoMethod = 1
	CryptoMethodRC4       CryptoMethod = 2
	AllSupportedCrypto                 = CryptoMethodPlaintext | CryptoMethodRC4
)

type CryptoMethod uint32

var (
	// Prime P according to the spec, and G, the generator.
	p, g big.Int
	// The rand.Int max arg for use in newPadLen()
	newPadLenMax big.Int
	// For use in initer's hashes
	req1 = []byte("req1")
	req2 = []byte("req2")
	req3 = []byte("req3")
	// Verification constant "VC" which is all zeroes in the bittorrent
	// implementation.
	vc [8]byte
	// Zero padding
	zeroPad [512]byte
	// Tracks counts of received crypto_provides
	cryptoProvidesCount = expvar.NewMap("mseCryptoProvides")
)

func init() {
	p.SetString("0xFFFFFFFFFFFFFFFFC90FDAA22168C234C4C6628B80DC1CD129024E088A67CC74020BBEA63B139B22514A08798E3404DDEF9519B3CD3A431B302B0A6DF25F14374FE1356D6D51C245E485B576625E7EC6F44C42E9A63A36210000000000090563", 0)
	g.SetInt64(2)
	newPadLenMax.SetInt64(maxPadLen + 1)
}

func hash(parts ...[]byte) []byte {
	h := sha1.New()
	for _, p := range parts {
		n, err := h.Write(p)
		if err != nil {
			panic(err)
		}
		if n != len(p) {
			panic(n)
		}
	}
	return h.Sum(nil)
}

func newEncrypt(initer bool, s []byte, skey []byte) (c *rc4.Cipher) {
	c, err := rc4.NewCipher(hash([]byte(func() string {
		if initer {
			return "keyA"
		} else {
			return "keyB"
		}
	}()), s, skey))
	if err != nil {
		panic(err)
	}
	var burnSrc, burnDst [1024]byte
	c.XORKeyStream(burnDst[:], burnSrc[:])
	return
}

type cipherReader struct {
	c  *rc4.Cipher
	r  io.Reader
	mu sync.Mutex
	be []byte
}

func (cr *cipherReader) Read(b []byte) (n int, err error) {
	var be []byte
	cr.mu.Lock()
	if len(cr.be) >= len(b) {
		be = cr.be
		cr.be = nil
		cr.mu.Unlock()
	} else {
		cr.mu.Unlock()
		be = make([]byte, len(b))
	}
	n, err = cr.r.Read(be[:len(b)])
	cr.c.XORKeyStream(b[:n], be[:n])
	cr.mu.Lock()
	if len(be) > len(cr.be) {
		cr.be = be
	}
	cr.mu.Unlock()
	return
}

func newCipherReader(c *rc4.Cipher, r io.Reader) io.Reader {
	return &cipherReader{c: c, r: r}
}

type cipherWriter struct {
	c *rc4.Cipher
	w io.Writer
	b []byte
}

func (cr *cipherWriter) Write(b []byte) (n int, err error) {
	be := func() []byte {
		if len(cr.b) < len(b) {
			return make([]byte, len(b))
		} else {
			ret := cr.b
			cr.b = nil
			return ret
		}
	}()
	cr.c.XORKeyStream(be[:], b)
	n, err = cr.w.Write(be[:len(b)])
	if n != len(b) {
		// The cipher will have advanced beyond the callers stream position.
		// We can't use the cipher anymore.
		cr.c = nil
	}
	if len(be) > len(cr.b) {
		cr.b = be
	}
	return
}

func newX() big.Int {
	var X big.Int
	X.SetBytes(func() []byte {
		var b [20]byte
		_, err := rand.Read(b[:])
		if err != nil {
			panic(err)
		}
		return b[:]
	}())
	return X
}

func paddedLeft(b []byte, _len int) []byte {
	if len(b) == _len {
		return b
	}
	ret := make([]byte, _len)
	if n := copy(ret[_len-len(b):], b); n != len(b) {
		panic(n)
	}
	return ret
}

// Calculate, and send Y, our public key.
func (h *handshake) postY(x *big.Int) error {
	var y big.Int
	y.Exp(&g, x, &p)
	return h.postWrite(paddedLeft(y.Bytes(), 96))
}

func (h *handshake) establishS() error {
	x := newX()
	h.postY(&x)
	var b [96]byte
	_, err := io.ReadFull(h.conn, b[:])
	if err != nil {
		return fmt.Errorf("error reading Y: %s", err)
	}
	var Y, S big.Int
	Y.SetBytes(b[:])
	S.Exp(&Y, &x, &p)
	sBytes := S.Bytes()
	copy(h.s[96-len(sBytes):96], sBytes)
	return nil
}

func newPadLen() int64 {
	i, err := rand.Int(rand.Reader, &newPadLenMax)
	if err != nil {
		panic(err)
	}
	ret := i.Int64()
	if ret < 0 || ret > maxPadLen {
		panic(ret)
	}
	return ret
}

// Manages state for both initiating and receiving handshakes.
type handshake struct {
	conn   io.ReadWriter
	s      [96]byte
	initer bool          // Whether we're initiating or receiving.
	skeys  SecretKeyIter // Skeys we'll accept if receiving.
	skey   []byte        // Skey we're initiating with.
	ia     []byte        // Initial payload. Only used by the initiator.
	// Return the bit for the crypto method the receiver wants to use.
	chooseMethod CryptoSelector
	// Sent to the receiver.
	cryptoProvides CryptoMethod

	writeMu    sync.Mutex
	writes     [][]byte
	writeErr   error
	writeCond  sync.Cond
	writeClose bool

	writerMu   sync.Mutex
	writerCond sync.Cond
	writerDone bool
}

func (h *handshake) finishWriting() {
	h.writeMu.Lock()
	h.writeClose = true
	h.writeCond.Broadcast()
	h.writeMu.Unlock()

	h.writerMu.Lock()
	for !h.writerDone {
		h.writerCond.Wait()
	}
	h.writerMu.Unlock()
}

func (h *handshake) writer() {
	defer func() {
		h.writerMu.Lock()
		h.writerDone = true
		h.writerCond.Broadcast()
		h.writerMu.Unlock()
	}()
	for {
		h.writeMu.Lock()
		for {
			if len(h.writes) != 0 {
				break
			}
			if h.writeClose {
				h.writeMu.Unlock()
				return
			}
			h.writeCond.Wait()
		}
		b := h.writes[0]
		h.writes = h.writes[1:]
		h.writeMu.Unlock()
		_, err := h.conn.Write(b)
		if err != nil {
			h.writeMu.Lock()
			h.writeErr = err
			h.writeMu.Unlock()
			return
		}
	}
}

func (h *handshake) postWrite(b []byte) error {
	h.writeMu.Lock()
	defer h.writeMu.Unlock()
	if h.writeErr != nil {
		return h.writeErr
	}
	h.writes = append(h.writes, b)
	h.writeCond.Signal()
	return nil
}

func xor(dst, src []byte) (ret []byte) {
	max := len(dst)
	if max > len(src) {
		max = len(src)
	}
	ret = make([]byte, 0, max)
	for i := range iter.N(max) {
		ret = append(ret, dst[i]^src[i])
	}
	return
}

func marshal(w io.Writer, data ...interface{}) (err error) {
	for _, data := range data {
		err = binary.Write(w, binary.BigEndian, data)
		if err != nil {
			break
		}
	}
	return
}

func unmarshal(r io.Reader, data ...interface{}) (err error) {
	for _, data := range data {
		err = binary.Read(r, binary.BigEndian, data)
		if err != nil {
			break
		}
	}
	return
}

// Looking for b at the end of a.
func suffixMatchLen(a, b []byte) int {
	if len(b) > len(a) {
		b = b[:len(a)]
	}
	// i is how much of b to try to match
	for i := len(b); i > 0; i-- {
		// j is how many chars we've compared
		j := 0
		for ; j < i; j++ {
			if b[i-1-j] != a[len(a)-1-j] {
				goto shorter
			}
		}
		return j
	shorter:
	}
	return 0
}

// Reads from r until b has been seen. Keeps the minimum amount of data in
// memory.
func readUntil(r io.Reader, b []byte) error {
	b1 := make([]byte, len(b))
	i := 0
	for {
		_, err := io.ReadFull(r, b1[i:])
		if err != nil {
			return err
		}
		i = suffixMatchLen(b1, b)
		if i == len(b) {
			break
		}
		if copy(b1, b1[len(b1)-i:]) != i {
			panic("wat")
		}
	}
	return nil
}

type readWriter struct {
	io.Reader
	io.Writer
}

func (h *handshake) newEncrypt(initer bool) *rc4.Cipher {
	return newEncrypt(initer, h.s[:], h.skey)
}

func (h *handshake) initerSteps() (ret io.ReadWriter, selected CryptoMethod, err error) {
	h.postWrite(hash(req1, h.s[:]))
	h.postWrite(xor(hash(req2, h.skey), hash(req3, h.s[:])))
	buf := &bytes.Buffer{}
	padLen := uint16(newPadLen())
	if len(h.ia) > math.MaxUint16 {
		err = errors.New("initial payload too large")
		return
	}
	err = marshal(buf, vc[:], h.cryptoProvides, padLen, zeroPad[:padLen], uint16(len(h.ia)), h.ia)
	if err != nil {
		return
	}
	e := h.newEncrypt(true)
	be := make([]byte, buf.Len())
	e.XORKeyStream(be, buf.Bytes())
	h.postWrite(be)
	bC := h.newEncrypt(false)
	var eVC [8]byte
	bC.XORKeyStream(eVC[:], vc[:])
	// Read until the all zero VC. At this point we've only read the 96 byte
	// public key, Y. There is potentially 512 byte padding, between us and
	// the 8 byte verification constant.
	err = readUntil(io.LimitReader(h.conn, 520), eVC[:])
	if err != nil {
		if err == io.EOF {
			err = errors.New("failed to synchronize on VC")
		} else {
			err = fmt.Errorf("error reading until VC: %s", err)
		}
		return
	}
	r := newCipherReader(bC, h.conn)
	var method CryptoMethod
	err = unmarshal(r, &method, &padLen)
	if err != nil {
		return
	}
	_, err = io.CopyN(ioutil.Discard, r, int64(padLen))
	if err != nil {
		return
	}
	selected = method & h.cryptoProvides
	switch selected {
	case CryptoMethodRC4:
		ret = readWriter{r, &cipherWriter{e, h.conn, nil}}
	case CryptoMethodPlaintext:
		ret = h.conn
	default:
		err = fmt.Errorf("receiver chose unsupported method: %x", method)
	}
	return
}

var ErrNoSecretKeyMatch = errors.New("no skey matched")

func (h *handshake) receiverSteps() (ret io.ReadWriter, chosen CryptoMethod, err error) {
	// There is up to 512 bytes of padding, then the 20 byte hash.
	err = readUntil(io.LimitReader(h.conn, 532), hash(req1, h.s[:]))
	if err != nil {
		if err == io.EOF {
			err = errors.New("failed to synchronize on S hash")
		}
		return
	}
	var b [20]byte
	_, err = io.ReadFull(h.conn, b[:])
	if err != nil {
		return
	}
	err = ErrNoSecretKeyMatch
	h.skeys(func(skey []byte) bool {
		if bytes.Equal(xor(hash(req2, skey), hash(req3, h.s[:])), b[:]) {
			h.skey = skey
			err = nil
			return false
		}
		return true
	})
	if err != nil {
		return
	}
	r := newCipherReader(newEncrypt(true, h.s[:], h.skey), h.conn)
	var (
		vc       [8]byte
		provides CryptoMethod
		padLen   uint16
	)

	err = unmarshal(r, vc[:], &provides, &padLen)
	if err != nil {
		return
	}
	cryptoProvidesCount.Add(strconv.FormatUint(uint64(provides), 16), 1)
	chosen = h.chooseMethod(provides)
	_, err = io.CopyN(ioutil.Discard, r, int64(padLen))
	if err != nil {
		return
	}
	var lenIA uint16
	unmarshal(r, &lenIA)
	if lenIA != 0 {
		h.ia = make([]byte, lenIA)
		unmarshal(r, h.ia)
	}
	buf := &bytes.Buffer{}
	w := cipherWriter{h.newEncrypt(false), buf, nil}
	padLen = uint16(newPadLen())
	err = marshal(&w, &vc, uint32(chosen), padLen, zeroPad[:padLen])
	if err != nil {
		return
	}
	err = h.postWrite(buf.Bytes())
	if err != nil {
		return
	}
	switch chosen {
	case CryptoMethodRC4:
		ret = readWriter{
			io.MultiReader(bytes.NewReader(h.ia), r),
			&cipherWriter{w.c, h.conn, nil},
		}
	case CryptoMethodPlaintext:
		ret = readWriter{
			io.MultiReader(bytes.NewReader(h.ia), h.conn),
			h.conn,
		}
	default:
		err = errors.New("chosen crypto method is not supported")
	}
	return
}

func (h *handshake) Do() (ret io.ReadWriter, method CryptoMethod, err error) {
	h.writeCond.L = &h.writeMu
	h.writerCond.L = &h.writerMu
	go h.writer()
	defer func() {
		h.finishWriting()
		if err == nil {
			err = h.writeErr
		}
	}()
	err = h.establishS()
	if err != nil {
		err = fmt.Errorf("error while establishing secret: %s", err)
		return
	}
	pad := make([]byte, newPadLen())
	io.ReadFull(rand.Reader, pad)
	err = h.postWrite(pad)
	if err != nil {
		return
	}
	if h.initer {
		ret, method, err = h.initerSteps()
	} else {
		ret, method, err = h.receiverSteps()
	}
	return
}

func InitiateHandshake(rw io.ReadWriter, skey []byte, initialPayload []byte, cryptoProvides CryptoMethod) (ret io.ReadWriter, method CryptoMethod, err error) {
	h := handshake{
		conn:           rw,
		initer:         true,
		skey:           skey,
		ia:             initialPayload,
		cryptoProvides: cryptoProvides,
	}
	defer perf.ScopeTimerErr(&err)()
	return h.Do()
}

func ReceiveHandshake(rw io.ReadWriter, skeys SecretKeyIter, selectCrypto CryptoSelector) (ret io.ReadWriter, method CryptoMethod, err error) {
	h := handshake{
		conn:         rw,
		initer:       false,
		skeys:        skeys,
		chooseMethod: selectCrypto,
	}
	return h.Do()
}

// A function that given a function, calls it with secret keys until it
// returns false or exhausted.
type SecretKeyIter func(callback func(skey []byte) (more bool))

func DefaultCryptoSelector(provided CryptoMethod) CryptoMethod {
	if provided&CryptoMethodPlaintext != 0 {
		return CryptoMethodPlaintext
	}
	return CryptoMethodRC4
}

type CryptoSelector func(CryptoMethod) CryptoMethod
