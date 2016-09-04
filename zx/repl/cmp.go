package repl

import (
	"clive/zx"
	"clive/cmd"
	"time"
	"fmt"
	"sort"
)


// Where a change was found
type Where int
const (
	Nowhere Where = iota
	Local
	Remote
	Both
)


// A zx file change made to a replicated file
type Chg struct {
	zx.Chg
	At Where
}

var (
	ignoredAttrs = [...]string{"mtime", "Wuid", "Sum", "size"}
	ignoredPutAttrs = [...]string{"Wuid", "size"}
)

func (w Where) String() string {
	switch w {
	case Nowhere:
		return "none"
	case Local:
		return "local"
	case Remote:
		return "remote"
	default:
		panic("bad chg location")
	}
}

func (c Chg) String() string {
	s := ""
	if c.D["err"] != "" {
		s = "\tERR "+s
	}
	switch c.Type {
	case zx.None:
		return "none"
	case zx.Add, zx.Data, zx.Meta, zx.Del, zx.DirFile:
		return fmt.Sprintf("%s %s%s", c.Type, c.D.Fmt(), s)
	default:
		panic("bad chg type")
	}
}

// Compare and compute changes for db to make it like ndb.
// Removes are noted in ndb using the mtime of the dir.
func (ndb *DB) ChangesFrom(db *DB) <-chan Chg {
	return ndb.changesFrom(db, Nowhere)
}

func (ndb *DB) changesFrom(db *DB, w Where) <-chan Chg {
	rc := make(chan Chg)
	nildb := db == nil || db.Root == nil || db.Root.D["path"] == ""
	nilndb := ndb == nil || ndb.Root == nil || ndb.Root.D["path"] == ""
	if nildb && nilndb {
		close(rc)
		return rc
	}
	if nildb || nilndb {
		close(rc, "tree is void")
		return rc
	}
	go func() {
		close(rc, changes(db.Root, ndb.Root, time.Now(), w, rc))
	}()
	return rc
}

func dataChanged(d0, d1 zx.Dir) bool {
	return d0["type"] != d1["type"] ||
		d0.Uint("size") != d1.Uint("size") ||
		d0.Uint("mtime") != d1.Uint("mtime") ||
		d0["Sum"] != "" && d1["Sum"] != "" && d0["Sum"] != d1["Sum"]
}

// does not check attributes that indicate that data changed.
func metaChanged(d0, d1 zx.Dir) bool {
	ud0 := d0.Dup()
	ud1 := d1.Dup()
	for _, k := range ignoredAttrs {
		delete(ud0, k)
		delete(ud1, k)
	}
	return !zx.EqualDirs(ud0, ud1)
}

// does not check attributes that indicate that data changed.
func dirMetaChanged(d0, d1 zx.Dir) bool {
	return metaChanged(d0, d1)
}

func (f *File) came(rc chan<- Chg, at Where) {
	rc <- Chg{Chg: zx.Chg{Type: zx.Add, Time: f.D.Time("mtime"), D: f.D}, At: at}
}

func changes(f0, f1 *File, metat time.Time, w Where, rc chan<- Chg) error {
	d0 := f0.D
	d1 := f1.D
	if d0["rm"] != "" && d1["rm"] != "" {
		return nil
	}
	// Important: ignore files with errors or an error might make us
	// wipe out a replicated subtree, like in plan 9's replica.
	if d0["err"] != "" || d1["err"] != "" {
		if d0["err"] != "pruned" && d1["err"] != "pruned" {
			cmd.Warn("%s: file ignored (%s)", d0["path"], d0["err"])
		}
		return nil
	}
	if d0["path"] != d1["path"] {
		return fmt.Errorf("path '%s' does not match '%s'", d0["path"], d1["path"])
	}
	if d0["rm"] != "" {
		f1.came(rc, w)
		return nil
	}
	d1time := d1.Time("mtime")
	if d1["rm"] != "" {
		rc <- Chg{Chg: zx.Chg{Type: zx.Del, Time: d1time, D: d0}, At: w}
		return nil
	}
	if d0["type"] != d1["type"] {
		rc <- Chg{Chg: zx.Chg{Type: zx.DirFile, Time: d1time, D: d1}, At: w}
		return nil
	}
	if d0["type"] != "d" {
		if dataChanged(d0, d1) {
			rc <- Chg{Chg: zx.Chg{Type: zx.Data, Time: d1time, D: d1}, At: w}
		} else if metaChanged(d0, d1) {
			rc <- Chg{Chg: zx.Chg{Type: zx.Meta, Time: metat, D: d1}, At: w}
		}
		return nil
	}
	if dirMetaChanged(d0, d1) {
		rc <- Chg{Chg: zx.Chg{Type: zx.Meta, Time: metat, D: d1}, At: w}
	}
	names := make([]string, 0, len(f0.Child)+len(f1.Child))
	for _, c0 := range f0.Child {
		names = append(names, c0.D["name"])
	}
	for _, c1 := range f1.Child {
		i := 0
		name := c1.D["name"]
		for ; i < len(names); i++ {
			if names[i] == name {
				break
			}
		}
		if i == len(names) {
			names = append(names, name)
		}
	}
	names = sort.StringSlice(names)
	dels := []zx.Dir{}
	for _, n := range names {
		c0, err0 := f0.Walk1(n)
		c1, err1 := f1.Walk1(n)
		if err0 != nil {
			if c1.D["err"] != "" {
				if c1.D["err"] != "pruned" {
					cmd.Warn("%s: file ignored (%s)",
						c1.D["path"], c1.D["err"])
				}
				continue
			}
			c1.came(rc, w)
			continue
		}
		if err1 != nil {
			if c0.D["err"] != "" || c0.D["rm"] != "" {
				if c1 != nil && c1.D["err"] != "pruned" {
					cmd.Warn("%s: file ignored (%s)",
						c0.D["path"], c0.D["err"])
				}
				continue
			}
			dels = append(dels, c0.D)
			ok := rc <- Chg{
				Chg: zx.Chg{Type: zx.Del, Time: metat, D: c0.D},
				At: w}
			if !ok {
				return nil
			}
			continue
		}
		changes(c0, c1, d1time, w, rc)
	}
	if len(dels) != 0 {
		for _, d := range dels {
			nf := &File{D: d.Dup()}
			nf.D["rm"] = "y"
			nf.D["mtime"] = f1.D["mtime"]
			f1.Child = append(f1.Child, nf)
		}
		sort.Sort(byName(f1.Child))
	}
	return nil
}
