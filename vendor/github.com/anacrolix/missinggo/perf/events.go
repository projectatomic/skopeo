package perf

import (
	"fmt"
	"io"
	"net/http"
	"sort"
	"sync"
	"text/tabwriter"
)

var (
	mu     sync.RWMutex
	events = map[string]*Event{}
)

func init() {
	http.HandleFunc("/debug/perf", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=UTF-8")
		WriteEventsTable(w)
	})
}

func WriteEventsTable(w io.Writer) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprint(tw, "description\ttotal\tcount\tmin\tmean\tmax\n")
	type t struct {
		d string
		e Event
	}
	mu.RLock()
	es := make([]t, 0, len(events))
	for d, e := range events {
		e.Mu.RLock()
		es = append(es, t{d, *e})
		e.Mu.RUnlock()
	}
	mu.RUnlock()
	sort.Slice(es, func(i, j int) bool {
		return es[i].e.Total > es[j].e.Total
	})
	for _, el := range es {
		e := el.e
		fmt.Fprintf(tw, "%s\t%v\t%v\t%v\t%v\t%v\n", el.d, e.Total, e.Count, e.Min, e.MeanTime(), e.Max)
	}
	tw.Flush()
}
