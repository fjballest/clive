package zux

import (
	"clive/net/auth"
	"clive/u"
	"clive/zx"
	"clive/zx/fstest"
	"os"
	"testing"
)

const (
	tdir = "/tmp/zx_test"
)

var ai = &auth.Info{Uid: u.Uid, SpeaksFor: u.Uid, Ok: true}

func runTest(t *testing.T, fn fstest.TestFunc) {
	os.Args[0] = "zux.test"
	fstest.Verb = testing.Verbose()
	fstest.MkTree(t, tdir)
	defer os.RemoveAll(tdir)

	fs, err := NewZX(tdir)
	if err != nil {
		t.Fatal(err)
	}
	afs, err := fs.Auth(ai)
	if err != nil {
		t.Fatal(err)
	}
	fn(t, afs)
	fs.Sync()
}

func TestAttrCache(t *testing.T) {
	os.Remove("/tmp/.zx")
	d := zx.Dir{"foo": "bar"}
	ac.set("/tmp/one", d)
	d = zx.Dir{"foo1": "bar1"}
	ac.set("/tmp/one", d)
	d = zx.Dir{"foo2": "bar2"}
	ac.set("/tmp/two", d)
	d = zx.Dir{}
	ac.get("/tmp/one", d)
	t.Logf("did get %s\n", d.LongFmt())
	if d.LongFmt() != `one foo:"bar" foo1:"bar1"` {
		t.Fatalf("didn't get attrs")
	}
	d = zx.Dir{}
	ac.get("/tmp/two", d)
	t.Logf("did get %s\n", d.LongFmt())
	if d.LongFmt() != `two foo2:"bar2"` {
		t.Fatalf("didn't get attrs")
	}
	ac.sync()
	d = zx.Dir{}
	ac.get("/tmp/one", d)
	t.Logf("did get %s\n", d.LongFmt())
	if d.LongFmt() != `one foo:"bar" foo1:"bar1"` {
		t.Fatalf("didn't get attrs after sync")
	}
	d = zx.Dir{"foo": "bar"}
	ac.set("/tmp/two", d)
	d = zx.Dir{}
	ac.get("/tmp/two", d)
	t.Logf("did get %s\n", d.LongFmt())
	if d.LongFmt() != `two foo:"bar" foo2:"bar2"` {
		t.Fatalf("didn't get attrs after sync")
	}
}

func TestStats(t *testing.T) {
	runTest(t, fstest.Stats)
}

func TestGetCtl(t *testing.T) {
	runTest(t, fstest.GetCtl)
}

func TestGets(t *testing.T) {
	runTest(t, fstest.Gets)
}

func TestFinds(t *testing.T) {
	runTest(t, fstest.Finds)
}

func TestFindGets(t *testing.T) {
	runTest(t, fstest.FindGets)
}

func TestPuts(t *testing.T) {
	runTest(t, fstest.Puts)
}

func TestMkdirs(t *testing.T) {
	runTest(t, fstest.Mkdirs)
}

func TestRemoves(t *testing.T) {
	runTest(t, fstest.Removes)
}

func TestWstats(t *testing.T) {
	runTest(t, fstest.Wstats)
}

func TestAttrs(t *testing.T) {
	runTest(t, fstest.Attrs)
}

func TestMoves(t *testing.T) {
	runTest(t, fstest.Moves)
}

func TestAsAFile(t *testing.T) {
	runTest(t, fstest.AsAFile)
}
