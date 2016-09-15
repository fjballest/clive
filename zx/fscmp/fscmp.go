/*
	Compare two file systems.
	For testing mostly and not using find.
	Refer to zx/repl for a better way using find.
*/
package fscmp

import (
	"clive/zx"
)

// Compute changes for fs1 to become like fs2 and send them through the
// returned chan.
// If no path is given, "/" is used.
func Diff(fs1, fs2 zx.Getter, path ...string) <-chan zx.Chg {
	p := "/"
	if len(path) > 0 {
		p = path[0]
	}
	rc := make(chan zx.Chg)
	// TODO: this should use find instead of stat+get
	go func() {
		var d1, d2 zx.Dir
		var err error
		d1, err = zx.Stat(fs1, p)
		if err == nil {
			d2, err = zx.Stat(fs2, p)
		}
		if err == nil {
			err = diff(d1, d2, fs1, fs2, rc)
		}
		close(rc, err)
	}()
	return rc
}

// Like Diff, but return a slice of changes (mostly for testing)
func Diffs(fs1, fs2 zx.Getter, path ...string) ([]zx.Chg, error) {
	cc := Diff(fs1, fs2, path...)
	cs := []zx.Chg{}
	for c := range cc {
		cs = append(cs, c)
	}
	return cs, cerror(cc)
}

func diffrec(k zx.ChgType, d zx.Dir, fs zx.Getter, c chan<- zx.Chg) error {
	var ds []zx.Dir
	var err error
	var chg zx.Chg
	if d["type"] == "d" {
		ds, err = zx.GetDir(fs, d["path"])
	}
	if err == nil {
		chg = zx.Chg{Type: k, D: d}
	} else {
		chg = zx.Chg{Type: zx.Err, D: d, Err: err}
	}
	if ok := c <- chg; !ok {
		return cerror(c)
	}
	for _, d := range ds {
		if err := diffrec(k, d, fs, c); err != nil {
			return err
		}
	}
	return nil
}

func diff(d1, d2 zx.Dir, fs1, fs2 zx.Getter, c chan<- zx.Chg) error {
	// TODO: zx.Err should go in favor of chg.D["err"] = ...
	if d1["path"] != d2["path"] {
		panic("diff bug, paths differ")
	}
	if d1["type"] != d2["type"] {
		chg := zx.Chg{Type: zx.DirFile, D: d2}
		if ok := c <- chg; !ok {
			return cerror(c)
		}
		return nil
	}
	metachanged := !zx.EqualDirs(d1, d2)
	datachanged := metachanged &&
		(d1["mtime"] != d2["mtime"] || d1["size"] != d2["size"])
	typ := d1["type"]
	if typ != "d" {
		var chg zx.Chg
		if datachanged {
			chg = zx.Chg{Type: zx.Data, D: d2}
		} else if metachanged {
			chg = zx.Chg{Type: zx.Meta, D: d2}
		} else {
			return nil
		}
		if ok := c <- chg; !ok {
			return cerror(c)
		}
		return nil
	}
	var ds1, ds2 []zx.Dir
	var err error
	var chg zx.Chg
	ds1, err = zx.GetDir(fs1, d1["path"])
	if err != nil {
		chg = zx.Chg{Type: zx.Err, D: d1, Err: err}
	} else {
		ds2, err = zx.GetDir(fs2, d2["path"])
		if err != nil {
			chg = zx.Chg{Type: zx.Err, D: d1, Err: err}
		}
	}
	if err != nil {
		if ok := c <- chg; !ok {
			return cerror(c)
		}
		return nil
	}
	i, j := 0, 0
	for i < len(ds1) && j < len(ds2) {
		c1 := ds1[i]
		c2 := ds2[j]
		if c1["name"] < c2["name"] {
			if err := diffrec(zx.Del, c1, fs1, c); err != nil {
				return err
			}
			i++
			continue
		}

		if c2["name"] < c1["name"] {
			if err := diffrec(zx.Add, c2, fs2, c); err != nil {
				return err
			}
			j++
			continue
		}
		if err := diff(c1, c2, fs1, fs2, c); err != nil {
			return err
		}
		i++
		j++
	}
	for ; i < len(ds1); i++ {
		if err := diffrec(zx.Del, ds1[i], fs1, c); err != nil {
			return err
		}
	}
	for ; j < len(ds2); j++ {
		if err := diffrec(zx.Add, ds2[j], fs2, c); err != nil {
			return err
		}
	}
	return nil
}
