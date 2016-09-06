package repl

import (
	"clive/cmd"
	"clive/dbg"
	"clive/net/auth"
	"clive/zx/zux"
	"clive/zx/rzx"
	"clive/zx"
	"clive/u"
	"clive/zx/fstest"
	"os"
	"testing"
	"bytes"
)

const (
	tdir = "/tmp/repl_test"
	rtdir = "/tmp/repl_testrzx"
	tdb = "/tmp/repl_test.db"
)

var (
	ai   = &auth.Info{Uid: u.Uid, SpeaksFor: u.Uid, Ok: true}
	debug bool
	dprintf =  dbg.FlagPrintf(&debug)
)

func mkdb(t *testing.T, tdir string, excl ...string) *DB {
	debug = testing.Verbose()
	db, err := NewDB("adb", tdir, excl...)
	if err != nil {
		t.Fatalf("failed %v", err)
	}
	db.Debug = testing.Verbose()
	dprintf("db is %s\n", db)
	err = db.Scan()
	if err != nil {
		t.Fatalf("failed %v", err)
	}
	if testing.Verbose() {
		db.DumpTo(os.Stderr)
	}
	return db
}

func mktest(t *testing.T, tdir string, excl ...string) (*DB, func() ) {
	cmd.UnixIO("in", "out", "err")
	cmd.Warn("testing")
	os.Args[0] = "repl.test"
	fstest.Verb = testing.Verbose()
	os.RemoveAll(tdir)
	os.Remove(tdb)
	fstest.MkTree(t, tdir)
	fn := func() {
		os.RemoveAll(tdir)
		os.RemoveAll(tdb)
	}
	db := mkdb(t, tdir, excl...)
	return db, fn
}

func mkrtest(t *testing.T, rtdir string, excl ...string) (*DB, func() ) {
	cmd.UnixIO("in", "out", "err")
	os.Args[0] = "repl.test"
	os.Mkdir(rtdir+"/p", 0755)
	fstest.Verb = testing.Verbose()
	fstest.MkTree(t, rtdir+"/p")
	os.Remove("/tmp/clive.9898")
	fs, err := zux.NewZX(rtdir)
	if err != nil {
		os.RemoveAll(rtdir)
		os.Remove("/tmp/clive.9898")
		t.Fatal(err)
	}
	srv, err := rzx.NewServer("unix!local!9898", auth.TLSserver)
	if err != nil {
		os.RemoveAll(rtdir)
		os.Remove("/tmp/clive.9898")
		t.Fatal(err)
	}
	if err := srv.Serve("main", fs); err != nil {
		os.RemoveAll(rtdir)
		os.Remove("/tmp/clive.9898")
		t.Fatal(err)
	}
	db := mkdb(t, "unix!local!9898!/p", excl...)
	fn := func() {
		db.Close()
		os.RemoveAll(rtdir)
		os.Remove("/tmp/clive.9898")
		srv.Close()
	}

	return db, fn
}

func chkFiles(t *testing.T, db *DB, files []string, list string) {
	dprintf("chk %s files at %s:\n", db, db.Addr)
	all := map[string]bool{}
	for _, f := range files {
		all[f] = true
	}
	fc := db.Files()
	s := ""
	for f := range fc {
		dprintf("%s\n", f)
		if f.D["rm"] != "" {
			continue
		}
		s += f.D.Fmt() + "\n"
		if files != nil && !all[f.D["path"]] {
			t.Fatalf("bad file %s", f.D.Fmt());
		}
		delete(all, f.D["path"])
	}
	if files != nil && len(all) > 0 {
		t.Fatalf("don't have %s", all)
	}
	dprintf("%s files ok\n\n", db)
	if list != "" && s != list {
		t.Fatalf("bad files listing")
	}
}

func TestDbScan(t *testing.T) {
	db, fn := mktest(t, tdir)
	defer fn()
	chkFiles(t, db, fstest.AllFiles, fstest.AllFilesList)
}

func TestRzxDbScan(t *testing.T) {
	db, fn := mkrtest(t, rtdir)
	defer fn()
	chkFiles(t, db, fstest.AllFiles, fstest.AllFilesList)
}

var xfiles = `d rwxr-xr-x      0 /
- rw-r--r--      0 /1
- rw-r--r--  30.9k /2
d rwxr-xr-x      0 /a
d rwxr-xr-x      0 /a/b
d rwxr-xr-x      0 /a/b/c
- rw-r--r--  43.9k /a/b/c/c3
d rwxr-xr-x      0 /d
d rwxr-xr-x      0 /e
d rwxr-xr-x      0 /e/f
`

