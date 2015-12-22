package lfs

import (
	"clive/dbg"
	"clive/zx"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// name for attributes file;	// known to zxdump(1)
const afname = ".#zx"

// at least don't race within the same go program to update zx attributes
var attrlk sync.RWMutex

func readAttrFile(path string) ([]byte, error) {
	return ioutil.ReadFile(path)
}

func writeAttrFile(path string, dat []byte) error {
	return ioutil.WriteFile(path, dat, 0600)
}

// CAUTION: If the attributes file or its format is ever changed, update zxdump to
// create it using the same format.

func appAttrFile(path string, dat []byte) error {
	attrlk.Lock()
	defer attrlk.Unlock()
	fd, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		fd, err = os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	}
	if err != nil {
		return err
	}
	defer fd.Close()
	_, err = fd.Write(dat)
	return err
}

func setUids(d zx.Dir) {
	if d["Uid"] == "" {
		u := dbg.Usr
		if strings.HasPrefix(d["path"], "/usr/") {
			els := zx.Elems(d["path"])
			if len(els) > 1 {
				u = els[1]
			}
		}
		d["Uid"] = u
	}
	if d["Gid"] == "" {
		d["Gid"] = d["Uid"]
	}
	if d["Wuid"] == "" {
		d["Wuid"] = d["Uid"]
	}
}

func (t *Lfs) dirAttrs(dpath string) (map[string]zx.Dir, error) {
	attrlk.Lock()
	defer attrlk.Unlock()
	fn := path.Join(dpath, afname)
	dat, err := readAttrFile(fn)
	if err != nil {
		return nil, err
	}
	ds := make(map[string]zx.Dir)
	var d zx.Dir
	n := 0
	for len(dat) > 0 {
		n++
		d, dat, err = zx.UnpackDir(dat)
		if err != nil {
			return ds, err
		}
		nm := d["name"]
		setUids(d)
		if !DoSum {
			delete(d, "Sum")
		}
		ds[nm] = d
	}
	if n > len(ds)*2 && t.saveattrs {
		t.writeDirAttrs(dpath, ds)
	}
	return ds, nil
}

func (t *Lfs) writeDirAttrs(dpath string, ds map[string]zx.Dir) error {
	fn := path.Join(dpath, afname)
	dat := make([]byte, 0, 1024)
	for _, d := range ds {
		wd := d.Dup()
		if !DoSum {
			delete(wd, "Sum")
		}
		for k := range d {
			r, _ := utf8.DecodeRuneInString(k)
			if k != "name" && !(unicode.IsUpper(r)) {
				delete(wd, k)
			}
		}
		dat = append(dat, wd.Pack()...)
	}
	return writeAttrFile(fn, dat)
}

func (t *Lfs) fileAttrs(fpath string, d zx.Dir) {
	dpath := path.Dir(fpath)
	ds, _ := t.dirAttrs(dpath)
	if len(ds) >= 0 {
		xd := ds[d["name"]]
		if xd["name"] == d["name"] {
			ty, m, s := xd["type"], xd["mtime"], xd["size"]
			for k, v := range xd {
				if zx.IsUpper(k) {
					d[k] = v
				}
			}
			if d["type"] == "-" && t.saveattrs &&
				((d["Sum"] == "" && DoSum) || d["type"] != ty ||
					d["mtime"] != m || d["size"] != s) {
				fi, err := os.Stat(fpath)
				if err == nil {
					xd["type"] = "-"
					xd.SetTime("mtime", fi.ModTime())
					xd["size"] = strconv.FormatInt(fi.Size(), 10)
				}
				if DoSum {
					xd["Sum"] = newFileSum(fpath)
					d["Sum"] = xd["Sum"]
				}
				fn := path.Join(dpath, afname)
				appAttrFile(fn, xd.Pack())
			}
			setUids(d)
			return
		}
	}
	setUids(d)
}

func newFileSum(path string) string {
	if !DoSum {
		return ""
	}
	fd, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer fd.Close()
	h := sha1.New()
	io.Copy(h, fd)
	sum := h.Sum(nil)
	return fmt.Sprintf("%040x", sum)
}

func (t *Lfs) writeFileAttrs(fpath string, d zx.Dir) error {
	dpath := path.Dir(fpath)
	ds, _ := t.dirAttrs(dpath)
	nm := path.Base(fpath)
	od := ds[nm]
	if od == nil {
		od = zx.Dir{}
	}
	od["name"] = nm
	if d["type"] != "d" && (d["size"] == "" || d["type"] == "" || d["mtime"] == "") {
		// record type, size, and mtime for files, to know when to recompute Sum
		fi, err := os.Stat(fpath)
		if err == nil && !fi.IsDir() {
			od["type"] = "-"
			od.SetTime("mtime", fi.ModTime())
			od["size"] = strconv.FormatInt(fi.Size(), 10)
		}
	}
	for k, v := range d {
		if zx.IsUpper(k) {
			if v == "" {
				delete(od, k)
			} else {
				od[k] = v
			}
		}
	}
	if len(od) == 0 {
		return nil
	}
	fn := path.Join(dpath, afname)
	if !DoSum {
		delete(od, "Sum")
	}
	return appAttrFile(fn, od.Pack())
}

// Walk to the given rid checking out perms when t.ai is not nil.
// Returns also the gid and mode even when no perms are checked.
func (t *Lfs) canWalkTo(rid string, forwhat int) (string, uint64, error) {
	if !t.readattrs || rid == "/Ctl" {
		return dbg.Usr, 0777, nil
	}
	noperm := t.NoPermCheck || t.ai == nil
	elems := zx.Elems(rid)
	fpath := t.path
	pgid := ""
	var pmode uint64
	if len(elems) == 0 {
		elems = append(elems, "/")
	}
	for len(elems) > 0 {
		if noperm {
			// skip to just the final step
			if len(elems) > 1 {
				fpath = zx.Path(t.path, path.Dir(rid))
				elems = elems[len(elems)-1:]
			}
		}
		pd := zx.Dir{
			"name": elems[0],
		}
		fpath = zx.Path(fpath, elems[0])
		elems = elems[1:]

		st, err := os.Stat(fpath)
		if err != nil {
			return "", 0, err
		}
		mode := st.Mode()
		m := int64(mode & 0777)
		pd["mode"] = "0" + strconv.FormatInt(m, 8)
		t.fileAttrs(fpath, pd)
		if !noperm && len(elems) > 0 && !pd.CanWalk(t.ai) {
			return "", 0, dbg.ErrPerm
		}
		if len(elems) == 0 {
			pgid = pd["Gid"]
			pmode = uint64(pd.Int("mode"))
		}
		if !noperm && len(elems) == 0 && forwhat != 0 {
			if !pd.Can(t.ai, forwhat) {
				return "", 0, dbg.ErrPerm
			}
		}
	}
	if pgid == "" {
		pgid = dbg.Usr
	}
	if pmode == 0 {
		pmode = 0775
	}
	return pgid, pmode, nil
}
