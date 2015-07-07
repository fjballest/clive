package nspace

import (
	"clive/dbg"
	"clive/net/auth"
	"clive/net/fifo"
	"clive/zx"
	"clive/zx/mfs"
	"clive/zx/fstest"
	"clive/zx/lfs"
	"clive/zx/rfs"
	"os"
	"time"
	"testing"
)

const tdir = "/tmp/lfs_test"

var (
	printf = dbg.FuncPrintf(os.Stderr, testing.Verbose)
	mktrees = mklfstrees
	moreverb = false
)


func TestParse(t *testing.T) {
	os.Args[0] = "nstest"
	s := `/	/
/tmp	*!*!lfs!main!/tmp
`

	outs := `path:"/" name:"/" type:"d" mode:"0755" proto:"lfs" spath:"/" tpath:"/"
path:"/tmp" name:"tmp" type:"d" mode:"0777" proto:"lfs" spath:"/" tpath:"/tmp"
`
	ns, err := Parse(s)
	if err != nil {
		t.Fatalf("parse: %s", err)
	}
	printf("ns is `%s`\n", ns)
	if ns.String() != outs {
		t.Fatal("bad ns")
	}

	s = `/ /
/zx	tcp!zx.lsub.org
/dump	tcp!zx.lsub.org!zx!dump
`
	ns, err = Parse(s)
	if err != nil {
		t.Fatalf("parse: %s", err)
	}
	printf("ns is `%s`\n", ns)
}

func mklfstrees(t *testing.T) (zx.RWTree, zx.RWTree) {
	fstest.MkTree(t, tdir)
	lfs1, err := lfs.New(tdir, tdir, lfs.RW)
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	lfs2, err := lfs.New(tdir, tdir, lfs.RW)
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	return lfs1, lfs2
}

func mkmfstrees(t *testing.T) (zx.RWTree, zx.RWTree) {
	lfs1, err := mfs.New(tdir)
	if err != nil {
		t.Fatalf("mfs: %s", err)
	}
	fstest.MkZXTree(t, lfs1)
	d, err := zx.Stat(lfs1, "/")
	if err != nil {
		t.Fatalf("mfs: %s", err)
	}
	zx.RegisterProcTree(lfs1, d["tpath"])
	lfs1.Dbg = testing.Verbose() && moreverb
	return lfs1, lfs1
}

func mkns(t *testing.T, d bool) *Tree {
	lfs1, lfs2 := mktrees(t)

	ns := New()
	ns.Debug = d
	ns.DebugFind = d

	root1, err := zx.Stat(lfs1, "/")
	if err != nil {
		t.Fatalf("stat: %s", err)
	}
	err = <-ns.Mount("/", root1, Repl)
	if err != nil {
		t.Fatalf("mount: %s", err)
	}

	root2, err := zx.Stat(lfs2, "/")
	if err != nil {
		t.Fatalf("stat: %s", err)
	}
	err = <-ns.Mount("/a/b", root2, Repl)
	if err != nil {
		t.Fatalf("mount: %s", err)
	}
	d1 := zx.Dir{"path": "x", "name": "x", "proto": "p1"}
	err = <-ns.Mount("/d", d1, Before)
	if err != nil {
		t.Fatalf("mount: %s", err)
	}
	d2 := zx.Dir{"path": "x", "name": "x", "proto": "p2"}
	err = <-ns.Mount("/d", d2, After)
	if err != nil {
		t.Fatalf("mount: %s", err)
	}
	err = <-ns.Mount("/d", d2, After)
	if err != nil {
		t.Fatalf("mount: %s", err)
	}
	printf("ns is `%s`\n", ns)
	return ns
}

