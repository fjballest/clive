package ns

import (
	"bytes"
	"clive/dbg"
	"clive/net"
	"clive/zx"
	"clive/zx/fstest"
	"clive/zx/rzx"
	"clive/zx/zux"
	"clive/zx/zxc"
	"fmt"
	"os"
	fpath "path"
	"strings"
	"testing"
)

const tdir = "/tmp/ns_test"

var (
	verb     = false
	printf   = dbg.FlagPrintf(&verb)
	moreverb = false
	ns1      = `/ /
/tmp
/tmp	lfs!/!/tmp
/tmp	lfs!/tmp
/usr
/usr/nemo	zx!unix!8089!/tmp
path:"/x"	io:"0"	addr:"zx!unix!8089!/tmp"
`

	ns1out = `/
/tmp
/tmp	lfs!/!/tmp
/tmp
/usr
/usr/nemo	zx!unix!8089!/tmp!main!/
name:"x" type:"p" mode:"0644" path:"/x" addr:"zx!unix!8089!/tmp" io:"0"
`
)

func TestParse(t *testing.T) {
	os.Args[0] = "nstest"
	verb = testing.Verbose()
	moreverb = verb
	ns, err := Parse(ns1)
	if err != nil {
		t.Fatalf("parse: %s", err)
	}
	printf("ns is `%s`\n", ns)
	out := ns.String()
	if out != ns1out {
		t.Fatalf("bad ns")
	}
	ns2, err := Parse(out)
	if err != nil {
		t.Fatalf("parse: %s", err)
	}
	if ns2.String() != out {
		t.Fatalf("bad ns2")
	}
}

func mkns(t *testing.T, lns string) *NS {
	os.Args[0] = "nstest"
	verb = testing.Verbose()
	moreverb = verb
	ns, err := Parse(lns)
	if err != nil {
		t.Fatalf("parse: %s", err)
	}
	ns.Debug = verb
	return ns
}

struct nsTest {
	path     string
	d        zx.Dir
	isumount bool
	flag     Flag
	fails    bool
	res      string
}

var nstests = [...]nsTest{
	nsTest{
		flag:  After,
		d:     zx.Dir{"path": "foo", "addr": "fooaddr"},
		fails: true,
	},
	nsTest{
		flag: After,
		d:    zx.Dir{"path": "/foo", "addr": "fooaddr"},
		res: `/	/tmp/ns_test
/foo	fooaddr
`,
	},
	nsTest{
		flag: After,
		d:    zx.Dir{"path": "/foo", "addr": "fooaddr2"},
		res: `/	/tmp/ns_test
/foo	fooaddr
/foo	fooaddr2
`,
	},
	nsTest{
		flag: Before,
		d:    zx.Dir{"path": "/foo", "addr": "fooaddr0"},
		res: `/	/tmp/ns_test
/foo	fooaddr0
/foo	fooaddr
/foo	fooaddr2
`,
	},
	nsTest{
		flag: Repl,
		d:    zx.Dir{"path": "/foo", "addr": "fooaddrx"},
		res: `/	/tmp/ns_test
/foo	fooaddrx
`,
	},
	nsTest{
		flag: Repl,
		d:    zx.Dir{"path": "/foo/bar", "addr": "baraddr1"},
		res: `/	/tmp/ns_test
/foo	fooaddrx
/foo/bar	baraddr1
`,
	},
	nsTest{
		path:     "/foo",
		isumount: true,
		res: `/	/tmp/ns_test
/foo/bar	baraddr1
`,
	},
	nsTest{
		flag: After,
		d:    zx.Dir{"path": "/foo", "addr": "fooaddrx1"},
		res: `/	/tmp/ns_test
/foo	fooaddrx1
/foo/bar	baraddr1
`,
	},
	nsTest{
		flag: Before,
		d:    zx.Dir{"path": "/foo", "addr": "1staddr"},
		res: `/	/tmp/ns_test
/foo	1staddr
/foo	fooaddrx1
/foo/bar	baraddr1
`,
	},
	nsTest{
		path:     "/foxo",
		isumount: true,
		fails:    true,
	},

	nsTest{
		path:     "/foo",
		isumount: true,
		d:        zx.Dir{"path": "/foo", "addr": "1staddr"},
		res: `/	/tmp/ns_test
/foo	fooaddrx1
/foo/bar	baraddr1
`,
	},
	nsTest{
		path:     "/foo",
		isumount: true,
		res: `/	/tmp/ns_test
/foo/bar	baraddr1
`,
	},
}

