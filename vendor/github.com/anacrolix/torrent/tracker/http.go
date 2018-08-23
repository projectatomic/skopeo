package tracker

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/anacrolix/dht/krpc"
	"github.com/anacrolix/missinggo/httptoo"
	"github.com/anacrolix/torrent/bencode"
)

type httpResponse struct {
	FailureReason string `bencode:"failure reason"`
	Interval      int32  `bencode:"interval"`
	TrackerId     string `bencode:"tracker id"`
	Complete      int32  `bencode:"complete"`
	Incomplete    int32  `bencode:"incomplete"`
	Peers         Peers  `bencode:"peers"`
	// BEP 7
	Peers6 krpc.CompactIPv6NodeAddrs `bencode:"peers6"`
}

type Peers []Peer

func (me *Peers) UnmarshalBencode(b []byte) (err error) {
	var _v interface{}
	err = bencode.Unmarshal(b, &_v)
	if err != nil {
		return
	}
	switch v := _v.(type) {
	case string:
		vars.Add("http responses with string peers", 1)
		var cnas krpc.CompactIPv4NodeAddrs
		err = cnas.UnmarshalBinary([]byte(v))
		if err != nil {
			return
		}
		for _, cp := range cnas {
			*me = append(*me, Peer{
				IP:   cp.IP[:],
				Port: int(cp.Port),
			})
		}
		return
	case []interface{}:
		vars.Add("http responses with list peers", 1)
		for _, i := range v {
			var p Peer
			p.fromDictInterface(i.(map[string]interface{}))
			*me = append(*me, p)
		}
		return
	default:
		vars.Add("http responses with unhandled peers type", 1)
		err = fmt.Errorf("unsupported type: %T", _v)
		return
	}
}

func setAnnounceParams(_url *url.URL, ar *AnnounceRequest, opts Announce) {
	q := _url.Query()

	q.Set("info_hash", string(ar.InfoHash[:]))
	q.Set("peer_id", string(ar.PeerId[:]))
	// AFAICT, port is mandatory, and there's no implied port key.
	q.Set("port", fmt.Sprintf("%d", ar.Port))
	q.Set("uploaded", strconv.FormatInt(ar.Uploaded, 10))
	q.Set("downloaded", strconv.FormatInt(ar.Downloaded, 10))
	q.Set("left", strconv.FormatUint(ar.Left, 10))
	if ar.Event != None {
		q.Set("event", ar.Event.String())
	}
	// http://stackoverflow.com/questions/17418004/why-does-tracker-server-not-understand-my-request-bittorrent-protocol
	q.Set("compact", "1")
	// According to https://wiki.vuze.com/w/Message_Stream_Encryption. TODO:
	// Take EncryptionPolicy or something like it as a parameter.
	q.Set("supportcrypto", "1")
	if opts.ClientIp4.IP != nil {
		q.Set("ipv4", opts.ClientIp4.String())
	}
	if opts.ClientIp6.IP != nil {
		q.Set("ipv6", opts.ClientIp6.String())
	}
	_url.RawQuery = q.Encode()
}

func announceHTTP(opt Announce, _url *url.URL) (ret AnnounceResponse, err error) {
	_url = httptoo.CopyURL(_url)
	setAnnounceParams(_url, &opt.Request, opt)
	req, err := http.NewRequest("GET", _url.String(), nil)
	req.Header.Set("User-Agent", opt.UserAgent)
	req.Host = opt.HostHeader
	if opt.Context != nil {
		req = req.WithContext(opt.Context)
	}
	resp, err := (&http.Client{
		Timeout: time.Second * 15,
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 15 * time.Second,
			}).Dial,
			Proxy:               opt.HTTPProxy,
			TLSHandshakeTimeout: 15 * time.Second,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         opt.ServerName,
			},
		},
	}).Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	io.Copy(&buf, resp.Body)
	if resp.StatusCode != 200 {
		err = fmt.Errorf("response from tracker: %s: %s", resp.Status, buf.String())
		return
	}
	var trackerResponse httpResponse
	err = bencode.Unmarshal(buf.Bytes(), &trackerResponse)
	if _, ok := err.(bencode.ErrUnusedTrailingBytes); ok {
		err = nil
	} else if err != nil {
		err = fmt.Errorf("error decoding %q: %s", buf.Bytes(), err)
		return
	}
	if trackerResponse.FailureReason != "" {
		err = fmt.Errorf("tracker gave failure reason: %q", trackerResponse.FailureReason)
		return
	}
	vars.Add("successful http announces", 1)
	ret.Interval = trackerResponse.Interval
	ret.Leechers = trackerResponse.Incomplete
	ret.Seeders = trackerResponse.Complete
	if len(trackerResponse.Peers) != 0 {
		vars.Add("http responses with nonempty peers key", 1)
	}
	ret.Peers = trackerResponse.Peers
	if len(trackerResponse.Peers6) != 0 {
		vars.Add("http responses with nonempty peers6 key", 1)
	}
	for _, na := range trackerResponse.Peers6 {
		ret.Peers = append(ret.Peers, Peer{
			IP:   na.IP,
			Port: na.Port,
		})
	}
	return
}
