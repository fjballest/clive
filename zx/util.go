package zx

import (
	"bytes"
	"clive/dbg"
	"clive/nchan"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"os"
)

var DebugSend bool	// set to debug sends/receives of full trees
var dprintf = dbg.FlagPrintf(os.Stderr, &DebugSend)

func BadName(elem string) error {
	if strings.Contains(elem, "/") || elem=="." || elem==".." {
		return fmt.Errorf("bad element name '%s'", elem)
	}
	return nil
}

// Make sure s is an abslute path and return it cleaned and never empty.
func AbsPath(s string) (string, error) {
	if len(s)==0 || s[0]!='/' {
		return "", fmt.Errorf("'%s' is not an absolute path", s)
	}
	return path.Clean(s), nil
}

// Report if pref is a prefix path name for name.
// Assume absolute paths.
func HasPrefix(name, pref string) bool {
	if name == pref {
		return true
	}
	if pref=="/" || pref=="" {
		return name=="/" || len(name)>0 && name[0]=='/'
	}
	return len(name)>len(pref) && strings.HasPrefix(name, pref) && name[len(pref)]=='/'
}

// Report if suff is a suffix path name for name.
// Assume absolute paths.
func HasSuffix(name, suff string) bool {
	if name == "" {
		name = "/"
	}
	if suff == "/" {
		suff = ""
	}
	if suff == "" {
		return true
	}
	if name == suff {
		return true
	}
	if len(name) <= len(suff) {
		return false
	}
	return strings.HasSuffix(name, suff) &&
		(suff[0]=='/' || name[len(name)-len(suff)-1]=='/')
}

// If HasPrefix(name,pref) this returns the suffix for pref to be name (suffix starts with "/").
// Assume absolute paths.
// The empty suffix is returned as "/".
func Suffix(name, pref string) string {
	if !HasPrefix(name, pref) {
		return pref
	}
	if name == pref {
		return "/"
	}
	if pref == "/" {
		return name
	}
	return name[len(pref):]
}

// Split path into elements, even if it is empty, absolute or relative.
func Elems(path string) []string {
	if path=="" || path=="/" {
		return []string{}
	}
	if path[0] == '/' {
		return strings.Split(path[1:], "/")
	}
	return strings.Split(path, "/")
}

// Join path names and return a valid non-empty path. Handles correctly
// empty names.
func Path(names ...string) string {
	p := path.Join(names...)
	if p == "" {
		return "/"
	}
	return p
}

// Join path element names and return an absolute, valid, non-empty path.
func ElemsPath(els ...string) string {
	p := path.Join(els...)
	if p == "" {
		return "/"
	}
	if p[0] != '/' {
		return "/" + p
	}
	return p
}

// Return the elements for the common ancestor for the two names, perhaps "/".
// Both names must be absolute paths or "/" will be returned.
func CommonElems(path0, path1 string) []string {
	els0 := Elems(path0)
	els1 := Elems(path1)
	for i := 0; i<len(els0) && i<len(els1); i++ {
		if els0[i] != els1[i] {
			return els0[0:i]
		}
	}
	if len(els0) < len(els1) {
		return els0
	} else {
		return els1
	}
}

// returns <0, 0, >0 if the path a is found before, at or after b
// like string compare but operates on one element at a time to compare.
func PathCmp(path0, path1 string) int {
	els0 := Elems(path0)
	els1 := Elems(path1)
	for i := 0; i<len(els0) && i<len(els1); i++ {
		if els0[i] < els1[i] {
			return -1
		}
		if els0[i] > els1[i] {
			return 1
		}
	}
	if len(els0) < len(els1) {
		return -1
	}
	if len(els0) > len(els1) {
		return 1
	}
	return 0
}

// Part of the RWTree interface, as used by MkdirAll.
type DirMaker interface {
	Stat(path string) chan Dir
	Mkdir(path string, d Dir) chan error
}