func mkrns(t *testing.T, d bool) *Tree {
	fstest.MkTree(t, tdir)
	lfs1, err := lfs.New(tdir, tdir, lfs.RW)
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	hs, hc := fifo.NewChanHandler()
	s := fifo.New("rfs", "rfs", hs)
	if err = s.Serve(); err != nil {
		t.Fatalf("%s", err)
	}
	go func() {
		for c := range hc {
			ai, err := auth.AtServer(*c, "", "zx", "finder")
			if err!=nil && err!=auth.ErrDisabled {
				dbg.Warn("auth %s: %s\n", c.Tag, err)
				close(c.In, err)
				close(c.Out, err)
				return
			}
			rfs.Serve("srv", *c, ai, rfs.RW, lfs1)
		}
	}()
	rfs1, err := rfs.Import("fifo!*!rfs")
	if err != nil {
		t.Fatalf("lfs: %s", err)
	}
	ns := New()

	ns.Debug = d
	ns.DebugFind = d

	root1, err := zx.Stat(lfs1, "/")
	if err != nil {
		t.Fatalf("stat: %s", err)
	}
	err = <-ns.Mount("/", root1, Repl)
	if err != nil {
		t.Fatalf("mount: %s", err)
	}

	root2, err := zx.Stat(rfs1, "/")
	if err != nil {
		t.Fatalf("stat: %s", err)
	}
	err = <-ns.Mount("/a/b", root2, Repl)
	if err != nil {
		t.Fatalf("mount: %s", err)
	}
	d1 := zx.Dir{"path": "x", "name": "x", "proto": "p1"}
	err = <-ns.Mount("/d", d1, Before)
	if err != nil {
		t.Fatalf("mount: %s", err)
	}
	d2 := zx.Dir{"path": "x", "name": "x", "proto": "p2"}
	err = <-ns.Mount("/d", d2, After)
	if err != nil {
		t.Fatalf("mount: %s", err)
	}
	err = <-ns.Mount("/d", d2, After)
	if err != nil {
		t.Fatalf("mount: %s", err)
	}
	printf("ns is `%s`\n", ns)
	return ns
}

func TestRNS(t *testing.T) {
	ns := mkrns(t, false)
	defer fstest.RmTree(t, tdir)
	d2 := zx.Dir{"path": "x", "name": "x", "proto": "p2"}
	err := <-ns.Mount("d", d2, After)
	if err == nil {
		t.Fatalf("mount: could mount")
	}
	err = <-ns.Unmount("/d", d2)
	if err != nil {
		t.Fatalf("unmount dir: %s", err)
	}
	printf("ns is `%s`\n", ns)

	s := ns.String()
	ns2, err := Parse(s)
	if err != nil {
		t.Fatalf("parse: %s", err)
	}
	printf("ns2 is `%s`\n", ns2)
	if ns2.String() != s {
		t.Fatalf("parsed ns differs")
	}

	outs := `path:"/" name:"/" type:"d" mode:"0755" proto:"lfs" spath:"/" tpath:"/tmp/lfs_test"
path:"/a/b" name:"b" type:"d" mode:"0755" addr:"fifo!*!rfs" proto:"zx" spath:"/"
path:"/d" name:"d" type:"p" mode:"0644" proto:"p1"
path:"/d" name:"d" type:"p" mode:"0644" proto:"p2"
path:"/d" name:"d" type:"p" mode:"0644" proto:"p2"
`
	if s != outs {
		t.Fatalf("bad ns contents")
	}
	err = <-ns.Unmount("/a/b", nil)
	if err != nil {
		t.Fatalf("unmount dir: %s", err)
	}
	printf("ns is `%s`\n", ns)
}

func TestNS(t *testing.T) {
	ns := mkns(t, testing.Verbose())
	defer fstest.RmTree(t, tdir)
	d2 := zx.Dir{"path": "x", "name": "x", "proto": "p2"}
	err := <-ns.Mount("d", d2, After)
	if err == nil {
		t.Fatalf("mount: could mount")
	}
	err = <-ns.Unmount("/d", d2)
	if err != nil {
		t.Fatalf("unmount dir: %s", err)
	}
	printf("ns is `%s`\n", ns)

	s := ns.String()
	ns2, err := Parse(s)
	if err != nil {
		t.Fatalf("parse: %s", err)
	}
	printf("ns2 is `%s`\n", ns2)
	if ns2.String() != s {
		t.Fatalf("parsed ns differs")
	}

	outs := `path:"/" name:"/" type:"d" mode:"0755" proto:"lfs" spath:"/" tpath:"/tmp/lfs_test"
path:"/a/b" name:"b" type:"d" mode:"0755" proto:"lfs" spath:"/" tpath:"/tmp/lfs_test"
path:"/d" name:"d" type:"p" mode:"0644" proto:"p1"
path:"/d" name:"d" type:"p" mode:"0644" proto:"p2"
path:"/d" name:"d" type:"p" mode:"0644" proto:"p2"
`
	if s != outs {
		t.Fatalf("bad ns contents")
	}
	err = <-ns.Unmount("/a/b", nil)
	if err != nil {
		t.Fatalf("unmount dir: %s", err)
	}
	printf("ns is `%s`\n", ns)
}

