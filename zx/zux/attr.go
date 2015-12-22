package zux

import (
	"sync"
	fpath "path"
	"io/ioutil"
	"clive/zx"
	"os"
)

// File used to store zx attributes
const AttrFile = ".zx"

struct dirAttrs {
	ents map[string]zx.Dir
	dirty bool
}

struct aCache {
	sync.Mutex
	nents int
	dirs map[string]*dirAttrs
	syncc, syncrc chan bool
}

var (
	ac = &aCache{
		dirs: make(map[string]*dirAttrs),
		syncc: make(chan bool),
		syncrc: make(chan bool),
	}
)

func init() {
	go ac.syncer()
}

func (ac *aCache) readDir(dpath string) error {
	da := &dirAttrs{ents: make(map[string]zx.Dir)}
	ac.dirs[dpath] = da
	afname := fpath.Join(dpath, AttrFile)
	dat, err := ioutil.ReadFile(afname)
	if err != nil {
		ac.dirs[dpath] = da
		return err
	}
	n := 0
	for len(dat) > 0 {
		var d zx.Dir
		d, dat, err = zx.UnpackDir(dat)
		if err != nil {
			break
		}
		da.ents[d["name"]] = d
		n++
	}
	ac.nents += len(da.ents)
	if n > 2*len(da.ents) {
		da.sync(dpath, true)
	}
	return nil
}

func (da *dirAttrs) sync(dpath string, creat bool) error {
	if da == nil || !da.dirty {
		return nil
	}
	afname := fpath.Join(dpath, AttrFile)
	flg := os.O_WRONLY|os.O_APPEND|os.O_CREATE
	if creat {
		flg |= os.O_TRUNC
	}
	fd, err := os.OpenFile(afname, flg, 0600)
	if err != nil {
		return err
	}
	defer fd.Close()
	for _, d := range da.ents {
		if _, err := fd.Write(d.Bytes()); err != nil {
			return err
		}
	}
	return nil
}

func (ac *aCache) dosync() {
	ac.Lock()
	defer ac.Unlock()
	for dpath, da := range ac.dirs {
		delete(ac.dirs, dpath)
		ac.nents -= len(da.ents)
		da.sync(dpath, false)
	}
}

func (ac *aCache) syncer() {
	for {
		ok := <- ac.syncc
		if !ok {
			break
		}
		ac.dosync()
		ac.syncrc <- true
	}
}

func (ac *aCache) sync() {
	ac.syncc <- true
	<-ac.syncrc
}

func (ac *aCache) get(path string, d zx.Dir) error {
	ac.Lock()
	defer ac.Unlock()
	dpath := fpath.Dir(path)
	da := ac.dirs[dpath]
	if da == nil {
		if err := ac.readDir(dpath); err != nil {
			return err
		}
	}
	fa := da.ents[d["name"]]
	if fa == nil {
		return nil
	}
	for k, v := range fa {
		d[k] = v
	}
	return nil
}

func (ac *aCache) set(path string, d zx.Dir) error {
	ac.Lock()
	defer ac.Unlock()
	dpath := fpath.Dir(path)
	da := ac.dirs[dpath]
	if da == nil {
		if err := ac.readDir(dpath); err != nil {
			return err
		}
	}
	d = d.Dup()
	delete(d, "path")
	delete(d, "addr")
	delete(d, "mode")
	delete(d, "size")
	delete(d, "mtime")
	delete(d, "type")
	nm := d["name"]
	if _, ok := da.ents[nm]; !ok {
		ac.nents++
	}
	da.ents[nm] = d
	da.dirty = true
	return nil
}

