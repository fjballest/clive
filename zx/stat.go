/*
	IO statistics for ZX servers and clients
*/
package zx

import (
	"bytes"
	"fmt"
	"sync"
	"time"
)

type Times struct {
	Min, Max, Tot, Avg time.Duration
}

type Sizes struct {
	Min, Max, Tot, N, Avg int64
}

// Statistics
const (
	Sstat      = iota // calls to stat
	Sget              // calls to get
	Sput              // calls to put
	Smkdir            // calls to mkdir
	Smove             // calls to move
	Sremove           // calls to remove
	Sremoveall        // calls to removeall
	Swstat            // calls to wstat
	Sfind             // calls to find
	Sfindget          // calls to findget
	Scall             // totals for all calls.
	Nstats            // number of stats.
)

var name = [...]string{
	"stat", "get", "put", "mkdir", "move",
	"remove", "removeall", "wstat", "find", "findget", "tot",
}

/*
	IOstat kept for a single zx call.
*/
type IOstat struct {
	Tstart        Times // Times between call and the first reply
	Tend          Times // Times between call and the last reply
	Tsizes        Sizes // message requests sizes
	Rsizes        Sizes // message replies sizes
	Ncalls, Nerrs int64
}

/*
	Set of IO statistics for a ZX client/server.
	It's ok to call its methods with nil, which is a nop.
*/
type IOstats struct {
	Tag string
	sync.Mutex
	For [Nstats]IOstat
}

// Count t as a new time in times.
func (t *Times) count(delta time.Duration) {
	t.Tot += delta
	if t.Min == 0 || t.Min > delta {
		t.Min = delta
	}
	if t.Max == 0 || t.Max < delta {
		t.Max = delta
	}
}

func (t *Times) averages(n int64) {
	t.Avg = time.Duration(int64(t.Tot) / n)
}

func (t Times) String() string {
	return fmt.Sprintf("min %10v\tavg %10v\tmax %10v", t.Min, t.Avg, t.Max)
}

// Count sz as a new size in sizes.
func (s *Sizes) count(n, sz int64) {
	s.N += n
	s.Tot += sz
	if s.Min == 0 || s.Min > sz && sz > 0 {
		s.Min = sz
	}
	if s.Max == 0 || s.Max < sz {
		s.Max = sz
	}
}

func (s *Sizes) averages() {
	if s.N > 0 {
		s.Avg = s.Tot / s.N
	}
}

func (s Sizes) String() string {
	return fmt.Sprintf("msgs %9v\tmin %10v\tavg %10v\tmax %10v", s.N, s.Min, s.Avg, s.Max)
}

// starting a reply to a call.
func (s *IOstat) start(call time.Time) {
	s.Ncalls++
	s.Tstart.count(time.Since(call))
}

// received further data for the call.
func (s *IOstat) recv(n, sz int64) {
	s.Tsizes.count(n, sz)
}

// sending further data for the call.
func (s *IOstat) send(n, sz int64) {
	s.Rsizes.count(n, sz)
}

func (s *IOstat) end(call time.Time, failed bool) {
	s.Tend.count(time.Since(call))
	if failed {
		s.Nerrs++
	}
}

func (s *IOstat) averages() {
	if s.Ncalls > 0 {
		s.Tstart.averages(s.Ncalls)
		s.Tend.averages(s.Ncalls)
	}
	s.Tsizes.averages()
	s.Rsizes.averages()
}

func (s *IOstat) String() string {
	rs := fmt.Sprintf("%d calls %d errs %d msgs %d bytes\n", s.Ncalls, s.Nerrs,
		s.Tsizes.N+s.Rsizes.N, s.Tsizes.Tot+s.Rsizes.Tot)
	rs += fmt.Sprintf("\treq: %s\n", s.Tsizes)
	rs += fmt.Sprintf("\trep: %s\n", s.Rsizes)
	rs += fmt.Sprintf("\tbgn: %s\n", s.Tstart)
	rs += fmt.Sprintf("\tend: %s\n", s.Tend)
	return rs
}