type ResolveTest  {
	Path  string
	Dirs  []string
	Paths []string
	Fails bool
}

var resolves = []ResolveTest{
	{
		Path: "/",
		Dirs: []string{
			`path:"/" name:"/" type:"d" mode:"0755" proto:"lfs" spath:"/" tpath:"/tmp/lfs_test"`,
		},
		Paths: []string{
			`/`,
		},
	},

	{
		Path: "/a",
		Dirs: []string{
			`path:"/" name:"/" type:"d" mode:"0755" proto:"lfs" spath:"/" tpath:"/tmp/lfs_test"`,
		},
		Paths: []string{
			`/a`,
		},
	},

	{
		Path: "/a/b",
		Dirs: []string{
			`path:"/a/b" name:"b" type:"d" mode:"0755" proto:"lfs" spath:"/" tpath:"/tmp/lfs_test"`,
		},
		Paths: []string{
			`/`,
		},
	},

	{
		Path: "/a/b/a",
		Dirs: []string{
			`path:"/a/b" name:"b" type:"d" mode:"0755" proto:"lfs" spath:"/" tpath:"/tmp/lfs_test"`,
		},
		Paths: []string{
			`/a`,
		},
	},

	{
		Path: "/a/b/notthere",
		Dirs: []string{
			`path:"/a/b" name:"b" type:"d" mode:"0755" proto:"lfs" spath:"/" tpath:"/tmp/lfs_test"`,
		},
		Paths: []string{
			`/notthere`,
		},
	},
	{
		Path: "/zzzz",
		Dirs: []string{
			`path:"/" name:"/" type:"d" mode:"0755" proto:"lfs" spath:"/" tpath:"/tmp/lfs_test"`,
		},
		Paths: []string{
			`/zzzz`,
		},
	},
	{
		Path: "/d",
		Dirs: []string{
			`path:"/d" name:"d" proto:"p1"`,
			`path:"/d" name:"d" proto:"p2"`,
			`path:"/d" name:"d" proto:"p2"`,
		},
		Paths: []string{
			`/`,
			`/`,
			`/`,
		},
	},
	{
		Path:  "/d/and/not/a/finder",
		Fails: true,
	},
}

func TestResolve(t *testing.T) {
	ns := mkns(t, false)
	defer fstest.RmTree(t, tdir)
	ns.Debug = testing.Verbose()
	ns.DebugFind = testing.Verbose()

	for _, r := range resolves {
		_, dirs, paths, err := ns.Resolve(r.Path)
		printf("sts %v\n", err)
		if err!=nil && !r.Fails {
			t.Fatalf("failed with %v", err)
		}
		if err==nil && r.Fails {
			t.Fatal("didn't fail")
		}
		if len(dirs) != len(paths) {
			t.Fatal("wrong lengths")
		}
		printf("dirs:\n")
		for _, d := range dirs {
			delete(d, "Uid")
			delete(d, "Gid")
			delete(d, "Wuid")
			delete(d, "Sum")
			printf("\t`%s`,\n", d)
		}
		printf("paths:\n")
		for _, p := range paths {
			printf("\t`%s`,\n", p)
		}
		for i := 0; i<len(r.Dirs) && i<len(dirs); i++ {
			if r.Dirs[i] != dirs[i].String() {
				t.Fatalf("bad result [%d]\n\tgot %s\n\twant %s\n",
					i, dirs[i], r.Dirs[i])
			}
		}
		if r.Dirs != nil {
			if len(dirs) > len(r.Dirs) {
				t.Fatalf("unexpected %s", dirs[len(r.Dirs)])
			}
			if len(dirs) < len(r.Dirs) {
				t.Fatalf("did expect %s", r.Dirs[len(dirs)])
			}
		}
		for i := 0; i<len(r.Paths) && i<len(paths); i++ {
			if r.Paths[i] != paths[i] {
				t.Fatalf("bad result [%d]\n\tgot %s\n\twant %s\n",
					i, paths[i], r.Paths[i])
			}
		}
		if r.Paths != nil {
			if len(paths) > len(r.Paths) {
				t.Fatalf("unexpected %s", paths[len(r.Paths)])
			}
			if len(paths) < len(r.Paths) {
				t.Fatalf("did expect %s", r.Paths[len(paths)])
			}
		}
	}
}

type FindTest  {
	Path         string
	Pred         string
	Spref, Dpref string
	Depth        int
	Res          []string
	Fails        bool
}

