// +build containers_image_include_torrent

package torrent

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	ts "github.com/anacrolix/torrent/storage"
	"github.com/containers/image/docker/reference"
	"github.com/containers/image/manifest"
	"github.com/containers/image/pkg/blobinfocache"
	"github.com/containers/image/transports"
	"github.com/containers/image/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Client allows to pull/seed layers using BitTorrent
type Client struct {
	c       *torrent.Client
	sys     *types.SystemContext
	dir     string
	seeding []types.ImageSource
	timeout time.Duration
}

type pieceCompletion struct {
}

func (p *pieceCompletion) Close() error {
	return nil
}

func (p *pieceCompletion) Get(metainfo.PieceKey) (ts.Completion, error) {
	return ts.Completion{
		Complete: true,
		Ok:       true,
	}, nil
}

func (p *pieceCompletion) Set(metainfo.PieceKey, bool) error {
	return nil
}

// MakeClient creates a client that can be used both to seed and pull layers using the BitTorrent protocol
// It is possible to specify a timeout value for the first byte to be received.  If no data is received in
// the specified interval, then the download is cancelled.  This doesn't prevent the download to stall
// later on if there are no other peers available.
func MakeClient(sys *types.SystemContext, debug bool, seed bool, timeout time.Duration) (*Client, error) {
	conf := torrent.NewDefaultClientConfig()
	if seed {
		conf.Seed = true
	}
	if debug {
		conf.Debug = true
	}
	dir, err := ioutil.TempDir("", "torrent")
	if err != nil {
		return nil, err
	}
	conf.NoDHT = true
	conf.DataDir = dir
	conf.NoDefaultPortForwarding = true
	conf.DisableAcceptRateLimiting = true

	c, err := torrent.NewClient(conf)
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}

	return &Client{
		c:       c,
		dir:     dir,
		sys:     sys,
		timeout: timeout,
	}, nil
}

func (t *Client) getTorrentUrl(ctx context.Context, info types.BlobInfo, registry string, ref reference.Named) url.URL {
	blobSum := info.Digest.String()
	torrentURL := url.URL{
		Scheme: "https",
		Host:   registry,
		Path:   fmt.Sprintf("/c1/torrent/%s/blobs/%s", reference.Path(ref), blobSum),
	}
	if t.sys.DockerInsecureSkipTLSVerify == types.OptionalBoolTrue {
		torrentURL.Scheme = "http"
	}
	if t.sys.DockerAuthConfig != nil {
		torrentURL.User = url.UserPassword(t.sys.DockerAuthConfig.Username, t.sys.DockerAuthConfig.Password)
	}
	return torrentURL
}

// GetBlobTorrent pulls a layer using BitTorrent from the specified registry.  Optionally it is possible to specify different trackers to use.
func (t *Client) GetBlobTorrent(ctx context.Context, info types.BlobInfo, registry string, ref reference.Named, trackers []string) (io.ReadCloser, int64, error) {
	mi, err := t.makeMetaInfo(ctx, registry, info, ref)
	if err != nil {
		return nil, -1, err
	}
	infoTorrent, err := mi.UnmarshalInfo()
	if err != nil {
		return nil, -1, err
	}
	if infoTorrent.Length < (1 << 20) {
		logrus.Debugf("skip blob %s with Torrent", info.Digest.String())
		return nil, -1, errors.New("blob too small")
	}

	if trackers != nil && len(trackers) > 0 {
		mi.Announce = ""
		mi.AnnounceList = nil
	}

	torrent, err := t.c.AddTorrent(mi)
	if err != nil {
		return nil, -1, err
	}

	<-torrent.GotInfo()

	if trackers != nil {
		torrent.AddTrackers([][]string{trackers})
	}

	torrent.DownloadAll()

	len := torrent.Info().TotalLength()

	spc := torrent.SubscribePieceStateChanges()
	defer spc.Close()
	if t.timeout > 0 {
	wait:
		for {
			select {
			case <-time.After(t.timeout):
				if torrent.BytesMissing() == len {
					return nil, -1, errors.New("timeout waiting for download to start")
				}
			case <-spc.Values:
				m := torrent.BytesMissing()
				// As soon as we fetched some data, pass the reader to the caller,
				// so that the torrent data can be handled immediately.  This can hang
				// if no other peers are available later.
				if m != len {
					break wait
				}
			}
		}
	}
	r, err := newTorrentReadCloser(t.c, torrent)
	if err != nil {
		return nil, -1, err
	}

	return r, len, nil
}

