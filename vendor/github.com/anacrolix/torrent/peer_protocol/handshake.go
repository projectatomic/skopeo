package peer_protocol

import (
	"encoding/hex"
	"fmt"
	"io"

	"github.com/anacrolix/missinggo"
	"github.com/anacrolix/torrent/metainfo"
)

type ExtensionBit uint

const (
	ExtensionBitDHT      = 0  // http://www.bittorrent.org/beps/bep_0005.html
	ExtensionBitExtended = 20 // http://www.bittorrent.org/beps/bep_0010.html
	ExtensionBitFast     = 2  // http://www.bittorrent.org/beps/bep_0006.html
)

func handshakeWriter(w io.Writer, bb <-chan []byte, done chan<- error) {
	var err error
	for b := range bb {
		_, err = w.Write(b)
		if err != nil {
			break
		}
	}
	done <- err
}

type (
	PeerExtensionBits [8]byte
)

func (me PeerExtensionBits) String() string {
	return hex.EncodeToString(me[:])
}

func NewPeerExtensionBytes(bits ...ExtensionBit) (ret PeerExtensionBits) {
	for _, b := range bits {
		ret.SetBit(b)
	}
	return
}

func (pex PeerExtensionBits) SupportsExtended() bool {
	return pex.GetBit(ExtensionBitExtended)
}

func (pex PeerExtensionBits) SupportsDHT() bool {
	return pex.GetBit(ExtensionBitDHT)
}

func (pex PeerExtensionBits) SupportsFast() bool {
	return pex.GetBit(ExtensionBitFast)
}

func (pex *PeerExtensionBits) SetBit(bit ExtensionBit) {
	pex[7-bit/8] |= 1 << (bit % 8)
}

func (pex PeerExtensionBits) GetBit(bit ExtensionBit) bool {
	return pex[7-bit/8]&(1<<(bit%8)) != 0
}

type HandshakeResult struct {
	PeerExtensionBits
	PeerID [20]byte
	metainfo.Hash
}

// ih is nil if we expect the peer to declare the InfoHash, such as when the
// peer initiated the connection. Returns ok if the Handshake was successful,
// and err if there was an unexpected condition other than the peer simply
// abandoning the Handshake.
func Handshake(sock io.ReadWriter, ih *metainfo.Hash, peerID [20]byte, extensions PeerExtensionBits) (res HandshakeResult, ok bool, err error) {
	// Bytes to be sent to the peer. Should never block the sender.
	postCh := make(chan []byte, 4)
	// A single error value sent when the writer completes.
	writeDone := make(chan error, 1)
	// Performs writes to the socket and ensures posts don't block.
	go handshakeWriter(sock, postCh, writeDone)

	defer func() {
		close(postCh) // Done writing.
		if !ok {
			return
		}
		if err != nil {
			panic(err)
		}
		// Wait until writes complete before returning from handshake.
		err = <-writeDone
		if err != nil {
			err = fmt.Errorf("error writing: %s", err)
		}
	}()

	post := func(bb []byte) {
		select {
		case postCh <- bb:
		default:
			panic("mustn't block while posting")
		}
	}

	post([]byte(Protocol))
	post(extensions[:])
	if ih != nil { // We already know what we want.
		post(ih[:])
		post(peerID[:])
	}
	var b [68]byte
	_, err = io.ReadFull(sock, b[:68])
	if err != nil {
		err = nil
		return
	}
	if string(b[:20]) != Protocol {
		return
	}
	missinggo.CopyExact(&res.PeerExtensionBits, b[20:28])
	missinggo.CopyExact(&res.Hash, b[28:48])
	missinggo.CopyExact(&res.PeerID, b[48:68])
	// peerExtensions.Add(res.PeerExtensionBits.String(), 1)

	// TODO: Maybe we can just drop peers here if we're not interested. This
	// could prevent them trying to reconnect, falsely believing there was
	// just a problem.
	if ih == nil { // We were waiting for the peer to tell us what they wanted.
		post(res.Hash[:])
		post(peerID[:])
	}

	ok = true
	return
}
