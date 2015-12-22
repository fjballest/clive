/*
	Utilities to aid in tests of zx file systems packages
*/
package fstest

import (
	"clive/dbg"
	"clive/zx"
	"fmt"
	"io/ioutil"
	"os"
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

type TestFunc func(t Fataler, fs zx.Fs)

const tdir = "/tmp/zx_test"

var (
	Verb bool
	Printf = dbg.FlagPrintf(&Verb)
	xt     int64

	// directories created
	Dirs = [...]string{"/", "/a", "/a/b", "/a/b/c", "/d", "/e", "/e/f"}

	// files created
	Files = [...]string{"/1", "/a/a1", "/a/a2", "/a/b/c/c3", "/2"}

	// dirs and files
	AllFiles = append(Dirs[:], Files[:]...)

	// file paths not in the test tree
	NotThere = [...]string{"/n", "/a/n1", "/e/f/n2"}

	// bad file paths; the 1st should be /, others should fail
	BadPaths = [...]string{"/a/../..", "a", "..", "/1/b"}

	// data stored in each file
	FileData = map[string][]byte{}

	Repeats = 1
)

// Create a tree with Dirs and Files at tdir at the underlying OS
func MkTree(t Fataler, tdir string) {
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