func TestDbExcl(t *testing.T) {
	db, fn := mktest(t, tdir, "/*/a1", "a?")
	defer fn()
	chkFiles(t, db, nil, xfiles)
}

func TestDBFile(t *testing.T) {
	db, fn := mktest(t, tdir)
	defer fn()
	if err := db.Save(tdb); err != nil {
		t.Fatalf("saving: %s", err)
	}
	dprintf("loading\n")
	ndb, err := LoadDB(tdb)
	if err != nil {
		t.Fatalf("reading: %s", err)
	}
	chkFiles(t, ndb, fstest.AllFiles, fstest.AllFilesList)
	var b1, b2 bytes.Buffer

	db.DumpTo(&b1)
	ndb.DumpTo(&b2)
	if b1.String() != b2.String() {
		t.Fatalf("dbs do not match")
	}
}

var chg1 = []Chg {
	Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/a/a1"}}},
	Chg{Chg: zx.Chg{Type: zx.Meta, D: zx.Dir{"path": "/a/a2"}}},
	Chg{Chg: zx.Chg{Type: zx.Del, D: zx.Dir{"path": "/a/b/c"}}},
	Chg{Chg: zx.Chg{Type: zx.Add, D: zx.Dir{"path": "/a/n"}}},
}

var chg2 = []Chg {
	Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/1"}}},
	Chg{Chg: zx.Chg{Type: zx.DirFile, D: zx.Dir{"path": "/2"}}},
}

var chg3 = []Chg {
	Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/1"}}},
	Chg{Chg: zx.Chg{Type: zx.DirFile, D: zx.Dir{"path": "/2"}}},
	Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/a/a1"}}},
	Chg{Chg: zx.Chg{Type: zx.Meta, D: zx.Dir{"path": "/a/a2"}}},
	Chg{Chg: zx.Chg{Type: zx.Del, D: zx.Dir{"path": "/a/b/c"}}},
	Chg{Chg: zx.Chg{Type: zx.Add, D: zx.Dir{"path": "/a/n"}}},
}

func cmpChgs(t *testing.T, xc []Chg, gc []Chg) {
	for i := 0; i < len(xc) && i < len(gc); i++ {
		if xc[i].Type != gc[i].Type || xc[i].D["path"] != gc[i].D["path"] {
			t.Fatalf("bad chg %s", gc[i])
		}
		if xc[i].At != Nowhere {
			if xc[i].At != gc[i].At {
				t.Logf("expected %s", xc[i])
				t.Fatalf("bad chg %s", gc[i])
			}
		}
	}
	if len(xc) != len(gc) {
		t.Fatalf("expected %d changes but got %d", len(xc), len(gc))
	}
}

func TestCmp(t *testing.T) {
	db, fn := mktest(t, tdir)
	defer fn()
	fstest.MkChgs(t, tdir)
	dprintf("db after changes\n")
	ndb := mkdb(t, tdir)
	chkFiles(t, ndb, fstest.AllChgFiles, fstest.AllChgFilesList)

	cc := ndb.ChangesFrom(db)
	chgs := []Chg{}
	for c := range cc {
		chgs = append(chgs, c)
		dprintf("chg %s\n", c)
	}
	cmpChgs(t, chg1, chgs)
	fstest.MkChgs2(t, tdir)
	dprintf("\ndb after changes2\n")
	ndb2 := mkdb(t, tdir)
	chkFiles(t, ndb2, fstest.AllChg2Files, fstest.AllChg2FilesList)

	dprintf("\nchanges...\n")
	chgs = []Chg{}
	cc = ndb2.ChangesFrom(ndb)
	for c := range cc {
		chgs = append(chgs, c)
		dprintf("chg %s\n", c)
	}
	cmpChgs(t, chg2, chgs)
	dprintf("all chgs\n")
	chgs = []Chg{}
	cc = ndb2.ChangesFrom(db)
	for c := range cc {
		chgs = append(chgs, c)
		dprintf("chg %s\n", c)
	}
	cmpChgs(t, chg3, chgs)
}

