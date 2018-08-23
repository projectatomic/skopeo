package torrent

import (
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
)

// Specifies a new torrent for adding to a client. There are helpers for
// magnet URIs and torrent metainfo files.
type TorrentSpec struct {
	// The tiered tracker URIs.
	Trackers  [][]string
	InfoHash  metainfo.Hash
	InfoBytes []byte
	// The name to use if the Name field from the Info isn't available.
	DisplayName string
	// The chunk size to use for outbound requests. Defaults to 16KiB if not
	// set.
	ChunkSize int
	Storage   storage.ClientImpl
}

func TorrentSpecFromMagnetURI(uri string) (spec *TorrentSpec, err error) {
	m, err := metainfo.ParseMagnetURI(uri)
	if err != nil {
		return
	}
	spec = &TorrentSpec{
		Trackers:    [][]string{m.Trackers},
		DisplayName: m.DisplayName,
		InfoHash:    m.InfoHash,
	}
	return
}

func TorrentSpecFromMetaInfo(mi *metainfo.MetaInfo) (spec *TorrentSpec) {
	info, _ := mi.UnmarshalInfo()
	spec = &TorrentSpec{
		Trackers:    mi.AnnounceList,
		InfoBytes:   mi.InfoBytes,
		DisplayName: info.Name,
		InfoHash:    mi.HashInfoBytes(),
	}
	if spec.Trackers == nil && mi.Announce != "" {
		spec.Trackers = [][]string{{mi.Announce}}
	}
	return
}
