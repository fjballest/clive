/*
	Helpers for clive commands used in UNIX.

	It is likely that you should be using the clive/app package instead.
	This package will go once all its commands are adapted to Clive.
*/
package cmd

import (
	"clive/dbg"
	"clive/nchan"
	"clive/nspace"
	"clive/zx"
	"clive/zx/lfs"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	// Name space
	Ns zx.Finder

	// set to set the debug flags in types built by this package.
	Debug   bool
	dprintf = dbg.FlagPrintf(os.Stderr, &Debug)
)

// Context to execute a command. Useful to write commands
// that can be both built-ins in ql and stand-alone commands.
type Ctx struct {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
	Intrc          <-chan bool
	Args           []string
}

// Printf to stderr, prefixed with program name and terminating with \n.
func (c *Ctx) Warn(str string, args ...interface{}) {
	if c.Args == nil {
		c.Args = os.Args
	}
	if c.Stderr == nil {
		c.Stderr = os.Stderr
	}
	c.Eprintf("%s: %s\n", c.Args[0], fmt.Sprintf(str, args...))
}

// Printf to c's stderr.
func (c *Ctx) Eprintf(str string, args ...interface{}) {
	fmt.Fprintf(c.Stderr, str, args...)
}

// Printf to c's stdout.
func (c *Ctx) Printf(str string, args ...interface{}) {
	fmt.Fprintf(c.Stdout, str, args...)
}

// get input; used by RunFile
func (c *Ctx) In() io.Reader {
	return c.Stdin
}

// get intrc; used by RunFile
func (c *Ctx) Intr() <-chan bool {
	return c.Intrc
}

// Initialize the name space from $NS or using "/".
func MkNS() {
	var fndr zx.Finder
	s := os.Getenv("NS")
	if s != "" {
		ns, err := nspace.Parse(s)
		if err != nil {
			dbg.Fatal("ns: %s", err)
		}
		dprintf("ns is %s\n", ns)
		ns.Debug = Debug
		ns.DebugFind = Debug
		fndr = ns
	} else {
		fs, err := lfs.New("lfs", "/", lfs.RW)
		if err != nil {
			dbg.Fatal("lfs: %s", err)
		}
		fs.SaveAttrs(true)
		fndr = fs
		fs.Dbg = Debug
	}
	Ns = fndr
	if Ns == nil {
		dbg.Fatal("no name space")
	}
}

// Return the prefix resolving path, the array of trees involved
// (one if it's not a union), and the array of path names in the server of the resource.
func ResolveTree(path string) (string, []zx.RWTree, []string, error) {
	if t, ok := Ns.(*lfs.Lfs); ok {
		return "/", []zx.RWTree{t}, []string{path}, nil
	}
	ns := Ns.(*nspace.Tree)
	return ns.ResolveTree(path)
}

// Issue a find for these names ("filename,predicate")
// If no predicate is given, then "depth<1" is used.
// eg. /a -> just /a; /a, -> subtree at /a
// Found entries with errors lead to a printed warning and are
// not sent through the channel.
// The path attribute in the dir entries returned mimics the paths given
// in the names. If you want absolute paths, supply absolute paths.
func Files(names ...string) <-chan zx.Dir {
	if Ns == nil {
		dbg.Fatal("Files: no name space")
	}
	rc := make(chan zx.Dir)
	go func() {
		var err error
		for _, name := range names {
			toks := strings.SplitN(name, ",", 2)
			if len(toks) == 1 {
				toks = append(toks, "depth<1")
			}
			if toks[0] == "" {
				toks[0] = "."
			}
			toks[0] = path.Clean(toks[0])
			name, _ = filepath.Abs(toks[0])
			dc := Ns.Find(name, toks[1], "/", "/", 0)
			for d := range dc {
				if d == nil {
					break
				}
				if toks[0] != name && zx.HasPrefix(d["path"], name) {
					u := zx.Path(toks[0], zx.Suffix(d["path"], name))
					d["path"] = u
				}
				if d["err"] != "" {
					if d["err"] != "pruned" {
						dbg.Warn("%s: %s", d["path"], d["err"])
					}
					continue
				}
				if ok := rc <- d; !ok {
					close(dc, cerror(rc))
					return
				}
			}
			if derr := cerror(dc); derr != nil {
				dbg.Warn("%s: %s", name, derr)
				err = derr
			} else {
				close(dc) // in case a null was sent but no error
			}
		}
		close(rc, err)
	}()
	return rc
}

// Like Files(names...) but issues a FindGet to retrieve data for all regular
// files involved.
// Remember that you must drain the DirData.Datac channels before receiving
// further DirDatas from the returned channel.
// Directories matching names are also sent.
func GetFiles(names ...string) <-chan zx.DirData {
	if Ns == nil {
		dbg.Fatal("GetFiles: no name space")
	}
	rc := make(chan zx.DirData)
	go func() {
		var err error
		for _, name := range names {
			toks := strings.SplitN(name, ",", 2)
			if len(toks) == 1 {
				toks = append(toks, "depth<1")
			}
			if toks[0] == "" {
				toks[0] = "."
			}
			toks[0] = path.Clean(toks[0])
			name, _ = filepath.Abs(toks[0])
			dprintf("getfiles: findget %s %s\n", name, toks[1])
			gc := Ns.FindGet(name, toks[1], "/", "/", 0)
			for g := range gc {
				dprintf("getfiles: findget -> %v\n", g)
				d := g.Dir
				if d == nil {
					break
				}
				if toks[0] != name && zx.HasPrefix(d["path"], name) {
					u := zx.Path(toks[0], zx.Suffix(d["path"], name))
					d["path"] = u
				}
				if d["err"] != "" {
					if d["err"] != "pruned" {
						dbg.Warn("%s: %s", d["path"], d["err"])
					}
					continue
				}
				if g.Datac == nil && d["type"] != "d" {
					g.Datac = nchan.Null
				}
				if ok := rc <- g; !ok {
					close(gc, cerror(rc))
					return
				}
			}
			if derr := cerror(gc); derr != nil {
				dbg.Warn("%s: %s", name, derr)
				err = derr
			} else {
				close(gc) // in case a null was sent but no error
			}
		}
		close(rc, err)
	}()
	return rc
}

// Implementors of FileRunner may use RunFiles.
type FileRunner interface {
	Warn(fmt string, args ...interface{})
	In() io.Reader
	Intr() <-chan bool
	RunFile(d zx.Dir, dc <-chan []byte) error
}

// Call x.RunFile on each of the files indicated in args (or stdin if none),
// supplying at least the file name in dir and the chan for file data (or nil if it's a directory).
func RunFiles(x FileRunner, args ...string) error {
	if len(args) == 0 {
		dc := make(chan []byte)
		go func() {
			_, _, err := nchan.ReadBytesFrom(x.In(), dc)
			close(dc, err)
		}()
		return x.RunFile(zx.Dir{"path": "-"}, dc)
	}
	gc := GetFiles(args...)
	defer func() {
		close(gc, "done")
	}()
	var sts error
	doselect {
	case <-x.Intr():
		close(gc, "interrupted")
		return errors.New("interrupted")
	case g, ok := <-gc:
		if !ok {
			break
		}
		dir := g.Dir
		if dir["err"] != "" {
			err := errors.New(dir["err"])
			x.Warn("%s: %s", dir["path"], err)
			sts = err
			continue
		}
		if err := x.RunFile(dir, g.Datac); err != nil {
			x.Warn("%s: %s", dir["path"], err)
			sts = err
		}
	}
	if err := cerror(gc); err != nil {
		x.Warn("%s", err)
		sts = err
	}
	return sts
}
