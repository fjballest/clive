/*
	Utilities to aid in tests of fuse file systems.
	Mimics zx/fstest but using OS interfaces.
*/
package ostest

import (
	"bytes"
	"clive/dbg"
	"clive/mblk/rwtest"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	fpath "path"
	"sort"
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
interface Fataler {
	Fatalf(format string, args ...face{})
	Logf(format string, args ...face{})
	Fail()
}

var (
	xt int64

	// directories created
	Dirs = [...]string{"/a", "/a/b", "/a/b/c", "/d", "/e", "/e/f"}

	// files created
	Files = [...]string{"/1", "/a/a1", "/a/a2", "/a/b/c/c3", "/2"}

	// data stored in each file
	FileData = map[string][]byte{}

	Repeats = 1
)

func printf(x string, arg ...face{}) {
	if testing.Verbose() {
		dbg.Printf(x, arg...)
	}
}

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

func RmTree(t Fataler, tdir string) {
	os.RemoveAll(tdir)
}

// arg to Diff
const (
	WithMtime    = true
	WithoutMtime = false
)

// Compare trees and report differences
func Diff(meta bool, dirs ...string) ([]string, error) {
	if len(dirs) == 0 {
		return nil, errors.New("not enough dirs")
	}
	res := []string{}
	rc := make(chan string)
	go func() {
		diff(rc, "/", meta, dirs...)
		close(rc)
	}()
	for s := range rc {
		res = append(res, s)
	}
	return res, cerror(rc)
}

func diff1(p0, p1 string, name, mtime bool) string {
	fi0, err0 := os.Stat(p0)
	fi1, err1 := os.Stat(p1)
	if err0 != nil && err1 != nil {
		return ""
	}
	if err0 != nil || err1 != nil {
		return fmt.Sprintf("errors differ")
	}
	printf("%s %s %d %s %d\n", p0, fi0.Mode(), fi0.ModTime().UnixNano(), fi1.Mode(), fi1.ModTime().UnixNano())
	if name && fi0.Name() != fi1.Name() {
		return fmt.Sprintf("name '%s' vs '%s'", fi0.Name(), fi1.Name())
	}
	if fi0.Mode() != fi1.Mode() {
		return fmt.Sprintf("mode '%s' vs '%s'", fi0.Mode(), fi1.Mode())
	}
	if fi0.IsDir() != fi1.IsDir() {
		return fmt.Sprintf("isdir '%s' vs '%s'", fi0.IsDir(), fi1.IsDir())
	}
	if fi0.IsDir() {
		return ""
	}
	if fi0.Size() != fi1.Size() {
		return fmt.Sprintf("size %d vs %d", fi0.Size(), fi1.Size())
	}
	dat0, err0 := ioutil.ReadFile(p0)
	dat1, err1 := ioutil.ReadFile(p1)
	if err0 != nil && err1 != nil {
		return ""
	}
	if err0 != nil || err1 != nil {
		return fmt.Sprintf("errors differ")
	}
	if bytes.Compare(dat0, dat1) != 0 {
		return fmt.Sprintf("data differs")
	}
	if mtime && !fi0.ModTime().Equal(fi1.ModTime()) {
		return fmt.Sprintf("mtimes '%d' vs '%d'", fi0.ModTime().UnixNano(), fi1.ModTime().UnixNano())
	}
	return ""
}

// Return a sorted list of child names for dir at p; ignore /Ctl and dot files
func Children(p string) []string {
	fis, err := ioutil.ReadDir(p)
	if err != nil {
		return nil
	}
	names := []string{}
	for _, fi := range fis {
		if fi.Name() == "Ctl" || fi.Name()[0] == '.' {
			continue
		}
		names = append(names, fi.Name())
	}
	sort.Sort(sort.StringSlice(names))
	return names
}

func member(n string, ns []string) bool {
	for _, x := range ns {
		if n == x {
			return true
		}
	}
	return false
}

func diff(rc chan<- string, fn string, mtime bool, dirs ...string) {
	d0 := dirs[0]
	dirs = dirs[1:]
	for _, di := range dirs {
		p0 := fpath.Join(d0, fn)
		pi := fpath.Join(di, fn)
		s := diff1(p0, pi, fn != "/", mtime)
		if s != "" {
			rc <- fmt.Sprintf("%s: chg %s %s", di, fn, s)
		}
		c0 := Children(p0)
		c1 := Children(pi)
		if len(c0) == 0 && len(c1) == 0 {
			continue
		}
		for _, c := range c0 {
			cf := fpath.Join(fn, c)
			if !member(c, c1) {
				rc <- fmt.Sprintf("%s: del %s", di, cf)
				continue
			}
			diff(rc, cf, mtime, d0, di)
		}
		for _, c := range c1 {
			if !member(c, c0) {
				cf := fpath.Join(fn, c)
				rc <- fmt.Sprintf("%s: add %s", di, cf)
			}
		}
	}
}

// Make sure /foo behaves as a file issuing calls from 3 fds.
func AsAFile(t Fataler, dirs ...string) {
	if len(dirs) == 0 {
		t.Fatalf("not enough dirs")
	}
	fn := "/foo"
	for _, d := range dirs {
		p := fpath.Join(d, fn)
		fd, err := os.Create(p)
		if err != nil {
			t.Fatalf("create: %s: %s", d, err)
		}
		fd1, err := os.Create(p)
		if err != nil {
			t.Fatalf("create: %s: %s", d, err)
		}
		fd2, err := os.Create(p)
		if err != nil {
			t.Fatalf("create: %s: %s", d, err)
		}
		fds := []rwtest.Object{fd, fd1, fd2}
		rwtest.AsAConcFile(t, fds, 1000, 128*1024, 3803)
	}
}

struct StatTest {
	Path  string
	Res   string
	Fails bool
}

var StatTests = []StatTest{
	{"/a/b/c", `c drwxr-xr-x`, false},
	{"/a", `a drwxr-xr-x`, false},
	{"/a/b/c/c3", `c3 -rw-r--r-- 44970`, false},
	{"/2", `2 -rw-r--r-- 31658`, false},
	{"zz", `no stat`, true},
	{"/a/b/c/c5", `no stat`, true},
	{"/a/b/../", `a drwxr-xr-x`, false},
}

func stat(fi os.FileInfo) string {
	if fi == nil {
		return "no stat"
	}
	if fi.IsDir() {
		return fmt.Sprintf("%s %s", fi.Name(), fi.Mode())
	}
	return fmt.Sprintf("%s %s %d", fi.Name(), fi.Mode(), fi.Size())
}

func Stats(t Fataler, dirs ...string) {
	if len(dirs) == 0 {
		t.Fatalf("not enough dirs")
	}
	for _, d := range dirs {
		for _, st := range StatTests {
			p := fpath.Join(d, st.Path)
			fi, err := os.Stat(p)
			if err == nil && st.Fails {
				t.Fatalf("%s did not fail", st.Path)
			}
			if err != nil && !st.Fails {
				t.Fatalf("%s did fail", st.Path)
			}
			printf("\t`%s`,\n", stat(fi))
			if !st.Fails && stat(fi) != st.Res {
				t.Fatalf("wrong stat <%s> vs <%s>", stat(fi), st.Res)
			}
		}
	}
}

var (
	GetFPaths = []string{"/1", "/a/a2", "/2"}
	GetDPaths = []string{"/"}
	GetDOuts  = map[string]string{
		"/": `1 2 a d e`,
	}
	BadPaths = []string{"zz", "/a/b/c/c5"}
)

func Gets(t Fataler, dirs ...string) {
	if len(dirs) == 0 {
		t.Fatalf("not enough dirs")
	}
	for _, d := range dirs {
		for _, fp := range GetFPaths {
			p := fpath.Join(d, fp)
			dat, err := ioutil.ReadFile(p)
			if err != nil {
				t.Fatalf("get %s: %s", p, err)
			}
			printf("got %d bytes \n\n", len(dat))
			if string(dat) != string(FileData[fp]) {
				printf("got <%s>\nexpected<%s>\n",
					string(dat), string(FileData[fp]))
				t.Fatalf("%s: bad data", p)
			}
		}
		for _, dp := range GetDPaths {
			printf("getall %s\n", dp)
			p := fpath.Join(d, dp)
			cs := Children(p)
			printf("children `%s`\n", strings.Join(cs, " "))
			if strings.Join(cs, " ") != GetDOuts[dp] {
				t.Logf("got %s", strings.Join(cs, " "))
				t.Fatalf("bad dir data for %s", dp)
			}
		}
		for _, fp := range BadPaths {
			p := fpath.Join(d, fp)
			_, err := ioutil.ReadFile(p)
			if err == nil {
				t.Fatalf("%s did not fail", fp)
			}
		}
	}
}

struct PutTest {
	Path  string
	Mode  string
	Fails bool
}

var PutTests = []PutTest{
	PutTest{Path: "/n1"},
	PutTest{Path: "/n1"},
	PutTest{Path: "/a/n2"},
	PutTest{Path: "/", Fails: true},
	PutTest{Path: "/a", Fails: true},
	PutTest{Path: "/a/b/c/d/e/f", Fails: true},
}

func Puts(t Fataler, dirs ...string) {
	if len(dirs) == 0 {
		t.Fatalf("not enough dirs")
	}
	for _, d := range dirs {
		nn := 0
		for _, pt := range PutTests {
			printf("put %s\n", pt.Path)
			p := fpath.Join(d, pt.Path)
			// 1. use create
			fd, err := os.Create(p)
			if err != nil && !pt.Fails {
				t.Fatalf("create: %s", err)
			}
			if pt.Fails && err == nil {
				t.Fatalf("create did not fail")
			}
			if pt.Fails {
				continue
			}
			nn++
			var buf bytes.Buffer
			for i := 0; i < 1000*nn; i++ {
				fmt.Fprintf(fd, "hi %s %d\n", pt.Path, i)
				fmt.Fprintf(&buf, "hi %s %d\n", pt.Path, i)
			}
			fd.Close()
			printf("%s: %d bytes\n", pt.Path, buf.Len())
			dat, err := ioutil.ReadFile(p)
			if err != nil {
				t.Fatalf("read: %s", err)
			}
			if bytes.Compare(buf.Bytes(), dat) != 0 {
				t.Fatalf("didn't put the bytes")
			}

			// 2. use openfile with truncate
			fd, err = os.OpenFile(p, os.O_TRUNC|os.O_RDWR, 0644)
			if err != nil {
				t.Fatalf("read: %s", err)
			}
			for i := 0; i < 1000*nn; i++ {
				fmt.Fprintf(fd, "hi %s %d\n", pt.Path, i)
			}
			fd.Close()
			dat, err = ioutil.ReadFile(p)
			if err != nil {
				t.Fatalf("read: %s", err)
			}
			if bytes.Compare(buf.Bytes(), dat) != 0 {
				t.Fatalf("didn't put the bytes")
			}
		}
	}
}

var (
	MkdirPaths    = []string{"/nd", "/nd/nd2", "/nd/nd22", "/nd/nd23", "/nd3"}
	BadMkdirPaths = []string{"/", "/nd", "/a", "/1"}
)

func Mkdirs(t Fataler, dirs ...string) {
	if len(dirs) == 0 {
		t.Fatalf("not enough dirs")
	}
	for _, d := range dirs {
		for _, dp := range MkdirPaths {
			printf("mkdir %s\n", dp)
			p := fpath.Join(d, dp)
			if err := os.Mkdir(p, 0750); err != nil {
				t.Fatalf("mkdir %s: %s", p, err)
			}
			fi, err := os.Stat(p)
			if err != nil {
				t.Fatalf("didn't stat: %s", err)
			}
			if !fi.IsDir() {
				t.Fatalf("didn't create a dir")
			}
			if fi.Mode().String() != "drwxr-x---" {
				t.Fatalf("bad dir mode")
			}
		}
		for _, dp := range BadMkdirPaths {
			printf("mkdir %s\n", dp)
			p := fpath.Join(d, dp)
			if err := os.Mkdir(p, 0750); err == nil {
				t.Fatalf("mkdir %s: didn't fail")
			}
		}
	}
}

var (
	RemovePaths    = []string{"/d", "/e/f", "/e", "/a/a2"}
	BadRemovePaths = []string{"/", "/xxx", "/a"}
)

func Removes(t Fataler, dirs ...string) {
	if len(dirs) == 0 {
		t.Fatalf("not enough dirs")
	}
	for _, d := range dirs {
		for _, dp := range RemovePaths {
			printf("remove %s\n", dp)
			p := fpath.Join(d, dp)
			if err := os.Remove(p); err != nil {
				t.Fatalf("rm %s: %s", p, err)
			}
		}
		for _, dp := range BadRemovePaths {
			printf("remove %s\n", dp)
			p := fpath.Join(d, dp)
			if err := os.Remove(p); err == nil {
				t.Fatalf("rm %s: did not fail", p)
			}
		}
	}
}

struct WstatTest {
	Path  string
	Mode  os.FileMode
	Mtime int64
	Fails bool
}

var WstatTests = []WstatTest{
	WstatTest{
		Path:  "/d",
		Mode:  0700,
		Mtime: 5,
	},
	WstatTest{
		Path:  "/e/f",
		Mode:  0704,
		Mtime: 500,
	},
	WstatTest{
		Path:  "/e",
		Mode:  0704,
		Mtime: 500,
	},
	WstatTest{
		Path:  "/a/a2",
		Mode:  0704,
		Mtime: 500,
	},
	WstatTest{
		Path:  "/xxx",
		Fails: true,
	},
	WstatTest{
		Path:  "/a/xxx",
		Fails: true,
	},
}

func Wstats(t Fataler, dirs ...string) {
	if len(dirs) == 0 {
		t.Fatalf("not enough dirs")
	}
	for _, d := range dirs {
		for _, dp := range WstatTests {
			p := fpath.Join(d, dp.Path)
			printf("wstat %s %o %d\n", dp.Path, os.FileMode(dp.Mode)&0777, dp.Mtime)
			if err := os.Chmod(p, dp.Mode); err != nil {
				if dp.Fails {
					continue
				}
				t.Fatalf("chmod: %s", err)
			}
			if dp.Fails {
				t.Fatalf("wstat didn't fail")
			}
			mt := time.Unix(dp.Mtime, 0)
			if err := os.Chtimes(p, mt, mt); err != nil {
				t.Fatalf("chtime: %s", err)
			}
			fi, err := os.Stat(p)
			if err != nil {
				t.Fatalf("stat: %s", err)
			}
			if uint(fi.Mode())&0777 != uint(dp.Mode) {
				t.Fatalf("wrong mode")
			}
			if fi.ModTime().Unix() != dp.Mtime {
				t.Fatalf("wrong mtime")
			}
		}
	}
}

func All(t Fataler, dirs ...string) {
	diffs, err := Diff(WithoutMtime, dirs...)
	if err != nil {
		t.Fatalf("diff errors")
	}
	if len(diffs) != 0 {
		t.Fatalf("initial diffs")
	}
	Stats(t, dirs...)
	Gets(t, dirs...)
	AsAFile(t, dirs...)
	Puts(t, dirs...)
	Mkdirs(t, dirs...)
	Wstats(t, dirs...)
	Removes(t, dirs...)
	for _, d := range dirs {
		os.Remove(fpath.Join(d, "/foo"))
	}
	diffs, err = Diff(WithoutMtime, dirs...)
	if err != nil {
		t.Fatalf("diff errors")
	}
	for _, d := range diffs {
		printf("diff %s\n", d)
	}
	if len(diffs) != 0 {
		t.Fatalf("final diffs")
	}
}
