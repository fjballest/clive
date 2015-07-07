package cfs

import (
	"clive/dbg"
	"clive/zx"
	"bytes"
	"fmt"
	"strings"
	"clive/nchan"
)

var ctldir = zx.Dir {
	"path": "/Ctl",
	"spath": "/Ctl",
	"name": "Ctl",
	"size": "8192",	// using size 0 prevents fuse from reading the file
	"type": "-",	// Using "c" makes fuse return an empty file
	"Uid":  dbg.Usr,
	"Gid":  dbg.Usr,
	"Wuid": dbg.Usr,
//	"Sum":  zsum,
	"mode": "0644",
	"proto": "proc",
}

var chgdir = zx.Dir {
	"path": "/Chg",
	"spath": "/Chg",
	"name": "Chg",
	"size": "0",
	"type": "c",
	"Uid":  dbg.Usr,
	"Gid":  dbg.Usr,
	"Wuid": dbg.Usr,
//	"Sum":  zsum,
	"mode": "0440",
	"proto": "proc",
}

func (fs *Cfs) getCtl(off, count int64, pred string, c chan<- []byte, cs *zx.CallStat) (int, error){
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s:\n", fs.Name())

	users := fs.Users()
	for _, k := range users {
		fmt.Fprintf(&buf, "user\t%s\n", k)
	}

	fmt.Fprintf(&buf, "%s", fs.Flags)

	fs.IOstats.Averages()
	fmt.Fprintf(&buf, "%s\n", fs.IOstats.String())

	lctl, _ := zx.GetAll(fs.lfs, "/Ctl")
	if len(lctl) > 0 {
		buf.Write(lctl)
	}

	rctl, _ := zx.GetAll(fs.rfs, "/Ctl")
	if len(rctl) > 0 {
		buf.Write(rctl)
	}
	resp := buf.Bytes()
	o := int(off)
	if o >= len(resp) {
		o = len(resp)
	}
	resp = resp[o:]
	n := int(count)
	if n>len(resp) || n<0 {
		n = len(resp)
	}
	resp = resp[:n]
	cs.Send(int64(len(resp)))
	c <- resp
	return n, nil
}

func (fs *Cfs) putCtl(dc <-chan []byte) error {
	if !fs.NoPermCheck && !ctldir.CanWrite(fs.ai) {
		return fmt.Errorf("/Ctl: %s", dbg.ErrPerm)
	}
	ctl, err := nchan.String(dc)
	if err != nil {
		return fmt.Errorf("/Ctl: %s", err)
	}
	if strings.HasPrefix(ctl, "pass ") {
		nctl := ctl[5:]
		fs.dprintf("pass ctl <%s>\n", nctl)
		return zx.PutAll(fs.rfs, "/Ctl", nil, []byte(nctl))
	}
	fs.dprintf("ctl <%s>\n", ctl)
	if ctl == "sync" {
		fs.cache.Sync(nil)
		return nil
	}
	return fs.Ctl(ctl)
}

