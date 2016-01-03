package fscmp

import (
	"clive/dbg"
	"clive/zx/fstest"
	"clive/zx/zux"
	"fmt"
	"os"
	"testing"
)

var (
	tdir   = "/tmp/fscmptest"
	tdir2  = "/tmp/fscmptest2"
	Verb   bool
	Printf = dbg.FlagPrintf(&fstest.Verb)
)

func TestDiffs(t *testing.T) {
	os.Args[0] = "fscmp.test"
	fstest.Verb = testing.Verbose()
	fstest.MkTree(t, tdir)
	fstest.MkChgs(t, tdir)
	defer os.RemoveAll(tdir)
	fstest.ResetTime()
	fstest.MkTree(t, tdir2)
	fstest.MkChgs2(t, tdir2)
	defer os.RemoveAll(tdir2)

	Printf("changes...\n")
	fs, err := zux.NewZX(tdir)
	if err != nil {
		t.Fatal(err)
	}
	fs2, err := zux.NewZX(tdir2)
	if err != nil {
		t.Fatal(err)
	}
	rc := Diff(fs, fs2)
	out := ""
	for c := range rc {
		s := fmt.Sprintf("chg %s %s\n", c.Type, c.D.Fmt())
		Printf("%s", s)
		out += s
	}
	xout := `chg data - rw-r--r--     50 /1
chg dirfile d rwxr-x---      0 /2
chg data - rw-r--r--   9.9k /a/a1
chg meta - rw-r--r--  20.9k /a/a2
chg add d rwxr-xr-x      0 /a/b/c
chg add - rw-r--r--  43.9k /a/b/c/c3
chg del d rwxr-x---      0 /a/n
chg del d rwxr-x---      0 /a/n/m
chg del - rw-r-----     11 /a/n/m/m1
`
	if out != xout {
		t.Fatalf("bad set of changes")
	}
}
