package fstest

import (
	"clive/zx"
)

struct wstatTest {
	Path  string
	Dir   zx.Dir
	Fails bool
	Res string
}

var wstats = []wstatTest {
	{
		Path: "/notthere",
		Dir:  zx.Dir{"type": "-", "size": "0", "mode": "0640"},
		Fails: true,
	},
	{
		Path: "/1",
		Dir:  zx.Dir{"type": "x", "size": "10", "mode": "0660", "mtime": "0"},
		Res: `- rw-rw----     10 /1`,
	},
	{
		Path: "/a/not/n2",
		Dir:  zx.Dir{"type": "-", "size": "0", "mode": "0640"},
		Fails: true,
	},
	{
		Path:  "/d",
		Dir:   zx.Dir{"mode": "0740"},
		Res: `d rwxr-----      0 /d`,
	},
	{
		Path:  "/d",
		Dir:   zx.Dir{"size": "5"},
		Fails: true,
	},
}

func Wstats(t Fataler, xfs zx.Fs) {
	fs, ok := xfs.(zx.Wstater)
	if !ok {
		t.Fatalf("not a Wstater")
	}
	// We try twice so we check if wstat changed anything behind our back
	ntry := 0
Loop:	for i, pt := range wstats {
		Printf("wstat #%d %s %s\n", i, pt.Path, pt.Dir.Fmt())
		rc := fs.Wstat(pt.Path, pt.Dir)
		rd := <-rc
		if !pt.Fails && cerror(rc) != nil {
			t.Fatalf("did fail")
		}
		Printf("rd `%s`,\n", rd.Fmt())
		if pt.Fails {
			if cerror(rc) == nil {
				t.Fatalf("didn't fail")
			}
			continue
		}
		if pt.Res != rd.Fmt() {
			t.Fatalf("bad resulting dir")
		}
		if pt.Dir["mtime"] != "" && rd["mtime"] != pt.Dir["mtime"] {
			t.Fatalf("bad mtime: %s", rd["mtime"])
		}
	}
	if ntry == 0 {
		ntry++
		goto Loop
	}
}