var finds = []FindTest{
	{
		Path:  "/",
		Pred:  "depth<=1",
		Spref: "/",
		Dpref: "/",
	},

	{
		Path:  "/",
		Pred:  "",
		Spref: "/",
		Dpref: "/",
		Res: []string{
			`path / name / type d mode 0755 size 6`,
			`path /Ctl name Ctl type - mode 0644 size 0`,
			`path /1 name 1 type - mode 0644 size 0`,
			`path /2 name 2 type - mode 0644 size 31658`,
			`path /a name a type d mode 0755 size 3`,
			`path /a/a1 name a1 type - mode 0644 size 10154`,
			`path /a/a2 name a2 type - mode 0644 size 21418`,
			`path /a/b name b type d mode 0755 size 6`,
			`path /a/b/Ctl name Ctl type - mode 0644 size 0`,
			`path /a/b/1 name 1 type - mode 0644 size 0`,
			`path /a/b/2 name 2 type - mode 0644 size 31658`,
			`path /a/b/a name a type d mode 0755 size 3`,
			`path /a/b/a/a1 name a1 type - mode 0644 size 10154`,
			`path /a/b/a/a2 name a2 type - mode 0644 size 21418`,
			`path /a/b/a/b name b type d mode 0755 size 1`,
			`path /a/b/a/b/c name c type d mode 0755 size 1`,
			`path /a/b/a/b/c/c3 name c3 type - mode 0644 size 44970`,
			`path /a/b/d name d type d mode 0755 size 0`,
			`path /a/b/e name e type d mode 0755 size 1`,
			`path /a/b/e/f name f type d mode 0755 size 0`,
			`path /d name d type  mode  size `,
			`path /d name d type  mode  size `,
			`path /d name d type  mode  size `,
			`path /e name e type d mode 0755 size 1`,
			`path /e/f name f type d mode 0755 size 0`,
		},
	},
	{
		Path:  "/",
		Pred:  "type=d",
		Spref: "/",
		Dpref: "/",
		Res: []string{
			`path / name / type d mode 0755 size 6`,
			`path /a name a type d mode 0755 size 3`,
			`path /a/b name b type d mode 0755 size 6`,
			`path /a/b/a name a type d mode 0755 size 3`,
			`path /a/b/a/b name b type d mode 0755 size 1`,
			`path /a/b/a/b/c name c type d mode 0755 size 1`,
			`path /a/b/d name d type d mode 0755 size 0`,
			`path /a/b/e name e type d mode 0755 size 1`,
			`path /a/b/e/f name f type d mode 0755 size 0`,
			`path /e name e type d mode 0755 size 1`,
			`path /e/f name f type d mode 0755 size 0`,
		},
	},
	{
		Path:  "/a",
		Pred:  "type=d",
		Spref: "/",
		Dpref: "/",
		Res: []string{
			`path /a name a type d mode 0755 size 3`,
			`path /a/b name b type d mode 0755 size 6`,
			`path /a/b/a name a type d mode 0755 size 3`,
			`path /a/b/a/b name b type d mode 0755 size 1`,
			`path /a/b/a/b/c name c type d mode 0755 size 1`,
			`path /a/b/d name d type d mode 0755 size 0`,
			`path /a/b/e name e type d mode 0755 size 1`,
			`path /a/b/e/f name f type d mode 0755 size 0`,
		},
	},
	{
		Path:  "/a/b",
		Pred:  "type=d",
		Spref: "/",
		Dpref: "/",
		Res: []string{
			`path /a/b name b type d mode 0755 size 6`,
			`path /a/b/a name a type d mode 0755 size 3`,
			`path /a/b/a/b name b type d mode 0755 size 1`,
			`path /a/b/a/b/c name c type d mode 0755 size 1`,
			`path /a/b/d name d type d mode 0755 size 0`,
			`path /a/b/e name e type d mode 0755 size 1`,
			`path /a/b/e/f name f type d mode 0755 size 0`,
		},
	},
	{
		Path:  "/a/b/a/b/c",
		Pred:  "type=d",
		Spref: "/",
		Dpref: "/",
		Res: []string{
			`path /a/b/a/b/c name c type d mode 0755 size 1`,
		},
	},
	{
		Path:  "/a/b/a",
		Pred:  "-,1",
		Spref: "/",
		Dpref: "/",
		Res: []string{
			`path /a/b/a/a1 name a1 type - mode 0644 size 10154`,
			`path /a/b/a/a2 name a2 type - mode 0644 size 21418`,
		},
	},
}

