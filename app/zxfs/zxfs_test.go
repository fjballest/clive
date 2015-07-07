package main

import (
	"testing"
	xfs "clive/fuse"
	"clive/zx"
	"clive/zx/cfs"
	"clive/zx/fstest"
	"clive/fuse/ostest"
	"clive/zx/mfs"
	"clive/zx/lfs"
	"clive/zx/zxfs"
	"clive/dbg"
	"os/exec"
	"os"
	"time"
)

const (
	tdir = "/tmp/zxfs_test"
	tmnt = "/tmp/zxfs_testfs"
)

type setup {
	fs zx.RWTree
	deferred func()
	debug func()
	errc chan error
}

var (
	printf = dbg.FuncPrintf(os.Stdout, testing.Verbose)
	moreverb = false
	mktfs  = mktlfs
)

func umount() error {
	cmd := exec.Command("/sbin/umount", tmnt)
	return cmd.Run()
}

func mktlfs(t *testing.T) *setup {
	umount()
	os.RemoveAll(tdir)
	os.RemoveAll(tmnt)
	if err := os.Mkdir(tdir, 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	ostest.MkTree(t, tdir)
	if err := os.Mkdir(tmnt, 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	s := &setup{}
	fs, err := lfs.New("tlfs", tdir, lfs.RW)
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	fs.SaveAttrs(true)
	fs.IOstats = &zx.IOstats{}
	s.fs = fs
	s.errc = make(chan error, 1)
	done := func() {
		
		printf("umounted %v\n", umount())
		fs.Close(nil)
		os.RemoveAll(tdir)
		os.RemoveAll(tmnt)
	}
	if testing.Verbose() {
		s.deferred = func() {
			fs.Dump(os.Stdout)
			fs.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", fs, fs.IOstats)

			done()
		}
	} else {
		s.deferred = done
	}
	s.debug = func() {
		fs.IOstats.Clear()
		fs.Dbg = testing.Verbose()
		zxfs.Debug = testing.Verbose()
		zxfs.Verb = zxfs.Debug && moreverb
		xfs.Debug = zxfs.Debug && moreverb
	}
	mntdir = tmnt
	go func() {
		err := ncmount(fs)
		printf("umount sts %v\n", err)
		s.errc <- err
		close(s.errc)
	}()
	time.Sleep(300*time.Millisecond)
	if _, err := os.Stat(tmnt + "/d"); err != nil {
		done()
		t.Fatalf("not mounted")
	}
	return s
}

func TestMount(t *testing.T) {
	s := mktfs(t)
	defer s.deferred()
	s.debug()
}

var testfn = testfsfn

func testfsfn(t *testing.T, fn func(t ostest.Fataler, dirs ...string)) {
	s := mktfs(t)
	defer s.deferred()
	diffs, err := ostest.Diff(ostest.WithMtime, tmnt, tdir)
	if err != nil {
		t.Fatalf("can't diff: %s", err)
	}
	for _, d := range diffs {
		t.Logf("diff %s\n", d)
	}
	if len(diffs) != 0 {
		t.Fatalf("trees differ: %s", diffs[0])
	}
	s.debug()
	if fn != nil {
		fn(t, tmnt)
	}
}

func TestStats(t *testing.T) {
	testfn(t, ostest.Stats)
}

func TestGets(t *testing.T) {
	testfn(t, ostest.Gets)
}

func TestPuts(t *testing.T) {
	testfn(t, ostest.Puts)
}

func TestMkdirs(t *testing.T) {
	testfn(t, ostest.Mkdirs)
}

func TestRemoves(t *testing.T) {
	testfn(t, ostest.Removes)
}

func TestWstats(t *testing.T) {
	testfn(t, ostest.Wstats)
}

func TestAsAFile(t *testing.T) {
	testfn(t, ostest.AsAFile)
}

func mktmfs(t *testing.T) *setup {
	umount()
	os.RemoveAll(tdir)
	os.RemoveAll(tmnt)
	if err := os.Mkdir(tmnt, 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	s := &setup{}
	fs, err := mfs.New("tmfs")
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	fs.IOstats = &zx.IOstats{}
	ostest.MkTree(t, tdir)	// needed to compare trees and file data
	fstest.MkZXTree(t, fs)
	s.fs = fs
	done := func() {
		umount()
		fs.Close(nil)
		os.RemoveAll(tdir)
		os.RemoveAll(tmnt)
	}
	if testing.Verbose() {
		s.deferred = func() {
			fs.Dump(os.Stdout)
			fs.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", fs, fs.IOstats)

			done()
		}
	} else {
		s.deferred = done
	}
	s.debug = func() {
		fs.IOstats.Clear()
		fs.Dbg = testing.Verbose()
		zxfs.Debug = testing.Verbose()
		zxfs.Verb = zxfs.Debug && moreverb
		xfs.Debug = zxfs.Debug && moreverb
	}
	mntdir = tmnt
	s.errc = make(chan error, 1)
	go func() {
		err := ncmount(fs)
		printf("umount sts %v\n", err)
		s.errc <- err
		close(s.errc)
	}()
	time.Sleep(300*time.Millisecond)
	if _, err := os.Stat(tmnt + "/d"); err != nil {
		t.Fatalf("not mounted")
	}
	return s
}

func mktcfsmfslfs(t *testing.T) *setup {
	umount()
	os.RemoveAll(tdir)
	os.RemoveAll(tmnt)
	if err := os.Mkdir(tdir, 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	ostest.MkTree(t, tdir)
	if err := os.Mkdir(tmnt, 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	s := &setup{}
	fs1, err := mfs.New("mfs")
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	fs1.NoPermCheck = true
	fs1.IOstats = &zx.IOstats{}

	fs2, err := lfs.New("lfs", tdir, lfs.RW)
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	fs2.SaveAttrs(true)
	fs2.IOstats = &zx.IOstats{}
	fs, err := cfs.New("cfs", fs1, fs2, cfs.RW)
	if err != nil {
		t.Fatalf("cfs: %s", err)
	}
	fs.IOstats = &zx.IOstats{}
	s.fs = fs
	done := func() {
		umount()
		fs.Close(nil)
		os.RemoveAll(tdir)
	}
	if testing.Verbose() {
		s.deferred = func() {
			fs1.Dump(os.Stdout)
			fs2.Dump(os.Stdout)
			fs.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", fs, fs.IOstats)
			fs1.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", fs1, fs1.IOstats)
			fs2.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", fs2, fs2.IOstats)
			done()
		}
	} else {
		s.deferred = done
	}
	s.debug = func() {
		fs.IOstats.Clear()
		fs1.IOstats.Clear()
		fs2.IOstats.Clear()
		fs.Flags.Dbg = testing.Verbose()
		fs1.Dbg = testing.Verbose()
		fs2.Dbg = testing.Verbose()
		zxfs.Debug = testing.Verbose()
		zxfs.Verb = zxfs.Debug && moreverb
		xfs.Debug = zxfs.Debug && moreverb
	}
	mntdir = tmnt
	s.errc = make(chan error, 1)
	go func() {
		err := ncmount(fs)
		printf("umount sts %v\n", err)
		s.errc <- err
		close(s.errc)
	}()
	time.Sleep(300*time.Millisecond)
	if _, err := os.Stat(tmnt + "/d"); err != nil {
		t.Fatalf("not mounted")
	}
	return s
}

func testcfsfn(t *testing.T, fn func(t ostest.Fataler, dirs ...string)) {
	s := mktfs(t)
	defer s.deferred()
	s.debug()
	if fn != nil {
		fn(t, tmnt)
	}
}

func mktcfslfslfs(t *testing.T) *setup {
	umount()
	os.RemoveAll(tdir)
	os.RemoveAll(tdir+"c")
	os.RemoveAll(tmnt)
	if err := os.Mkdir(tdir, 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	if err := os.Mkdir(tdir+"c", 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	ostest.MkTree(t, tdir)
	if err := os.Mkdir(tmnt, 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	s := &setup{}
	fs1, err := lfs.New("lfs1", tdir+"c", lfs.RW)
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	fs1.SaveAttrs(true)
	fs1.NoPermCheck = true
	fs1.IOstats = &zx.IOstats{}

	fs2, err := lfs.New("lfs2", tdir, lfs.RW)
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	fs2.SaveAttrs(true)
	fs2.IOstats = &zx.IOstats{}
	fs, err := cfs.New("cfs", fs1, fs2, cfs.RW)
	if err != nil {
		t.Fatalf("cfs: %s", err)
	}
	fs.IOstats = &zx.IOstats{}
	s.fs = fs
	done := func() {
		umount()
		fs.Close(nil)
		os.RemoveAll(tdir)
		os.RemoveAll(tdir+"c")
	}
	if testing.Verbose() {
		s.deferred = func() {
			fs1.Dump(os.Stdout)
			fs2.Dump(os.Stdout)
			fs.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", fs, fs.IOstats)
			fs1.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", fs1, fs1.IOstats)
			fs2.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", fs2, fs2.IOstats)
			done()
		}
	} else {
		s.deferred = done
	}
	s.debug = func() {
		fs.IOstats.Clear()
		fs1.IOstats.Clear()
		fs2.IOstats.Clear()
		fs.Flags.Dbg = testing.Verbose()
		fs1.Dbg = testing.Verbose()
		fs2.Dbg = testing.Verbose()
		zxfs.Debug = testing.Verbose()
		zxfs.Verb = zxfs.Debug && moreverb
		xfs.Debug = zxfs.Debug && moreverb
	}
	mntdir = tmnt
	s.errc = make(chan error, 1)
	go func() {
		err := ncmount(fs)
		printf("umount sts %v\n", err)
		s.errc <- err
		close(s.errc)
	}()
	time.Sleep(300*time.Millisecond)
	if _, err := os.Stat(tmnt + "/d"); err != nil {
		t.Fatalf("not mounted")
	}
	return s
}

func xTestAsAFs(t *testing.T) {
	testfn(t, ostest.AsAFs)
}

func xTestFuseLfs(t *testing.T) {
	mktfs = mktlfs
	TestStats(t)
	TestGets(t)
	TestPuts(t)
	TestMkdirs(t)
	TestRemoves(t)
	TestWstats(t)
	TestAsAFile(t)
}

func xTestFuseMfs(t *testing.T) {
	mktfs = mktmfs
	TestStats(t)
	TestGets(t)
	TestPuts(t)
	TestMkdirs(t)
	TestRemoves(t)
	TestWstats(t)
	TestAsAFile(t)
}

func xTestFuseCfsLfsLfs(t *testing.T) {
	mktfs = mktcfslfslfs
	testfn = testcfsfn
	TestStats(t)
	TestGets(t)
	TestPuts(t)
	TestMkdirs(t)
	TestRemoves(t)
	TestWstats(t)
	TestAsAFile(t)
}

func xTestFuseCfsMfsLfs(t *testing.T) {
	mktfs = mktcfsmfslfs
	testfn = testcfsfn
	TestStats(t)
	TestGets(t)
	TestPuts(t)
	TestMkdirs(t)
	TestRemoves(t)
	TestWstats(t)
	TestAsAFile(t)
}

