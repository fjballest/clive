package fstest

import (
	"clive/zx"
	"path"
	"strings"
)

struct mvTest {
	From, To string
	Fails    bool
	Child    string
	Res      string
	Stats    []string
}

// Use size X for /
var mvs = []mvTest{
	{"/1", "/1", false, "", `- rw-r--r--      0 /1`, []string{
		`- rw-r--r--      0 /1`,
		`- rw-r--r--      0 /1`,
		`d rwxr-xr-x      0 /`,
		`d rwxr-xr-x      0 /`,
	}},
	// keep the next one, used to check zx attrs
	{"/2", "/n2", false, "",
		`- rw-r--r--  30.9k /n2`, []string{
			`<nil dir>`,
			`- rw-r--r--  30.9k /n2`,
			`d rwxr-xr-x      0 /`,
			`d rwxr-xr-x      0 /`,
		}},
	{"/", "/", false, "",
		"d rwxr-xr-x      0 /", []string{
			`d rwxr-xr-x      0 /`,
			`d rwxr-xr-x      0 /`,
			`d rwxr-xr-x      0 /`,
		}},
	{"/a/a1", "/a/a2", false, "",
		`- rw-r--r--   9.9k /a/a2`, []string{
			`<nil dir>`,
			`- rw-r--r--   9.9k /a/a2`,
			`d rwxr-xr-x      0 /a`,
			`d rwxr-xr-x      0 /a`,
		}},
	{"/a/a2", "/a3", false, "",
		`- rw-r--r--   9.9k /a3`, []string{
			`<nil dir>`,
			`- rw-r--r--   9.9k /a3`,
			`d rwxr-xr-x      0 /a`,
			`d rwxr-xr-x      0 /`,
		}},
	{"/a/b", "/d/b", false, "/d/b/c/c3",
		`d rwxr-xr-x      0 /d/b`, []string{
			`<nil dir>`,
			`d rwxr-xr-x      0 /d/b`,
			`d rwxr-xr-x      0 /a`,
			`d rwxr-xr-x      0 /d`,
		}},
	{"/1", "/e", true, "", "", nil},
	{"/e", "/1", true, "", "", nil},
	{"/Ctl", "/x", true, "", "", nil},
	{"/x", "/Ctl", true, "", "", nil},
	{"/d", "/d/b", true, "", ``, nil},
	{"/", "/x", true, "", ``, nil},
	{"/", "/a", true, "", ``, nil},
	{"/a/a2", "/a", true, "", ``, nil},
}

func Moves(t Fataler, xfs zx.Fs) {
	fs, ok := xfs.(zx.Mover)
	if !ok {
		t.Fatalf("not a Mover")
	}
	wfs, ok := xfs.(zx.Wstater)
	if ok {
		rc := wfs.Wstat("/2", zx.Dir{"foo": "bar"})
		<-rc
		if err := cerror(rc); err != nil {
			t.Fatalf("can't wstat")
		}
	}
	for i, mv := range mvs {
		Printf("mv #%d %s %s\n", i, mv.From, mv.To)
		err := <-fs.Move(mv.From, mv.To)
		if mv.Fails {
			if err == nil {
				t.Fatalf("mv %s didn't fail", mv.From)
			}
			continue
		}
		if err != nil {
			t.Fatalf("mv %s: %s", mv.From, err)
		}
		d, err := zx.Stat(xfs, mv.To)
		if err != nil {
			t.Fatalf("stat %s: %s", mv.From, err)
		}
		Printf("\t`%s`,\n", d.Fmt())
		if mv.Res != "" && !strings.HasPrefix(d.Fmt(), mv.Res) {
			t.Fatalf("bad stat. ")
		}
		if mv.Child != "" {
			d, err = zx.Stat(xfs, mv.Child)
			if err != nil {
				t.Fatalf("stat %s: %s", mv.Child, err)
			}
			if d["path"] != mv.Child {
				t.Logf("%s: bad path %s", mv.Child, d["path"])
			}
		}
		paths := []string{mv.From, mv.To, path.Dir(mv.From), path.Dir(mv.To)}
		for i, p := range paths {
			d, _ := zx.Stat(xfs, p)
			Printf("\t\t`%s`,\n", d.Fmt())
			if i < len(mv.Stats) && !strings.HasPrefix(d.Fmt(), mv.Stats[i]) {
				t.Fatalf("bad stat")
			}
		}
	}
	if wfs != nil {
		d, err := zx.Stat(xfs, "/n2")
		if err != nil {
			t.Fatalf("stat /n2: %s", err)
		}
		if d["foo"] != "bar" {
			t.Fatalf("didn't move the foo attr for /2 -> /n2")
		}
	}

}
