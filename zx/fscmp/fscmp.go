/*
	Compare two file systems
*/
package fscmp

import (
	"clive/zx"
	"fmt"
)

// A type of change between two trees
type ChgType int

const (
	None    ChgType = iota
	Add             // file was added
	Data            // file data was changed
	Meta            // file metadata was changed
	Del             // file was deleted
	DirFile         // dir replaced with file or file replaced with dir
	Err             // had an error while proceding the dir
	// implies a del of the old tree at file
)

// A change made to a tree wrt another tree
struct Chg {
	Type ChgType
	D    zx.Dir
	Err  error
}

func (ct ChgType) String() string {
	switch ct {
	case None:
		return "none"
	case Add:
		return "add"
	case Data:
		return "data"
	case Meta:
		return "meta"
	case Del:
		return "del"
	case DirFile:
		return "dirfile"
	case Err:
		return "error"
	default:
		panic("bad chg type")
	}
}

func (c Chg) String() string {
	switch c.Type {
	case None:
		return "none"
	case Add, Data, Meta:
		return fmt.Sprintf("%s %s", c.Type, c.D)
	case Del:
		return fmt.Sprintf("%s %s", c.Type, c.D)
	case DirFile:
		return fmt.Sprintf("%s %s", c.Type, c.D)
	case Err:
		return fmt.Sprintf("%s %s %s", c.Type, c.D["path"], c.Err)
	default:
		panic("bad chg type")
	}
}

// Compute changes for fs1 to become like fs2 and send them to the
// returned chan.
// If no path is given, "/" is used.
func Diff(fs1, fs2 zx.Getter, path ...string) <-chan Chg {
	p := "/"
	if len(path) > 0 {
		p = path[0]
	}
	rc := make(chan Chg)
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
func Diffs(fs1, fs2 zx.Getter, path ...string) ([]Chg, error) {
	cc := Diff(fs1, fs2, path...)
	cs := []Chg{}
	for c := range cc {
		cs = append(cs, c)
	}
	return cs, cerror(cc)
}

func diffrec(k ChgType, d zx.Dir, fs zx.Getter, c chan<- Chg) error {
	var ds []zx.Dir
	var err error
	var chg Chg
	if d["type"] == "d" {
		ds, err = zx.GetDir(fs, d["path"])
	}
	if err == nil {
		chg = Chg{Type: k, D: d}
	} else {
		chg = Chg{Type: Err, D: d, Err: err}
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

func diff(d1, d2 zx.Dir, fs1, fs2 zx.Getter, c chan<- Chg) error {
	if d1["path"] != d2["path"] {
		panic("diff bug, paths differ")
	}
	if d1["type"] != d2["type"] {
		chg := Chg{Type: DirFile, D: d2}
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
		var chg Chg
		if datachanged {
			chg = Chg{Type: Data, D: d2}
		} else if metachanged {
			chg = Chg{Type: Meta, D: d2}
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
	var chg Chg
	ds1, err = zx.GetDir(fs1, d1["path"])
	if err != nil {
		chg = Chg{Type: Err, D: d1, Err: err}
	} else {
		ds2, err = zx.GetDir(fs2, d2["path"])
		if err != nil {
			chg = Chg{Type: Err, D: d1, Err: err}
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
			if err := diffrec(Del, c1, fs1, c); err != nil {
				return err
			}
			i++
			continue
		}

		if c2["name"] < c1["name"] {
			if err := diffrec(Add, c2, fs2, c); err != nil {
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
		if err := diffrec(Del, ds1[i], fs1, c); err != nil {
			return err
		}
	}
	for ; j < len(ds2); j++ {
		if err := diffrec(Add, ds2[j], fs2, c); err != nil {
			return err
		}
	}
	return nil
}
