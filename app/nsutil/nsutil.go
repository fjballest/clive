/*
	ZX utilities through the app name space
*/
package nsutil

import (
	"clive/nchan"
	"fmt"
	"clive/dbg"
	"clive/zx"
	"clive/app"
)

// zx.Stat using the app ns and dot.
func Stat(path string) (zx.Dir, error) {
	if len(path) > 0 && path[0] == '#' {
		_, err := app.IOarg(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %s", path, err)
		}
		d := zx.Dir{"path": path, "name": path,
			"upath": path, "type":"c"}
		return d, nil
	}
	path = app.AbsPath(path)
	_, trs, spaths, err := app.ResolveTree(path)
	if err != nil {
		return nil, err
	}
	return zx.Stat(trs[0], spaths[0])
}

func Get(path string, off, count int64, pred string) <-chan []byte {
	if len(path) > 0 && path[0] == '#' {
		rc := make(chan []byte)
		ioc, err := app.IOarg(path)
		if err != nil {
			close(rc, err)
			return rc
		}
		go func() {
			for x := range ioc {
				if b, ok := x.([]byte); ok {
					sok := rc <- b
					if !sok {
						close(ioc, cerror(rc))
						break
					}
				}
			}
			close(rc, cerror(ioc))
		}()
		return rc
	}
	path = app.AbsPath(path)
	_, trs, spaths, err := app.ResolveTree(path)
	if err != nil {
		rc := make(chan []byte)
		close(rc, err)
		return rc
	}
	return trs[0].Get(spaths[0], off, count, pred)
}

// zx.GetAll using the app ns and dot.
func GetAll(path string) ([]byte, error) {
	if len(path) > 0 && path[0] == '#' {
		return nchan.Bytes(Get(path, 0, -1, ""))
	}
	path = app.AbsPath(path)
	_, trs, spaths, err := app.ResolveTree(path)
	if err != nil {
		return nil, err
	}
	return zx.GetAll(trs[0], spaths[0])
}

func GetLines(path string) <-chan string {
	return nchan.Lines(Get(path, 0, -1, ""), '\n')
}

func GetDir(path string) ([]zx.Dir, error) {
	if len(path) > 0 && path[0] == '#' {
		return nil, fmt.Errorf("%s: %s", path, dbg.ErrPerm)
	}
	path = app.AbsPath(path)
	_, trs, spaths, err := app.ResolveTree(path)
	if err != nil {
		return nil, err
	}
	return zx.GetDir(trs[0], spaths[0])
}

func Put(path string, d zx.Dir, off int64, dc <-chan []byte, pred string) chan zx.Dir {
	if len(path) > 0 && path[0] == '#' {
		rc := make(chan zx.Dir, 1)
		if d != nil {
			close(rc, fmt.Errorf("%s: %s", path, dbg.ErrPerm))
			return rc
		}
		ioc, err := app.IOarg(path)
		if err != nil {
			close(rc, err)
			return rc
		}
		go func() {
			for x := range dc {
				sok := ioc <- x
				if !sok {
					close(rc, cerror(rc))
					break
				}
			}
			d := zx.Dir{"path": path, "name": path,
				"upath": path, "type":"c"}
			rc <- d
			close(ioc, cerror(dc))
		}()
		return rc
	}
	path = app.AbsPath(path)
	_, trs, spaths, err := app.ResolveTree(path)
	if err != nil {
		rc := make(chan zx.Dir)
		close(rc, err)
		return rc
	}
	return trs[0].Put(spaths[0], d, off, dc, pred)
}

// If d is nil, mode 644 is used instead.
func PutAll(path string, d zx.Dir, dat []byte) error {
	nmsgs := len(dat)/(32*1024) + 1
	dc := make(chan []byte, nmsgs)
	for len(dat) > 0 {
		n := len(dat)
		if n > 32*1024 {
			n = 32*1024
		}
		dc <- dat[:n]
		dat = dat[n:]
	}
	close(dc)
	if d == nil {
		d = zx.Dir{"mode": "0644"}
	}
	rc := Put(path, d, 0, dc, "")
	<-rc
	return cerror(rc)
}

func Mkdir(path string, d zx.Dir) error {
	if len(path) > 0 && path[0] == '#' {
		return fmt.Errorf("%s: %s", path, dbg.ErrPerm)
	}
	path = app.AbsPath(path)
	_, trs, spaths, err := app.ResolveTree(path)
	if err != nil {
		return err
	}
	return <-trs[0].Mkdir(spaths[0], d)
}