func TestNS(t *testing.T) {
	lns := `/ ` + tdir
	ns := mkns(t, lns)
	printf("ns is `%s`\n", ns)
	for _, nst := range nstests {
		var err error
		if nst.isumount {
			err = ns.Unmount(nst.path, nst.d)
		} else {
			err = ns.Mount(nst.d, nst.flag)
		}
		if nst.fails {
			if err == nil {
				t.Fatalf("did not fail")
			}
			continue
		}
		if err != nil {
			t.Fatalf("did fail with %s", err)
		}
		printf("ns is `%s`\n", ns)
		if nst.res != "" && nst.res != ns.String() {
			t.Fatalf("bad ns")
		}
	}
}

struct resTest {
	path  string
	pref  string
	mout  string
	fails bool
}

var restests = [...]resTest{
	resTest{
		path:  ".",
		fails: true,
	},
	resTest{
		path: "/",
		pref: "/",
		mout: `name:"/" path:"/" addr:"lfs!/tmp/ns_test!/"
`,
	},
	resTest{
		path: "/a",
		pref: "/",
		mout: `name:"/" path:"/" addr:"lfs!/tmp/ns_test!/a"
`,
	},
	resTest{
		path: "/x",
		pref: "/",
		mout: `name:"/" path:"/" addr:"lfs!/tmp/ns_test!/x"
`,
	},
	resTest{
		path: "/a/b/c/d",
		pref: "/a/b/c",
		mout: `name:"c" path:"/a/b/c" addr:"lfs!/tmp/ns_test!/d"
name:"c" path:"/a/b/c" addr:"lfs!/tmp/ns_test!/d"
`,
	},
}

func TestResolve(t *testing.T) {
	lns := fmt.Sprintf("/\t%s\n/a/b/c\t%s\n/a/b/c\t%s", tdir, tdir, tdir)
	ns := mkns(t, lns)
	printf("ns is `%s`\n", ns)

	for _, nst := range restests {
		printf("resolve %s\n", nst.path)
		pref, mnts, err := ns.Resolve(nst.path)
		printf("\tpref %v\n", pref)
		mout := ""
		for _, m := range mnts {
			printf("\tmnt %s\n", m)
			mout += m.String() + "\n"
		}
		printf("\tsts %v\n", err)
		if nst.fails {
			if err == nil {
				t.Fatalf("did not fail")
			}
			continue
		}
		if err != nil {
			t.Fatalf("did fail with %s", err)
		}
		if nst.pref != "" && nst.pref != pref {
			t.Fatalf("bad prefix")
		}
		if mout != nst.mout {
			printf("mout `%s`\n", mout)
			t.Fatalf("bad mnts")
		}
	}
}

struct findTest {
	path         string
	pred         string
	spref, dpref string
	depth        int
	res          []string
	fails        bool
}

