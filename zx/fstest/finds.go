package fstest

import (
	"bytes"
	"clive/zx"
)

struct findTest {
	Path         string
	Pred         string
	Spref, Dpref string
	Depth        int
	Res          []string
	Fails        bool
}

var finds = []findTest{
	{
		Path: "/",
		Pred: "",
		Res: []string{
			`d rwxr-xr-x      0 /`,
			`c rw-r--r--      0 /Ctl`,
			`- rw-r--r--      0 /1`,
			`- rw-r--r--  30.9k /2`,
			`d rwxr-xr-x      0 /a`,
			`- rw-r--r--   9.9k /a/a1`,
			`- rw-r--r--  20.9k /a/a2`,
			`d rwxr-xr-x      0 /a/b`,
			`d rwxr-xr-x      0 /a/b/c`,
			`- rw-r--r--  43.9k /a/b/c/c3`,
			`d rwxr-xr-x      0 /d`,
			`d rwxr-xr-x      0 /e`,
			`d rwxr-xr-x      0 /e/f`,
		},
	},

	{
		Path: "/",
		Pred: "type=d&depth>1",
		Res: []string{
			`d rwxr-xr-x      0 /a/b`,
			`d rwxr-xr-x      0 /a/b/c`,
			`d rwxr-xr-x      0 /e/f`,
		},
	},

	{
		Path:  "/xxx",
		Pred:  "type=d&depth>1",
		Res:   []string{},
		Fails: true,
	},

	{
		Path: "/a",
		Pred: "name=blah",
		Res:  []string{},
	},

	// this is similar to mounting /a at /x/y and doing a find at /x/y/b
	{
		Path:  "/a/b",
		Spref: "/a",
		Dpref: "/x/y",
		Pred:  "depth<=1",
		Res: []string{
			`d rwxr-xr-x      0 /x/y/b`,
			`d rwxr-xr-x      0 /x/y/b/c`,
		},
	},

	// this is similar to mounting /a at /x/y and doing a find at /x
	{
		Path:  "/a/b",
		Spref: "/a",
		Dpref: "/x/y",
		Depth: 1,
		Pred:  "depth<=2",
		Res: []string{
			`d rwxr-xr-x      0 /x/y/b`,
			`d rwxr-xr-x      0 /x/y/b/c`,
		},
	},

	// this is similar to mounting /a/b at /c and doing a find at /c
	{
		Path:  "/a/b",
		Spref: "/a/b",
		Dpref: "/c",
		Depth: 0,
		Pred:  "depth<=1",
		Res: []string{
			`d rwxr-xr-x      0 /c`,
			`d rwxr-xr-x      0 /c/c`,
		},
	},

	// prunes
	{
		Path:  "/a/b",
		Spref: "/",
		Dpref: "/",
		Pred:  `(path = "/a/b" | path = "/d") & prune | type = d`,
		Res: []string{
			`d rwxr-xr-x      0 /a/b pruned`,
		},
	},
}

func Finds(t Fataler, xfs zx.Fs) {
	fs, ok := xfs.(zx.Finder)
	if !ok {
		t.Fatalf("not a finder")
	}
	for i, ft := range finds {
		out := ""
		Printf("find #%d %s %q\n", i, ft.Path, ft.Pred)
		dc := fs.Find(ft.Path, ft.Pred, ft.Spref, ft.Dpref, ft.Depth)
		n := 0
		for d := range dc {
			ds := d.Fmt()
			Printf("\t`%s`,\n", ds)
			out += ds + "\n"
			if n >= len(ft.Res) || ft.Res[n] != ds {
				t.Fatalf("bad finding")
			}
			n++
		}
		err := cerror(dc)
		Printf("sts=%v\n\n", err)
		if err != nil && !ft.Fails {
			t.Fatalf("err %v\n", err)
		} else if err == nil && ft.Fails {
			t.Fatalf("didn't fail")
		}
	}
}

func chklast(t Fataler, last string, data []byte) {
	if last != "" && bytes.Compare(FileData[last], data) != 0 {
		Printf("%d bytes vs %d bytes\n", len(FileData[last]), len(data))
		t.Fatalf("bad file content for %s", last)
	}
}

func FindGets(t Fataler, xfs zx.Fs) {
	fs, ok := xfs.(zx.FindGetter)
	if !ok {
		t.Fatalf("not a finder")
	}
	for i, ft := range finds {
		out := ""
		Printf("find #%d %s %q\n", i, ft.Path, ft.Pred)
		dc := fs.FindGet(ft.Path, ft.Pred, ft.Spref, ft.Dpref, ft.Depth)
		n := 0
		data := []byte{}
		last := ""
		for d := range dc {
			switch d := d.(type) {
			case zx.Dir:
				if ft.Spref == "" {
					chklast(t, last, data)
				}
				data = data[:0]
				last = d["path"]
				ds := d.Fmt()
				Printf("\t`%s`,\n", ds)
				out += ds + "\n"
				if n >= len(ft.Res) || ft.Res[n] != ds {
					t.Fatalf("bad finding")
				}
				n++
			case []byte:
				data = append(data, d...)
			case error:
				data = data[:0]
				last = ""
				t.Fatalf("unexpected err %v", d)
			default:
				data = data[:0]
				last = ""
				t.Fatalf("bad msg type %T", d)
			}
		}
		if ft.Spref == "" {
			chklast(t, last, data)
		}
		err := cerror(dc)
		Printf("sts=%v\n\n", err)
		if err != nil && !ft.Fails {
			t.Fatalf("err %v\n", err)
		} else if err == nil && ft.Fails {
			t.Fatalf("didn't fail")
		}
	}
}