func TestRzxCmp(t *testing.T) {
	db, fn := mkrtest(t, rtdir)
	defer fn()
	fstest.MkChgs(t, rtdir+"/p")
	dprintf("db after changes\n")
	ndb := mkdb(t, "unix!local!9898!/p")
	chkFiles(t, ndb, fstest.AllChgFiles, fstest.AllChgFilesList)
	defer ndb.Close()
	cc := ndb.ChangesFrom(db)
	chgs := []Chg{}
	for c := range cc {
		chgs = append(chgs, c)
		dprintf("chg %s\n", c)
	}
	cmpChgs(t, chg1, chgs)
	fstest.MkChgs2(t, rtdir+"/p")
	dprintf("\ndb after changes2\n")
	ndb2 := mkdb(t, "unix!local!9898!/p")
	chkFiles(t, ndb2, fstest.AllChg2Files, fstest.AllChg2FilesList)
	defer ndb2.Close()
	chgs = []Chg{}
	cc = ndb2.ChangesFrom(ndb)
	for c := range cc {
		chgs = append(chgs, c)
		dprintf("chg %s\n", c)
	}
	cmpChgs(t, chg2, chgs)
	dprintf("all chgs\n")
	chgs = []Chg{}
	cc = ndb2.ChangesFrom(db)
	for c := range cc {
		chgs = append(chgs, c)
		dprintf("chg %s\n", c)
	}
	cmpChgs(t, chg3, chgs)
}

var schg = []Chg {
	Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/1"}}, At: Remote},
	Chg{Chg: zx.Chg{Type: zx.DirFile, D: zx.Dir{"path": "/2"}}, At: Remote},
	Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/a/a1"}}, At: Local},
	Chg{Chg: zx.Chg{Type: zx.Meta, D: zx.Dir{"path": "/a/a2"}}, At: Local},
	Chg{Chg: zx.Chg{Type: zx.Del, D: zx.Dir{"path": "/a/b/c"}}, At: Local},
	Chg{Chg: zx.Chg{Type: zx.Add, D: zx.Dir{"path": "/a/n"}}, At: Local},
}


func TestTreeChanges(t *testing.T) {
	db, fn := mkrtest(t, rtdir)
	defer fn()
	db.Close()
	db, fn2 := mktest(t, tdir)
	defer fn2()
	db.Close()

	tr, err := New("adb", tdir, "unix!local!9898!/p")
	if err != nil {
		t.Fatalf("tree %s", err)
	}
	tr.Debug = testing.Verbose()
	defer tr.Close()
	fstest.MkChgs(t, tdir)
	fstest.MkChgs2(t, rtdir+"/p")
	dprintf("pull\n")
	cc, err := tr.PullChanges()
	if err != nil {
		t.Fatalf("pull %s", err)
	}
	for c := range cc {
		dprintf("chg %s %s\n", c.At, c)
	}
	dprintf("push\n")
	cc, err = tr.PushChanges()
	if err != nil {
		t.Fatalf("push %s", err)
	}
	for c := range cc {
		dprintf("chg %s %s\n", c.At, c)
	}
	dprintf("sync\n")
	cc, err = tr.Changes()
	if err != nil {
		t.Fatalf("sync %s", err)
	}
	chgs := []Chg{}
	for c := range cc {
		chgs = append(chgs, c)
		dprintf("chg %s %s\n", c.At, c)
	}
	cmpChgs(t, schg, chgs)
}

func getChgs() (chan Chg, chan []Chg) {
	cc := make(chan Chg, 10)
	dc := make(chan []Chg, 1)
	go func() {
		cs := []Chg{}
		for c := range cc {
			cs = append(cs, c)
		}
		dc <- cs
		close(dc)
	}()
	return cc, dc
}

func logChgs(cs []Chg) {
	for _, c := range cs {
		dprintf("%s-> %s\n", c.At, c)
	}
}

var (
	pushcs = []Chg {
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/a/a1"}},
			At: Local},
		Chg{Chg: zx.Chg{Type: zx.Meta, D: zx.Dir{"path": "/a/a2"}},
			At: Local},
		Chg{Chg: zx.Chg{Type: zx.Del, D: zx.Dir{"path": "/a/b/c"}},
			At: Local},
		Chg{Chg: zx.Chg{Type: zx.Add, D: zx.Dir{"path": "/a/n"}},
			At: Local},
	}

	// MkChgs w/o MkChgs2
	pushldb = `d rwxr-xr-x      0 /
- rw-r--r--      0 /1
- rw-r--r--  30.9k /2
d rwxr-xr-x      0 /a
- rw-r--r--   9.9k /a/a1
- rwxr-x---  20.9k /a/a2
d rwxr-xr-x      0 /a/b
d rwxr-x---      0 /a/n
d rwxr-x---      0 /a/n/m
- rw-r-----     11 /a/n/m/m1
d rwxr-xr-x      0 /d
d rwxr-xr-x      0 /e
d rwxr-xr-x      0 /e/f
`
	pushrdb = `d rwxr-xr-x      0 /
- rw-r--r--      0 /1
- rw-r--r--  30.9k /2
d rwxr-xr-x      0 /a
- rw-r--r--   9.9k /a/a1
- rwxr-x---  20.9k /a/a2
d rwxr-xr-x      0 /a/b
d rwxr-x---      0 /a/n
d rwxr-x---      0 /a/n/m
- rw-r-----     11 /a/n/m/m1
d rwxr-xr-x      0 /d
d rwxr-xr-x      0 /e
d rwxr-xr-x      0 /e/f
`
)