// Helper to "mkdir -p" the given name starting at t. The dir entry
// provided is used to set attributes in the directories created.
// Note that in general this leads to one RPC per directory.
func MkdirAll(t DirMaker, name string, nd Dir) error {

	// We could issue all the requests and then pick the replies,
	// but local in-process trees would just spawn a proc to
	// handle most requests and calling the procedures in-order
	// does not mean they receive the request in-order
	// We we must do it one step at a time.
	s, err := AbsPath(name)
	if err != nil {
		return err
	}
	if s == "/" {
		return nil
	}
	els := Elems(s)
	ts := "/"
	for i := 0; i < len(els); i++ {
		ts = Path(ts, els[i])
		err := <-t.Mkdir(ts, nd)
		if err != nil && !dbg.IsExists(err) {
			return err
		}
	}
	d, err := Stat(t, ts)
	if err != nil {
		return err
	}
	if d["type"] != "d" {
		return fmt.Errorf("%s: %s", name, dbg.ErrExists)
	}
	return nil
}

// Helper to retrieve all the data from a Getter (eg, a Dir or File).
func GetAll(fs Getter, path string) ([]byte, error) {
	fc := fs.Get(path, 0, All, "")
	return nchan.Bytes(fc)
}

// Helper to set all the data in a Putter (eg, a File).
func PutAll(fs Putter, path string, d Dir, data []byte) error {
	dc := make(chan []byte, 1)
	dc <- data
	close(dc)
	rc := fs.Put(path, d, 0, dc, "")
	<-rc
	return cerror(rc)
}

// Helper similar to Get, but unpacks directory entries contained in the file and
// returns the array of unpacked directory entries.
// Note that using GetDir on a file might return [], nil, because this simply reads
// the file data assuming it's a set of directory entries. Stat can be used in such case.
func GetDir(fs Getter, path string) ([]Dir, error) {
	var c <-chan []byte
	c = fs.Get(path, 0, All, "")
	return RecvDirs(c)
}

// Like GetDir, but retrieves dirs from c.
func RecvDirs(c <-chan []byte) ([]Dir, error) {
	ds := []Dir{}
	for {
		d, err := RecvDir(c)
		if err!=nil || len(d)==0 {
			if err == nil {
				err = cerror(c)
			} else {
				close(c, err)
			}
			return ds, err
		}
		ds = append(ds, d)
	}

}

// Part of the Tree interface, as used by Walk.
type Walker interface {
	Stat(paths string) chan Dir
}

// Helper to walk now starting at d and obtain the resulting file.
func Walk(fs Walker, d Dir, name string) (Dir, error) {
	p := d["path"]
	np := Path(p, name)
	return Stat(fs, np)
}

// Helper to issue a Stat call and return the Dir entry now.
func Stat(fs Stater, path string) (Dir, error) {
	dc := fs.Stat(path)
	d := <-dc
	if d == nil {
		return nil, cerror(dc)
	}
	return d, nil
}

// Part of the File interface, as used by Send.
type Sender interface {
	Getter
}

/*
	Sprint a file tree rooted at d, for debugging.
*/
func Sprint(f Sender, d Dir) string {
	var buf bytes.Buffer
	Sprintf(&buf, f, d)
	return buf.String()
}

/*
	Dump a file tree rooted at d into w, for debugging.
*/
func Sprintf(w io.Writer, fs Sender, d Dir) error {
	return dump(w, fs, d, 0)
}