var finds = []findTest{
	findTest{
		path: "/",
		res: []string{
			`d rwxr-xr-x      0 /           addr lfs!/tmp/ns_test!/`,
			`c rw-r--r--      0 /Ctl        addr lfs!/tmp/ns_test!/Ctl`,
			`- rw-r--r--      0 /1          addr lfs!/tmp/ns_test!/1`,
			`- rw-r--r--  30.9k /2          addr lfs!/tmp/ns_test!/2`,
			`d rwxr-xr-x      0 /a          addr lfs!/tmp/ns_test!/a`,
			`- rw-r--r--   9.9k /a/a1       addr lfs!/tmp/ns_test!/a/a1`,
			`- rw-r--r--  20.9k /a/a2       addr lfs!/tmp/ns_test!/a/a2`,
			`d rwxr-xr-x      0 /a/b        addr lfs!/tmp/ns_test!/a/b`,
			`d rwxr-xr-x      0 /a/b/c      addr lfs!/tmp/ns_test!/`,
			`c rw-r--r--      0 /a/b/c/Ctl  addr lfs!/tmp/ns_test!/Ctl`,
			`- rw-r--r--      0 /a/b/c/1    addr lfs!/tmp/ns_test!/1`,
			`- rw-r--r--  30.9k /a/b/c/2    addr lfs!/tmp/ns_test!/2`,
			`d rwxr-xr-x      0 /a/b/c/a    addr lfs!/tmp/ns_test!/a`,
			`- rw-r--r--   9.9k /a/b/c/a/a1 addr lfs!/tmp/ns_test!/a/a1`,
			`- rw-r--r--  20.9k /a/b/c/a/a2 addr lfs!/tmp/ns_test!/a/a2`,
			`d rwxr-xr-x      0 /a/b/c/a/b  addr lfs!/tmp/ns_test!/a/b`,
			`d rwxr-xr-x      0 /a/b/c/a/b/c addr lfs!/tmp/ns_test!/a/b/c`,
			`- rw-r--r--  43.9k /a/b/c/a/b/c/c3 addr lfs!/tmp/ns_test!/a/b/c/c3`,
			`d rwxr-xr-x      0 /a/b/c/d    addr lfs!/tmp/ns_test!/d`,
			`d rwxr-xr-x      0 /a/b/c/e    addr lfs!/tmp/ns_test!/e`,
			`d rwxr-xr-x      0 /a/b/c/e/f  addr lfs!/tmp/ns_test!/e/f`,
			`d rwxr-xr-x      0 /a/b/c      addr lfs!/tmp/ns_test!/`,
			`c rw-r--r--      0 /a/b/c/Ctl  addr lfs!/tmp/ns_test!/Ctl`,
			`- rw-r--r--      0 /a/b/c/1    addr lfs!/tmp/ns_test!/1`,
			`- rw-r--r--  30.9k /a/b/c/2    addr lfs!/tmp/ns_test!/2`,
			`d rwxr-xr-x      0 /a/b/c/a    addr lfs!/tmp/ns_test!/a`,
			`- rw-r--r--   9.9k /a/b/c/a/a1 addr lfs!/tmp/ns_test!/a/a1`,
			`- rw-r--r--  20.9k /a/b/c/a/a2 addr lfs!/tmp/ns_test!/a/a2`,
			`d rwxr-xr-x      0 /a/b/c/a/b  addr lfs!/tmp/ns_test!/a/b`,
			`d rwxr-xr-x      0 /a/b/c/a/b/c addr lfs!/tmp/ns_test!/a/b/c`,
			`- rw-r--r--  43.9k /a/b/c/a/b/c/c3 addr lfs!/tmp/ns_test!/a/b/c/c3`,
			`d rwxr-xr-x      0 /a/b/c/d    addr lfs!/tmp/ns_test!/d`,
			`d rwxr-xr-x      0 /a/b/c/e    addr lfs!/tmp/ns_test!/e`,
			`d rwxr-xr-x      0 /a/b/c/e/f  addr lfs!/tmp/ns_test!/e/f`,
			`d rwxr-xr-x      0 /d          addr lfs!/tmp/ns_test!/d`,
			`d rwxr-xr-x      0 /e          addr lfs!/tmp/ns_test!/e`,
			`d rwxr-xr-x      0 /e/f        addr lfs!/tmp/ns_test!/e/f`,
		},
	},
	findTest{
		path: "/",
		pred: "1",
		res: []string{
			`d rwxr-xr-x      0 /           addr lfs!/tmp/ns_test!/`,
			`c rw-r--r--      0 /Ctl        addr lfs!/tmp/ns_test!/Ctl`,
			`- rw-r--r--      0 /1          addr lfs!/tmp/ns_test!/1`,
			`- rw-r--r--  30.9k /2          addr lfs!/tmp/ns_test!/2`,
			`d rwxr-xr-x      0 /a          addr lfs!/tmp/ns_test!/a`,
			`d rwxr-xr-x      0 /d          addr lfs!/tmp/ns_test!/d`,
			`d rwxr-xr-x      0 /e          addr lfs!/tmp/ns_test!/e`,
		},
	},
	findTest{
		path: "/a/b",
		pred: "d",
		res: []string{
			`d rwxr-xr-x      0 /a/b        addr lfs!/tmp/ns_test!/a/b`,
			`d rwxr-xr-x      0 /a/b/c      addr lfs!/tmp/ns_test!/`,
			`d rwxr-xr-x      0 /a/b/c/a    addr lfs!/tmp/ns_test!/a`,
			`d rwxr-xr-x      0 /a/b/c/a/b  addr lfs!/tmp/ns_test!/a/b`,
			`d rwxr-xr-x      0 /a/b/c/a/b/c addr lfs!/tmp/ns_test!/a/b/c`,
			`d rwxr-xr-x      0 /a/b/c/d    addr lfs!/tmp/ns_test!/d`,
			`d rwxr-xr-x      0 /a/b/c/e    addr lfs!/tmp/ns_test!/e`,
			`d rwxr-xr-x      0 /a/b/c/e/f  addr lfs!/tmp/ns_test!/e/f`,
			`d rwxr-xr-x      0 /a/b/c      addr lfs!/tmp/ns_test!/`,
			`d rwxr-xr-x      0 /a/b/c/a    addr lfs!/tmp/ns_test!/a`,
			`d rwxr-xr-x      0 /a/b/c/a/b  addr lfs!/tmp/ns_test!/a/b`,
			`d rwxr-xr-x      0 /a/b/c/a/b/c addr lfs!/tmp/ns_test!/a/b/c`,
			`d rwxr-xr-x      0 /a/b/c/d    addr lfs!/tmp/ns_test!/d`,
			`d rwxr-xr-x      0 /a/b/c/e    addr lfs!/tmp/ns_test!/e`,
			`d rwxr-xr-x      0 /a/b/c/e/f  addr lfs!/tmp/ns_test!/e/f`,
		},
	},
	findTest{
		path: "/a/b/c/a",
		pred: "d",
		res: []string{
			`d rwxr-xr-x      0 /a/b/c/a    addr lfs!/tmp/ns_test!/a`,
			`d rwxr-xr-x      0 /a/b/c/a/b  addr lfs!/tmp/ns_test!/a/b`,
			`d rwxr-xr-x      0 /a/b/c/a/b/c addr lfs!/tmp/ns_test!/a/b/c`,
			`d rwxr-xr-x      0 /a/b/c/a    addr lfs!/tmp/ns_test!/a`,
			`d rwxr-xr-x      0 /a/b/c/a/b  addr lfs!/tmp/ns_test!/a/b`,
			`d rwxr-xr-x      0 /a/b/c/a/b/c addr lfs!/tmp/ns_test!/a/b/c`,
		},
	},
}