func TestTreePush(t *testing.T) {
	db, fn := mkrtest(t, rtdir)
	defer fn()
	db.Close()
	db, fn2 := mktest(t, tdir)
	defer fn2()
	db.Close()

	tr, err := New("adb", tdir, "unix!local!9898!/p")
	if err != nil {
		t.Fatalf("tree %s", err)
	}
	tr.Debug = testing.Verbose()
	tr.Rdb.Debug = testing.Verbose()
	defer tr.Close()
	fstest.MkChgs(t, tdir)
	fstest.MkChgs2(t, rtdir+"/p")
	chkFiles(t, tr.Ldb, fstest.AllFiles, fstest.AllFilesList)
	chkFiles(t, tr.Rdb, fstest.AllFiles, fstest.AllFilesList)
	dprintf("\npush\n")
	cc, dc := getChgs()
	err = tr.BlindPush(cc)
	if err != nil {
		t.Fatalf("push %s", err)
	}
	cs := <-dc
	logChgs(cs)
	cmpChgs(t, pushcs, cs)
	chkFiles(t, tr.Ldb, nil, pushldb)
	chkFiles(t, tr.Rdb, nil, pushrdb)
	os.RemoveAll(rtdir+".push")
	os.Rename(rtdir+"/p", rtdir+".push")
}

var (
	pullcs = []Chg {
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/1"}}, At: Remote},
		Chg{Chg: zx.Chg{Type: zx.DirFile, D: zx.Dir{"path": "/2"}}, At: Remote},
	}

	// MkChgs2 w/o MkChgs
	pullrdb = `d rwxr-xr-x      0 /
- rw-r--r--     50 /1
d rwxr-x---      0 /2
d rwxr-x---      0 /2/n2
d rwxr-xr-x      0 /a
- rw-r--r--   9.9k /a/a1
- rw-r--r--  20.9k /a/a2
d rwxr-xr-x      0 /a/b
d rwxr-xr-x      0 /a/b/c
- rw-r--r--  43.9k /a/b/c/c3
d rwxr-xr-x      0 /d
d rwxr-xr-x      0 /e
d rwxr-xr-x      0 /e/f
`
)

func TestTreePull(t *testing.T) {
	db, fn := mkrtest(t, rtdir)
	defer fn()
	db.Close()
	db, fn2 := mktest(t, tdir)
	defer fn2()
	db.Close()

	tr, err := New("adb", tdir, "unix!local!9898!/p")
	if err != nil {
		t.Fatalf("tree %s", err)
	}
	tr.Debug = testing.Verbose()
	defer tr.Close()
	fstest.MkChgs(t, tdir)
	fstest.MkChgs2(t, rtdir+"/p")
	chkFiles(t, tr.Ldb, nil, "")
	dprintf("pull\n")
	cc, dc := getChgs()
	err = tr.BlindPull(cc)
	if err != nil {
		t.Fatalf("pull %s", err)
	}
	cs := <-dc
	logChgs(cs)
	cmpChgs(t, pullcs, cs)
	chkFiles(t, tr.Ldb, nil, pullrdb)
	chkFiles(t, tr.Rdb, nil, pullrdb)
	os.RemoveAll(tdir+".pull")
	os.Rename(tdir, tdir+".pull")
}

