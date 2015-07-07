package cfs

import (
	"sync"
	"io"
	"strings"
	"fmt"
	"clive/zx/trfs"
)

type Tracer {
	c <-chan string
	callslk sync.Mutex
	calls []string
	callsok chan bool
}

// Traces collected for Cfs using zx.trfs
type Traces []string

// Used by tests to check lfs and rfs requests made by each cfs call.
type TraceDep {
	Op string		// request
	Lfsops []string	// lfs requests implied
	Rfsops []string	// rfs requests implied
}

type TraceDeps []TraceDep

// Collect and keep traces for a Cfs.
// The chan passed corresponds to a trfs chan used
// for cfs and the two zx trees given to it.
// By convention, Tags should start with "cfs", "fs1", and "fs2" for
// Cfs, its cache fs, and its cached fs.
// Beware that this is used for testing and that the traces for
// concurrent calls made to the Cfs would be mixed.
// The testing functions use this only for sequential requests.
func Trace(c <-chan string) *Tracer {
	t := &Tracer{c: c, callsok: make(chan bool, 1)}
	go func() {
		for call := range c {
			t.callslk.Lock()
			t.calls = append(t.calls, call)
			t.callslk.Unlock()
		}
		t.callsok <- true
		close(t.callsok)
	}()
	return t
}

// Wait for all traces to arrive (ie, the trace chan is closed)
func (t *Tracer) Wait() {
	<-t.callsok
}

// Return the set of messages seen so far and clear the list.
func (t *Tracer) All() Traces {
	t.callslk.Lock()
	defer t.callslk.Unlock()
	c := t.calls
	t.calls = nil
	return c
}

// Return the set of request messages seen so far and clear the list.
func (t *Tracer) Calls() Traces {
	t.callslk.Lock()
	defer t.callslk.Unlock()
	var nt Traces
	for _, c := range t.calls {
		if strings.Contains(c, "->") {
			nt = append(nt, c)
		}
	}
	t.calls = nil
	return nt
}

func (t Traces) Deps() TraceDeps {
	tc := []TraceDep{}
	var tp *TraceDep
	for _, c := range t {
		o := trfs.Parse(c)
		c = o.Fs + o.Dir + o.Op
		if o.Dir == "->" && len(o.Args) > 0 {
			c += " " + o.Args[0]
		}
		switch {
		case o.Fs == "cfs":
			tc = append(tc, TraceDep{Op: c})
			tp = &tc[len(tc)-1]
		case tp != nil && o.Fs == "fs1":
			tp.Lfsops = append(tp.Lfsops, c)
		case tp != nil && o.Fs == "fs2":
			tp.Rfsops = append(tp.Rfsops, c)
		}
	}
	return tc
}

func (tc TraceDeps) WriteTo(w io.Writer) {
	for _, t := range tc {
		fmt.Fprintf(w, "%s\n", t.Op)
		for _, l := range t.Lfsops {
			fmt.Fprintf(w, "\t%s\n", l)
		}
		for _, l := range t.Rfsops {
			fmt.Fprintf(w, "\t\t%s\n", l)
		}
	}
}

func (tc TraceDeps) WriteRfsTo(w io.Writer) {
	for _, t := range tc {
		fmt.Fprintf(w, "%s\n", t.Op)
		for _, l := range t.Rfsops {
			fmt.Fprintf(w, "\t%s\n", l)
		}
	}
}

// Write the traces to w for debugging
func (t Traces) WriteTo(w io.Writer) {
	for _, c := range t {
		if strings.HasPrefix(c, "fs1") {
			c = "    " + c
		} else if strings.HasPrefix(c, "fs2") {
			c = "        " + c
		}
		fmt.Fprintf(w, "%s\n", c)
	}
}
