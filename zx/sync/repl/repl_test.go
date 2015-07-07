package repl

import (
	"clive/dbg"
	"clive/zx/fstest"
	"os"
	"testing"
	"bytes"
	"strings"
)

const (
	rdir = "/tmp/repl"
	rcfg = rdir + "/r"
	tdir = "/tmp/db_test"
	tdir2 = "/tmp/db_test2"
)

var (
	printf = dbg.FuncPrintf(os.Stdout, testing.Verbose)
)

func TestNew(t *testing.T) {
	fstest.ResetTime()
	fstest.RmTree(t, rdir)
	fstest.RmTree(t, tdir)
	fstest.RmTree(t, tdir2)
	if err := os.Mkdir(rdir, 0755); err != nil && !dbg.IsExists(err) {
		t.Fatalf("%s: %s", tdir2, err)
	}
	defer fstest.RmTree(t, rdir)
	fstest.MkTree(t, tdir)
	defer fstest.RmTree(t, tdir)
	if err := os.Mkdir(tdir2, 0755); err != nil {
		t.Fatalf("%s: %s", tdir2, err)
	}
	defer fstest.RmTree(t, tdir2)

	r, err := New("testrepl", "", tdir, tdir2)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	if err := r.Save(rcfg); err != nil {
		t.Fatalf("save: %s", err)
	}
	nr, err := Load(rcfg)
	if err != nil {
		t.Fatalf("load: %s", err)
	}
	if testing.Verbose() {
		nr.DumpTo(os.Stdout)
	}
	var b bytes.Buffer
	nr.DumpTo(&b)
	out := `testrepl '' /tmp/db_test /tmp/db_test2
testrepl[/tmp/db_test]
/              d 0755 nemo     nemo     nemo            6
/1             - 0644 none     none     none            0 0
/2             - 0644 none     none     none        31658 4000000000
/a             d 0755 none     none     none            3
/a/a1          - 0644 none     none     none        10154 1000000000
/a/a2          - 0644 none     none     none        21418 2000000000
/a/b           d 0755 none     none     none            1
/a/b/c         d 0755 none     none     none            1
/a/b/c/c3      - 0644 none     none     none        44970 3000000000
/d             d 0755 none     none     none            0
/e             d 0755 none     none     none            1
/e/f           d 0755 none     none     none            0
testrepl[/tmp/db_test2]
/              d 0755 nemo     nemo     nemo            1
`
	if b.String() != out {
		t.Fatal("bad repl content")
	}
}


func TestSyncNew(t *testing.T) {
	fstest.ResetTime()
	fstest.RmTree(t, rdir)
	fstest.RmTree(t, tdir)
	fstest.RmTree(t, tdir2)
	defer fstest.RmTree(t, tdir2)
	defer fstest.RmTree(t, rdir)
	defer fstest.RmTree(t, tdir) 
	if err := os.Mkdir(rdir, 0755); err != nil && !dbg.IsExists(err) {
		t.Fatalf("%s: %s", tdir2, err)
	}
	fstest.MkTree(t, tdir)
	if err := os.Mkdir(tdir2, 0755); err != nil {
		t.Fatalf("%s: %s", tdir2, err)
	}

	r, err := New("testrepl", "", tdir, tdir2)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	if testing.Verbose() {
		r.DumpTo(os.Stdout)
	}
	if err := r.Sync(); err != nil {
		t.Fatalf("sync: %s", err)
	}
	if err := r.Save(rcfg); err != nil {
		t.Fatalf("save: %s", err)
	}
	nr, err := Load(rcfg)
	if err != nil {
		t.Fatalf("load: %s", err)
	}
	if testing.Verbose() {
		nr.DumpTo(os.Stdout)
	}

	var b bytes.Buffer
	nr.DumpTo(&b)
	out := `testrepl '' /tmp/db_test /tmp/db_test2
testrepl[/tmp/db_test]
/              d 0755 none     none     none            6
/1             - 0644 none     none     none            0 0
/2             - 0644 none     none     none        31658 4000000000
/a             d 0755 none     none     none            3
/a/a1          - 0644 none     none     none        10154 1000000000
/a/a2          - 0644 none     none     none        21418 2000000000
/a/b           d 0755 none     none     none            1
/a/b/c         d 0755 none     none     none            1
/a/b/c/c3      - 0644 none     none     none        44970 3000000000
/d             d 0755 none     none     none            0
/e             d 0755 none     none     none            1
/e/f           d 0755 none     none     none            0
testrepl[/tmp/db_test2]
/              d 0755 none     none     none            6
/1             - 0644 none     none     none            0 0
/2             - 0644 none     none     none        31658 4000000000
/a             d 0755 none     none     none            3
/a/a1          - 0644 none     none     none        10154 1000000000
/a/a2          - 0644 none     none     none        21418 2000000000
/a/b           d 0755 none     none     none            1
/a/b/c         d 0755 none     none     none            1
/a/b/c/c3      - 0644 none     none     none        44970 3000000000
/d             d 0755 none     none     none            0
/e             d 0755 none     none     none            1
/e/f           d 0755 none     none     none            0
`
	if s := strings.Replace(b.String(), dbg.Usr, "none", -1); s != out {
		printf("<%s>\n", s)
		t.Fatalf("bad repl dbs")
	}
}

