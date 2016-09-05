package zx

import (
	"bytes"
	"clive/ch"
	"clive/dbg"
	"fmt"
	"io"
	"os"
	fpath "path"
	"testing"
)

var (
	debug  bool
	printf = dbg.FlagPrintf(&debug)
)

func TestAddr(t *testing.T) {
	debug = testing.Verbose()
	addrs := [...]string{
		"foo.c",
		"foo.c:3",
		"foo.c:3,5",
		":3",
		":3,5",
		"foo.c:#3",
		"foo.c:#3,#5",
		":#3",
		":#3,#5",
		"foo.c:3:#5,#7",
		"foo.c:3,4:#5,#7",
	}
	outs := [...]string{
		"foo.c:#0,#0",
		"foo.c:3",
		"foo.c:3,5",
		"in:3",
		"in:3,5",
		"foo.c:#3,#3",
		"foo.c:#3,#5",
		"in:#3,#3",
		"in:#3,#5",
		"foo.c:3:#5,#7",
		"foo.c:3,4:#5,#7",
	}
	for i, a := range addrs {
		xa := ParseAddr(a)
		printf("%q -> %s\n", a, xa)
		if xa.String() != outs[i] {
			t.Fatalf("bad addr")
		}
	}
}

func TestPaths(t *testing.T) {
	debug = testing.Verbose()

	prefs := [...]string{"", "/", "/a", "/b", "/a/b", "..", "z", "/c/d", "", "/a/b/..", "/a/b/../..", "/a/b/../../.."}
	suffs := [...]string{"", "", "/", "", "/b", "", "", "", "", "/", "", ""}
	rsuffs := [...]string{"", "/", "/a", "/b", "/a/b", "", "", "/c/d", "", "/a", "/", "/"}

	r := Suffix("/a/b", "/a")
	printf("suff /a/b /a %q\n", r)
	if r != "/b" {
		t.Fatalf("bad suffix")
	}
	for i, p := range prefs {
		r := Suffix(p, "/a")
		printf("suff %q %q ->  %q\n", p, "/a", r)
		if suffs[i] != r {
			t.Fatalf("bad /a suffix")
		}
		r = Suffix(p, "/")
		printf("suff %q %q ->  %q\n", p, "/", r)
		if rsuffs[i] != r {
			t.Fatalf("bad / suffix")
		}
		r = Suffix(p, "")
		printf("suff%q ''  ->  %q\n", p, r)
		if p == "" {
			if r != "" {
				t.Fatalf("bad '' suffix")
			}
			continue
		}
		if r != fpath.Clean(p) {
			t.Fatalf("bad '' suffix")
		}
	}
}

