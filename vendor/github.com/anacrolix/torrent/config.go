package torrent

import (
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/anacrolix/dht"
	"github.com/anacrolix/missinggo"
	"github.com/anacrolix/missinggo/conntrack"
	"github.com/anacrolix/missinggo/expect"
	"github.com/anacrolix/torrent/iplist"
	"github.com/anacrolix/torrent/storage"
	"golang.org/x/time/rate"
)

var DefaultHTTPUserAgent = "Go-Torrent/1.0"

// Probably not safe to modify this after it's given to a Client.
type ClientConfig struct {
	// Store torrent file data in this directory unless .DefaultStorage is
	// specified.
	DataDir string `long:"data-dir" description:"directory to store downloaded torrent data"`
	// The address to listen for new uTP and TCP bittorrent protocol
	// connections. DHT shares a UDP socket with uTP unless configured
	// otherwise.
	ListenHost              func(network string) string
	ListenPort              int
	NoDefaultPortForwarding bool
	// Don't announce to trackers. This only leaves DHT to discover peers.
	DisableTrackers bool `long:"disable-trackers"`
	DisablePEX      bool `long:"disable-pex"`

	// Don't create a DHT.
	NoDHT            bool `long:"disable-dht"`
	DhtStartingNodes dht.StartingNodesGetter
	// Never send chunks to peers.
	NoUpload bool `long:"no-upload"`
	// Disable uploading even when it isn't fair.
	DisableAggressiveUpload bool `long:"disable-aggressive-upload"`
	// Upload even after there's nothing in it for us. By default uploading is
	// not altruistic, we'll only upload to encourage the peer to reciprocate.
	Seed bool `long:"seed"`
	// Only applies to chunks uploaded to peers, to maintain responsiveness
	// communicating local Client state to peers. Each limiter token
	// represents one byte. The Limiter's burst must be large enough to fit a
	// whole chunk, which is usually 16 KiB (see TorrentSpec.ChunkSize).
	UploadRateLimiter *rate.Limiter
	// Rate limits all reads from connections to peers. Each limiter token
	// represents one byte. The Limiter's burst must be bigger than the
	// largest Read performed on a the underlying rate-limiting io.Reader
	// minus one. This is likely to be the larger of the main read loop buffer
	// (~4096), and the requested chunk size (~16KiB, see
	// TorrentSpec.ChunkSize).
	DownloadRateLimiter *rate.Limiter

	// User-provided Client peer ID. If not present, one is generated automatically.
	PeerID string
	// For the bittorrent protocol.
	DisableUTP bool
	// For the bittorrent protocol.
	DisableTCP bool `long:"disable-tcp"`
	// Called to instantiate storage for each added torrent. Builtin backends
	// are in the storage package. If not set, the "file" implementation is
	// used.
	DefaultStorage storage.ClientImpl

	EncryptionPolicy

	// Sets usage of Socks5 Proxy. Authentication should be included in the url if needed.
	// Examples: socks5://demo:demo@192.168.99.100:1080
	// 			 http://proxy.domain.com:3128
	ProxyURL string

	IPBlocklist      iplist.Ranger
	DisableIPv6      bool `long:"disable-ipv6"`
	DisableIPv4      bool
	DisableIPv4Peers bool
	// Perform logging and any other behaviour that will help debug.
	Debug bool `help:"enable debugging"`

	// HTTPProxy defines proxy for HTTP requests.
	// Format: func(*Request) (*url.URL, error),
	// or result of http.ProxyURL(HTTPProxy).
	// By default, it is composed from ClientConfig.ProxyURL,
	// if not set explicitly in ClientConfig struct
	HTTPProxy func(*http.Request) (*url.URL, error)
	// HTTPUserAgent changes default UserAgent for HTTP requests
	HTTPUserAgent string
	// Updated occasionally to when there's been some changes to client
	// behaviour in case other clients are assuming anything of us. See also
	// `bep20`.
	ExtendedHandshakeClientVersion string
	// Peer ID client identifier prefix. We'll update this occasionally to
	// reflect changes to client behaviour that other clients may depend on.
	// Also see `extendedHandshakeClientVersion`.
	Bep20 string

	// Peer dial timeout to use when there are limited peers.
	NominalDialTimeout time.Duration
	// Minimum peer dial timeout to use (even if we have lots of peers).
	MinDialTimeout             time.Duration
	EstablishedConnsPerTorrent int
	HalfOpenConnsPerTorrent    int
	// Maximum number of peer addresses in reserve.
	TorrentPeersHighWater int
	// Minumum number of peers before effort is made to obtain more peers.
	TorrentPeersLowWater int

	// Limit how long handshake can take. This is to reduce the lingering
	// impact of a few bad apples. 4s loses 1% of successful handshakes that
	// are obtained with 60s timeout, and 5% of unsuccessful handshakes.
	HandshakesTimeout time.Duration

	// The IP addresses as our peers should see them. May differ from the
	// local interfaces due to NAT or other network configurations.
	PublicIp4 net.IP
	PublicIp6 net.IP

	DisableAcceptRateLimiting bool
	// Don't add connections that have the same peer ID as an existing
	// connection for a given Torrent.
	dropDuplicatePeerIds bool

	ConnTracker *conntrack.Instance
}

func (cfg *ClientConfig) SetListenAddr(addr string) *ClientConfig {
	host, port, err := missinggo.ParseHostPort(addr)
	expect.Nil(err)
	cfg.ListenHost = func(string) string { return host }
	cfg.ListenPort = port
	return cfg
}

func NewDefaultClientConfig() *ClientConfig {
	return &ClientConfig{
		HTTPUserAgent:                  DefaultHTTPUserAgent,
		ExtendedHandshakeClientVersion: "go.torrent dev 20181121",
		Bep20:                      "-GT0002-",
		NominalDialTimeout:         20 * time.Second,
		MinDialTimeout:             3 * time.Second,
		EstablishedConnsPerTorrent: 50,
		HalfOpenConnsPerTorrent:    25,
		TorrentPeersHighWater:      500,
		TorrentPeersLowWater:       50,
		HandshakesTimeout:          4 * time.Second,
		DhtStartingNodes:           dht.GlobalBootstrapAddrs,
		ListenHost:                 func(string) string { return "" },
		UploadRateLimiter:          unlimited,
		DownloadRateLimiter:        unlimited,
		ConnTracker:                conntrack.NewInstance(),
	}
}

type EncryptionPolicy struct {
	DisableEncryption  bool
	ForceEncryption    bool // Don't allow unobfuscated connections.
	PreferNoEncryption bool
}