func TestTreeSync(t *testing.T) {
	db, fn := mkrtest(t, rtdir)
	defer fn()
	db.Close()
	db, fn2 := mktest(t, tdir)
	defer fn2()
	db.Close()

	tr, err := New("adb", tdir, "unix!local!9898!/p")
	if err != nil {
		t.Fatalf("tree %s", err)
	}
	tr.Debug = testing.Verbose()
	tr.Rdb.Debug = testing.Verbose()
	defer tr.Close()
	fstest.MkChgs(t, tdir)
	fstest.MkChgs2(t, rtdir+"/p")
	dprintf("\nsync\n")
	cc, dc := getChgs()
	err = tr.Sync(cc)
	if err != nil {
		t.Fatalf("sync %s", err)
	}
	cs := <-dc
	logChgs(cs)
	chkFiles(t, tr.Ldb, nil, "")
	chkFiles(t, tr.Rdb, nil, "")
	os.RemoveAll(tdir+".pull")
	os.Rename(tdir, tdir+".pull")
	os.RemoveAll(rtdir+".push")
	os.Rename(rtdir+"/p", rtdir+".push")
}

var (
	pulldchgs = []Chg {
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/1"}},
			At: Remote},
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/2"}},
			At: Remote},
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/a/a1"}},
			At: Remote},
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/a/a2"}},
			At: Remote},
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/a/b/c/c3"}},
			At: Remote},
	}
	pulldchgs2 = []Chg {
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/1"}},
			At: Remote},
		Chg{Chg: zx.Chg{Type: zx.DirFile, D: zx.Dir{"path": "/2"}},
			At: Remote},
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/a/a1"}},
			At: Remote},
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/a/a2"}},
			At: Remote},
		Chg{Chg: zx.Chg{Type: zx.Add, D: zx.Dir{"path": "/a/b/c"}},
			At: Remote},
		Chg{Chg: zx.Chg{Type: zx.Del, D: zx.Dir{"path": "/a/n"}},
			At: Remote},
	}
)

func TestTreeAllPullChanges(t *testing.T) {
	db, fn := mkrtest(t, rtdir)
	defer fn()
	db.Close()
	db, fn2 := mktest(t, tdir)
	defer fn2()
	db.Close()

	tr, err := New("adb", tdir, "unix!local!9898!/p")
	if err != nil {
		t.Fatalf("tree %s", err)
	}
	tr.Debug = testing.Verbose()
	tr.Rdb.Debug = testing.Verbose()
	defer tr.Close()
	dprintf("\ndiffs before making changes\n")
	cc, err := tr.AllPullChanges()
	if err != nil {
		t.Fatalf("sync %s", err)
	}
	chgs := []Chg{}
	for c := range cc {
		chgs = append(chgs, c)
		dprintf("chg %s\n", c)
	}
	cmpChgs(t, pulldchgs, chgs)

	dprintf("\ndiffs after making changes\n")
	fstest.MkChgs(t, tdir)
	fstest.MkChgs2(t, rtdir+"/p")
	cc, err = tr.AllPullChanges()
	if err != nil {
		t.Fatalf("sync %s", err)
	}
	chgs = []Chg{}
	for c := range cc {
		chgs = append(chgs, c)
		dprintf("chg2 %s\n", c)
	}
	cmpChgs(t, pulldchgs2, chgs)

	os.RemoveAll(tdir+".pull")
	os.Rename(tdir, tdir+".pull")
	os.RemoveAll(rtdir+".push")
	os.Rename(rtdir+"/p", rtdir+".push")
}

var (
	pushdchgs = []Chg {
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/1"}},
			At: Local},
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/2"}},
			At: Local},
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/a/a1"}},
			At: Local},
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/a/a2"}},
			At: Local},
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/a/b/c/c3"}},
			At: Local},
	}
	pushdchgs2 = []Chg {
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/1"}},
			At: Local},
		Chg{Chg: zx.Chg{Type: zx.DirFile, D: zx.Dir{"path": "/2"}},
			At: Local},
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/a/a1"}},
			At: Local},
		Chg{Chg: zx.Chg{Type: zx.Data, D: zx.Dir{"path": "/a/a2"}},
			At: Local},
		Chg{Chg: zx.Chg{Type: zx.Del, D: zx.Dir{"path": "/a/b/c"}},
			At: Local},
		Chg{Chg: zx.Chg{Type: zx.Add, D: zx.Dir{"path": "/a/n"}},
			At: Local},
	}
)

