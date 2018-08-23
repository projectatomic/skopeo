package sync

import (
	"sort"
	"sync"

	"github.com/anacrolix/missinggo/perf"
)

var (
	// Stats on lock usage by call graph.
	lockStatsMu      sync.Mutex
	lockStatsByStack map[lockStackKey]lockStats
)

type (
	lockStats    = perf.Event
	lockStackKey = [32]uintptr
	lockCount    = int64
)

type stackLockStats struct {
	stack lockStackKey
	lockStats
}

func sortedLockTimes() (ret []stackLockStats) {
	lockStatsMu.Lock()
	for stack, stats := range lockStatsByStack {
		ret = append(ret, stackLockStats{stack, stats})
	}
	lockStatsMu.Unlock()
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Total > ret[j].Total
	})
	return
}
