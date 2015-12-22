package fstest

import (
	"clive/zx"
	"testing"
	"time"
)

const nops = 512

type raceOp struct {
	fn func(t Fataler, fs zx.RWTree, fname string)
	nm string
}

var ops = []raceOp{
	raceOp{rcreate, "create"},
	raceOp{rremove, "remove"},
	raceOp{rmkdir, "mkdir"},
	raceOp{rget, "get"},
	raceOp{rstat, "stat"},
	raceOp{rwstat, "wstat"},
	raceOp{rmove, "move"},
}

func (r raceOp) String() string {
	return r.nm
}

func rcreate(t Fataler, fs zx.RWTree, fname string) {
	zx.PutAll(fs, fname, zx.Dir{"mode": "0644"}, []byte("hi there"))
}

func rremove(t Fataler, fs zx.RWTree, fname string) {
	<-fs.Remove(fname)
}

func rmkdir(t Fataler, fs zx.RWTree, fname string) {
	<-fs.Mkdir(fname, zx.Dir{"mode": "0755"})
}

func rget(t Fataler, fs zx.RWTree, fname string) {
	zx.GetAll(fs, fname)
}

func rstat(t Fataler, fs zx.RWTree, fname string) {
	zx.Stat(fs, fname)
}

func rwstat(t Fataler, fs zx.RWTree, fname string) {
	<-fs.Wstat(fname, zx.Dir{"mode": "0700", "Foo": "Bar"})
}

func rmove(t Fataler, fs zx.RWTree, fname string) {
	<-fs.Move(fname, fname+"o")
	<-fs.Move(fname+"o", fname)
}

// Independently of the final result, make sure the tree survives
// concurrent operations with no deadlocks.
func Races(t Fataler, fss ...zx.Tree) {
	if len(fss) == 0 {
		t.Fatalf("no fs given")
	}
	fs := fss[0].(zx.RWTree)
	donec := make(chan bool, 2*len(ops))
	for i := range ops {
		go func(i int) {
			printf("%s /a/racea...\n", ops[i])
			for n := 0; n < nops; n++ {
				ops[i].fn(t, fs, "/a/racea")
			}
			printf("%s /a/racea done\n", ops[i])
			donec <- true
		}(i)
		go func(i int) {
			printf("%s /a/raceb...\n", ops[i])
			for n := 0; n < nops; n++ {
				ops[i].fn(t, fs, "/a/raceb")
			}
			printf("%s /a/raceb done\n", ops[i])
			donec <- true
		}(i)
	}
	alldone := make(chan bool, 1)
	go func() {
		for i := 0; i < 2*len(ops); i++ {
			<-donec
		}
		alldone <- true
	}()
	tout := 60 * time.Second
	if testing.Verbose() {
		tout = 300 * time.Second
	}
	select {
	case <-alldone:
		printf("all done\n")
	case <-time.After(tout):
		t.Fatalf("deadlock after %v", tout)
	}
}
