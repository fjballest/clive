package fstest

import (
	"bytes"
	"clive/dbg"
	"clive/zx"
	"fmt"
)

struct putTest {
	Path  string
	Off   int64
	Dir   zx.Dir
	Fails bool
	data  []byte
}

var puts = []putTest{
	{
		Path: "/n1",
		Dir:  zx.Dir{"type": "-", "size": "0", "mode": "0640"},
	},
	{
		Path: "/n1",
		Dir:  zx.Dir{"type": "-", "size": "0", "mode": "0640"},
	},
	{
		Path: "/a/n2",
		Dir:  zx.Dir{"type": "-", "size": "0", "mode": "0640"},
	},
	{
		Path:  "/",
		Dir:   zx.Dir{"type": "-", "size": "0", "mode": "0640"},
		Fails: true,
	},
	{
		Path:  "/a",
		Dir:   zx.Dir{"type": "-", "size": "0", "mode": "0640"},
		Fails: true,
	},
	{
		Path:  "/a/b/c/d/e/f",
		Dir:   zx.Dir{"type": "-", "size": "0", "mode": "0640"},
		Fails: true,
	},
	{
		Path: "/newfile",
		Dir:  zx.Dir{"type": "-", "mode": "0600", "size": "50000"},
	},
	{
		Path:  "/newfile",
		Dir:   zx.Dir{"type": "xx"},
		Fails: true,
	},
	{
		Path:  "/newfile2",
		Dir:   zx.Dir{"mode": "0600", "size": "50000"},
		Fails: true,
	},
	{
		Path: "/n2",
		Off:  10,
		Dir:  zx.Dir{"type": "-", "size": "50000", "mode": "0640"},
	},
	{
		Path: "/n3",
		Off:  -1,
		Dir:  zx.Dir{"type": "-", "size": "50000", "mode": "0640"},
	},
}

func Puts(t Fataler, xfs zx.Fs) {
	fs, ok := xfs.(zx.Putter)
	if !ok {
		t.Fatalf("not a Putter")
	}
	// We try twice so we check both creates and rewrites
	ntry := 0
Loop:
	for i, pt := range puts {
		Printf("put #%d %s %d %s\n", i, pt.Path, pt.Off, pt.Dir.Fmt())
		dc := make(chan []byte, 1)
		xd := pt.Dir
		if pt.Off < 0 {
			pt.data = make([]byte, int(xd.Size()), int(xd.Size()))
			pt.data = append(pt.data, []byte("hola")...)
		} else if pt.Off != 0 {
			pt.data = make([]byte, pt.Off, pt.Off)
			pt.data = append(pt.data, []byte("hola")...)
			n := int(xd.Size()) - len(pt.data)
			data := make([]byte, int(xd.Size()), int(xd.Size()))
			pt.data = append(pt.data, data[:n]...)
		} else {
			pt.data = make([]byte, 0, 32*1024)
		}
		nn := i + 1
		rc := fs.Put(pt.Path, xd, pt.Off, dc)
		for i := 0; i < 1000*nn; i++ {
			var msg []byte
			if pt.Off != 0 {
				msg = []byte("hola")
			} else {
				msg = []byte(fmt.Sprintf("hi %s %d\n", pt.Path, i))
				pt.data = append(pt.data, msg...)
			}
			if ok := dc <- msg; !ok {
				err := cerror(dc)
				if !pt.Fails {
					t.Fatalf("%s: %s\n", pt.Path, err)
				}
				continue Loop
			}
			if pt.Off != 0 {
				break
			}
		}
		Printf("put %s: wrote %d bytes\n", pt.Path, len(pt.data))
		close(dc)
		rd := <-rc
		if pt.Fails {
			if rd != nil || cerror(rc) == nil {
				t.Fatalf("%s: didn't fail", pt.Path)
			}
			continue
		}
		if rd == nil || cerror(rc) != nil {
			t.Fatalf("%s: %s\n", pt.Path, cerror(rc))
		}
		got := rd.Fmt()
		Printf("got %s\n", got)
		for _, a := range []string{"mtime", "path", "mode"} {
			if xd[a] != "" && rd[a] != xd[a] {
				t.Fatalf("bad %s", a)
			}
		}
		if rd.Size() != int64(len(pt.data)) {
			t.Fatalf("bad resulting size %d vs %d", rd.Size(), len(pt.data))
		}
		pfs, ok := fs.(zx.Getter)
		if !ok {
			t.Logf("can't check file contents: not a getter")
			continue
		}
		dat, err := zx.GetAll(pfs, pt.Path)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if bytes.Compare(dat, pt.data) != 0 {
			Printf("%s\n%s\n", dbg.HexStr(dat, 30), dbg.HexStr(pt.data, 30))
			t.Fatalf("bad resulting data: %d vs %d bytes", len(dat), len(pt.data))
		}
	}
	if ntry == 0 {
		ntry++
		goto Loop
	}
}

var (
	mkdirs    = [...]string{"/nd", "/nd/nd2", "/nd/nd22", "/nd/nd23", "/nd3"}
	badmkdirs = [...]string{"/1", ".."}
)

func Mkdirs(t Fataler, xfs zx.Fs) {
	fs, ok := xfs.(zx.Putter)
	if !ok {
		t.Fatalf("not a Putter")
	}
	// We try twice so we check both creates and rewrites
	ntry := 0
Loop:
	for i, p := range mkdirs {
		xd := zx.Dir{"type": "d", "mode": "0750"}
		Printf("mkdir #%d %20s %s\n", i, p, xd)
		rc := fs.Put(p, xd, 0, nil)
		rd := <-rc
		if err := cerror(rc); err != nil {
			t.Fatalf("fails %s", err)
		}
		Printf("\t=> %s\n", rd.Fmt())
		if rd["type"] != "d" || rd["mode"] != "0750" || rd["path"] != p {
			t.Fatalf("bad result d")
		}
	}
	for i, p := range badmkdirs {
		xd := zx.Dir{"type": "d", "mode": "0750"}
		Printf("mkdir #%d %20s %s\n", i, p, xd)
		rc := fs.Put(p, xd, 0, nil)
		rd := <-rc
		if rd != nil || cerror(rc) == nil {
			t.Fatalf("did not fail")
		}
	}
	if ntry == 0 {
		ntry++
		goto Loop
	}
}