func TestSyncChgs(t *testing.T) {
	fstest.ResetTime()
	fstest.RmTree(t, rdir)
	fstest.RmTree(t, tdir)
	fstest.RmTree(t, tdir2)
	defer fstest.RmTree(t, tdir2)
	defer fstest.RmTree(t, rdir)
	defer fstest.RmTree(t, tdir) 
	if err := os.Mkdir(rdir, 0755); err != nil && !dbg.IsExists(err) {
		t.Fatalf("%s: %s", tdir2, err)
	}
	fstest.MkTree(t, tdir)
	if err := os.Mkdir(tdir2, 0755); err != nil {
		t.Fatalf("%s: %s", tdir2, err)
	}

	r, err := New("testrepl", "", tdir, tdir2)
	if err != nil {
		t.Fatalf("new: %s", err)
	}
	if err := r.Sync(); err != nil {
		t.Fatalf("sync: %s", err)
	}
	if err := r.Save(rcfg); err != nil {
		t.Fatalf("save: %s", err)
	}

	r, err = Load(rcfg)
	if err != nil {
		t.Fatalf("load: %s", err)
	}
	if testing.Verbose() {
		r.DumpTo(os.Stdout)
	}

	fstest.MkChgs2(t, tdir)
	fstest.MkChgs(t, tdir2)

	if err := r.Sync(); err != nil {
		t.Fatalf("sync: %s", err)
	}
	if testing.Verbose() {
		r.DumpTo(os.Stdout)
	}

	if err := r.Sync(); err != nil {
		t.Fatalf("sync: %s", err)
	}
	if err := r.Save(rcfg); err != nil {
		t.Fatalf("save: %s", err)
	}
	if err := r.Sync(); err != nil {
		t.Fatalf("sync: %s", err)
	}
	var b bytes.Buffer
	r.DumpTo(&b)
	out := `testrepl '' /tmp/db_test /tmp/db_test2
testrepl[/tmp/db_test]
/              d 0755 none     none     none            6
/1             - 0644 none     none     none           50 13000000000
/2             d 0750 none     none     none            1
/2/n2          d 0750 none     none     none            0
/a             d 0755 none     none     none            4
/a/a1          - 0644 none     none     none        10154 14000000000
/a/a2          - 0750 none     none     none        21418 2000000000
/a/b           d 0755 none     none     none            0
/a/b/c         d GONE none     none     none            1
/a/n           d 0750 none     none     none            1
/a/n/m         d 0750 none     none     none            1
/a/n/m/m1      - 0640 none     none     none           11 15000000000
/d             d 0755 none     none     none            0
/e             d 0755 none     none     none            1
/e/f           d 0755 none     none     none            0
testrepl[/tmp/db_test2]
/              d 0755 none     none     none            6
/1             - 0644 none     none     none           50 13000000000
/2             d 0750 none     none     none            1
/2/n2          d 0750 none     none     none            0
/a             d 0755 none     none     none            4
/a/a1          - 0644 none     none     none        10154 14000000000
/a/a2          - 0750 none     none     none        21418 2000000000
/a/b           d 0755 none     none     none            0
/a/b/c         d GONE none     none     none            1
/a/n           d 0750 none     none     none            1
/a/n/m         d 0750 none     none     none            1
/a/n/m/m1      - 0640 none     none     none           11 15000000000
/d             d 0755 none     none     none            0
/e             d 0755 none     none     none            1
/e/f           d 0755 none     none     none            0
`
	if strings.Replace(b.String(), dbg.Usr, "none", -1) != out {
		t.Fatal("bad repl")
	}
}