func TestNSFinds(t *testing.T) {
	os.RemoveAll(tdir)
	fstest.MkTree(t, tdir)
	defer os.RemoveAll(tdir)
	lns := fmt.Sprintf("/\t%s\n/a/b/c\t%s\n/a/b/c\t%s", tdir, tdir, tdir)
	ns := mkns(t, lns)
	printf("ns is `%s`\n", ns)
	ns.Debug = false
	ns.Verb = false
	AddLfsPath(tdir, nil)
	for _, f := range finds {
		printf("find %s %s\n", f.path, f.pred)
		rc := ns.Find(f.path, f.pred, f.spref, f.dpref, f.depth)
		n := 0
		for d := range rc {
			s := fmt.Sprintf("%-30s addr %s", d.Fmt(), d["addr"])
			printf("\t`%s`,\n", s)
			if d["err"] != "" {
				printf("\t\terr %s\n", d["err"])
			}
			if f.res != nil {
				if n >= len(f.res) || f.res[n] != s {
					t.Fatalf("bad finding")
				}
			}
			n++
		}
		if f.res != nil && n != len(f.res) {
			t.Fatalf("missing findings")
		}
		err := cerror(rc)
		printf("sts %v\n", err)
		if f.fails {
			if err == nil {
				t.Fatalf("did not fail")
			}
			continue
		}
		if err != nil {
			t.Fatalf("did fail with %s", err)
		}
	}
}

