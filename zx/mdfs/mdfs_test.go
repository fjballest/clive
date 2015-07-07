package mdfs

import (
	"clive/bufs"
	"clive/dbg"
	"clive/zx"
	"clive/zx/lfs"
	"clive/zx/fstest"
	"clive/net/auth"
	"os"
	"testing"
)

const tdir = "/tmp/mdfs_test"
const tlfsdir = "/tmp/mdfslfs_test"

var (
	printf = dbg.FuncPrintf(os.Stdout, testing.Verbose)
	moreverb = true
)

func ExampleNew() {
	// create a tree using the local directory /tmp/cache for the cache
	dfs, err := lfs.New("cache", "/tmp/cache", lfs.RW)
	if err != nil {
		dbg.Fatal("lfs: %s", err)
	}
	fs, err := New("example mdfs", dfs)
	if err != nil {
		dbg.Fatal("mfds: %s", err)
	}
	dbg.Warn("fs %s ready", fs)
	// Now use it...
}


func TestInitDirs(t *testing.T) {
	os.Args[0] = "mdfs_test"
	os.RemoveAll(tlfsdir)
	defer os.RemoveAll(tlfsdir)
	if err := os.Mkdir(tlfsdir, 0755); err != nil {
		t.Fatalf("lfs: %s", err)
	}
	dfs, err := lfs.New("	cache", tlfsdir, lfs.RW)
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	dfs.SaveAttrs(true)
	dfs.Dbg = moreverb && testing.Verbose()
	fs, err := New("example mdfs", dfs)
	if err != nil {
		t.Fatalf("mdfs: %s", err)
	}
	fs.Dbg = testing.Verbose()
	if fs.Dbg {
		defer func() {
			fs.Dbg = false
			dfs.Dbg = false
			fs.Dump(os.Stdout)
			dfs.Dump(os.Stdout)
		}()
	}
	for _, dn := range fstest.Dirs {
		if err := zx.MkdirAll(fs, dn, zx.Dir{"mode":"0755"}); err != nil {
			t.Fatalf("mkdir: %s", err)
		}
	}
}

func testfn(t *testing.T, fns ...func(t fstest.Fataler, fss ...zx.Tree)) {
	bufs.Size = 1*1024
	os.RemoveAll(tlfsdir)
	defer os.RemoveAll(tlfsdir)
	if err := os.Mkdir(tlfsdir, 0755); err != nil {
		t.Fatalf("lfs: %s", err)
	}
	os.Args[0] = "mdfs_test"
	dfs, err := lfs.New("	cache", tlfsdir, lfs.RW)
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	dfs.SaveAttrs(true)
	mfs, err := New("example mfs", dfs)
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	xfs, _:= mfs.AuthFor(&auth.Info{Uid: dbg.Usr, SpeaksFor: dbg.Usr, Ok: true})
	fs := xfs.(zx.RWTree)
	fstest.MkZXTree(t, fs)
	mfs.Dbg = testing.Verbose()
	dfs.Dbg = testing.Verbose() 
	var fn func(t fstest.Fataler, fss ...zx.Tree)
	if len(fns) > 0 {
		fn = fns[0]
	}
	if fn != nil {
		if mfs.Dbg {
			defer func() {
				mfs.Dump(os.Stdout)
				dfs.Dump(os.Stdout)
			}()
		}
		for _, fn := range fns {
			fn(t, fs)
		}
	} else {
		d1, _ := zx.Stat(mfs, "/")
		printf("mfs st:\t%s\n", d1)
		d1, _ = zx.Stat(dfs, "/")
		printf("lfs st:\t%s\n", d1)
		// recreate, to test a reload
		mfs, err = New("example mfs", dfs)
		if err != nil {
			t.Fatalf("lfs: %s", err)
		}
		mfs.Dbg = testing.Verbose()
		xfs, _= mfs.AuthFor(&auth.Info{Uid: dbg.Usr, SpeaksFor: dbg.Usr, Ok: true})
		fs = xfs.(zx.RWTree)
		if mfs.Dbg {
			defer func() {
				mfs.Dump(os.Stdout)
				dfs.Dump(os.Stdout)
			}()
		}
	}
	mfs.Dbg = false
	dfs.Dbg = false
	fstest.SameDump(t, mfs, dfs)
}

func TestReload(t *testing.T) {
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
	t.Skip("TODO: this does not work but we are no longer sending trees")
	os.RemoveAll(tlfsdir+"2")
	os.RemoveAll(tlfsdir)
	if err := os.Mkdir(tlfsdir, 0755); err != nil {
		t.Fatalf("lfs: %s", err)
	}
	if err := os.Mkdir(tlfsdir+"2", 0755); err != nil {
		t.Fatalf("lfs: %s", err)
	}
	defer os.RemoveAll(tlfsdir+"2")
	defer os.RemoveAll(tlfsdir)
	os.Args[0] = "mdfs_test"

	dfs1, err := lfs.New("	cache1", tlfsdir, lfs.RW)
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	dfs1.SaveAttrs(true)
	fs1, err := New("example mfs1", dfs1)
	if err != nil {
		t.Fatalf("mdfs: %s", err)
	}
	fs1.Dbg = testing.Verbose()

	fstest.MkZXTree(t, fs1)
	dfs2, err := lfs.New("	cache2", tlfsdir+"2", lfs.RW)
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	dfs2.SaveAttrs(true)
	fs2, err := New("example mfs2", dfs2)
	if err != nil {
		t.Fatalf("mdfs: %s", err)
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

