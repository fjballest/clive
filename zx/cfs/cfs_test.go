package cfs

import (
	"clive/dbg"
	"clive/zx/mfs"
	"clive/zx/mdfs"
	"clive/zx/cfs/cache"
	"clive/zx"
	"clive/zx/fstest"
	"clive/zx/lfs"
	"clive/zx/trfs"
	"clive/net/auth"
	"os"
	"testing"
	"strings"
	"bytes"
	"time"
	"io"
)


const tdir = "/tmp/ncfs_test"
const tdir2 = "/tmp/ncfs2_test"

type setup {
	fs1, fs2 zx.RWTree
	cfs zx.RWTree
	syncer *Cfs
	deferred func()
	debug func()

	trz *Tracer
}

/*
	Go test -run TestNewClose
	Go test -run TestOneMkdir
	Go test -run TestInitDirs
	Go test -run TestInitFiles
	Go test -run TestStats
	Go test -run TestGets
	Go test -run TestFinds
	Go test -run TestFindGets
	Go test -run TestPuts
	Go test -run TestMkdirs
	Go test -run TestRemoves
	Go test -run TestWstats
	Go test -v -run TestUsrWstats 
	Go test -run TestCtl
	Go test -run TestAsAFile
	Go test -run TestRaces
	Go test -v -run TestNewPerms	
	Go test -v -run TestRWXPerms

	Go test -run TestLfsChanges
	Go test -run TestLfsToutChanges
	Go test -v -run TestRfsChanges
	Go test -run TestLfsRfsChanges
	Go test -run TestSameChanges
	Go test -run TestFs2Changes
	Go test -run TestInvalInit
	Go test -run TestInvalChanges
*/

var (
	printf = dbg.FuncPrintf(os.Stdout, testing.Verbose)
	moreverb = true
	mkfs  = mkmfslfs	// MFS + LFS
	// mkfs = mklfslfs	// LFS + LFS
	// mkfs = mkmdfslfs	// MDFS(LFS) + LFS

)

func ExampleTrace() {
	var c chan string
	// create a tracer to collect calls given the trfs c chan
	trz := Trace(c)

	// use the fs here...

	close(c)
	// wait for traces to finish
	trz.Wait()

	// And now do ONE of the following:

	// write all messages to stdout
	trz.All().WriteTo(os.Stdout)

	// write all calls to stdout
	trz.Calls().WriteTo(os.Stdout)

	// write requests to cfs and the dependencies on lfs and rfs
	trz.Calls().Deps().WriteTo(os.Stdout)

	// write requests to cfs and dependencies on rfs
	trz.Calls().Deps().WriteRfsTo(os.Stdout)

	// In tests you could define a expected pattern
	expected := `cfs ->stat
cfs ->stat
cfs ->get
	fs2->get
cfs ->close
	fs2->close
`

	// write the trace to a buffer
	var buf bytes.Buffer
	trz.Calls().Deps().WriteRfsTo(&buf)

	// and compare to check out that calls triggered are as expected.
	if expected != buf.String() {
		// fail here
	}
}

func ExampleNew() {
	// create an in-memory tree 
	cachefs, err := mfs.New("example mfs")
	if err != nil {
		dbg.Fatal("mfs: %s", err)
	}
	cachefs.WstatAll = true	// cfs must be able to wstat all in the cache
	// perhaps set debug for it
	cachefs.Dbg = true

	// and perhaps record the calls made to it
	cachetrfs := trfs.New(cachefs)

	// create an on-disk tree (OR or RW)
	dfs, err := lfs.New("test fs", tdir, lfs.RW)
	if err != nil {
		dbg.Fatal("lfs: %s", err)
	}
	dfs.SaveAttrs(true)

	// create a cfs from them
	fs, err := New("cfs", cachetrfs, dfs, RW)
	if err != nil {
		dbg.Fatal("cfs: %s", err)
	}
	// perhaps set debug for it
	fs.Flags.Dbg = true

	// and now use it.
}