func chkdata(addr string, data []byte) bool {
	a := "lfs!/tmp/ns_test"
	if !strings.HasPrefix(addr, a) {
		return true
	}
	addr = addr[len(a):]
	if addr == "" {
		addr = "/"
	}
	return bytes.Compare(fstest.FileData[addr], data) == 0
}

func TestNSFindGets(t *testing.T) {
	os.RemoveAll(tdir)
	fstest.MkTree(t, tdir)
	defer os.RemoveAll(tdir)
	lns := fmt.Sprintf("/\t%s\n/a/b/c\t%s\n/a/b/c\t%s", tdir, tdir, tdir)
	ns := mkns(t, lns)
	printf("ns is `%s`\n", ns)
	ns.Debug = false
	ns.Verb = false
	AddLfsPath(tdir, nil)
	for _, f := range finds {
		printf("find %s %s\n", f.path, f.pred)
		rc := ns.FindGet(f.path, f.pred, f.spref, f.dpref, f.depth)
		n := 0
		dat := []byte{}
		last := ""
		for d := range rc {
			printf("got %T\n", d)
			if d, ok := d.([]byte); ok {
				dat = append(dat, d...)
				continue
			}
			d, ok := d.(zx.Dir)
			if !ok {
				continue
			}
			if !chkdata(last, dat) {
				t.Fatalf("bad data for %s", last)
			}
			last = d.SPath()
			dat = dat[:0]
			s := fmt.Sprintf("%-30s addr %s", d.Fmt(), d["addr"])
			printf("\t`%s`,\n", s)
			if d["err"] != "" {
				printf("\t\terr %s\n", d["err"])
			}
			if f.res != nil {
				if n >= len(f.res) || f.res[n] != s {
					t.Fatalf("bad finding")
				}
			}
			n++
		}
		if !chkdata(last, dat) {
			t.Fatalf("bad data for %s", last)
		}
		if f.res != nil && n != len(f.res) {
			t.Fatalf("missing findings")
		}
		err := cerror(rc)
		printf("sts %v\n", err)
		if f.fails {
			if err == nil {
				t.Fatalf("did not fail")
			}
			continue
		}
		if err != nil {
			t.Fatalf("did fail with %s", err)
		}
	}
}

func runTest(t *testing.T, fn fstest.TestFunc) {
	os.RemoveAll(tdir)
	fstest.MkTree(t, tdir)
	defer os.RemoveAll(tdir)
	lns := fmt.Sprintf("/\t%s\n", tdir)
	ns := mkns(t, lns)
	printf("ns is `%s`\n", ns)
	fstest.Verb = testing.Verbose()
	ns.Debug = false
	ns.Verb = false
	AddLfsPath(tdir, nil)
	if fn != nil {
		fn(t, ns)
	}
}

func TestFs(t *testing.T) {
	runTest(t, nil)
}

func TestStats(t *testing.T) {
	runTest(t, fstest.Stats)
}

func TestGetCtl(t *testing.T) {
	runTest(t, fstest.GetCtl)
}

func TestGets(t *testing.T) {
	runTest(t, fstest.Gets)
}

func TestPuts(t *testing.T) {
	runTest(t, fstest.Puts)
}

func TestMkdirs(t *testing.T) {
	runTest(t, fstest.Mkdirs)
}

func TestRemoves(t *testing.T) {
	runTest(t, fstest.Removes)
}

func TestWstats(t *testing.T) {
	runTest(t, fstest.Wstats)
}

func TestAsAFile(t *testing.T) {
	runTest(t, fstest.AsAFile)
}

func TestFinds(t *testing.T) {
	runTest(t, fstest.Finds)
}

func TestFindGets(t *testing.T) {
	runTest(t, fstest.FindGets)
}

