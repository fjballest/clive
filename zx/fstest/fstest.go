/*
	Utilities to aid in tests of zx file systems packages
*/
package fstest

import (
	"bytes"
	"clive/dbg"
	"clive/zx"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
	"time"
)

/*
	The tree created is as follows:

	/
		1
		2
		a/
			a1
			a2
			b/
				c/
					c3
		d/
		e/
			f/
*/

// Usually testing.T or testing.B
type Fataler interface {
	Fatalf(format string, args ...interface{})
	Logf(format string, args ...interface{})
	Fail()
}

var (
	printf = dbg.FuncPrintf(os.Stdout, testing.Verbose)
	xt     int64

	// directories created
	Dirs = [...]string{"/a", "/a/b", "/a/b/c", "/d", "/e", "/e/f"}

	// files created
	Files = [...]string{"/1", "/a/a1", "/a/a2", "/a/b/c/c3", "/2"}

	// data stored in each file
	FileData = map[string][]byte{}

	Repeats = 1
)

// Make some changes in the test tree.
//	- Touch /a/a1
//	- Chmod /a/a2
//	- Remove /a/b/c /a/b/c/c3
//	- Create /a/n /a/n/m /a/n/m/m1
func MkChgs(t Fataler, tdir string) {
	Touch(tdir + "/a/a1")
	if err := os.Chmod(tdir+"/a/a2", 0750); err != nil {
		t.Fatalf("chmod: %s", err)
	}
	if err := os.RemoveAll(tdir + "/a/b/c"); err != nil {
		t.Fatalf("rm: %s", err)
	}
	if err := os.MkdirAll(tdir+"/a/n/m", 0750); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	if err := ioutil.WriteFile(tdir+"/a/n/m/m1", []byte("a new file\n"), 0640); err != nil {
		t.Fatalf("new file: %s", err)
	}
	Touch(tdir + "/a/n/m/m1")
	Touch(tdir + "/a/n/m")
	Touch(tdir + "/a/n")
	Touch(tdir + "/a")
}

// Make some changes in the test zx tree.
//	- Touch /a/a1
//	- Chmod /a/a2
//	- Remove /a/b/c /a/b/c/c3
//	- Create /a/n /a/n/m /a/n/m/m1
func MkZXChgs(t Fataler, fs zx.RWTree) {
	TouchZX(fs, "/a/a1")
	if err := <-fs.Wstat("/a/a2", zx.Dir{"mode": "0750"}); err != nil {
		t.Fatalf("chmod: %s", err)
	}
	if err := <-fs.RemoveAll("/a/b/c"); err != nil {
		t.Fatalf("rm: %s", err)
	}
	<-fs.Mkdir("/a", zx.Dir{"mode": "0750"})
	<-fs.Mkdir("/a/n", zx.Dir{"mode": "0750"})
	if err := <-fs.Mkdir("/a/n/m", zx.Dir{"mode": "0750"}); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	err := zx.PutAll(fs, "/a/n/m/m1", zx.Dir{"mode": "0640"}, []byte("a new file\n"))
	if err != nil {
		t.Fatalf("new file: %s", err)
	}
	TouchZX(fs, "/a/n/m/m1")
	TouchZX(fs, "/a/n/m")
	TouchZX(fs, "/a/n")
	TouchZX(fs, "/a")
}

// Make some changes in the test tree, another version.
//	- Remove /2
//	- Create /2/n2
//	- Truncate /1
func MkChgs2(t Fataler, tdir string) {
	if err := os.Remove(tdir + "/2"); err != nil {
		t.Fatalf("rm: %s", err)
	}
	if err := os.MkdirAll(tdir+"/2/n2", 0750); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	Touch(tdir + "/2/n2")
	Touch(tdir + "/2")
	if err := os.Truncate(tdir+"/1", 50); err != nil {
		t.Fatalf("truncate: %s", err)
	}
	Touch(tdir + "/1")
}

// Make some changes in the test zx tree, another version.
//	- Remove /2
//	- Create /2/n2
//	- Truncate /1
func MkZXChgs2(t Fataler, fs zx.RWTree) {
	if err := <-fs.Remove("/2"); err != nil {
		t.Fatalf("rm: %s", err)
	}
	<-fs.Mkdir("/2", zx.Dir{"mode": "0750"})
	if err := <-fs.Mkdir("/2/n2", zx.Dir{"mode": "0750"}); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	TouchZX(fs, "/2/n2")
	TouchZX(fs, "/2")
	if err := <-fs.Wstat("/1", zx.Dir{"size": "50"}); err != nil {
		t.Fatalf("truncate: %s", err)
	}
	TouchZX(fs, "/1")
}

// Reset the time for files created
func ResetTime() {
	xt = 0
}

// set a fake mtime that can be predicted.
func Touch(path string) {
	tm := time.Unix(xt/1e9, xt%1e9)
	os.Chtimes(path, tm, tm)
	xt += 1e9
}

// set a fake mtime that can be predicted.
func TouchZX(fs zx.RWTree, path string) error {
	d := zx.Dir{}
	d.SetTime("mtime", time.Unix(xt/1e9, xt%1e9))
	xt += 1e9
	return <-fs.Wstat(path, d)
}

