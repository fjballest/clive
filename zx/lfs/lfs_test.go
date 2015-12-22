package lfs

import (
	"clive/dbg"
	"clive/net/auth"
	"clive/zx"
	"clive/zx/fstest"
	"os"
	"testing"
)

const tdir = "/tmp/lfs_test"

var (
	printf   = dbg.FuncPrintf(os.Stdout, testing.Verbose)
	moreverb = false
)

func ExampleNew() {
	// create a tree rooted at /bin in RO mode
	fs, err := New("example lfs", "/bin", RO)
	if err != nil {
		dbg.Fatal("lfs: %s", err)
	}
	dbg.Warn("fs %s ready", fs)
	// Now use it...
}

func ExampleLfs_Stat() {
	var fs *Lfs // = New("tag", path, RO|RW)

	dirc := fs.Stat("/ls")
	dir := <-dirc
	if dir == nil {
		dbg.Fatal("stat: %s", cerror(dirc))
	}
	dbg.Warn("stat was %s", dir)
}

func ExampleLfs_Get() {
	// let's use a tree rooted at / in RO mode
	fs, err := New("example lfs", "/", RO)
	if err != nil {
		dbg.Fatal("lfs: %s", err)
	}
	// Now do a "cat" of /etc/hosts
	dc := fs.Get("/etc/hosts", 0, zx.All, "")

	// if had an early error, it's reported by the close of dc
	// and we won't receive any data.
	for data := range dc {
		os.Stdout.Write(data)
	}
	if cerror(dc) != nil {
		dbg.Fatal("get: %s", cerror(dc))
	}
}

func testfn(t *testing.T, fns ...func(t fstest.Fataler, fss ...zx.Tree)) {
	fstest.RmTree(t, tdir)
	fstest.MkTree(t, tdir)
	defer fstest.RmTree(t, tdir)
	lfs, err := New(tdir, tdir, RW)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	lfs.SaveAttrs(true)
	lfs.Dbg = testing.Verbose()
	xfs, _ := lfs.AuthFor(&auth.Info{Uid: dbg.Usr, SpeaksFor: dbg.Usr, Ok: true})
	fs := xfs.(zx.RWTree)
	for _, fn := range fns {
		if fn != nil {
			fn(t, fs)
		}
	}
}

func TestStats(t *testing.T) {
	testfn(t, fstest.Stats)
}

func TestDirSizes(t *testing.T) {
	testfn(t, fstest.DirSizes)
}

func TestGets(t *testing.T) {
	testfn(t, fstest.Gets)
}

func TestFinds(t *testing.T) {
	testfn(t, fstest.Gets)
	fstest.MkTree(t, tdir)
	defer fstest.RmTree(t, tdir)
	lfs, err := New(tdir, tdir, RO)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	fstest.Finds(t, lfs)
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
	t.Skip("send/recv is broken by now")
	tdir2 := tdir + "2"
	fstest.RmTree(t, tdir)
	fstest.RmTree(t, tdir2)
	fstest.MkTree(t, tdir)
	os.Mkdir(tdir+"2", 0755)
	defer fstest.RmTree(t, tdir)
	defer fstest.RmTree(t, tdir2)

	fs, err := New(tdir, tdir, RW)
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	fs2, err := New(tdir2, tdir2, RW)
	if err != nil {
		t.Fatal(err)
	}
	fstest.SendRecv(t, fs, fs2)

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

func benchfn(b *testing.B, fn func(b *testing.B, t zx.Tree)) {
	b.StopTimer()
	defer func() {
		b.StopTimer()
		fstest.RmTree(b, tdir)
	}()
	fstest.RmTree(b, tdir)
	fstest.MkTree(b, tdir)
	lfs, err := New(tdir, tdir, RW)
	if err != nil {
		b.Fatalf("new: %s", err)
	}
	b.StartTimer()
	fn(b, lfs)
}

func BenchmarkStats(b *testing.B) {
	benchfn(b, fstest.StatsBench)
}

func BenchmarkGets(b *testing.B) {
	benchfn(b, fstest.GetsBench)
}

func BenchmarkFinds(b *testing.B) {
	benchfn(b, fstest.FindsBench)
}

func BenchmarkPuts(b *testing.B) {
	benchfn(b, fstest.PutsBench)
}

func BenchmarkMkdirRemoves(b *testing.B) {
	benchfn(b, fstest.MkdirRemoveBench)
}

func BenchmarkWstats(b *testing.B) {
	benchfn(b, fstest.WstatBench)
}
