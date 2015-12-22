package fstest

import (
	"clive/zx"
)

// Wrapper for a replicated or cached file system to be tested
type Repl struct {
	t    Fataler
	ts   []zx.Tree
	sync func()
}

// Create a setup to test a Repl. Trees must be initially synchronized
// or in a state that makes their Dump() match.
func NewRepl(t Fataler, syncfn func(), fss ...zx.Tree) *Repl {
	for _, fs := range fss {
		if _, ok := fs.(zx.RWTree); !ok {
			t.Fatalf("%s is not a RW tree", fs)
		}
	}
	if len(fss) < 2 {
		t.Fatalf("not enough RW trees")
	}
	return &Repl{t: t, sync: syncfn, ts: fss}
}

func ReadAll(t Fataler, fs zx.Tree) {
	bc := make(chan []byte)
	go func() {
		for range bc {
		}
	}()
	rd, err := zx.Stat(fs, "/")
	if err != nil {
		t.Fatalf("stat: %s", err)
	}
	zx.Send(fs, rd, bc)
}

// Apply MkZXChgs and MkZXChgs2 to the 1st tree and see they are synced.
func (r *Repl) LfsChanges() {
	// check that all trees are initially the same
	ReadAll(r.t, r.ts[0])
	SameDump(r.t, r.ts...)

	fs1 := r.ts[0]
	MkZXChgs(r.t, fs1.(zx.RWTree))
	MkZXChgs2(r.t, fs1.(zx.RWTree))

	r.sync()
	SameDump(r.t, r.ts...)
}

// Apply MkZXChgs and MkZXChgs2 to the last tree and see they are synced.
func (r *Repl) RfsChanges() {
	// check that all trees are initially the same
	ReadAll(r.t, r.ts[0])
	SameDump(r.t, r.ts...)

	fs2 := r.ts[len(r.ts)-1]
	MkZXChgs(r.t, fs2.(zx.RWTree))
	MkZXChgs2(r.t, fs2.(zx.RWTree))

	r.sync()
	ReadAll(r.t, r.ts[0])
	SameDump(r.t, r.ts...)
}

// Apply MkZXChgs to the 1st tree and  MkZXChgs2 to the last tree and see they are synced.
func (r *Repl) LfsRfsChanges() {
	// check that all trees are initially the same
	ReadAll(r.t, r.ts[0])
	SameDump(r.t, r.ts...)

	fs1 := r.ts[0]
	MkZXChgs(r.t, fs1.(zx.RWTree))
	fs2 := r.ts[len(r.ts)-1]
	MkZXChgs2(r.t, fs2.(zx.RWTree))

	r.sync()
	ReadAll(r.t, r.ts[0])
	SameDump(r.t, r.ts...)
}

// Apply MkZXChgs and  MkZXChgs2 to the 1st and last tree and see they are synced.
func (r *Repl) SameChanges() {
	// check that all trees are initially the same
	ReadAll(r.t, r.ts[0])
	SameDump(r.t, r.ts...)

	fs1 := r.ts[0]
	MkZXChgs(r.t, fs1.(zx.RWTree))
	MkZXChgs2(r.t, fs1.(zx.RWTree))
	fs2 := r.ts[len(r.ts)-1]
	MkZXChgs(r.t, fs2.(zx.RWTree))
	MkZXChgs2(r.t, fs2.(zx.RWTree))

	r.sync()
	ReadAll(r.t, r.ts[0])
	SameDump(r.t, r.ts...)
}
