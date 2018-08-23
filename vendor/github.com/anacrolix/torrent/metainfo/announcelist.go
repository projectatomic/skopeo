package metainfo

type AnnounceList [][]string

// Whether the AnnounceList should be preferred over a single URL announce.
func (al AnnounceList) OverridesAnnounce(announce string) bool {
	for _, tier := range al {
		for _, url := range tier {
			if url != "" || announce == "" {
				return true
			}
		}
	}
	return false
}

func (al AnnounceList) DistinctValues() (ret map[string]struct{}) {
	for _, tier := range al {
		for _, v := range tier {
			if ret == nil {
				ret = make(map[string]struct{})
			}
			ret[v] = struct{}{}
		}
	}
	return
}