// Create a tree with Dirs and Files at tdir at the underlying OS
func MkTree(t Fataler, tdir string) {
	os.Args[0] = "fstest"
	os.RemoveAll(tdir)
	if err := os.MkdirAll(tdir, 0755); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	for i := range Dirs {
		dn := tdir + Dirs[i]
		if err := os.MkdirAll(dn, 0755); err != nil {
			t.Fatalf("mkdir: %s", err)
		}
	}

	for i := range Files {
		data := []byte{}
		for k := 0; k < i*1024; k++ {
			txt := fmt.Sprintf("%s %d\n", Files[i], k)
			data = append(data, txt...)
		}
		fn := tdir + Files[i]
		if err := ioutil.WriteFile(fn, data, 0644); err != nil {
			t.Fatalf("writefile: %s", err)
		}
		FileData[Files[i]] = data
	}
	for i := range Files {
		Touch(tdir + Files[i])
	}
	for i := len(Dirs) - 1; i >= 0; i-- {
		Touch(tdir + Dirs[i])
	}
}

// Create a tree with Dirs and Files at tdir at the given zx tree.
func MkZXTree(t Fataler, fs zx.RWTree) {
	for _, dn := range Dirs {
		if err := zx.MkdirAll(fs, dn, zx.Dir{"mode": "0755"}); err != nil {
			t.Fatalf("mkdir: %s", err)
		}
	}
	for i, fn := range Files {
		data := []byte{}
		for k := 0; k < i*1024; k++ {
			txt := fmt.Sprintf("%s %d\n", Files[i], k)
			data = append(data, txt...)
		}
		if err := zx.PutAll(fs, fn, zx.Dir{"mode": "0644"}, data); err != nil {
			t.Fatalf("putall: %s", err)
		}
		FileData[Files[i]] = data
	}
	for _, fn := range Files {
		if err := TouchZX(fs, fn); err != nil {
			t.Fatalf("touch: %s: %s", fn, err)
		}
	}
	for _, dn := range Dirs {
		if err := TouchZX(fs, dn); err != nil {
			t.Fatalf("touch: %s: %s", dn, err)
		}
	}
}

func RmTree(t Fataler, tdir string) {
	os.RemoveAll(tdir)
}

func EqFiles(t Fataler, path string, fss ...zx.Tree) {
	dirs := []zx.Dir{}
	nerrs := 0
	for _, fs := range fss {
		d, err := zx.Stat(fs, path)
		if err != nil {
			printf("%s: not there: %s\n", fs.Name(), err)
			nerrs++
			continue
		}
		printf("%s: %s\n", fs.Name(), d.LongTestFmt())
		dirs = append(dirs, d)
	}
	if nerrs > 0 {
		if nerrs != len(fss) {
			t.Fatalf("exists only in some")
		}
		return
	}
	for i := 1; i < len(dirs); i++ {
		if dirs[i].TestFmt() != dirs[0].TestFmt() {
			t.Logf("dir%d %s\n", i, dirs[i].TestFmt())
			t.Logf("dir0 %s\n", dirs[0].TestFmt())
			t.Fatalf("dirs do not match")
		}
	}
	if dirs[0]["type"] == "d" {
		return
	}
	datas := [][]byte{}
	for _, fs := range fss {
		dat, err := zx.GetAll(fs, path)
		if err != nil {
			t.Fatalf("%s: %s: %s", fs, path, err)
		}
		datas = append(datas, dat)
	}
	for i := 1; i < len(datas); i++ {
		if bytes.Compare(datas[i], datas[0]) != 0 {
			t.Fatalf("data differs")
		}
	}

}

type StatTest struct {
	Path  string
	Name  string
	Res   string
	Fails bool
}

var StatTests = []StatTest{
	{"/a/b/c", "c", `path /a/b/c name c type d mode 0755 size 1`, false},
	{"/a", "a", `path /a name a type d mode 0755 size 3`, false},
	{"/a/b/c/c3", "c3", `path /a/b/c/c3 name c3 type - mode 0644 size 44970`, false},
	{"/2", "2", `path /2 name 2 type - mode 0644 size 31658`, false},
	{"zz", "", ``, true},
	{"..", "", ``, true},
	{"/a/b/c/c5", "", ``, true},
	{"/a/../../..", "/", `path / name / type d mode 0755 size xxx`, false},
}

func Stats(t Fataler, fss ...zx.Tree) {
	for i := 0; i < Repeats; i++ {
		for _, fs := range fss {
			for _, st := range StatTests {
				printf("stat %s\n", st.Path)
				dc := fs.Stat(st.Path)
				d := <-dc
				if st.Fails {
					if d != nil || cerror(dc) == nil {
						t.Fatalf("stat %s didn't fail", st.Path)
					}
					continue
				}
				if d == nil || cerror(dc) != nil {
					t.Fatalf("stat %s: %s", st.Path, cerror(dc))
				}
				printf("got %s\n", d.LongTestFmt())
				if d["path"] == "/" && st.Name == "/" {
					continue
					// ignore: ctl, chg
				}
				if s := d.TestFmt(); s != st.Res {
					t.Logf("got  <%v>", s)
					t.Logf("want <%v>", st.Res)
					t.Fatalf("bad output for %s %v", st.Path, s == st.Res)
				}
			}
			printf("\n")
		}
	}
}

type MoveTest struct {
	From, To string
	Fails    bool
	Child    string
	Res      string
}