func (t *Client) makeMetaInfo(ctx context.Context, registry string, info types.BlobInfo, ref reference.Named) (*metainfo.MetaInfo, error) {
	url := t.getTorrentUrl(ctx, info, registry, ref)
	logrus.Debugf("trying torrent from %s", url.String())

	resp, err := http.Get(url.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.New("cannot download Torrent metainfo")
	}

	return metainfo.Load(resp.Body)
}

type torrentReadClose struct {
	client  *torrent.Client
	torrent *torrent.Torrent
	reader  io.Reader
}

func (t *torrentReadClose) Read(p []byte) (n int, err error) {
	return t.reader.Read(p)
}

func (t *torrentReadClose) Close() error {
	t.torrent.Drop()
	return nil
}

func newTorrentReadCloser(c *torrent.Client, t *torrent.Torrent) (*torrentReadClose, error) {
	r := t.NewReader()
	rc := &torrentReadClose{
		client:  c,
		torrent: t,
		reader:  r,
	}
	return rc, nil
}

// Close cleanups the resources used by the BitTorrent client.
func (t *Client) Close() {
	if t.seeding != nil {
		for _, s := range t.seeding {
			s.Close()
		}
	}
	t.c.Close()
	os.RemoveAll(t.dir)
}

// Seed a layer using BitTorrent from the specified storage.
func (t *Client) Seed(ctx context.Context, srcCtx *types.SystemContext, ref types.ImageReference, refSrc types.ImageReference) (retErr error) {
	rawSource, err := refSrc.NewImageSource(ctx, srcCtx)
	if err != nil {
		return errors.Wrapf(err, "Error initializing source %s", transports.ImageName(refSrc))
	}
	t.seeding = append(t.seeding, rawSource)

	manifestBlob, manifestType, err := rawSource.GetManifest(ctx, nil)
	if err != nil {
		return err
	}
	manifest, err := manifest.FromBlob(manifestBlob, manifestType)
	if err != nil {
		return err
	}

	layerBlobs := manifest.LayerInfos()

	for _, layerBlob := range layerBlobs {
		if layerBlob.EmptyLayer {
			continue
		}
		dockerRef := ref.DockerReference()
		if dockerRef == nil {
			return errors.New("invalid src reference")

		}

		blobInfo := layerBlob.BlobInfo

		registry := reference.Domain(dockerRef)
		mi, err := t.makeMetaInfo(ctx, registry, blobInfo, ref.DockerReference())
		if err != nil {
			return err
		}

		info, err := mi.UnmarshalInfo()
		if err != nil {
			return err
		}

		pathStorage := t.dir

		readcloser, _, err := rawSource.GetBlob(ctx, blobInfo, blobinfocache.NoCache)
		if err != nil {
			logrus.Warningf("cannot find %s", blobInfo.Digest.String())
			continue
		}
		defer readcloser.Close()

		p := filepath.Join(pathStorage, info.Name)
		outFile, err := os.Create(p)
		if err != nil {
			return err
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, readcloser); err != nil {
			return err
		}

		completion := &pieceCompletion{}
		storage := ts.NewFileWithCompletion(t.dir, completion)
		torrent, _ := t.c.AddTorrentInfoHashWithStorage(mi.HashInfoBytes(), storage)
		if srcCtx.DockerTorrentTrackers != nil && len(srcCtx.DockerTorrentTrackers) > 0 {
			mi.Announce = ""
			mi.AnnounceList = nil
		}
		t.c.AddTorrent(mi)
		if srcCtx.DockerTorrentTrackers != nil {
			torrent.AddTrackers([][]string{srcCtx.DockerTorrentTrackers})
		}
		logrus.Infof("Seeding %s", blobInfo.Digest.String())
	}
	return nil
}

// WriteStatus writes debug information on the status of the BitTorrent client.
func (t *Client) WriteStatus(w io.Writer) {
	t.c.WriteStatus(w)
}