func TestFindLfs(t *testing.T) {
	testFind(t)
}

func TestFindMfs(t *testing.T) {
	mktrees = mkmfstrees
	defer func() {
		mktrees = mklfstrees
	}()
	testFind(t)
}

func testFind(t *testing.T) {
	ns := mkns(t, false)
	defer fstest.RmTree(t, tdir)
	ns.Debug = testing.Verbose()
	ns.DebugFind = testing.Verbose()
	for _, f := range finds {
		printf("\nfind %s %s\n", f.Path, f.Pred)
		dc := ns.Find(f.Path, f.Pred, f.Spref, f.Spref, f.Depth)
		outs := []string{}
		for d := range dc {
			printf("got %s err %s\n", d.TestFmt(), d["err"])
			if d["err"] != "" {
				continue
			}
			if d["type"] == "c" {		// for fuse&cfs
				d["type"] = "-"
			}
			outs = append(outs, d.TestFmt())
		}
		printf("done find %s %s sts %v\n", f.Path, f.Pred, cerror(dc))
		for i := 0; i<len(f.Res) && i<len(outs); i++ {
			if outs[i] != f.Res[i] {
				t.Fatalf("bad result [%d]\n\tgot <%s>\n\twant <%s>\n", i, outs[i], f.Res[i])
			}
		}
		if f.Res != nil {
			if len(outs) > len(f.Res) {
				t.Fatalf("unexpected %s", outs[len(f.Res)])
			}
			if len(outs) < len(f.Res) {
				t.Fatalf("did expect %s", f.Res[len(outs)])
			}
		}
	}
	time.Sleep(time.Second)
}

func TestFindGet(t *testing.T) {
	ns := mkns(t, false)
	defer fstest.RmTree(t, tdir)
	ns.Debug = testing.Verbose()
	ns.DebugFind = testing.Verbose()
	for _, f := range finds {
		gc := ns.FindGet(f.Path, f.Pred, f.Spref, f.Spref, f.Depth)
		outs := []string{}
		for g := range gc {
			d := g.Dir
			printf("got %s err %s\n", d.TestFmt(), d["err"])
			if d["err"] != "" {
				continue
			}
			if d["type"] == "c" {		// for fuse&cfs
				d["type"] = "-"
			}
			outs = append(outs, d.TestFmt())
			if d["type"] != "-" {
				continue
			}
			tot := 0
			for x := range g.Datac {
				tot += len(x)
			}
			printf("got %d bytes\n", tot)
		}
		printf("done find %s %s sts %v\n", f.Path, f.Pred, cerror(gc))
		for i := 0; i<len(f.Res) && i<len(outs); i++ {
			if outs[i] != f.Res[i] {
				t.Fatalf("bad result [%d]\n\tgot %s\n\twant %s\n", i, outs[i], f.Res[i])
			}
		}
		if f.Res != nil {
			if len(outs) > len(f.Res) {
				t.Fatalf("unexpected %s", outs[len(f.Res)])
			}
			if len(outs) < len(f.Res) {
				t.Fatalf("unexpected %s", f.Res[len(outs)])
			}
		}
	}
}

func TestRFind(t *testing.T) {
	ns := mkrns(t, false)
	defer fstest.RmTree(t, tdir)
	ns.Debug = testing.Verbose()
	ns.DebugFind = testing.Verbose()
	for _, f := range finds {
		dc := ns.Find(f.Path, f.Pred, f.Spref, f.Spref, f.Depth)
		outs := []string{}
		for d := range dc {
			printf("got %s err %s\n", d.TestFmt(), d["err"])
			if d["err"] != "" {
				continue
			}
			if d["type"] == "c" {		// for fuse&cfs
				d["type"] = "-"
			}
			outs = append(outs, d.TestFmt())
		}
		printf("done find %s %s sts %v\n", f.Path, f.Pred, cerror(dc))
		for i := 0; i<len(f.Res) && i<len(outs); i++ {
			if outs[i] != f.Res[i] {
				t.Fatalf("bad result [%d]\n\tgot %s\n\twant %s\n", i, outs[i], f.Res[i])
			}
		}
		if f.Res != nil {
			if len(outs) > len(f.Res) {
				t.Fatalf("unexpected %s", outs[len(f.Res)])
			}
			if len(outs) < len(f.Res) {
				t.Fatalf("unexpected %s", f.Res[len(outs)])
			}
		}
	}
}
