package fstest

import (
	"clive/net/auth"
	"clive/zx"
)

struct wstatTest {
	Path  string
	Dir   zx.Dir
	Fails bool
	Res   string
}

var wstats = []wstatTest{
	{
		Path:  "/notthere",
		Dir:   zx.Dir{"type": "-", "size": "0", "mode": "0640"},
		Fails: true,
	},
	{
		Path: "/1",
		Dir:  zx.Dir{"type": "x", "size": "10", "mode": "0660", "mtime": "0"},
		Res:  `- rw-rw----     10 /1`,
	},
	{
		Path:  "/a/not/n2",
		Dir:   zx.Dir{"type": "-", "size": "0", "mode": "0640"},
		Fails: true,
	},
	{
		Path: "/d",
		Dir:  zx.Dir{"mode": "0740"},
		Res:  `d rwxr-----      0 /d`,
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
Loop:
	for i, pt := range wstats {
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

func Attrs(t Fataler, xfs zx.Fs) {
	if afs, ok := xfs.(zx.Auther); !ok {
		t.Logf("fs is not auther; attr test skip")
		return
	} else {
		var err error
		xfs, err = afs.Auth(&auth.Info{Uid: "elf", SpeaksFor: "elf", Ok: true})
		if err != nil {
			t.Fatalf("auth failed: %s", err)
		}
	}
	fs, ok := xfs.(zx.Wstater)
	if !ok {
		t.Fatalf("not a Wstater")
	}
	paths := []string{"/1", "/a"}
	for _, p := range paths {
		rc := fs.Wstat(p, zx.Dir{"foo": "bar"})
		rd := <-rc
		if err := cerror(rc); err != nil {
			t.Fatalf("can't wstat")
		}
		Printf("rd: %s\n", rd.LongFmt())
		if rd["foo"] != "bar" {
			t.Fatalf("didn't wstat foo at rd")
		}
		d, err := zx.Stat(xfs, p)
		if err != nil {
			t.Fatalf("can't stat")
		}
		Printf("stat: %s\n", d.LongFmt())
		if rd["foo"] != "bar" {
			t.Fatalf("didn't wstat foo")
		}
	}
}
