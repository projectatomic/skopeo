// +build !containers_image_include_torrent

package torrent

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/containers/image/docker/reference"
	"github.com/containers/image/types"
)

// Client allows to pull/seed layers using BitTorrent
type Client struct {
}

// MakeClient creates a client that can be used both to seed and pull layers using the BitTorrent protocol
func MakeClient(sys *types.SystemContext, debug bool, seed bool, timeout time.Duration) (*Client, error) {
	return nil, errors.New("BitTorrent not supported")
}

// GetBlobTorrent pulls a layer using BitTorrent from the specified registry.  Optionally it is possible to specify different trackers to use.
func (t *Client) GetBlobTorrent(ctx context.Context, info types.BlobInfo, registry string, ref reference.Named, trackers []string) (io.ReadCloser, int64, error) {
	return nil, -1, errors.New("BitTorrent not supported")
}

// Close cleanups the resources used by the BitTorrent client.
func (t *Client) Close() {
}

// Seed a layer using BitTorrent from the specified storage.
func (t *Client) Seed(ctx context.Context, srcCtx *types.SystemContext, ref types.ImageReference, refSrc types.ImageReference) error {
	return errors.New("BitTorrent not supported")
}

// WriteStatus writes debug information on the status of the BitTorrent client.
func (t *Client) WriteStatus(w io.Writer) {
}
