package rfs

import (
	"clive/dbg"
	"clive/nchan"
	"clive/zx"
	"clive/zx/fstest"
	"clive/zx/lfs"
	"fmt"
	"os"
	"testing"
)

const tdir = "/tmp/rfs_test"

var (
	printf = dbg.FuncPrintf(os.Stdout, testing.Verbose)

	showstats = true
	moreverb  = true
)

func TestBadImport(t *testing.T) {
	_, err := Import("tcp!blah!zx")
	if err == nil {
		t.Fatal("did work")
	}
	printf("import sts is %s\n", err)

}

func ExampleRfs() {
	xfs, err := Import("tcp!whatever!rfs")
	if err != nil {
		dbg.Fatal("import: %s", err)
	}
	fs := xfs.(*Rfs)
	// perhaps enable IO stats
	fs.IOstats = &zx.IOstats{}
	defer func() {
		fs.IOstats.Averages()
		fmt.Printf("iostats:\n%s\n", fs.IOstats)
	}()

	// use it, eg. to do a stat
	dc := fs.Stat("/a/file")
	d := <-dc
	if d == nil {
		dbg.Fatal("stat: %s", cerror(dc))
	}
	dbg.Warn("got dir %s", dc)

	// stop rfs when done.
	fs.Close(nil)
}

func ExampleSrv() {
	// Assume we have two trees and a conn to a client
	var t0, t1 zx.Tree
	var c nchan.Conn

	srv := Serve("srv", c, nil, RW, t0, t1)
	srv.Debug = testing.Verbose()

	// we can let it run or shut it down by calling
	srv.Close(nil)
}

func testfs(t fstest.Fataler) (*lfs.Lfs, *Rfs) {
	fs, err := lfs.New(tdir, tdir, lfs.RW)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	c1, c2 := nchan.NewConnPipe(0)
	srv := Serve("srv", c1, nil, RW, fs)
	srv.Debug = testing.Verbose() && moreverb
	rfs, err := New(nchan.NewMux(c2, true), "")
	if err != nil {
		t.Fatalf("%s", err)
	}
	rfs.Tag = "cli"
	rfs.Dbg = testing.Verbose()
	VerbDebug = moreverb
	return fs, rfs
}

func testfn(t *testing.T, fn func(t fstest.Fataler, fss ...zx.Tree)) {
	fstest.MkTree(t, tdir)
	defer fstest.RmTree(t, tdir)
	lfs, rfs := testfs(t)
	if testing.Verbose() || showstats {
		rfs.IOstats = &zx.IOstats{}
		defer func() {
			rfs.IOstats.Averages()
			fmt.Printf("rfs iostats:\n%s\n", rfs.IOstats)
		}()
	}

	fn(t, rfs, lfs)
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

func benchfn(b *testing.B, fn func(b *testing.B, t zx.Tree)) {
	b.StopTimer()
	fstest.RmTree(b, tdir)
	fstest.MkTree(b, tdir)
	defer func() {
		b.StopTimer()
		fstest.RmTree(b, tdir)
	}()
	rfs, _ := testfs(b)
	b.StartTimer()
	fn(b, rfs)
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

func BenchmarkMkdirRemoves(b *testing.B) {
	benchfn(b, fstest.MkdirRemoveBench)
}

func BenchmarkWstats(b *testing.B) {
	benchfn(b, fstest.WstatBench)
}

/*
	This can be used to bench multiple transports...
	it can be used like in...

	The last time we tried, pipe was the fastest, then tcp, and then
	(and very slow) fifo. surprise.

	ns, rfs, err := benchFS(net)
	if err != nil {
		b.Fatalf("lfs: %s", err)
	}
	...
	rfs.Close(nil)
	if ns != nil {
		ns.Stop(true)
	}

func benchFS(net string) (ns *srv.Srv, rfs *Rfs, err error) {
	fs, err := lfs.New("bench lfs", "/", lfs.RW)
	if err != nil {
		return nil, nil, err
	}
	switch net {
	case "pipe":
		c1, c2 := nchan.NewConnPipe(0)
		Serve("srv", c2, nil, RW, fs)
		rfs, err = New(nchan.NewMux(c1, true), "")
	case "fifo":
		hs, hc := fifo.NewChanHandler()
		s := fifo.New("rfs", "rfs", hs)
		if err = s.Serve(); err != nil {
			break
		}
		go func() {
			c := <-hc
			auth.AtServer(*c, "rfs", "zx")
			Serve("srv", *c, nil, RW, fs)
		}()
		rfs, err = Import("fifo!*!rfs")
	case "tcp":
		localhost := "localhost"
		hs, hc := srv.NewChanHandler()
		ns = srv.New("srv", "tcp", localhost, "9898", hs)
		if err = ns.Serve(); err != nil {
			break
		}
		go func() {
			c := <-hc
			auth.AtServer(*c, "rfs", "zx")
			Serve("srv", *c, nil, RW, fs)

		}()
		rfs, err = Import("tcp!"+localhost+"!9898")
	default:
		err = errors.New("unknown bench net")
	}
	return ns, rfs, err
}
*/
