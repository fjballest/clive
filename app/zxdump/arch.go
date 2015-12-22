package main

import (
	"clive/dbg"
	"clive/zx"
	"crypto/sha1"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func waitDumpTime(name string) {
	t := time.Now()
	dt := time.Date(t.Year(), t.Month(), t.Day(), 5, 0, 0, 0, time.Local)
	if dt.Before(time.Now()) {
		dt = dt.Add(24 * time.Hour)
	}
	dbg.Warn("next %s dump at %v", name, dt)
	delta := dt.Sub(t)
	time.Sleep(delta)
}

func dump(dir string, t zx.Tree, ec chan bool) {
	defer func() {
		ec <- true
	}()
	name := t.Name()
	data := zx.Path(dir, "data")
	doskip := Skip
	for {
		if doskip {
			waitDumpTime(name)
			doskip = false
			continue
		}
		dbg.Warn("snap %s...", name)
		if err := os.MkdirAll(data, 0750); err != nil {
			dbg.Warn("%s: %s", data, err)
			return
		}
		rd, err := zx.Stat(t, "/")
		if err != nil {
			dbg.Warn("%s: %s", name, err)
			continue
		}
		// make sure it's not empty
		ds, err := zx.GetDir(t, "/")
		if err != nil {
			dbg.Warn("%s: %s", name, err)
			continue
		}
		if len(ds) == 0 {
			dbg.Warn("%s: file system is empty. ignored.", name)
			continue
		}
		s, err := dumpDir(data, name, zx.File{t, rd})
		if err != nil {
			dbg.Warn("%s: %s", name, err)
		}
		ts := time.Now().Format("2006/0102")
		tree := strings.Replace(name, "/", ".", -1)
		tspath0 := zx.Path(dir, tree, ts)
		os.MkdirAll(path.Dir(tspath0), 0755)
		spath := zx.Path(data, s)
		tspath := tspath0
		for i := 1; ; i++ {
			fi, _ := os.Stat(tspath)
			if fi == nil {
				break
			}
			tspath = fmt.Sprintf("%s.%d", tspath0, i)
		}
		os.MkdirAll(path.Dir(tspath), 0755)
		if err := os.Symlink(spath, tspath); err != nil {
			dbg.Warn("%s: %s", name, err)
		}
		dbg.Warn("snap %s %s", tspath, s)
		if Once {
			break
		}
		waitDumpTime(name)
	}
}

func excluded(name string) bool {
	for _, x := range Xcludes {
		ok, err := filepath.Match(x, name)
		if err == nil && ok {
			return true
		}
	}
	return false
}

func dumpDir(dir, name string, rf zx.File) (string, error) {
	dprintf("dump dir %s %s %s\n", dir, name, rf.D["path"])
	ds := []zx.Dir{}
	istmp := rf.D["name"] == "tmp"
	if !istmp {
		var err error
		ds, err = zx.GetDir(rf.T, rf.D["path"])
		if err != nil {
			dbg.Warn("dumpdir: %s: %s", rf.D["path"], err)
			return "", err
		}
	} else {
		vprintf("%s: temp %s\n", os.Args[0], name)
	}
	dhash := []string{}
	nds := []zx.Dir{}
	for _, d := range ds {
		if d["name"] == "" {
			dbg.Warn("dumpdir: %s: no name", rf.D["path"])
		}
		if d["name"] == "NODUMP" {
			nds = []zx.Dir{}
			dbg.Warn("dumpdir: %s: no dump", rf.D["path"])
			break
		}
		if d["name"] == "FROZEN" {
			vprintf("%s: XXX frozen %s\n", os.Args[0], name)
			/* dir is frozen and the FROZEN contains
			 * the dump path for dir.
			 */
			data, err := zx.GetAll(rf.T, d["path"])
			if err != nil {
				return "", err
			}
			s := string(data)
			if len(strings.Split(s, "/")) != 3 {
				return "", fmt.Errorf("wrong contents in %s", d["path"])
			}
			vprintf("%s: frozen %s\n", os.Args[0], name)
			return strings.TrimSpace(s), nil
		}
		if excluded(d["name"]) {
			dprintf("dump ignored %s\n", d["path"])
			continue
		}
		if t := d["type"]; t != "d" && t != "-" {
			dbg.Warn("dump ignored %s type '%s'\n", d["path"], t)
			continue
		}
		nds = append(nds, d)
	}
	ds = nds
	for _, d := range ds {
		dspath := zx.Path(name, d["name"])
		df, err := zx.Walk(rf.T, rf.D, d["name"])
		if dspath == name {
			panic("zx dump bug")
		}
		if err != nil {
			dbg.Warn("walk: %s: %s", df["path"], err)
			dhash = append(dhash, "")
			dhash = append(dhash, "")
			continue
		}
		var s string
		var e error
		switch d["type"] {
		case "d":
			s, e = dumpDir(dir, dspath, zx.File{rf.T, df})
		case "-":
			s, e = dumpFile(dir, dspath, zx.File{rf.T, df})
		default:
			panic("dump dir type bug")
		}
		if e == nil {
			dhash = append(dhash, d["name"])
			dhash = append(dhash, s)
		} else {
			dbg.Warn("%s: %s", df["path"], e)
		}
		if err == nil && e != nil {
			err = e
			dhash = append(dhash, "")
			dhash = append(dhash, "")
		}
	}
	dval := strings.Join(dhash, "\n")
	h := sha1.New()
	h.Write([]byte(dval))
	sum := h.Sum(nil)
	s := fmt.Sprintf("%02x/%02x/%036x", sum[0], sum[1], sum[2:])
	dprintf("dump dir %s %s\n", name, s)
	dfpath := zx.Path(dir, s)
	fi, _ := os.Stat(dfpath)
	if fi != nil {
		return s, nil
	}
	vprintf("%s: new %s\n", os.Args[0], name)
	return s, newDumpDir(dir, dfpath, rf, ds, dhash)
}

// CAUTION: If the attributes file or its format is ever changed in lfs, then
// saveAttrs and afname must be updated.

// name for attributes file (see lfs)
const afname = ".#zx"

func saveAttrs(dpath string, d zx.Dir) {
	nd := zx.Dir{"name": d["name"]}
	for k, v := range d {
		if zx.IsUpper(k) {
			nd[k] = v
		}
	}
	if len(nd) == 1 {
		return
	}
	dprintf("wrattr %s/%s %v\n", dpath, d["name"], nd)
	fn := path.Join(dpath, afname)
	fd, err := os.OpenFile(fn, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		fd, err = os.OpenFile(fn, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0600)
	}
	if err != nil {
		return
	}
	defer fd.Close()
	fd.Write(nd.Pack())
}

func newDumpDir(dir, dfpath string, rf zx.File, ds []zx.Dir, dhash []string) error {
	if len(dhash) != 2*len(ds) {
		panic("newDumpDir: dhash length bug")
	}
	dprintf("create %s\t%s\n", dfpath, rf.D["path"])
	if err := os.MkdirAll(dfpath, 0750); err != nil {
		return err
	}
	var err error
	for i := 0; i < len(ds); i++ {
		cname := dhash[2*i]
		if cname == "" {
			continue // entry had errors
		}
		if cname != ds[i]["name"] {
			panic("newDumpDir: bad entry")
		}
		cdfpath := zx.Path(dir, dhash[2*i+1])
		cpath := zx.Path(dfpath, cname)
		dprintf("link %s\t<- %s\n", cdfpath, cpath)
		var e error
		e = os.Symlink(cdfpath, cpath)
		// TODO: save user attributes from ds[i] for cname at dfpath/.#zx
		if err == nil && e != nil {
			err = e
		}
		if e == nil {
			saveAttrs(dfpath, ds[i])
		}
	}
	mt := rf.D.Time("mtime")
	os.Chtimes(dfpath, mt, mt)
	mode := rf.D.Int("mode")
	if mode != 0 {
		// uncomment to set dir modes -w
		// mode = mode &^ 0222
		os.Chmod(dfpath, os.FileMode(mode))
	}
	return err
}

func dumpFile(dir, name string, f zx.File) (string, error) {
	dc := f.T.Get(f.D["path"], 0, zx.All, "")
	h := sha1.New()
	for dat := range dc {
		h.Write(dat)
	}
	err := cerror(dc)
	if err != nil {
		dprintf("dump file %s: get: %s\n", f.D["path"], err)
		return "", err
	}
	sum := h.Sum(nil)
	s := fmt.Sprintf("%02x/%02x/%036x", sum[0], sum[1], sum[2:])
	dfpath := zx.Path(dir, s)
	fi, err := os.Stat(dfpath)
	dprintf("dump file %s\t%s\n", name, s)
	if fi != nil {
		return s, nil
	}
	vprintf("%s: new %s\n", os.Args[0], name)
	return s, newDumpFile(dfpath, f)
}

func newDumpFile(dfpath string, f zx.File) error {
	dprintf("create %s\t%s\n", dfpath, f.D["path"])
	d := path.Dir(dfpath)
	if err := os.MkdirAll(d, 0750); err != nil {
		dprintf("%s: mkdir: %s\n", d, err)
		return err
	}
	df, err := os.Create(dfpath + "#")
	if err != nil {
		dprintf("%s#: create: %s\n", dfpath, err)
		return err
	}
	dc := f.T.Get(f.D["path"], 0, zx.All, "")
	for dat := range dc {
		if _, err := df.Write(dat); err != nil {
			dprintf("%s#: write: %s\n", dfpath, err)
			df.Close()
			os.Remove(dfpath + "#")
			return err
		}
	}
	err = cerror(dc)
	if e := df.Close(); err == nil && e != nil {
		err = e
	}
	if err != nil {
		dprintf("%s#: write: %s\n", dfpath, err)
		os.Remove(dfpath + "#")
		return err
	}
	if err := os.Rename(dfpath+"#", dfpath); err != nil {
		dprintf("%s: mv: %s\n", dfpath, err)
		os.Remove(dfpath + "#")
		return err
	}
	mt := f.D.Time("mtime")
	os.Chtimes(dfpath, mt, mt)
	mode := f.D.Int("mode")
	if mode != 0 {
		os.Chmod(dfpath, os.FileMode(mode))
	}
	return nil
}