func dump(w io.Writer, fs Sender, d Dir, lvl int) error {
	t := strings.Repeat("    ", lvl)
	if fs == nil {
		if _, err := fmt.Fprintf(w, "%s<nil file>\n", t); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "%s%s", t, d); err != nil {
		return err
	}
	if d[Type] == "d" {
		if _, err := fmt.Fprintf(w, "\n"); err != nil {
			return err
		}
		ds, err := GetDir(fs, d["path"])
		if err != nil {
			return err
		}
		for _, xd := range ds {
			if xd["path"] == "/Ctl" {
				continue
			}
			if d[Name] == "" {
				return errors.New("child with no name")
			}
			if err := dump(w, fs, xd, lvl+1); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err := fmt.Fprintf(w, "\n%s[", t); err != nil {
		return err
	}
	dc := fs.Get(d["path"], 0, All, "")
	for m := range dc {
		if len(m) == 0 {
			break
		}
		s := strings.Replace(string(m), "\n", "\n"+t, -1)
		if _, err := fmt.Fprintf(w, "%s", s); err != nil {
			return err
		}
	}
	if err := cerror(dc); err != nil {
		return err
	}
	_, err := fmt.Fprintf(w, "]\n")
	return err
}

func orerr(e1, e2 error) error {
	if e1 != nil {
		return e1
	}
	return e2
}

/*
	Send the tree rooted at d through c.
	The first error is returned, but processing continues to send what we can.

	If the file supports WhiteOutFile, whited out entries are sent as well.
*/
func Send(fs Sender, d Dir, c chan<- []byte) (rerr error) {
	dprintf("=> %s\n", d)
	if fs == nil {
		return nil
	}
	var ds []Dir
	var err error
	isdir := d["type"] == "d"
	if isdir && d["rm"]!="y" {
		ds, err = GetDir(fs, d["path"])
		if err != nil {
			d["err"] = err.Error()
			d["size"] = "0"
			d.Send(c)
			dprintf("=> ERR %s\n", err)
			return err
		}
		if d["path"] == "/" {
			adj := 0
			for _, cd := range ds {
				if cd["name"]=="Ctl" || cd["name"]=="Chg" {
					adj++
				}
			}
			if adj != 0 {
				d["size"] = strconv.Itoa(d.Int("size") - adj)
			}
		}
	}
	if _, err := d.Send(c); err != nil {
		dprintf("=> ERR %s\n", err)
		return err
	}
	rerr = nil
	for _, cd := range ds {
		if cd["path"]!="/Ctl" && cd["path"]!="/Chg" {
			rerr = orerr(rerr, Send(fs, cd, c))
		}
	}
	if !isdir && d["rm"]!="y" && err==nil && d["size"] != "0" {
		cc := fs.Get(d["path"], 0, All, "")
		for x := range cc {
			if len(x) == 0 {
				close(cc)
				break
			}
			dprintf("=> send %d bytes\n", len(x))
			c <- x
		}
		dprintf("=> send 0 bytes\n")
		c <- []byte{}
		rerr = orerr(rerr, cerror(cc))
	}
	return rerr

}

// Part of the RWTree interface as used by Recv.
type Recver interface {
	Put(path string, d Dir, off int64, dc <-chan []byte, pred string) chan Dir
	Mkdir(path string, d Dir) chan error
	Wstat(path string, d Dir) chan error
}

/*
	Receive an entire tree from c updating it at fs.
	Calls Put, Mkdir, and Wstat
	FS operations on the given receiver to process the received files.

	If files carry no data but have non-zero sizes, their
	sizes are updated (to update metadata-only trees).

	It's ok if fs is nil, to receive trees discarding any data.
*/
func Recv(fs Recver, c <-chan []byte) error {
	d, err := RecvDir(c)
	if err != nil {
		dprintf("<= ERR %s\n", err)
		return err
	}
	dprintf("<= %s\n", d)
	mpath := d["path"]
	if mpath == "" {
		dprintf("<= ERR no path\n")
		return fmt.Errorf("no path in dir <%v>", d)
	}
	if d["err"] != "" {
		dprintf("<= ERR %s\n", err)
		return errors.New(d["err"])
	}
	if d["type"] == "d" {
		if fs!=nil && d["path"]!="/" {
			err = <-fs.Mkdir(mpath, d)
			if err != nil {
				dprintf("<= ERR %s\n", err)
			}
		}
		msize := int(d.Int(Size))
		dprintf("<=%s  %d DIRS\n", d["path"], msize)
		for i := 0; i < int(msize); i++ {
			err = orerr(err, Recv(fs, c))
		}
		if fs != nil {
			nd := Dir{}
			nd[Mtime] = d[Mtime]
			fs.Wstat(mpath, nd) // error ignored
		}
	} else {
		if fs == nil {
			for data := <-c; len(data) > 0; data = <-c {
			}
		} else {
			dc := c
			if d["size"] == "0" {
				dc = make(chan []byte)
				close(dc)
			}
			delete(d, "size")
			dirc := fs.Put(mpath, d, 0, dc, "")
			x := <-dirc
			if x == nil {
				dprintf("<= put ERR\n")
				close(c, cerror(dirc))
				err = orerr(err, cerror(dirc))
			}
			dprintf("<= put recv %v\n", x)
			if cerror(c) != nil {
				dprintf("<= recv ERR\n")
			}
		}
	}

	return err
}