func runRfsTest(t *testing.T, fn fstest.TestFunc) {
	delLfsPath("/")
	os.Args[0] = "ns.test"
	os.Remove("/tmp/clive.9898")
	defer os.Remove("/tmp/clive.9898")
	os.RemoveAll(tdir)
	fstest.MkTree(t, tdir)
	defer os.RemoveAll(tdir)

	fs, err := zux.NewZX(tdir)
	if err != nil {
		t.Fatal(err)
	}
	cfs, err := zxc.New(fs)
	if err != nil {
		t.Fatal(err)
	}
	defer cfs.Close()

	ccfg, err := net.TLSCfg("/Users/nemo/.ssh/client")
	if err != nil {
		t.Logf("no certs found, no tls conn")
	}
	scfg, err := net.TLSCfg("/Users/nemo/.ssh/server")
	if err != nil || ccfg == nil {
		ccfg = nil
		scfg = nil
		t.Logf("no certs found, no tls conn")
	}
	srv, err := rzx.NewServer("unix!local!9898", scfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Serve("main", cfs); err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	rfs, err := rzx.Dial("unix!local!9898", ccfg)
	if err != nil {
		t.Fatal(err)
	}
	rfs.Debug = testing.Verbose()
	ts := rfs.Trees()
	t.Logf("trees: %v", ts)

	lns := fmt.Sprintf("/\tzx!unix!local!9898")
	ns := mkns(t, lns)
	printf("ns is `%s`\n", ns)
	ns.Debug = false
	ns.Verb = false
	if fn != nil {
		fn(t, ns)
	}
}

func TestRfsStats(t *testing.T) {
	delLfsPath("/")
	runRfsTest(t, fstest.Stats)
}

func TestRfsFinds(t *testing.T) {
	delLfsPath("/")
	runRfsTest(t, fstest.Finds)
}

func TestPaths(t *testing.T) {
	verb = testing.Verbose()
	os.RemoveAll(tdir + "empty")
	os.RemoveAll(tdir)
	os.MkdirAll(tdir+"empty/mnt", 0755)
	fstest.MkTree(t, tdir)
	defer os.RemoveAll(tdir)
	defer os.RemoveAll(tdir + "empty")
	AddLfsPath(tdir+"empty", nil)
	AddLfsPath(tdir, nil)
	lns := `/ ` + tdir + `empty
	/mnt	` + tdir
	ns := mkns(t, lns)
	ns.Debug = testing.Verbose()
	printf("ns is `%s`\n", ns)
	dc := ns.Stat("/mnt/a/a1")
	d := <-dc
	printf("sts %v\n", cerror(dc))
	printf("got %s\n", d.TestFmt())
	if d["path"] != "/mnt/a/a1" {
		t.Fatalf("bad path")
	}
	dirs, err := zx.GetDir(ns, "/mnt/a")
	printf("sts %v\n", err)
	for _, d := range dirs {
		printf("dir %s\n", d.TestFmt())
		if d["path"] != d.SPath() {
			t.Fatalf("bad getdir path")
		}
	}
	if len(dirs) != 3 {
		t.Fatalf("bad nb of dirs in getdir")
	}
	printf("\nfind /mnt...\n")
	// ns.Verb = testing.Verbose()
	dc = ns.Find("/mnt", "type=d", "/", "/", 0)
	n := 0
	for d := range dc {
		printf("found %s\n", d.TestFmt())
		if d["path"] != fpath.Join("/mnt", d.SPath()) {
			t.Fatalf("bad find path")
		}
		n++
	}
	if n != 7 {
		t.Fatalf("bad nb of dirs in find")
	}
	printf("\nfind /...\n")
	// ns.Verb = testing.Verbose()
	dc = ns.Find("/", "type=d", "/", "/", 0)
	for d := range dc {
		printf("found %s\n", d.TestFmt())
		if d["path"] != fpath.Join("/mnt", d.SPath()) {
			t.Fatalf("bad find path")
		}
		n++
	}
	if n != 8 {
		t.Fatalf("bad nb of dirs in find")
	}
}
