package fstest

import (
	"clive/zx"
	"bytes"
)

var dirs = []string {
`c rw-r--r--      0 /Ctl
- rw-r--r--      0 /1
- rw-r--r--  30.9k /2
d rwxr-xr-x      0 /a
d rwxr-xr-x      0 /d
d rwxr-xr-x      0 /e
`,
`- rw-r--r--   9.9k /a/a1
- rw-r--r--  20.9k /a/a2
d rwxr-xr-x      0 /a/b
`,
`d rwxr-xr-x      0 /a/b/c
`,
`- rw-r--r--  43.9k /a/b/c/c3
`,
``,
`d rwxr-xr-x      0 /e/f
`,
``,
}

var dirs23 = []string {
`- rw-r--r--  30.9k /2
d rwxr-xr-x      0 /a
d rwxr-xr-x      0 /d
`,
`d rwxr-xr-x      0 /a/b
`,
``,
``,
``,
``,
``,
}

// Get all contents for a file
func get(fs zx.Getter, p string, off, count int64) ([]byte, error) {
	gc := fs.Get(p, off, count)
	data := make([]byte, 0, 1024)
	for d := range gc {
		data = append(data, d...)
	}
	return data, cerror(gc)
}

func slice(b []byte, o, c int) []byte {
	if o >= len(b) {
		return b[:0]
	}
	b = b[o:]
	if c >= len(b) {
		return b
	}
	return b[:c]
}

func Gets(t Fataler, xfs zx.Fs) {
	fs, ok := xfs.(zx.Getter)
	if !ok {
		t.Fatalf("not a getter")
	}
	for _, p := range Files {
		data, err := zx.GetAll(fs, p)
		Printf("get %s: %d bytes sts %v\n", p, len(data), err)
		if err != nil {
			t.Fatalf("get %s: %s", p, err)
		}
		if bytes.Compare(FileData[p], data) != 0 {
			t.Fatalf("get %s: bad data", p)
		}
	}

	for _, p := range Files {
		data, err := get(fs, p, 1024, 10*1024)
		Printf("get %s 1k 2k: %d bytes sts %v\n", p, len(data), err)
		if err != nil {
			t.Fatalf("get %s: %s", p, err)
		}
		if bytes.Compare(slice(FileData[p], 1024, 10*1024), data) != 0 {
			t.Fatalf("get %s: bad data", p)
		}
	}

	for i, p := range Dirs {
		Printf("get %s\n", p)
		out := ""
		gc := fs.Get(p, 0, -1)
		for b := range gc {
			dir, err := zx.UnpackDir(b)
			Printf("%s\n", dir.Fmt())
			if err != nil {
				t.Fatalf("get sts %v", err)
			}
			out += dir.Fmt() + "\n"
		}
		Printf("\n")
		if i > len(dirs) || dirs[i] != out {
			t.Fatalf("bad dir contents")
		}
	}

	for i, p := range Dirs {
		Printf("get %s 2 3\n", p)
		out := ""
		gc := fs.Get(p, 2, 3)
		for b := range gc {
			dir, err := zx.UnpackDir(b)
			Printf("%s\n", dir.Fmt())
			if err != nil {
				t.Fatalf("get sts %v", err)
			}
			out += dir.Fmt() + "\n"
		}
		Printf("\n")
		if i > len(dirs23) || dirs23[i] != out {
			t.Fatalf("bad dir contents")
		}
		if err := cerror(gc); err != nil {
			t.Fatalf("err %v", err)
		}
	}

	for _, p := range BadPaths[1:] {
		data, err := zx.GetAll(fs, p)
		Printf("get %s: %s\n", p, err)
		if err == nil || len(data) > 0 {
			t.Fatalf("could get")
		}
	}
}
