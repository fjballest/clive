package zxc

import (
	"clive/net"
	"clive/net/auth"
	"clive/u"
	"clive/zx"
	"clive/zx/fscmp"
	"clive/zx/fstest"
	"clive/zx/zux"
	"fmt"
	"os"
	"testing"
	"time"
)

var (
	tdir = "/tmp/zxctest"
	ai   = &auth.Info{Uid: u.Uid, SpeaksFor: u.Uid, Ok: true}
)

func runTest(t *testing.T, fn fstest.TestFunc) {
	os.Args[0] = "rzx.test"
	fstest.Verb = testing.Verbose()
	ccfg, err := net.TLSCfg("/Users/nemo/.ssh/client")
	if err != nil {
		t.Logf("no certs found, no tls conn")
	}
	scfg, err := net.TLSCfg("/Users/nemo/.ssh/server")
	if err != nil || ccfg == nil {
		ccfg = nil
		scfg = nil
		t.Logf("no certs found, no tls conn")
	}
	_, _ = scfg, ccfg
	fstest.MkTree(t, tdir)
	defer os.RemoveAll(tdir)
	lfs, err := zux.NewZX(tdir)
	if err != nil {
		t.Fatal(err)
	}
	defer lfs.Sync()

	cfs, err := New(lfs)
	if err != nil {
		t.Fatal(err)
	}
	defer cfs.Close()
	cfs.Debug = testing.Verbose()
	lfs.Debug = testing.Verbose()
	cfs.Flags.Set("rfsdebug", cfs.Debug)
	cfs.Flags.Set("cachedebug", cfs.Debug)
	if fn != nil {
		fn(t, cfs)
	} else {
		d, err := zx.Stat(cfs, "/")
		if err != nil {
			t.Fatalf("stat /: %v", err)
		}
		t.Logf("/ stat is %s\n", d.TestFmt())
	}
	if cfs.Debug {
		cfs.c.dump()
	}
}

func TestSrv(t *testing.T) {
	runTest(t, nil)
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

func TestSync(t *testing.T) {
	os.Args[0] = "rzx.test"
	fstest.Verb = testing.Verbose()
	ccfg, err := net.TLSCfg("/Users/nemo/.ssh/client")
	if err != nil {
		t.Logf("no certs found, no tls conn")
	}
	scfg, err := net.TLSCfg("/Users/nemo/.ssh/server")
	if err != nil || ccfg == nil {
		ccfg = nil
		scfg = nil
		t.Logf("no certs found, no tls conn")
	}
	_, _ = scfg, ccfg
	fstest.MkTree(t, tdir)
	defer os.RemoveAll(tdir)
	lfs, err := zux.NewZX(tdir)
	if err != nil {
		t.Fatal(err)
	}
	defer lfs.Sync()

	cfs, err := New(lfs)
	if err != nil {
		t.Fatal(err)
	}
	defer cfs.Close()
	cfs.Debug = testing.Verbose()
	lfs.Debug = testing.Verbose()
	cfs.Flags.Set("rfsdebug", cfs.Debug)
	cfs.Flags.Set("cachedebug", cfs.Debug)
	cfs.Flags.Set("verb", cfs.Debug)

	fstest.MkZXChgs(t, cfs)
	fstest.MkZXChgs2(t, cfs)
	rc := fscmp.Diff(lfs, cfs)
	out := ""
	for c := range rc {
		s := fmt.Sprintf("chg %s %s\n", c.Type, c.D.Fmt())
		out += s
	}
	cfs.Dprintf("%s", out)
	if err := cfs.Sync(); err != nil {
		t.Fatalf("sync %s", err)
	}
	rc = fscmp.Diff(lfs, cfs)
	out = ""
	for c := range rc {
		s := fmt.Sprintf("chg %s %s\n", c.Type, c.D.Fmt())
		out += s
	}
	cfs.Dprintf("%s", out)
	if len(out) > 0 {
		t.Fatalf("didn't sync; have changes")
	}
	if cfs.Debug {
		cfs.c.dump()
	}
}

func TestExtChanges(t *testing.T) {
	os.Args[0] = "rzx.test"
	fstest.Verb = testing.Verbose()
	ccfg, err := net.TLSCfg("/Users/nemo/.ssh/client")
	if err != nil {
		t.Logf("no certs found, no tls conn")
	}
	scfg, err := net.TLSCfg("/Users/nemo/.ssh/server")
	if err != nil || ccfg == nil {
		ccfg = nil
		scfg = nil
		t.Logf("no certs found, no tls conn")
	}
	_, _ = scfg, ccfg
	fstest.MkTree(t, tdir)
	defer os.RemoveAll(tdir)
	lfs, err := zux.NewZX(tdir)
	if err != nil {
		t.Fatal(err)
	}
	defer lfs.Sync()

	cfs, err := New(lfs)
	if err != nil {
		t.Fatal(err)
	}
	defer cfs.Close()
	cfs.Debug = testing.Verbose()
	lfs.Debug = testing.Verbose()
	cfs.Flags.Set("rfsdebug", cfs.Debug)
	cfs.Flags.Set("cachedebug", cfs.Debug)
	cfs.Flags.Set("verb", cfs.Debug)
	rc := fscmp.Diff(lfs, cfs)
	out := ""
	for c := range rc {
		s := fmt.Sprintf("pre chg %s %s\n", c.Type, c.D.Fmt())
		out += s
	}
	if len(out) > 0 {
		t.Fatalf("had changes")
	}
	cfs.Dprintf("%s", out)
	fstest.MkZXChgs(t, lfs)
	fstest.MkZXChgs2(t, lfs)
	cacheTout = time.Millisecond
	time.Sleep(cacheTout)
	rc = fscmp.Diff(lfs, cfs)
	out = ""
	for c := range rc {
		s := fmt.Sprintf("post chg %s %s\n", c.Type, c.D.Fmt())
		out += s
	}
	cfs.Dprintf("%s", out)
	if len(out) > 0 {
		t.Fatalf("had missed external changes")
	}
	if cfs.Debug {
		cfs.c.dump()
	}
}