func TestTreeAllPushChanges(t *testing.T) {
	db, fn := mkrtest(t, rtdir)
	defer fn()
	db.Close()
	db, fn2 := mktest(t, tdir)
	defer fn2()
	db.Close()

	tr, err := New("adb", tdir, "unix!local!9898!/p")
	if err != nil {
		t.Fatalf("tree %s", err)
	}
	tr.Debug = testing.Verbose()
	tr.Rdb.Debug = testing.Verbose()
	defer tr.Close()
	dprintf("\ndiffs before making changes\n")
	cc, err := tr.AllPushChanges()
	if err != nil {
		t.Fatalf("sync %s", err)
	}
	chgs := []Chg{}
	for c := range cc {
		chgs = append(chgs, c)
		dprintf("chg %s\n", c)
	}
	cmpChgs(t, pushdchgs, chgs)

	dprintf("\ndiffs after making changes\n")
	fstest.MkChgs(t, tdir)
	fstest.MkChgs2(t, rtdir+"/p")
	cc, err = tr.AllPushChanges()
	if err != nil {
		t.Fatalf("sync %s", err)
	}
	chgs = []Chg{}
	for c := range cc {
		chgs = append(chgs, c)
		dprintf("chg2 %s\n", c)
	}
	cmpChgs(t, pushdchgs2, chgs)

	os.RemoveAll(tdir+".pull")
	os.Rename(tdir, tdir+".pull")
	os.RemoveAll(rtdir+".push")
	os.Rename(rtdir+"/p", rtdir+".push")
}

func TestTreePullAll(t *testing.T) {
	db, fn := mkrtest(t, rtdir)
	defer fn()
	db.Close()
	db, fn2 := mktest(t, tdir)
	defer fn2()
	db.Close()
	fstest.MkChgs(t, tdir)
	fstest.MkChgs2(t, rtdir+"/p")

	tr, err := New("adb", tdir, "unix!local!9898!/p")
	if err != nil {
		t.Fatalf("tree %s", err)
	}
	tr.Debug = testing.Verbose()
	tr.Rdb.Debug = testing.Verbose()
	defer tr.Close()
	dprintf("\ninitial:\n")
	chkFiles(t, tr.Ldb, nil, "")
	chkFiles(t, tr.Rdb, nil, "")
	dprintf("\npullall\n")
	cc, dc := getChgs()
	err = tr.PullAll(cc)
	if err != nil {
		t.Fatalf("pullall %s", err)
	}
	dprintf("\npullall done\n")
	cs := <-dc
	logChgs(cs)
	chkFiles(t, tr.Ldb, nil, pullrdb)
	chkFiles(t, tr.Rdb, nil, pullrdb)
	os.RemoveAll(tdir+".pull")
	os.Rename(tdir, tdir+".pull")
	os.RemoveAll(rtdir+".push")
	os.Rename(rtdir+"/p", rtdir+".push")
}

func TestTreeSaveLoad(t *testing.T) {

	db, fn := mkrtest(t, rtdir)
	defer fn()
	db.Close()
	db, fn2 := mktest(t, tdir)
	defer fn2()
	db.Close()
	fstest.MkChgs(t, tdir)
	fstest.MkChgs2(t, rtdir+"/p")
	os.Remove(tdir+"repl.ldb")
	os.Remove(tdir+"repl.rdb")
	defer os.Remove(tdir+"repl.ldb")
	defer os.Remove(tdir+"repl.rdb")
	tr, err := New("adb", tdir, "unix!local!9898!/p")
	if err != nil {
		t.Fatalf("tree %s", err)
	}
	tr.Debug = testing.Verbose()
	tr.Rdb.Debug = testing.Verbose()
	defer tr.Close()
	dprintf("\ninitial:\n")
	chkFiles(t, tr.Ldb, nil, "")
	chkFiles(t, tr.Rdb, nil, "")

	dprintf("\nsave & load:\n")
	if err := tr.Save(tdir+"repl"); err != nil {
		t.Fatalf("save %s", err)
	}
	if tr, err = Load(tdir+"repl"); err != nil {
		t.Fatalf("load %s", err)
	}
	chkFiles(t, tr.Ldb, nil, "")
	chkFiles(t, tr.Rdb, nil, "")

	// now continue as in the previous test, to check
	// it all is ok.
	dprintf("\npullall\n")
	cc, dc := getChgs()
	err = tr.PullAll(cc)
	if err != nil {
		t.Fatalf("pullall %s", err)
	}
	dprintf("\npullall done\n")
	cs := <-dc
	logChgs(cs)
	chkFiles(t, tr.Ldb, nil, pullrdb)
	chkFiles(t, tr.Rdb, nil, pullrdb)
	os.RemoveAll(tdir+".pull")
	os.Rename(tdir, tdir+".pull")
	os.RemoveAll(rtdir+".push")
	os.Rename(rtdir+"/p", rtdir+".push")

}

// TODO: test excluded files, test errors on files
