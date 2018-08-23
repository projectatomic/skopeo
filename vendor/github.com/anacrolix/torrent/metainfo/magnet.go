package metainfo

import (
	"encoding/base32"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

// Magnet link components.
type Magnet struct {
	InfoHash    Hash
	Trackers    []string
	DisplayName string
}

const xtPrefix = "urn:btih:"

func (m Magnet) String() string {
	// net.URL likes to assume //, and encodes ':' on us, so we do most of
	// this manually.
	ret := "magnet:?xt="
	ret += xtPrefix + hex.EncodeToString(m.InfoHash[:])
	if m.DisplayName != "" {
		ret += "&dn=" + url.QueryEscape(m.DisplayName)
	}
	for _, tr := range m.Trackers {
		ret += "&tr=" + url.QueryEscape(tr)
	}
	return ret
}

// ParseMagnetURI parses Magnet-formatted URIs into a Magnet instance
func ParseMagnetURI(uri string) (m Magnet, err error) {
	u, err := url.Parse(uri)
	if err != nil {
		err = fmt.Errorf("error parsing uri: %s", err)
		return
	}
	if u.Scheme != "magnet" {
		err = fmt.Errorf("unexpected scheme: %q", u.Scheme)
		return
	}
	xt := u.Query().Get("xt")
	if !strings.HasPrefix(xt, xtPrefix) {
		err = fmt.Errorf("bad xt parameter")
		return
	}
	infoHash := xt[len(xtPrefix):]

	// BTIH hash can be in HEX or BASE32 encoding
	// will assign appropriate func judging from symbol length
	var decode func(dst, src []byte) (int, error)
	switch len(infoHash) {
	case 40:
		decode = hex.Decode
	case 32:
		decode = base32.StdEncoding.Decode
	}

	if decode == nil {
		err = fmt.Errorf("unhandled xt parameter encoding: encoded length %d", len(infoHash))
		return
	}
	n, err := decode(m.InfoHash[:], []byte(infoHash))
	if err != nil {
		err = fmt.Errorf("error decoding xt: %s", err)
		return
	}
	if n != 20 {
		panic(n)
	}
	m.DisplayName = u.Query().Get("dn")
	m.Trackers = u.Query()["tr"]
	return
}
