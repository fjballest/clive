package fstest

import (
	"clive/zx"
	"path"
)

type mvTest struct {
	From, To string
	Fails    bool
	Child    string
	Res      string
}

// Use size X for /
var mvs = []mvTest{
	{"/1", "/1", false, "", ""},
	{"/2", "/n2", false, "",
		`path /n2 name n2 type - mode 0644 size 31658 Gid nemo Uid nemo`,
	},
	{"/", "/", false, "",
		"path / name / type d mode 0755 size X Gid nemo Uid nemo",
	},
	{"/a/a1", "/a/a2", false, "",
		`path /a/a2 name a2 type - mode 0644 size 10154 Gid nemo Uid nemo`,
	},
	{"/a/a2", "/a3", false, "",
		`path /a3 name a3 type - mode 0644 size 10154 Gid nemo Uid nemo`,
	},
	{"/a/b", "/d/b", false, "/d/b/c/c3",
		`path /d/b name b type d mode 0755 size 1 Gid nemo Uid nemo`,
	},
	{"/1", "/e", true, "", ""},
	{"/e", "/1", true, "", ""},
	{"/Ctl", "/x", true, "", ""},
	{"/x", "/Ctl", true, "", ""},
	{"/d", "/d/b", true, "", ``},
	{"/", "/x", true, "", ``},
	{"/", "/a", true, "", ``},
	{"/a/a2", "/a", true, "", ``},
}

func Moves(t Fataler, xfs zx.Fs) {
	fs, ok := xfs.(zx.Mover)
	if !ok {
		t.Fatalf("not a Mover")
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
		Printf("new stat: %s\n", d.Fmt())
		if mv.Child != "" {
			d, err = zx.Stat(xfs, mv.Child)
			if err != nil {
				t.Fatalf("stat %s: %s", mv.Child, err)
			}
			if d["path"] != mv.Child {
				t.Fatalf("%s: bad path %s", mv.Child, d["path"])
			}
		}
		paths := []string{mv.From, mv.To, path.Dir(mv.From), path.Dir(mv.To)}
		for _, p := range paths {
			d, err := zx.Stat(xfs, mv.To)
			if err != nil {
				t.Fatalf("stat %s: %s", p, err)
			}
			Printf("new child stat: %s\n", d.Fmt())
		}
	}
}
