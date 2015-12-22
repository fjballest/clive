package mfs

import (
	"clive/bufs"
	"clive/dbg"
	"clive/net/auth"
	"clive/zx"
	"clive/zx/fstest"
	"os"
	"testing"
)

const tdir = "/tmp/mfs_test"

var (
	printf   = dbg.FuncPrintf(os.Stdout, testing.Verbose)
	moreverb = false
)

func ExampleNew() {
	// create a tree
	fs, err := New("example mfs")
	if err != nil {
		dbg.Fatal("lfs: %s", err)
	}
	dbg.Warn("fs %s ready", fs)
	// Now use it...
}

func TestInitDirs(t *testing.T) {
	fs, err := New("example mfs")
	if err != nil {
		t.Fatalf("mfs: %s", err)
	}
	fs.Dbg = testing.Verbose()
	if fs.Dbg {
		defer fs.Dump(os.Stdout)
	}
	for _, dn := range fstest.Dirs {
		if err := zx.MkdirAll(fs, dn, zx.Dir{"mode": "0755"}); err != nil {
			t.Fatalf("mkdir: %s", err)
		}
	}
}

func testfn(t *testing.T, fns ...func(t fstest.Fataler, fss ...zx.Tree)) {
	bufs.Size = 1 * 1024
	mfs, err := New("example mfs")
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	xfs, _ := mfs.AuthFor(&auth.Info{Uid: dbg.Usr, SpeaksFor: dbg.Usr, Ok: true})
	fs := xfs.(zx.RWTree)
	fstest.MkZXTree(t, fs)
	mfs.Dbg = testing.Verbose()
	if mfs.Dbg {
		defer mfs.Dump(os.Stdout)
	}
	for _, fn := range fns {
		if fn != nil {
			fn(t, fs)
		}
	}
}

func TestInitFiles(t *testing.T) {
	testfn(t, nil)
}

func TestStats(t *testing.T) {
	testfn(t, fstest.Stats)
}

func TestGets(t *testing.T) {
	testfn(t, fstest.Gets)
}

func TestFinds(t *testing.T) {
	testfn(t, fstest.Finds)
}

func TestFindGets(t *testing.T) {
	testfn(t, fstest.FindGets)
}

func TestPuts(t *testing.T) {
	testfn(t, fstest.Puts)
}

func TestMkdirs(t *testing.T) {
	testfn(t, fstest.Mkdirs)
}

func TestRemoves(t *testing.T) {
	testfn(t, fstest.Removes)
}

func TestWstats(t *testing.T) {
	testfn(t, fstest.Wstats)
}

func TestUsrWstats(t *testing.T) {
	fstest.Repeats = 1
	testfn(t, fstest.UsrWstats)
}

func TestGetCtl(t *testing.T) {
	fstest.Repeats = 1
	testfn(t, fstest.GetCtl)
}

func TestPutCtl(t *testing.T) {
	fstest.Repeats = 1
	testfn(t, fstest.PutCtl)
}

func TestMoves(t *testing.T) {
	fstest.Repeats = 1
	testfn(t, fstest.Moves)
}

func TestSendRecv(t *testing.T) {
	os.Args[0] = "mfs_test"
	fs1, err := New("example mfs1")
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	fs1.Dbg = testing.Verbose()
	fstest.MkZXTree(t, fs1)
	fs2, err := New("example mfs2")
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	if fs1.Dbg {
		defer fs2.Dump(os.Stdout)
		defer fs1.Dump(os.Stdout)
	}
	fstest.SendRecv(t, fs1, fs2)

}

func TestAsAFile(t *testing.T) {
	testfn(t, fstest.AsAFile)
}

func TestRaces(t *testing.T) {
	testfn(t, fstest.Races, fstest.DirSizes)
}

func TestNewPerms(t *testing.T) {
	testfn(t, fstest.NewPerms)
}

func TestRWXPerms(t *testing.T) {
	testfn(t, fstest.RWXPerms)
}
