package zx

import (
	"bytes"
	"fmt"
	"sync"
)

// Statistics
type Call int

const (
	Sstat   Call = iota // calls to stat
	Sget                // calls to get
	Sput                // calls to put
	Smove               // calls to move
	Slink               // calls to link
	Sremove             // calls to remove/all
	Swstat              // calls to wstat
	Sfind               // calls to find
	Scall               // totals for all calls.
	Nstats              // number of stats.
)

// Stats for FS implementors
struct Stats {
	sync.Mutex
	Nb [Nstats]int64
}

var name = [...]string{
	"stats", "gets", "puts", "moves", "links", "removes", "wstats", "finds", "total",
}

func (s *Stats) Count(what Call) {
	s.Lock()
	s.Nb[what]++
	s.Unlock()
}

func (s *Stats) String() string {
	var buf bytes.Buffer

	s.Lock()
	defer s.Unlock()
	for i := Call(0); i < Nstats; i++ {
		fmt.Fprintf(&buf, "%6d %s\n", s.Nb[i], name[i])
	}
	return buf.String()
}

func (s *Stats) Clear() {
	s.Lock()
	defer s.Unlock()
	for i := Call(0); i < Nstats; i++ {
		s.Nb[i] = 0
	}
}
