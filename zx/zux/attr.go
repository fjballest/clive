package zux

import (
	"clive/zx"
	"io/ioutil"
	"os"
	fpath "path"
	"sync"
	"time"
)

// File used to store zx attributes
const AttrFile = ".zx"

struct dirAttrs {
	ents  map[string]zx.Dir
	dirty bool
}

struct aCache {
	sync.Mutex
	nents         int
	dirs          map[string]*dirAttrs
	syncc, syncrc chan bool
}

var (
	ac = &aCache{
		dirs:   make(map[string]*dirAttrs),
		syncc:  make(chan bool),
		syncrc: make(chan bool),
	}
)

func init() {
	go ac.syncer()
}

func (ac *aCache) readDir(dpath string) *dirAttrs {
	da := ac.dirs[dpath]
	if da != nil {
		return da
	}
	da = &dirAttrs{ents: make(map[string]zx.Dir)}
	ac.dirs[dpath] = da
	afname := fpath.Join(dpath, AttrFile)
	dat, err := ioutil.ReadFile(afname)
	if err != nil {
		ac.dirs[dpath] = da
		return da
	}
	n := 0
	for len(dat) > 0 {
		var d zx.Dir
		dat, d, err = zx.UnpackDir(dat)
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
	return da
}

// CAUTION: If the format of the file changes, zxdump/arch.go:^/saveAttrs must
// be updated as well.
func (da *dirAttrs) sync(dpath string, creat bool) error {
	if da == nil || !da.dirty {
		return nil
	}
	afname := fpath.Join(dpath, AttrFile)
	flg := os.O_WRONLY | os.O_APPEND | os.O_CREATE
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
	tick := time.Tick(15 * time.Second)
	doselect {
	case ok := <-ac.syncc:
		if !ok {
			break
		}
		ac.dosync()
		ac.syncrc <- true
	case <-tick:
		ac.dosync()
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
	nm := fpath.Base(path)
	da := ac.readDir(dpath)
	fa := da.ents[nm]
	for k, v := range fa {
		d[k] = v
	}
	return nil
}

func (ac *aCache) set(path string, d zx.Dir) error {
	ac.Lock()
	defer ac.Unlock()
	d = d.Dup()
	nm := fpath.Base(path)
	d["name"] = nm
	dpath := fpath.Dir(path)
	da := ac.readDir(dpath)
	delete(d, "path")
	delete(d, "addr")
	delete(d, "mode")
	delete(d, "size")
	delete(d, "mtime")
	delete(d, "type")

	if od := da.ents[nm]; od != nil {
		for k, v := range d {
			od[k] = v
		}
		d = od
	} else {
		ac.nents++
	}
	da.ents[nm] = d
	da.dirty = true
	return nil
}
