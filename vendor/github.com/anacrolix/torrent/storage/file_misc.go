package storage

import "github.com/anacrolix/torrent/metainfo"

func extentCompleteRequiredLengths(info *metainfo.Info, off, n int64) (ret []metainfo.FileInfo) {
	if n == 0 {
		return
	}
	for _, fi := range info.UpvertedFiles() {
		if off >= fi.Length {
			off -= fi.Length
			continue
		}
		n1 := n
		if off+n1 > fi.Length {
			n1 = fi.Length - off
		}
		ret = append(ret, metainfo.FileInfo{
			Path:   fi.Path,
			Length: off + n1,
		})
		n -= n1
		if n == 0 {
			return
		}
		off = 0
	}
	panic("extent exceeds torrent bounds")
}
