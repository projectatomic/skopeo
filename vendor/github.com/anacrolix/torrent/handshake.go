package torrent

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/anacrolix/torrent/mse"
	pp "github.com/anacrolix/torrent/peer_protocol"
)

// Wraps a raw connection and provides the interface we want for using the
// connection in the message loop.
type deadlineReader struct {
	nc net.Conn
	r  io.Reader
}

func (r deadlineReader) Read(b []byte) (int, error) {
	// Keep-alives should be received every 2 mins. Give a bit of gracetime.
	err := r.nc.SetReadDeadline(time.Now().Add(150 * time.Second))
	if err != nil {
		return 0, fmt.Errorf("error setting read deadline: %s", err)
	}
	return r.r.Read(b)
}

func handleEncryption(
	rw io.ReadWriter,
	skeys mse.SecretKeyIter,
	policy EncryptionPolicy,
) (
	ret io.ReadWriter,
	headerEncrypted bool,
	cryptoMethod mse.CryptoMethod,
	err error,
) {
	if !policy.ForceEncryption {
		var protocol [len(pp.Protocol)]byte
		_, err = io.ReadFull(rw, protocol[:])
		if err != nil {
			return
		}
		rw = struct {
			io.Reader
			io.Writer
		}{
			io.MultiReader(bytes.NewReader(protocol[:]), rw),
			rw,
		}
		if string(protocol[:]) == pp.Protocol {
			ret = rw
			return
		}
	}
	headerEncrypted = true
	ret, cryptoMethod, err = mse.ReceiveHandshake(rw, skeys, func(provides mse.CryptoMethod) mse.CryptoMethod {
		switch {
		case policy.ForceEncryption:
			return mse.CryptoMethodRC4
		case policy.DisableEncryption:
			return mse.CryptoMethodPlaintext
		case policy.PreferNoEncryption && provides&mse.CryptoMethodPlaintext != 0:
			return mse.CryptoMethodPlaintext
		default:
			return mse.DefaultCryptoSelector(provides)
		}
	})
	return
}

type PeerExtensionBits = pp.PeerExtensionBits