func TestPack(t *testing.T) {
	var buf bytes.Buffer
	var a1 Addr
	a2 := Addr{"a file", 1, 2, 3, 4}
	var d1 Dir
	d2 := Dir{"key1": "val1", "Key2": ""}
	n, err := ch.WriteMsg(&buf, 1, a1)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("+%d\tsz = %d\n", n, buf.Len())
	n, err = ch.WriteMsg(&buf, 1, a2)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("+%d\tsz = %d\n", n, buf.Len())
	n, err = ch.WriteMsg(&buf, 1, d1)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("+%d\tsz = %d\n", n, buf.Len())
	n, err = ch.WriteMsg(&buf, 1, d2)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("+%d\tsz = %d\n", n, buf.Len())

	outs := []string{
		"30 1 zx.Addr :0,0 <nil>",
		"36 1 zx.Addr a file:1,2 <nil>",
		"14 1 zx.Dir  <nil>",
		`42 1 zx.Dir Key2:"" key1:"val1" <nil>`,
	}

	for _, s := range outs {
		n, tag, m, err := ch.ReadMsg(&buf)
		t.Logf("%d %d %T %v %v\n", n, tag, m, m, err)
		if s != "" && s != fmt.Sprintf("%d %d %T %v %v", n, tag, m, m, err) {
			t.Fatal("bad msg")
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

struct tb {
	r io.ReadCloser
	w io.WriteCloser
}

func (b *tb) Write(dat []byte) (int, error) {
	return b.w.Write(dat)
}

func (b *tb) Read(dat []byte) (int, error) {
	return b.r.Read(dat)
}

func (b *tb) CloseWrite() error {
	return b.w.Close()
}

func (b *tb) CloseRead() error {
	return b.r.Close()
}

func TestConn(t *testing.T) {
	fd := &tb{}
	fd.r, fd.w, _ = os.Pipe()
	p := ch.NewConn(fd, 300, nil)
	d1 := Dir{}
	d2 := Dir{"key1": "val1", "key2": ""}
	d3 := Dir{"Key1": "val1", "Key2": ""}
	a1 := Addr{}
	a2 := Addr{"a file", 1, 2, 3, 4}
	p.Out <- d1
	p.Out <- d2
	p.Out <- d3
	p.Out <- a1
	p.Out <- ErrBug
	p.Out <- a2
	close(p.Out)
	outs := []string{
		`zx.Dir, `,
		`zx.Dir, key1:"val1" key2:""`,
		`zx.Dir, Key1:"val1" Key2:""`,
		`zx.Addr, :0,0`,
		`*errors.errorString, buggered or not implemented`,
		`zx.Addr, a file:1,2`,
	}
	for o := range p.In {
		out := fmt.Sprintf("%T, %s", o, o)
		t.Logf("\t`%s`,\n", out)
		if len(outs) == 0 || outs[0] != out {
			t.Fatalf("bad output")
		}
		outs = outs[1:]
	}
	t.Logf("sts %v", cerror(p.In))
}

func TestDir(t *testing.T) {
	d := Dir{
		"name":  "f3",
		"type":  "-",
		"mode":  "0644",
		"size":  "23",
		"mtime": "4000000000",
		"foo":   `quoted "bar"`,
		"foo 2": "quoted` `bar2",
	}
	ds := d.String()
	printf("dir is:\n%s\n", ds)
	nd, err := ParseDir(ds)
	if err != nil {
		t.Fatalf("parse %s", err)
	}
	printf("parsed: %s\n", nd)
	if nd.String() != ds {
		t.Fatal("unexpected parsed dir string")
	}
	ds = ds + `"`
	if _, err := ParseDir(ds); err == nil {
		t.Fatalf("parse didn't fail")
	}
	d2 := Dir{}
	ds = d2.String()
	printf("dir is:\n%s\n", ds)
	nd, err = ParseDir(ds)
	if err != nil {
		t.Fatalf("parse %s", err)
	}
	printf("parsed: %s\n", d2)
	if d2.String() != ds {
		t.Fatalf("unexpected parsed dir string")
	}

}

struct ptest {
	p, e string
	m bool
}

func TestPathPrefixMatch(t *testing.T) {
	debug = testing.Verbose()
	ts := []ptest {
		{"/a/b/c", "a", true},
		{"/a/b/c", "a.*", false},
		{"/a/b/c", "*1", false},
		{"/a/b/c", "a[1]*", false},
		{"/a/b/c", "/", true},
		{"/a/b/c", "/a/*1", false},
		{"/a/b/c", "/a/*1/a*", false},
		{"/a/a1/a11", "a", true},
		{"/a/a1/a11", "a.*", false},
		{"/a/a1/a11", "*1", true},
		{"/a/a1/a11", "a[1]*", true},
		{"/a/a1/a11", "/", true},
		{"/a/a1/a11", "/a/*1", true},
		{"/a/a1/a11", "/a/*1/a*", true},
		{"/a/a1/g11", "a", true},
		{"/a/a1/g11", "a.*", false},
		{"/a/a1/g11", "*1", true},
		{"/a/a1/g11", "a[1]*", true},
		{"/a/a1/g11", "/", true},
		{"/a/a1/g11", "/a/*1", true},
		{"/a/a1/g11", "/a/*1/a*", false},
		{"/b/b1/b11", "a", false},
		{"/b/b1/b11", "a.*", false},
		{"/b/b1/b11", "*1", true},
		{"/b/b1/b11", "a[1]*", false},
		{"/b/b1/b11", "/", true},
		{"/b/b1/b11", "/a/*1", false},
		{"/b/b1/b11", "/a/*1/a*", false},
		{"/", "a", false},
		{"/", "a.*", false},
		{"/", "*1", false},
		{"/", "a[1]*", false},
		{"/", "/", true},
		{"/", "/a/*1", false},
		{"/", "/a/*1/a*", false},
	}
	paths := []string {
		"/a/b/c",
		"/a/a1/a11",
		"/a/a1/g11",
		"/b/b1/b11",
		"/",
	}
	exprs := []string {
		"a",
		"a.*",
		"*1",
		"a[1]*",
		"/",
		"/a/*1",
		"/a/*1/a*",
	}
	i := 0
	for _, p := range paths {
		for _, e := range exprs {
			v := PathPrefixMatch(p, e)
			printf("\t{\"%s\", \"%s\", %v},\n", p, e, v)
			if v != ts[i].m {
				t.Fatalf("bad match")
			}
			i++
		}
	}
}