func mkmdfslfs(t *testing.T) *setup {
Debug = true
	os.Args[0] = "cfs_test"
	os.RemoveAll(tdir)
	os.RemoveAll(tdir2)
	if err := os.Mkdir(tdir, 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	if err := os.Mkdir(tdir2, 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}

	c := make(chan string)
	s := &setup{}
	s.trz = Trace(c)

	/* MDFS + LFS
	*/
	lfs1, err := lfs.New("lfs1", tdir2, lfs.RW)
	if err != nil {
		os.RemoveAll(tdir)
		os.RemoveAll(tdir2)
		t.Fatalf("lfs1: %s", err)
	}
	lfs1.SaveAttrs(true)
	lfs1.WstatAll = true

	lfs1.IOstats = &zx.IOstats{}
	fs1, err := mdfs.New("fs1", lfs1)
	if err != nil {
		os.RemoveAll(tdir)
		os.RemoveAll(tdir2)
		t.Fatalf("fs1: %s", err)
	}
	fs1.WstatAll = true
	tr1 := trfs.New(fs1)
	tr1.Tag ="fs1"
	tr1.C = c
	fs2, err := lfs.New("fs2", tdir, lfs.RW)
	if err != nil {
		os.RemoveAll(tdir)
		os.RemoveAll(tdir2)
		t.Fatalf("fs2: %s", err)
	}
	fs2.SaveAttrs(true)
	fs2.IOstats = &zx.IOstats{}
	tr2 := trfs.New(fs2)
	tr2.Tag ="fs2"
	tr2.C = c
	if testing.Verbose() {
		defer fs2.Dump(os.Stdout)
		defer fs1.Dump(os.Stdout)
	}

	fs, err := New("cfs", tr1, tr2, RW)
	if err != nil {
		os.RemoveAll(tdir)
		os.RemoveAll(tdir2)
		t.Fatalf("cfs: %s", err)
	}
	fs.IOstats = &zx.IOstats{}
	trcfs := trfs.New(fs)
	trcfs.Tag ="cfs"
	trcfs.C = c
	ai := &auth.Info{Uid: dbg.Usr, SpeaksFor: dbg.Usr, Ok: true}
	afs, _:= trcfs.AuthFor(ai)
	if testing.Verbose() {
		s.deferred = func() {
			fs1.Dump(os.Stdout)
			fs2.Dump(os.Stdout)
			afs.Close(nil)
			close(c)
			s.trz.Wait()
			if moreverb {
				printf("calls:\n")
			//	s.trz.All().WriteTo(os.Stdout)
			//	s.trz.Calls().WriteTo(os.Stdout)
				s.trz.Calls().Deps().WriteTo(os.Stdout)
			//	s.trz.Calls().Deps().WriteRfsTo(os.Stdout)
			}
			os.RemoveAll(tdir)
			os.RemoveAll(tdir2)
			fs.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", afs, fs.IOstats)
			fs1.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", fs1, fs1.IOstats)
			fs2.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", fs2, fs2.IOstats)
		}
	} else {
		s.deferred = func() {
			afs.Close(nil)
			close(c)
			os.RemoveAll(tdir)
			os.RemoveAll(tdir2)
		}
	}
	s.fs1 = tr1
	s.fs2 = tr2
	s.syncer = fs
	s.cfs = afs.(zx.RWTree)
	s.debug = func() {
		s.trz.All()
		fs.IOstats.Clear()
		fs1.IOstats.Clear()
		fs2.IOstats.Clear()
		fs.Flags.Dbg = testing.Verbose()
		fs1.Dbg = testing.Verbose()
		fs2.Dbg = testing.Verbose()
	}
	return s
}

func mklfslfs(t *testing.T) *setup {
	os.Args[0] = "cfs_test"
	os.RemoveAll(tdir)
	os.RemoveAll(tdir2)
	if err := os.Mkdir(tdir, 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	if err := os.Mkdir(tdir2, 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}

	c := make(chan string)
	s := &setup{}
	s.trz = Trace(c)

	/* LFS + LFS
	*/
	fs1, err := lfs.New("fs1", tdir2, lfs.RW)
	if err != nil {
		os.RemoveAll(tdir)
		os.RemoveAll(tdir2)
		t.Fatalf("fs1: %s", err)
	}
	fs1.SaveAttrs(true)
	fs1.WstatAll = true


	fs1.IOstats = &zx.IOstats{}
	tr1 := trfs.New(fs1)
	tr1.Tag ="fs1"
	tr1.C = c
	fs2, err := lfs.New("fs2", tdir, lfs.RW)
	if err != nil {
		os.RemoveAll(tdir)
		os.RemoveAll(tdir2)
		t.Fatalf("fs2: %s", err)
	}
	fs2.SaveAttrs(true)
	fs2.IOstats = &zx.IOstats{}
	tr2 := trfs.New(fs2)
	tr2.Tag ="fs2"
	tr2.C = c
	if testing.Verbose() {
		defer fs2.Dump(os.Stdout)
		defer fs1.Dump(os.Stdout)
	}

	fs, err := New("cfs", tr1, tr2, RW)
	if err != nil {
		os.RemoveAll(tdir)
		os.RemoveAll(tdir2)
		t.Fatalf("cfs: %s", err)
	}
	fs.IOstats = &zx.IOstats{}
	trcfs := trfs.New(fs)
	trcfs.Tag ="cfs"
	trcfs.C = c
	ai := &auth.Info{Uid: dbg.Usr, SpeaksFor: dbg.Usr, Ok: true}
	afs, _:= trcfs.AuthFor(ai)
	if testing.Verbose() {
		s.deferred = func() {
			fs1.Dump(os.Stdout)
			fs2.Dump(os.Stdout)
			afs.Close(nil)
			close(c)
			s.trz.Wait()
			if moreverb {
				printf("calls:\n")
			//	s.trz.All().WriteTo(os.Stdout)
			//	s.trz.Calls().WriteTo(os.Stdout)
				s.trz.Calls().Deps().WriteTo(os.Stdout)
			//	s.trz.Calls().Deps().WriteRfsTo(os.Stdout)
			}
			os.RemoveAll(tdir)
			os.RemoveAll(tdir2)
			fs.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", afs, fs.IOstats)
			fs1.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", fs1, fs1.IOstats)
			fs2.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", fs2, fs2.IOstats)
		}
	} else {
		s.deferred = func() {
			afs.Close(nil)
			close(c)
			os.RemoveAll(tdir)
			os.RemoveAll(tdir2)
		}
	}
	s.fs1 = tr1
	s.fs2 = tr2
	s.syncer = fs
	s.cfs = afs.(zx.RWTree)
	s.debug = func() {
		s.trz.All()
		fs.IOstats.Clear()
		fs1.IOstats.Clear()
		fs2.IOstats.Clear()
		fs.Flags.Dbg = testing.Verbose()
		fs1.Dbg = testing.Verbose()
		fs2.Dbg = testing.Verbose()
	}
	return s
}

func mkmfslfs(t *testing.T) *setup {
	os.Args[0] = "cfs_test"
	os.RemoveAll(tdir)
	os.RemoveAll(tdir2)
	if err := os.Mkdir(tdir, 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	if err := os.Mkdir(tdir2, 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}

	c := make(chan string)
	s := &setup{}
	s.trz = Trace(c)

	/* MFS  + LFS
	*/
	fs1, err := mfs.New("fs1")
	if err != nil {
		os.RemoveAll(tdir)
		os.RemoveAll(tdir2)
		t.Fatalf("fs1: %s", err)
	}

	fs1.WstatAll = true
	fs1.IOstats = &zx.IOstats{}
	tr1 := trfs.New(fs1)
	tr1.Tag ="fs1"
	tr1.C = c
	fs2, err := lfs.New("fs2", tdir, lfs.RW)
	if err != nil {
		os.RemoveAll(tdir)
		os.RemoveAll(tdir2)
		t.Fatalf("fs2: %s", err)
	}
	fs2.SaveAttrs(true)
	fs2.IOstats = &zx.IOstats{}
	tr2 := trfs.New(fs2)
	tr2.Tag ="fs2"
	tr2.C = c
	if testing.Verbose() {
		defer fs2.Dump(os.Stdout)
		defer fs1.Dump(os.Stdout)
	}

	fs, err := New("cfs", tr1, tr2, RW)
	if err != nil {
		os.RemoveAll(tdir)
		os.RemoveAll(tdir2)
		t.Fatalf("cfs: %s", err)
	}
	fs.IOstats = &zx.IOstats{}
	trcfs := trfs.New(fs)
	trcfs.Tag ="cfs"
	trcfs.C = c
	ai := &auth.Info{Uid: dbg.Usr, SpeaksFor: dbg.Usr, Ok: true}
	afs, _:= trcfs.AuthFor(ai)
	if testing.Verbose() {
		s.deferred = func() {
			fs1.Dump(os.Stdout)
			fs2.Dump(os.Stdout)
			afs.Close(nil)
			close(c)
			s.trz.Wait()
			if moreverb {
				printf("calls:\n")
			//	s.trz.All().WriteTo(os.Stdout)
			//	s.trz.Calls().WriteTo(os.Stdout)
				s.trz.Calls().Deps().WriteTo(os.Stdout)
			//	s.trz.Calls().Deps().WriteRfsTo(os.Stdout)
			}
			os.RemoveAll(tdir)
			os.RemoveAll(tdir2)
			fs.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", afs, fs.IOstats)
			fs1.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", fs1, fs1.IOstats)
			fs2.IOstats.Averages()
			printf("\n\n%s iostats:\n%s", fs2, fs2.IOstats)
		}
	} else {
		s.deferred = func() {
			afs.Close(nil)
			close(c)
			os.RemoveAll(tdir)
			os.RemoveAll(tdir2)
		}
	}
	s.fs1 = tr1
	s.fs2 = tr2
	s.syncer = fs
	s.cfs = afs.(zx.RWTree)
	s.debug = func() {
		s.trz.All()
		fs.IOstats.Clear()
		fs1.IOstats.Clear()
		fs2.IOstats.Clear()
		fs.Flags.Dbg = testing.Verbose()
		fs1.Dbg = testing.Verbose()
		fs2.Dbg = testing.Verbose()
	}
	return s
}

func TestDeadlocks(t *testing.T) {
	lk := &lockTrzs{}
	fn := func() {
		defer lk.NoLocks()
		lk.Locking("a", 0)
		lk.Locking("b", 0)
		lk.Unlocking("a")
		lk.Unlocking("b")
	}
	fn()

}

func TestNewClose(t *testing.T) {
	Debug = testing.Verbose()
	s := mkfs(t)
	defer s.deferred()
}

func TestOneMkdir(t *testing.T) {
	s := mkfs(t)
	defer s.deferred()

	s.debug()
	err := <-s.cfs.Mkdir("/blah", zx.Dir{"mode": "0744"})
	if err != nil {
		t.Fatalf("%s", err)
	}
	s.syncer.Sync()
}

func TestInitDirs(t *testing.T) {
	s := mkfs(t)
	defer s.deferred()

	s.debug()
	for _, dn := range fstest.Dirs {
		if err := zx.MkdirAll(s.cfs, dn, zx.Dir{"mode":"0755"}); err != nil {
			t.Fatalf("mkdir: %s", err)
		}
	}
	s.syncer.Sync()
}

func testfn(t *testing.T, fn func(t fstest.Fataler, fss ...zx.Tree)) {
	x := fstest.Repeats
	defer func() {
		fstest.Repeats = x
	}()
	fstest.Repeats = 1
	s := mkfs(t)
	defer s.deferred()
	if fn == nil {
		s.debug()
	}
	fstest.MkZXTree(t, s.cfs)
	s.syncer.Sync()
	if fn != nil {
		s.debug()
		fn(t, s.cfs)
		s.syncer.Sync()
	}
	fstest.SameDump(t, s.cfs, s.fs1, s.fs2)
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
	testfn(t, fstest.UsrWstats)
}

func TestMoves(t *testing.T) {
	testfn(t, fstest.Moves)
}

func TestCtl(t *testing.T) {
	x := fstest.Repeats
	defer func() {
		fstest.Repeats = x
	}()
	fstest.Repeats = 1
	s := mkfs(t)
	defer s.deferred()

	fstest.MkZXTree(t, s.cfs)
	s.syncer.Sync()
	fstest.Stats(t, s.cfs)
	s.syncer.Sync()
	fstest.GetCtl(t, s.cfs)
	fstest.PutCtl(t, s.cfs)
	s.debug()
	zx.PutAll(s.cfs, "/atleastaput", zx.Dir{"mode": "0664"}, []byte("hi there"))
	dat, err := zx.GetAll(s.cfs, "/Ctl")
	if err != nil {
		t.Fatalf("ctl %s", err)
	}
	ctl := string(dat)
	printf("ctl is:\n%s-------\n", string(dat))
	if !strings.Contains(ctl, `cfs:`) {
		t.Fatalf("ctl does not include cfs")
	}
	if !strings.Contains(ctl, `fs1:`) {
		t.Fatalf("ctl does not include fs1")
	}
	if !strings.Contains(ctl, `fs2:`) {
		t.Fatalf("ctl does not include fs2")
	}

}

func TestAsAFile(t *testing.T) {
	testfn(t, fstest.AsAFile)
}

func TestRaces(t *testing.T) {
	t.Skip("only by hand")
	testfn(t, fstest.Races)
}

func TestNewPerms(t *testing.T) {
	testfn(t, fstest.NewPerms)
}

func TestRWXPerms(t *testing.T) {
	testfn(t, fstest.RWXPerms)
}

func TestLfsChanges(t *testing.T) {
	t.Skip("only by hand")
	x := fstest.Repeats
	defer func() {
		fstest.Repeats = x
	}()
	fstest.Repeats = 1
	s := mkfs(t)
	defer s.deferred()

	// make the tree and push to fs2
	fstest.MkZXTree(t, s.cfs)
	s.syncer.Sync()

	syncfn := func() {
		printf("syncing...\n")
		// push changes
		s.syncer.Sync()
		// await until it's all invalid
		time.Sleep(CacheTout)
	}

	s.debug()
	r := fstest.NewRepl(t, syncfn, s.cfs, s.fs1, s.fs2)
	r.LfsChanges()
}

func TestLfsToutChanges(t *testing.T) {
	t.Skip("only by hand")
	x := fstest.Repeats
	defer func() {
		fstest.Repeats = x
	}()
	fstest.Repeats = 1
	s := mkfs(t)
	defer s.deferred()

	// make the tree and push to fs2
	fstest.MkZXTree(t, s.cfs)
	s.syncer.Sync()

	syncfn := func() {
		printf("syncing...\n")
		// await until it's all invalid, no sync call
		time.Sleep(cache.MaxSyncDelay)
	}

	s.debug()
	r := fstest.NewRepl(t, syncfn, s.cfs, s.fs1, s.fs2)
	r.LfsChanges()
}

func TestRfsChanges(t *testing.T) {
	PollIval = 2*CacheTout
	t.Skip("only by hand")
	x := fstest.Repeats
	defer func() {
		fstest.Repeats = x
	}()
	fstest.Repeats = 1
	s := mkfs(t)
	defer s.deferred()

	// make the tree and push to fs2
	fstest.MkZXTree(t, s.cfs)
	s.syncer.Sync()

	syncfn := func() {
		printf("syncing...\n")
		// push changes
		s.syncer.Sync()
		// await until it's all invalid
		time.Sleep(3*CacheTout)
	}

	s.debug()
	r := fstest.NewRepl(t, syncfn, s.cfs, s.fs1, s.fs2)
	r.RfsChanges()
}

func TestLfsRfsChanges(t *testing.T) {
	PollIval = 2*CacheTout
	t.Skip("only by hand")
	x := fstest.Repeats
	defer func() {
		fstest.Repeats = x
	}()
	fstest.Repeats = 1
	s := mkfs(t)
	defer s.deferred()

	// make the tree and push to fs2
	fstest.MkZXTree(t, s.cfs)
	s.syncer.Sync()

	syncfn := func() {
		printf("syncing...\n")
		// push changes
		s.syncer.Sync()
		// await until it's all invalid
		time.Sleep(3*CacheTout)
	}

	s.debug()
	r := fstest.NewRepl(t, syncfn, s.cfs, s.fs1, s.fs2)
	r.LfsRfsChanges()
}

func TestSameChanges(t *testing.T) {
	PollIval = 2*CacheTout
	t.Skip("only by hand")
	x := fstest.Repeats
	defer func() {
		fstest.Repeats = x
	}()
	fstest.Repeats = 1
	s := mkfs(t)
	defer s.deferred()

	// make the tree and push to fs2
	fstest.MkZXTree(t, s.cfs)
	s.syncer.Sync()

	syncfn := func() {
		printf("syncing...\n")
		// push changes
		s.syncer.Sync()
		// await until it's all invalid
		time.Sleep(3*CacheTout)
	}

	s.debug()
	r := fstest.NewRepl(t, syncfn, s.cfs, s.fs1, s.fs2)
	r.SameChanges()
}

func TestFs2Changes(t *testing.T) {
	PollIval = 2*CacheTout
	t.Skip("only by hand")
	x := fstest.Repeats
	defer func() {
		fstest.Repeats = x
	}()
	fstest.Repeats = 1
	s := mkfs(t)
	defer s.deferred()

	// make the tree and push to fs2
	fstest.MkZXTree(t, s.cfs)
	s.fs1.(zx.Dumper).Dump(os.Stderr)
	s.syncer.Sync()

	fstest.SameDump(t, s.cfs, s.fs1, s.fs2)

	var obuf, nbuf bytes.Buffer
	s.fs1.(zx.Dumper).Dump(&obuf)

	s.debug()
	printf("changing fs2\n")
	// make external changes to fs2
	fstest.MkChgs(t, tdir)
	printf("changed\n")

	// check out that fs1 remains as it was before invalidations
	fstest.ReadAll(t, s.cfs)
	s.fs1.(zx.Dumper).Dump(&nbuf)
	if obuf.String() != nbuf.String() {
		t.Logf("old %s", obuf)
		t.Logf("new %s", nbuf)
		t.Fatalf("did fetch changes before inval")
	}

	// wait until all metadata gets stale
	time.Sleep(3*CacheTout)
	fstest.ReadAll(t, s.cfs)

	s.fs1.(zx.Dumper).Dump(os.Stdout)
	s.fs2.(zx.Dumper).Dump(os.Stdout)

	// check that now they are the same
	fstest.SameDump(t, s.cfs, s.fs1, s.fs2)


	// make more changes
	printf("changing2 fs2\n")
	fstest.MkChgs2(t, tdir)
	printf("changed\n")

	// wait until all metadata gets stale
	s.debug()
	time.Sleep(3*CacheTout)
	fstest.ReadAll(t, s.cfs)

	// check that now they are the same
	fstest.SameDump(t, s.cfs, s.fs1, s.fs2)
}

/*
	Inval protocol testing:
	create a mfs+lfs setup like in the std. tests
	create a mfs+rfs setup for a first
	create a mfs+rfs setup for a second client

	- make changes at the first client and see what happens

	- make changes at the second client and see what happens

	- make external changes to the lfs and see what happens
*/

type invalsetup {
	cache1, cache2, cache *mfs.Fs
	cfs1, cfs2, cfs *Cfs
	lfs *lfs.Lfs
}

func (s *invalsetup) Dump(w io.Writer) {
	s.cache1.Dump(w)
	s.cache2.Dump(w)
	s.cache.Dump(w)
	s.lfs.Dump(w)
}

func mkinval(t *testing.T) *invalsetup {
	os.RemoveAll(tdir)
	os.Args[0] = "cfs_test"
	if err := os.Mkdir(tdir, 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	fstest.ResetTime()
	fstest.MkTree(t, tdir)
	cache, err := mfs.New("mfs")
	if err != nil {
		t.Fatalf("cache: %s", err)
	}
	cache.WstatAll = true
	sfs, err := lfs.New("sfs", tdir, lfs.RW)
	if err != nil {
		t.Fatalf("sfs: %s", err)
	}
	sfs.SaveAttrs(true)

	cfs, err:= New("cfs", cache, sfs, RW)
	if err != nil {
		t.Fatalf("cfs: %s", err)
	}
	cfs.Flags.Dbg = testing.Verbose()

	ci1 := &zx.ClientInfo{
		Ai: &auth.Info{Uid: dbg.Usr, SpeaksFor: dbg.Usr, Ok: true},
		Tag: "cli1",
		Id: 1,
	}
	clicfs1, err := cfs.ServerFor(ci1)
	if err != nil {
		t.Fatalf("clicfs1 %s", err)
	}
	cache1, err := mfs.New("mfs1")
	if err != nil {
		dbg.Fatal("mfs1: %s", err)
	}
	cache1.WstatAll = true	// cfs does this
	cfs1, err := New("cfs1", cache1, clicfs1, RW)
	if err != nil {
		dbg.Fatal("cfs1: %s", err)
	}
	cfs1.Flags.Dbg = testing.Verbose()
	acfs1, _:= cfs1.AuthFor(ci1.Ai)

	ci2 := &zx.ClientInfo{
		Ai: &auth.Info{Uid: dbg.Usr, SpeaksFor: dbg.Usr, Ok: true},
		Tag: "cli2",
		Id: 2,
	}
	clicfs2, err := cfs.ServerFor(ci2)
	if err != nil {
		t.Fatalf("clicfs2 %s", err)
	}
	cache2, err := mfs.New("mfs2")
	if err != nil {
		dbg.Fatal("mfs2: %s", err)
	}
	cache2.WstatAll = true	// cfs does this
	cfs2, err := New("cfs2", cache2, clicfs2, RW)
	if err != nil {
		dbg.Fatal("cfs2: %s", err)
	}
	cfs2.Flags.Dbg = testing.Verbose()
	acfs2, _:= cfs2.AuthFor(ci2.Ai)

	return &invalsetup {
		cache1: cache1,
		cache2: cache2,
		cache: cache,
		cfs1: acfs1.(*Cfs),
		cfs2: acfs2.(*Cfs),
		cfs: cfs,
		lfs: sfs,
	}
}

var emptydump =`tree [mfs1] path 1
path / name / type d mode 0755 size 0

tree [mfs2] path 2
path / name / type d mode 0755 size 0

tree [mfs] path 0
path / name / type d mode 0755 size 0

tree [sfs] path /tmp/ncfs_test
path / name / type d mode 0755 size 0
    path /1 name 1 type - mode 0644 size 0
      0 bytes
    path /2 name 2 type - mode 0644 size 31658
      31658 bytes
    path /a name a type d mode 0755 size 3
        path /a/a1 name a1 type - mode 0644 size 10154
          10154 bytes
        path /a/a2 name a2 type - mode 0644 size 21418
          21418 bytes
        path /a/b name b type d mode 0755 size 1
            path /a/b/c name c type d mode 0755 size 1
                path /a/b/c/c3 name c3 type - mode 0644 size 44970
                  44970 bytes
    path /d name d type d mode 0755 size 0
    path /e name e type d mode 0755 size 1
        path /e/f name f type d mode 0755 size 0

`
func TestInvalInit(t *testing.T) {
	t.Skip("only by hand")
	PollIval = 2*CacheTout
	s := mkinval(t)
	defer os.RemoveAll(tdir)
	if testing.Verbose() {
		s.Dump(os.Stdout)
	}
	var buf bytes.Buffer
	s.Dump(&buf)
	out := strings.Split(buf.String(), "\n")
	ex := strings.Split(emptydump, "\n")
	printf("%d lines'n", len(out))
	if len(out) != len(ex) {
		t.Fatalf("trees do not match")
	}
	for i := 0; i < len(out); i++ {
		if strings.HasPrefix(out[i], "tree ") {
			out[i]= ""
		}
		if strings.HasPrefix(ex[i], "tree ") {
			ex[i]= ""
		}
	}
	if strings.Join(out, "\n") != strings.Join(ex, "\n") {
		t.Fatalf("trees do not match")
	}
}

func TestInvalChanges(t *testing.T) {
	t.Skip("only by hand")
	PollIval = 2*CacheTout
	s := mkinval(t)
	defer os.RemoveAll(tdir)
	printf("\ngetdir /a/b\n")
	ds, err := zx.GetDir(s.cfs1, "/a/b")
	if err != nil {
		t.Fatalf("getdir: %s", err)
	}
	if len(ds) != 1 {
		t.Fatalf("getdir: bad len")
	}
	if testing.Verbose() {
		s.Dump(os.Stdout)
	}
	for i := 0; i < 2; i++ {
		printf("\nget /a/a1\n")
		dat, err := zx.GetAll(s.cfs1, "/a/a1")
		if err != nil {
			t.Fatalf("getall: %s", err)
		}
		if bytes.Compare(dat, fstest.FileData["/a/a1"]) != 0 {
			t.Fatalf("bad file data")
		}
	}
	printf("\nput /a/n2\n")
	err = zx.PutAll(s.cfs1, "/a/n2", zx.Dir{"mode":"0644"}, fstest.FileData["/a/a1"])
	if err != nil {
		t.Fatalf("put: %s", err)
	}
	s.cfs1.Sync()
	time.Sleep(3*CacheTout)

	for i := 0; i < 2; i++ {
		printf("\nget2 /a/n2\n")
		dat2, err := zx.GetAll(s.cfs2, "/a/n2")
		if err != nil {
			t.Fatalf("get2: %s", err)
		}
		if bytes.Compare(dat2, fstest.FileData["/a/a1"]) != 0 {
			t.Fatalf("bad file data")
		}
	}

	// chmod /a/n2
	printf("\nwstat2 /a/n2\n")
	err = <-s.cfs2.Wstat("/a/n2", zx.Dir{"Foo": "Bar"})
	if err != nil {
		t.Fatalf("put: %s", err)
	}
	s.cfs2.Sync()
	time.Sleep(3*CacheTout)
	printf("\nstat /a/n2\n")
	d1, err1 := zx.Stat(s.cfs1, "/a/n2")
	if err1 != nil {
		t.Fatalf("stat: %s", err1)
	}
	printf("\nstat2 /a/n2\n")
	d2, err2 := zx.Stat(s.cfs2, "/a/n2")
	if err2 != nil {
		t.Fatalf("stat: %s", err2)
	}
	s1 := d1.LongTestFmt()
	s2 := d2.LongTestFmt()
	printf("d1 %s\n", d1)
	printf("d2 %s\n", d2)
	if s1 != s2 {
		t.Fatalf("stats do not match")
	}

	printf("\nrm -r /e\n")
	err = <-s.cfs.RemoveAll("/e")
	if err != nil {
		t.Fatalf("rm: %s", err)
	}
	time.Sleep(3*CacheTout)
	_, err = zx.Stat(s.cfs2, "/e")
	if err == nil {
		t.Fatalf("didn't remove")
	}

	s.cfs.Sync()
	s.cfs1.Sync()
	s.cfs2.Sync()
	if testing.Verbose() {
		s.Dump(os.Stdout)
	}
}