var zt time.Time

// Ongoing call stat (see IOstats).
// It's ok to call its method with a nil stat (nop).
type CallStat struct {
	io      *IOstats
	what    int
	replied bool
	t0      time.Time
}

// Start stats for a new call accounting the request sz
func (io *IOstats) NewCallSize(what, sz int) *CallStat {
	return io.newCall(what, sz)
}

// Start stats for a new call.
func (io *IOstats) NewCall(what int) *CallStat {
	return io.newCall(what, 0)
}

// Start stats for a new call.
func (io *IOstats) newCall(what, sz int) *CallStat {
	if io == nil {
		return nil
	}
	io.Lock()
	defer io.Unlock()
	io.For[what].recv(1, int64(sz)) // account for the call itself.
	return &CallStat{
		io:   io,
		what: what,
		t0:   time.Now(),
	}
}

// More data was received for the call.
func (cs *CallStat) Recv(sz int64) {
	if cs == nil {
		return
	}
	cs.io.Lock()
	defer cs.io.Unlock()
	cs.io.For[cs.what].recv(1, sz)
}

// More data was received for the call.
func (cs *CallStat) Recvs(n, sz int64) {
	if cs == nil {
		return
	}
	if n <= 0 {
		n = 1
	}
	cs.io.Lock()
	defer cs.io.Unlock()
	cs.io.For[cs.what].recv(n, sz)
}

// For cases when we want to track the start of the reply without
// calling Send.
func (cs *CallStat) Sending() {
	if cs == nil {
		return
	}
	cs.io.Lock()
	defer cs.io.Unlock()
	if !cs.replied {
		cs.io.For[cs.what].start(cs.t0)
		cs.replied = true
	}

}

// Data is being sent for the call.
func (cs *CallStat) Send(sz int64) {
	if cs == nil {
		return
	}
	cs.io.Lock()
	defer cs.io.Unlock()
	if !cs.replied {
		cs.io.For[cs.what].start(cs.t0)
		cs.replied = true
	}
	cs.io.For[cs.what].send(1, sz)
}

// Data is being sent for the call.
func (cs *CallStat) Sends(nmsgs int64, sz int64) {
	if cs == nil {
		return
	}
	cs.io.Lock()
	defer cs.io.Unlock()
	if !cs.replied {
		cs.io.For[cs.what].start(cs.t0)
		cs.replied = true
	}
	if nmsgs <= 0 {
		nmsgs = 1
	}
	cs.io.For[cs.what].send(nmsgs, sz)
}

// The call is done or failed.
func (cs *CallStat) End(failed bool) {
	if cs == nil {
		return
	}
	cs.io.Lock()
	defer cs.io.Unlock()
	if !cs.replied {
		cs.io.For[cs.what].start(cs.t0)
		cs.replied = true
	}
	// account for the close of the reply
	cs.io.For[cs.what].send(1, 0)

	cs.io.For[cs.what].end(cs.t0, failed)
}

// Compute averages for all iostats.
func (io *IOstats) Averages() {
	if io == nil {
		return
	}
	io.Lock()
	defer io.Unlock()
	for i := 0; i < len(io.For); i++ {
		io.For[i].averages()
	}
}

// Reset all iostats to zero
func (io *IOstats) Clear() {
	if io == nil {
		return
	}
	io.Lock()
	defer io.Unlock()
	for i := 0; i < len(io.For); i++ {
		io.For[i] = IOstat{}
	}
}

func (io *IOstats) String() string {
	var buf bytes.Buffer

	if io == nil {
		return "no stats\n"
	}
	if io.Tag != "" {
		fmt.Fprintf(&buf, "%s stats:\n", io.Tag)
	}
	for i := 0; i < len(io.For); i++ {
		if io.For[i].Ncalls > 0 {
			fmt.Fprintf(&buf, "%-s\t%s", name[i], &io.For[i])
		}
	}
	s := buf.String()
	if len(s) == 0 {
		return "no calls\n"
	}
	return s
}