// Use size X for /
var MoveTests = []MoveTest{
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

func Moves(t Fataler, fss ...zx.Tree) {
	fs := fss[0].(zx.RWTree)
	for _, mv := range MoveTests {
		printf("mv %s %s\n", mv.From, mv.To)
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
		d, err := zx.Stat(fs, mv.To)
		if err != nil {
			t.Fatalf("stat %s: %s", mv.From, err)
		}
		delete(d, "Sum")
		r := d.LongTestFmt()
		printf("new stat: %s\n", r)
		if strings.Contains(mv.Res, "size X") { // as is, with ctl, with ctl and chg
			r = strings.Replace(r, "size 5", "size X", 1)
			r = strings.Replace(r, "size 6", "size X", 1)
			r = strings.Replace(r, "size 7", "size X", 1)
		}
		r = strings.Replace(r, dbg.Usr, "nemo", -1)
		if mv.Res != "" && !strings.HasPrefix(r, mv.Res) {
			t.Fatalf("bad new stat %s", r)
		}
		if mv.Child != "" {
			d, err = zx.Stat(fs, mv.Child)
			if err != nil {
				t.Fatalf("stat %s: %s", mv.Child, err)
			}
			if d["path"] != mv.Child {
				t.Fatalf("%s: bad path %s", mv.Child, d["path"])
			}
		}
		paths := []string{mv.From, mv.To, path.Dir(mv.From), path.Dir(mv.To)}
		for _, p := range paths {
			d, err := zx.Stat(fs, mv.To)
			if err != nil {
				t.Fatalf("stat %s: %s", p, err)
			}
			chkdirs(t, fs, d, false)
		}
	}
	printf("\n")
}

func chkdirs(t Fataler, fs zx.Tree, d zx.Dir, recur bool) {
	if d["path"] == "" {
		t.Fatalf("no path in <%s>", d)
	}
	if d["type"] != "d" {
		return
	}
	dents, err := zx.GetDir(fs, d["path"])
	if err != nil {
		t.Fatalf("getdir: %s: %s", d["path"], err)
	}
	if d["size"] != fmt.Sprintf("%d", len(dents)) {
		t.Logf("%s: size %s len(dents) %d", d["path"], d["size"], len(dents))
		t.Logf("d: %s\n", d.Long())
		for _, cd := range dents {
			t.Logf("\t%s\n", cd.Long())
		}
		t.Fatalf("bad dir size")
	}
	if recur {
		for _, cd := range dents {
			chkdirs(t, fs, cd, true)
		}
	}
}

func DirSizes(t Fataler, fss ...zx.Tree) {
	for _, fs := range fss {
		rf, err := zx.Stat(fs, "/")
		if err != nil {
			t.Fatalf("stat: %s", err)
		}
		chkdirs(t, fs, rf, true)
	}
}

func StatsBench(b *testing.B, fs zx.Tree) {
	for bi := 0; bi < b.N; bi++ {
		i := bi % len(StatTests)
		st := StatTests[i]
		if st.Fails {
			continue
		}
		dc := fs.Stat(st.Path)
		d := <-dc
		if d == nil {
			b.Fatalf("stat %s: %s", st.Path, cerror(dc))
		}
	}
}

var (
	GetFPaths = []string{"/1", "/a/a2", "/2"}
	GetDPaths = []string{"/"}
	BadPaths  = []string{"zz", "..", ".", "/a/b/c/c5"}
	GetDOuts  = map[string][]string{
		"/": {
			"path /1 name 1 type - mode 0644 size 0",
			"path /2 name 2 type - mode 0644 size 31658",
			"path /a name a type d mode 0755 size 3",
			"path /d name d type d mode 0755 size 0",
			"path /e name e type d mode 0755 size 1",
		},
	}
)

func Gets(t Fataler, fss ...zx.Tree) {
	for i := 0; i < Repeats; i++ {
		for _, fs := range fss {
			for _, p := range GetFPaths {
				printf("getall %s\n", p)
				dat, err := zx.GetAll(fs, p)
				if err != nil {
					t.Fatalf("get %s: %s", p, err)
				}
				printf("got %d bytes \n\n", len(dat))
				if string(dat) != string(FileData[p]) {
					printf("got <%s>\nexpected<%s>\n",
						string(dat), string(FileData[p]))
					t.Fatalf("%s: bad data", p)
				}
			}

			for _, p := range GetDPaths {
				printf("getall %s\n", p)
				dat, err := zx.GetAll(fs, p)
				if err != nil {
					t.Fatalf("get %s: %s", p, err)
				}
				var d zx.Dir
				ents := []string{}
				for len(dat) > 0 {
					d, dat, err = zx.UnpackDir(dat)
					if err != nil {
						t.Fatalf("dir: %s", err)
					}
					if d["path"] == "/Ctl" || d["path"] == "/Chg" {
						continue
					}
					ents = append(ents, d.TestFmt())
				}
				if strings.Join(GetDOuts[p], "\n") != strings.Join(ents, "\n") {
					t.Fatalf("bad dir data for %s", p)
				}
				printf("got %d ents (ctl, chg excluded)\n", len(ents))
			}
			for _, p := range BadPaths {
				dat, err := zx.GetAll(fs, p)
				if err == nil || len(dat) > 0 {
					t.Fatalf("get %s didn't fail", p)
				}
			}
			printf("\n")
		}
	}
}

func GetsBench(b *testing.B, fs zx.Tree) {
	for bi := 0; bi < b.N; bi++ {
		dc := fs.Get("/2", 0, -1, "")
		tot := 0
		for m := range dc {
			tot += len(m)
		}
		if tot != 31658 {
			b.Fatalf("bad total")
		}
	}
}

type FindTest struct {
	Path         string
	Pred         string
	Spref, Dpref string
	Depth        int
	Res          []string
	Fails        bool
}

var FindTests = []FindTest{
	{
		Path: "/",
		Pred: "",
		Res: []string{
			`/`,
			`path /1 name 1 type - mode 0644 size 0`,
			`path /2 name 2 type - mode 0644 size 31658`,
			`path /a name a type d mode 0755 size 3`,
			`path /a/a1 name a1 type - mode 0644 size 10154`,
			`path /a/a2 name a2 type - mode 0644 size 21418`,
			`path /a/b name b type d mode 0755 size 1`,
			`path /a/b/c name c type d mode 0755 size 1`,
			`path /a/b/c/c3 name c3 type - mode 0644 size 44970`,
			`path /d name d type d mode 0755 size 0`,
			`path /e name e type d mode 0755 size 1`,
			`path /e/f name f type d mode 0755 size 0`,
		},
	},
	{
		Path: "/",
		Pred: "type=d&depth>1",
		Res: []string{
			`path /a/b name b type d mode 0755 size 1`,
			`path /a/b/c name c type d mode 0755 size 1`,
			`path /e/f name f type d mode 0755 size 0`,
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

	// next is similar to mounting /a at /x/y and doing a find at /x/y/b
	{
		Path:  "/a/b",
		Spref: "/a",
		Dpref: "/x/y",
		Pred:  "depth<=1",
		Res: []string{
			`path /x/y/b name b type d mode 0755 size 1`,
			`path /x/y/b/c name c type d mode 0755 size 1`,
		},
	},
	// next is similar to mounting /a at /x/y and doing a find at /x
	{
		Path:  "/a/b",
		Spref: "/a",
		Dpref: "/x/y",
		Depth: 1,
		Pred:  "depth<=2",
		Res: []string{
			`path /x/y/b name b type d mode 0755 size 1`,
			`path /x/y/b/c name c type d mode 0755 size 1`,
		},
	},

	// prunes
	{
		Path:  "/a/b",
		Spref: "/",
		Dpref: "/",
		Pred:  `(path = "/a/b" | path = "/d") & prune | type = d`,
		Res:   []string{},
	},
}

func FindGets(t Fataler, fss ...zx.Tree) {
	for i := 0; i < Repeats; i++ {
		for _, fs := range fss {
			for _, f := range FindTests {
				if f.Spref == "" {
					f.Spref = "/"
				}
				if f.Dpref == "" {
					f.Dpref = "/"
				}
				printf("findget %s %s sp %s dp %s d %d\n", f.Path, f.Pred, f.Spref, f.Dpref, f.Depth)
				dents := []string{}
				fdatas := []string{}
				gc := fs.FindGet(f.Path, f.Pred, f.Spref, f.Dpref, f.Depth)
				for g := range gc {
					d := g.Dir
					if d == nil {
						break
					}
					if d["path"] == "/" {
						d["size"] = "0" // get rid of ctl, chg
					}
					if d["err"] != "" {
						t.Logf("find %s: %s\n", d["path"], d["err"])
						continue
					}
					fdata := []byte{}
					if g.Datac != nil {
						for d := range g.Datac {
							fdata = append(fdata, d...)
						}
					}
					if d["path"] == "/Ctl" || d["path"] == "/Chg" {
						continue
					}
					printf("\t%s err=%v data=[%d]\n", d.LongTestFmt(), d["err"], len(fdata))
					if d["path"] == "/" {
						dents = append(dents, "/")
					} else {
						dents = append(dents, d.TestFmt())
					}
					data := string(fdata)
					fdatas = append(fdatas, data)
					fd := FileData[d["path"]]
					if len(data) > 0 && len(fd) > 0 && data != string(fd) {
						t.Fail()
						t.Logf("bad content for %s", d["path"])
						t.Logf("file is <%s>\n\n\n", data)
						t.Logf("file should be <%s>\n\n\n", string(fd))
					}
				}
				err := cerror(gc)
				if f.Fails {
					if err == nil {
						t.Fatalf("%s didn't fail", f.Path)
					}
					continue
				}
				if !f.Fails && err != nil {
					t.Fatalf("%s failed: %s", f.Path, err)
				}
				if f.Res != nil && strings.Join(f.Res, "\n") != strings.Join(dents, "\n") {
					t.Logf("expected <%s>\n",
						strings.Join(f.Res, "\n"))
					t.Logf("got <%s>\n",
						strings.Join(dents, "\n"))
					t.Fatalf("%s: bad findings", f.Path)
				}
			}
		}
	}
}

func Finds(t Fataler, fss ...zx.Tree) {
	for i := 0; i < Repeats; i++ {
		for _, fs := range fss {
			for _, f := range FindTests {
				if f.Spref == "" {
					f.Spref = "/"
				}
				if f.Dpref == "" {
					f.Dpref = "/"
				}
				printf("find %s %s sp %s dp %s d %d\n", f.Path, f.Pred, f.Spref, f.Dpref, f.Depth)
				dents := []string{}
				dc := fs.Find(f.Path, f.Pred, f.Spref, f.Dpref, f.Depth)
				for d := range dc {
					if d == nil {
						break
					}
					if d["path"] == "/" {
						d["size"] = "0" // get dir of ctl, chg
					}
					if d["path"] == "/Ctl" || d["path"] == "/Chg" {
						continue
					}
					if d["err"] != "" {
						t.Logf("find %s: %s\n", d["path"], d["err"])
						continue
					}
					printf("\t%s err=%v\n", d.LongTestFmt(), d["err"])
					if d["path"] == "/" {
						dents = append(dents, "/")
					} else {
						dents = append(dents, d.TestFmt())
					}
				}
				err := cerror(dc)
				if f.Fails {
					if err == nil {
						t.Fatalf("%s didn't fail", f.Path)
					}
					continue
				}
				if !f.Fails && err != nil {
					t.Fatalf("%s failed: %s", f.Path, err)
				}
				if f.Res != nil && strings.Join(f.Res, "\n") != strings.Join(dents, "\n") {
					t.Logf("expected <%s>\n",
						strings.Join(f.Res, "\n"))
					t.Logf("got <%s>\n",
						strings.Join(dents, "\n"))
					t.Fatalf("%s: bad findings", f.Path)
				}
			}
		}
	}
}

func FindsBench(b *testing.B, fs zx.Tree) {
	for bi := 0; bi < b.N; bi++ {
		dc := fs.Find("/", "", "/", "/", 0)
		tot := 0
		for d := range dc {
			if d["path"] == "/Ctl" || d["path"] == "/Chg" {
				continue
			}
			tot++
		}
		if tot != 12 {
			b.Fatalf("bad find tot")
		}
	}
}

type PutTest struct {
	Path  string
	Dir   zx.Dir
	Res   string
	Rdir  string
	Fails bool
	N     int
	data  []byte
}

var PutTests = []PutTest{
	{
		Path: "/n1",
		Dir:  zx.Dir{"mode": "0640"},
		Res:  "path  name  type  mode  size 10890",
		Rdir: "path /n1 name n1 type - mode 0640 size 10890",
	},
	{
		Path: "/n1",
		Dir:  zx.Dir{"mode": "0640"},
		Res:  "path  name  type  mode  size 22890",
		Rdir: "path /n1 name n1 type - mode 0640 size 22890",
	},
	{
		Path: "/a/n2",
		Dir:  zx.Dir{"mode": "0640"},
		Res:  "path  name  type  mode  size 40890",
		Rdir: "path /a/n2 name n2 type - mode 0640 size 40890",
	},
	{
		Path:  "/",
		Dir:   zx.Dir{"mode": "0640"},
		Fails: true,
	},
	{
		Path:  "/a",
		Dir:   zx.Dir{"mode": "0640"},
		Fails: true,
	},
	{
		Path:  "/a/b/c/d/e/f",
		Dir:   zx.Dir{"mode": "0640"},
		Fails: true,
	},
	{
		Path: "/newfile",
		Dir:  zx.Dir{"mode": "0600", "size": "50000"},
		Res:  "path  name  type  mode  size 50000",
		N:    50000,
	},
	{
		Path:  "/newfile",
		Dir:   zx.Dir{"Wuid": "xx"},
		Fails: true,
	},
	{
		Path:  "/newfile",
		Dir:   zx.Dir{"type": "xx"},
		Fails: true,
	},
	/*
			won't work for lfs w/o attrs
		{
			Path: "/newfile",
			Dir: zx.Dir{"mtime": "7000000000", "size": "40003", "X": "Y"},
			Res: "path  name  type  mode  size 40003 mtime 7000000000",
			// mode is 640 because 40 is inherited in previous put.
			Rdir: "path /newfile name newfile type - mode 0640 size 40003 mtime 7000000000 X Y",
			N: 40003,
		},
	*/
}

func Puts(t Fataler, fss ...zx.Tree) {
	if len(fss) == 0 {
		t.Fatalf("no fs given")
	}
	fs := fss[0].(zx.RWTree)
	rd, err := zx.Stat(fs, "/")
	if err != nil {
		t.Fatalf("root stat %s", err)
	}
	printf("root %s\n", rd.TestFmt())
	for z := 0; z < Repeats; z++ {
	Loop:
		for nfi, nf := range PutTests {
			dc := make(chan []byte, 1)
			nf.data = make([]byte, 0, 32*1024)
			xd := nf.Dir.Dup()
			printf("put %s %v\n", nf.Path, xd)
			nn := nfi + 1
			if nf.N != 0 {
				nn = 2
			}
			xc := fs.Put(nf.Path, xd, 0, dc, "")
			for i := 0; i < 1000*nn; i++ {
				msg := []byte(fmt.Sprintf("hi %s %d\n", nf.Path, i))
				nf.data = append(nf.data, msg...)
				if ok := dc <- msg; !ok {
					err := cerror(dc)
					if !nf.Fails {
						t.Fatalf("%s: %s\n", nf.Path, err)
					}
					continue Loop
				}
			}
			printf("put %s: sent %d bytes\n", nf.Path, len(nf.data))
			close(dc)
			xd = <-xc
			if nf.Fails {
				if xd != nil || cerror(xc) == nil {
					t.Fatalf("%s: didn't fail", nf.Path)
				}
				continue
			}
			if xd == nil || cerror(xc) != nil {
				t.Fatalf("%s: %s\n", nf.Path, cerror(xc))
			}
			got := xd.TestFmt()
			printf("got %s\n", got)
			if nf.Dir["mtime"] != "" {
				got += " mtime " + xd["mtime"]
			}
			if got != nf.Res {
				t.Logf("expected %s\n", nf.Res)
				t.Logf("got %s\n", got)
				t.Fatalf("%s: bad dir output", nf.Path)
			}
			fd, err := zx.Stat(fs, nf.Path)
			if err != nil {
				t.Fatalf("stat: %s", err)
			}
			got = fd.TestFmt()
			if nf.Dir["mtime"] != "" {
				got += " mtime " + fd["mtime"]
			}
			if nf.Dir["X"] != "" {
				got += " X " + fd["X"]
			}
			printf("after put: %s\n", got)
			if nf.Rdir != "" && nf.Rdir != got {
				t.Logf("expected <%s>", nf.Rdir)
				t.Fatalf("got <%s>", got)
			}
			for _, fs := range fss {
				dat, err := zx.GetAll(fs, nf.Path)
				if err != nil {
					t.Fatalf("couldn't get: %s", err)
				}
				if nf.N != 0 && len(dat) != nf.N {
					t.Fatalf("bad output size: %d", len(dat))
				}
				if nf.N != 0 && len(dat) > len(nf.data) {
					dat = dat[:len(nf.data)]
				}
				if nf.N != 0 && len(dat) < len(nf.data) {
					nf.data = nf.data[:len(dat)]
				}
				if string(dat) != string(nf.data) {
					t.Fatalf("bad data %d vs %d bytes", len(dat), len(nf.data))
				}
			}
		}
	}
}

func GetCtl(t Fataler, fss ...zx.Tree) {
	if len(fss) == 0 {
		t.Fatalf("no fs given")
	}
	fs := fss[0].(zx.RWTree)
	vals := []bool{false, true}
	outs := []string{"debug off", "debug on"}
	for i, v := range vals {
		d := fs.(zx.Debugger).Debug()
		*d = v
		printf("get ctl: (debug %v)\n", v)
		dat, err := zx.GetAll(fs, "/Ctl")
		if err != nil {
			t.Fatalf("get /ctl: %s", err)
		}
		ctls := string(dat)
		printf("<%s>\n\n", ctls)
		if !strings.Contains(ctls, outs[i]) {
			t.Fatalf("wrong ctl output")
		}
	}
}

func PutCtl(t Fataler, fss ...zx.Tree) {
	if len(fss) == 0 {
		t.Fatalf("no fs given")
	}
	fs := fss[0].(zx.RWTree)
	vals := []bool{false, true, false, true}
	ins := []string{"nodebug", "debug", "debug off", "debug on"}
	outs := []string{"debug off", "debug on", "debug off", "debug on"}
	for i, v := range vals {
		printf("put ctl: (debug %v)\n", v)
		err := zx.PutAll(fs, "/Ctl", nil, []byte(ins[i]))
		if err != nil {
			t.Fatalf("put /ctl: %s", err)
		}
		printf("get ctl: (debug %v)\n", v)
		dat, err := zx.GetAll(fs, "/Ctl")
		if err != nil {
			t.Fatalf("get /ctl: %s", err)
		}
		ctls := string(dat)
		printf("<%s>\n\n", ctls)
		if !strings.Contains(ctls, outs[i]) {
			t.Fatalf("wrong ctl output")
		}
	}
	err := zx.PutAll(fs, "/Ctl", nil, []byte("bad ctl request"))
	if err == nil {
		t.Fatalf("bad put did not fail")
	}
	printf("put sts %s\n", err)
}

func PutsBench(b *testing.B, xfs zx.Tree) {
	fs := xfs.(zx.RWTree)
	b.StopTimer()
	var buf [1024]byte
	copy(buf[0:], "hola")
	<-fs.Remove("/nfb")
	b.StartTimer()
	for bi := 0; bi < b.N; bi++ {
		dc := make(chan []byte)
		xc := fs.Put("/nfb", zx.Dir{"mode": "0644"}, 0, dc, "")
		for i := 0; i < 128; i++ {
			if ok := dc <- buf[:]; !ok {
				b.Fatalf("put failed")
			}
		}
		close(dc)
		if len(<-xc) == 0 {
			b.Fatalf("put failed")
		}
	}
}

var (
	MkdirPaths    = []string{"/nd", "/nd/nd2", "/nd/nd22", "/nd/nd23", "/nd3"}
	BadMkdirPaths = []string{"/", "/nd", "/a"}
)

func Mkdirs(t Fataler, fss ...zx.Tree) {
	if len(fss) == 0 {
		t.Fatalf("no fs given")
	}
	fs := fss[0].(zx.RWTree)
	for i := 0; i < Repeats; i++ {
		for _, p := range MkdirPaths {
			printf("mkdir %s\n", p)
			ec := fs.Mkdir(p, zx.Dir{"mode": "0750"})
			err := <-ec
			if i > 0 && err == nil {
				t.Fatalf("could re-mkdir %s", p)
			}
			if i == 0 && err != nil {
				t.Fatalf("%s: %s", p, err)
			}
			for _, fs := range fss {
				d, err := zx.Stat(fs, p)
				if err != nil || d["type"] != "d" || d["mode"] != "0750" {
					t.Fatalf("mkdir not there %v %v", err, d)
				}
			}
		}
		for _, p := range BadMkdirPaths {
			printf("mkdir %s\n", p)
			ec := fs.Mkdir(p, zx.Dir{"mode": "0750"})
			err := <-ec
			if err == nil {
				t.Fatalf("mkdir %s worked", p)
			}
		}
	}
}

var (
	RemovePaths    = []string{"/d", "/e/f", "/e", "/a/a2"}
	BadRemovePaths = []string{"/", "/xxx", "/a"}
)

func Removes(t Fataler, fss ...zx.Tree) {
	if len(fss) == 0 {
		t.Fatalf("no fs given")
	}
	fs := fss[0].(zx.RWTree)
	for i := 0; i < Repeats; i++ {
		for _, p := range RemovePaths {
			printf("remove %s\n", p)
			ec := fs.Remove(p)
			err := <-ec
			if i > 0 && err == nil {
				t.Fatalf("could re-remove %s", p)
			}
			if i == 0 && err != nil {
				t.Fatalf("%s: %s", p, err)
			}
			for _, fs := range fss {
				d, err := zx.Stat(fs, p)
				if err == nil || d != nil {
					t.Fatalf("%s still there", p)
				}
			}
		}
		for _, p := range BadRemovePaths {
			ec := fs.Remove(p)
			err := <-ec
			if err == nil {
				t.Fatalf("remove %s worked", p)
			}
		}
	}
}

func MkdirRemoveBench(b *testing.B, xfs zx.Tree) {
	fs := xfs.(zx.RWTree)
	b.StopTimer()
	<-fs.Remove("/mrb")
	b.StartTimer()
	for bi := 0; bi < b.N; bi++ {
		xc := fs.Mkdir("/mrb", zx.Dir{"mode": "0755"})
		if <-xc != nil {
			b.Fatalf("mkdir failed")
		}
		xc = fs.Remove("/mrb")
		if <-xc != nil {
			b.Fatalf("remove failed")
		}
	}
}

type WstatTest struct {
	Path   string
	Dir    zx.Dir
	AppDir bool
	Res    string
	Fails  bool
}

var WstatTests = []WstatTest{
	{
		Path: "/d",
		Dir:  zx.Dir{"mode": "0704", "foo": "bar"},
		Res:  `path /d name d type d mode 0704 size 0`,
	},
	{
		Path: "/e/f",
		Dir:  zx.Dir{"mode": "0704", "foo": "bar"},
		Res:  `path /e/f name f type d mode 0704 size 0`,
	},
	{
		Path: "/e",
		Dir:  zx.Dir{"mode": "0704", "foo": "bar"},
		Res:  `path /e name e type d mode 0704 size 1`,
	},
	{
		Path: "/a/a2",
		Dir:  zx.Dir{"mode": "0704", "foo": "bar"},
		Res:  `path /a/a2 name a2 type - mode 0704 size 21418`,
	},
	{
		Path: "/",
		Dir:  zx.Dir{"mode": "0704", "foo": "bar"},
		Res:  `path / name / type d mode 0704 size 0`,
	},
	{
		Path:  "/xxx",
		Dir:   zx.Dir{"mode": "0704", "foo": "bar"},
		Fails: true,
	},
	{
		Path:  "/a/xxx",
		Dir:   zx.Dir{"mode": "0704", "foo": "bar"},
		Fails: true,
	},
	{
		Path:  "/dx",
		Dir:   zx.Dir{"mode": "0704", "foo": "bar"},
		Fails: true,
	},
}

func Wstats(t Fataler, fss ...zx.Tree) {
	if len(fss) == 0 {
		t.Fatalf("no fs given")
	}
	fs := fss[0].(zx.RWTree)
	for i := 0; i < Repeats; i++ {
		for _, f := range WstatTests {
			printf("wstat %s %v\n", f.Path, f.Dir)
			ec := fs.Wstat(f.Path, f.Dir)
			err := <-ec
			if f.Fails {
				if err == nil {
					t.Fatalf("%s didn't fail", f.Path)
				}
				continue
			}
			if !f.Fails && err != nil {
				t.Fatalf("%s: %s", f.Path, err)
			}
			for _, fs := range fss {
				d, err := zx.Stat(fs, f.Path)
				if err != nil {
					t.Fatalf("stat %s: %s", f.Path, err)
				}
				if d["path"] == "/" {
					d["size"] = "0" // ctl, chg
				}
				if d.TestFmt() != f.Res {
					t.Logf("for %s\n", fs.Name())
					t.Logf("expected %s\n", f.Res)
					t.Logf("got %s\n", d.TestFmt())
					t.Fatalf("stat %s: didn't wstat it", f.Path)
				}
			}
		}
	}
}

var UsrWstatTests = []WstatTest{
	// extra bits in mode
	{
		Path: "/a",
		Dir:  zx.Dir{"mode": "07755"},
		Res:  `path /a name a type d mode 0755 size 3 Gid nemo Uid nemo Wuid nemo`,
	},
	// changing size in a dir is ignored if writing existing attributes.
	{
		Path:   "/a",
		Dir:    zx.Dir{"size": "5"},
		AppDir: true,
		Res:    `path /a name a type d mode 0755 size 3 Gid nemo Uid nemo Wuid nemo`,
	},
	// writing a non  user attribute
	{
		Path:  "/a",
		Dir:   zx.Dir{"foo": "bar"},
		Fails: true,
	},
	// writing a ignored non  user attribute
	{
		Path: "/a",
		Dir:  zx.Dir{"mtime": "0", "foo": "bar"},
		Res:  `path /a name a type d mode 0755 size 3 Gid nemo Uid nemo Wuid nemo`,
	},
	// Adding a user attribute
	{
		Path: "/a",
		Dir:  zx.Dir{"Dir": "X"},
		Res:  `path /a name a type d mode 0755 size 3 Dir X Gid nemo Uid nemo Wuid nemo`,
	},
	// Same, using the previous dir
	{
		Path:   "/a",
		Dir:    zx.Dir{"Dir": "X"},
		AppDir: true,
		Res:    `path /a name a type d mode 0755 size 3 Dir X Gid nemo Uid nemo Wuid nemo`,
	},
	// Adding a two user attributes
	{
		Path: "/a",
		Dir:  zx.Dir{"Dir": "X", "Abc": "A"},
		Res:  `path /a name a type d mode 0755 size 3 Abc A Dir X Gid nemo Uid nemo Wuid nemo`,
	},
	// Removing a non existing user attribute
	{
		Path: "/a",
		Dir:  zx.Dir{"Non": ""},
		Res:  `path /a name a type d mode 0755 size 3 Abc A Dir X Gid nemo Uid nemo Wuid nemo`,
	},
	// Rewriting a user attribute
	{
		Path: "/a",
		Dir:  zx.Dir{"Abc": "B"},
		Res:  `path /a name a type d mode 0755 size 3 Abc B Dir X Gid nemo Uid nemo Wuid nemo`,
	},
	// Removing a user attribute
	{
		Path: "/a",
		Dir:  zx.Dir{"Abc": ""},
		Res:  `path /a name a type d mode 0755 size 3 Dir X Gid nemo Uid nemo Wuid nemo`,
	},
	// Type change
	{
		Path:  "/a",
		Dir:   zx.Dir{"type": "x"},
		Fails: true,
	},
	// Removing a sys attribute
	{
		Path:  "/a",
		Dir:   zx.Dir{"type": "", "Dir": "X"},
		Fails: true,
	},
	// Trying to change size
	{
		Path:  "/a",
		Dir:   zx.Dir{"size": "55"},
		Fails: true,
	},
	// Bad path
	{
		Path:  "2",
		Dir:   zx.Dir{"size": "55"},
		Fails: true,
	},
	// Truncating a file
	{
		Path: "/2",
		Dir:  zx.Dir{"size": "55"},
		Res:  `path /2 name 2 type - mode 0644 size 55 Gid nemo Uid nemo Wuid nemo`,
	},
}

func UsrWstats(t Fataler, fss ...zx.Tree) {
	if len(fss) == 0 {
		t.Fatalf("no fs given")
	}
	fs := fss[0].(zx.RWTree)
	for i := 0; i < 1; i++ {
		for _, f := range UsrWstatTests {
			d := f.Dir.Dup()
			if f.AppDir {
				od, err := zx.Stat(fs, f.Path)
				if err != nil {
					t.Fatalf("stat %s: %s", f.Path, err)
				}
				for k, v := range d {
					od[k] = v
				}
				d = od
			}
			printf("wstat %s %s\n", f.Path, d.LongTestFmt())
			ec := fs.Wstat(f.Path, d)
			err := <-ec
			if f.Fails {
				if err == nil {
					t.Fatalf("%s didn't fail", f.Path)
				}
				continue
			}
			if !f.Fails && err != nil {
				t.Fatalf("%s: %s", f.Path, err)
			}
			for _, fs := range fss {
				d, err := zx.Stat(fs, f.Path)
				if err != nil {
					t.Fatalf("stat %s: %s", f.Path, err)
				}
				printf("result %s %s\n", f.Path, d.LongTestFmt())
				if d["foo"] != "" {
					t.Fatalf("could write foo")
				}
				if f.Res == "" {
					continue
				}
				if d["path"] == "/" {
					d["size"] = "0" // ctl, chg
				}
				delete(d, "Sum")
				ds := strings.Replace(d.LongTestFmt(), "nemo", "none", -1)
				ds2 := strings.Replace(f.Res, "nemo", "none", -1)
				if ds != ds2 {
					t.Logf("for fs %s\n", fs.Name())
					t.Logf("expected %s\n", f.Res)
					t.Logf("got %s\n", d.LongTestFmt())
					t.Fatalf("stat %s: didn't wstat it", f.Path)
				}
			}
		}
	}
}

func WstatBench(b *testing.B, xfs zx.Tree) {
	fs := xfs.(zx.RWTree)
	for bi := 0; bi < b.N; bi++ {
		xc := <-fs.Wstat("/e", zx.Dir{"mode": "0755"})
		if xc != nil {
			b.Fatalf("wstat failed")
		}
	}
}

var SendOuts = []string{
	`/`,
	`path /1 name 1 type - mode 0644 size 0`,
	`path /2 name 2 type - mode 0644 size 31658`,
	`path /a name a type d mode 0755 size 3`,
	`path /a/a1 name a1 type - mode 0644 size 10154`,
	`path /a/a2 name a2 type - mode 0644 size 21418`,
	`path /a/b name b type d mode 0755 size 1`,
	`path /a/b/c name c type d mode 0755 size 1`,
	`path /a/b/c/c3 name c3 type - mode 0644 size 44970`,
	`path /d name d type d mode 0755 size 0`,
	`path /e name e type d mode 0755 size 1`,
	`path /e/f name f type d mode 0755 size 0`,
}

func SendRecv(t Fataler, sfs, rfs zx.RWTree) {
	dc := make(chan []byte)
	rf, err := zx.Stat(sfs, "/")
	if err != nil || rf == nil {
		t.Fatalf("stat: %s", err)
	}
	zx.DebugSend = testing.Verbose()
	go func() {
		err := zx.Send(sfs, rf, dc)
		close(dc, err)
	}()
	err = zx.Recv(rfs, dc)
	if cerror(dc) != nil {
		t.Logf("send err %s", cerror(dc))
	}
	if err != nil {
		t.Fatalf("recv: %s", err)
	}
	err = cerror(dc)
	if err != nil {
		t.Fatalf("send: %s", err)
	}
	fc := rfs.Find("/", "", "/", "/", 0)
	tot := 0
	outs := []string{}
	for d := range fc {
		printf("\t`%s`,\n", d.LongTestFmt())
		if d["path"] == "/" {
			outs = append(outs, "/")
			tot++
			continue
		}
		if d["path"] == "/" {
			d["size"] = "0" // get rid of ctl, chg
		}
		if d["path"] == "/Ctl" || d["path"] == "/Chg" {
			continue
		}
		tot++
		outs = append(outs, d.TestFmt())
	}
	if tot != 12 {
		t.Fatalf("bad find tot: got %d", tot)
	}
	if strings.Join(outs, "\n") != strings.Join(SendOuts, "\n") {
		t.Fatalf("bad received tree")
	}
}

// Check out if fs.Dump() is the same for all trees
func SameDump(t Fataler, fss ...zx.Tree) {
	old := ""
	oname := ""
	nname := ""
	for _, tfs := range fss {
		var buf bytes.Buffer
		fs, ok := tfs.(zx.Dumper)
		if !ok {
			continue
		}
		fs.Dump(&buf)
		ns := buf.String()
		toks := strings.SplitN(ns, "\n", 2)
		if len(toks) > 1 {
			ns = toks[1]
			nname = toks[0]
		}
		if old == "" {
			old = ns
			oname = nname
		} else if old != ns {
			t.Logf("%s\n<%s>\n", oname, old)
			t.Logf("%s\n<%s>\n", nname, ns)
			i := 0
			for i < len(old) && i < len(ns) {
				if old[i] != ns[i] {
					break
				}
				i++
			}
			t.Logf("suffix1 <%s>\n", old[i:])
			t.Logf("suffix2 <%s>\n", ns[i:])
			t.Fatalf("trees do not match")
		}
	}
}
