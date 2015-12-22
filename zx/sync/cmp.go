package sync

import (
	"bytes"
	"clive/dbg"
	"clive/zx"
	"fmt"
	"sort"
	"time"
)

// A change between two trees
type ChgType int

const (
	None    ChgType = iota
	Add             // file was added
	Data            // file data was changed
	Meta            // file metadata was changed
	Del             // file was deleted
	DirFile         // dir replaced with file or file replaced with dir
	// implies a del of the old tree at file
)

type Chg struct {
	Type ChgType
	Time time.Time
	D    zx.Dir
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
	default:
		panic("bad chg type")
	}
}

func nouid(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// Print d in DB format
func DbFmt(d zx.Dir) string {
	var b bytes.Buffer

	fmt.Fprintf(&b, "%-14s", d["path"])
	typ := d["type"]
	if typ == "" {
		fmt.Fprintf(&b, " -")
	} else {
		fmt.Fprintf(&b, " %s", typ)
	}
	if d["rm"] != "" {
		fmt.Fprintf(&b, " GONE")
	} else {
		fmt.Fprintf(&b, " 0%o", d.Mode())
	}
	uid := nouid(d["Uid"])
	gid := nouid(d["Gid"])
	wuid := nouid(d["Wuid"])
	fmt.Fprintf(&b, " %-8s %-8s %-8s", uid, gid, wuid)
	fmt.Fprintf(&b, " %8d", d.Int64("size"))
	if d["type"] != "d" {
		fmt.Fprintf(&b, " %d", d.Uint64("mtime"))
	}
	if d["err"] != "" {
		fmt.Fprintf(&b, " %s", d["err"])
	}
	return b.String()
}

func (c Chg) String() string {
	switch c.Type {
	case None:
		return "none"
	case Add, Data, Meta:
		return fmt.Sprintf("%s %s", c.Type, DbFmt(c.D))
	case Del:
		return fmt.Sprintf("%s %s", c.Type, DbFmt(c.D))
	case DirFile:
		return fmt.Sprintf("%s %s", c.Type, DbFmt(c.D))
	default:
		panic("bad chg type")
	}
}

// Compare and compute changes for db to make it like ndb.
// Removes are noted in ndb using the mtime of the dir.
func (db *DB) ChangesTo(ndb *DB) <-chan Chg {
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
		close(rc, changes(db.Root, ndb.Root, time.Now(), rc))
	}()
	return rc
}

// Update the db with changes found on the given finder
func (db *DB) Update(fs Finder) error {
	ndb, err := NewDB(db.Name, db.Pred, fs)
	if err != nil {
		return err
	}
	cc := db.ChangesTo(ndb)
	for range cc {
	}
	db.Root = ndb.Root
	db.lastpf = nil
	db.lastpdir = ""
	return cerror(cc)
}

// Compare and compute changes between db0 and db1 considered as peers.
// That is, changes detected won't try to make db0 like db1,
// they will try to make both the same, relying on times to determine which change
// has to be made.
// Changes to be made at db0 are reported at pullc, those for db1 at pushc.
// To make sure removals propagate the right way, both trees should record
// removed file entries (d["rm"]="y") for files already gone.
// This implies using db.ChangesTo(ndb) on each tree before calling Changes.
// Otherwise, if a file is seen at one tree and not at the other, no removal time is
// assumed and, instead, it is added to where it's missing.
//
func Changes(db0, db1 *DB) (pullsc, pushesc <-chan Chg) {
	pullc := make(chan Chg)
	pushc := make(chan Chg)
	nildb0 := db0 == nil || db0.Root == nil || db0.Root.D["path"] == ""
	nildb1 := db1 == nil || db1.Root == nil || db1.Root.D["path"] == ""
	if nildb0 && nildb1 {
		close(pullc)
		close(pushc)
		return pullc, pushc
	}
	if nildb0 || nildb1 {
		close(pullc, "tree is void")
		close(pushc, "tree is void")
		return pullc, pushc
	}
	go func() {
		t := time.Now()
		err := sync(db0.Root, db1.Root, t, t, pullc, pushc)
		close(pullc, err)
		close(pushc, err)
	}()
	return pullc, pushc
}

var ignoredAttrs = [...]string{"mtime", "Wuid", "Sum", "size"}
var ignoredPutAttrs = [...]string{"Wuid", "size"}

func dataChanged(d0, d1 zx.Dir) bool {
	return d0["type"] != d1["type"] ||
		d0.Int64("size") != d1.Int64("size") ||
		d0.Int64("mtime") != d1.Int64("mtime") ||
		d0["Sum"] != "" && d1["Sum"] != "" && d0["Sum"] != d1["Sum"]
}

// does not check attributes that indicate that data changed.
func metaChanged(d0, d1 zx.Dir) bool {
	ud0 := d0.UsrAttrs()
	ud1 := d1.UsrAttrs()
	for _, k := range ignoredAttrs {
		delete(ud0, k)
		delete(ud1, k)
	}
	return !zx.EqDir(ud0, ud1)
}

// does not check attributes that indicate that data changed.
func dirMetaChanged(d0, d1 zx.Dir) bool {
	return metaChanged(d0, d1)
}

func (f *File) came(rc chan<- Chg) {
	rc <- Chg{Type: Add, Time: f.D.Time("mtime"), D: f.D}
}

func changes(f0, f1 *File, metat time.Time, rc chan<- Chg) error {
	d0 := f0.D
	d1 := f1.D
	if d0["rm"] != "" && d1["rm"] != "" {
		return nil
	}
	// Important: ignore files with errors or an error might make us
	// wipe out a replicated subtree, like in plan 9's replica.
	if d0["err"] != "" || d1["err"] != "" {
		if d0["err"] != "pruned" && d1["err"] != "pruned" {
			dbg.Warn("%s: file ignored (%s)", d0["path"], d0["err"])
		}
		return nil
	}
	if d0["path"] != d1["path"] {
		return fmt.Errorf("path '%s' does not match '%s'", d0["path"], d1["path"])
	}
	if d0["rm"] != "" {
		f1.came(rc)
		return nil
	}
	d1time := d1.Time("mtime")
	if d1["rm"] != "" {
		rc <- Chg{Type: Del, Time: d1time, D: d0}
		return nil
	}
	if d0["type"] != d1["type"] {
		rc <- Chg{Type: DirFile, Time: d1time, D: d1}
		return nil
	}
	if d0["type"] != "d" {
		if dataChanged(d0, d1) {
			rc <- Chg{Type: Data, Time: d1time, D: d1}
		} else if metaChanged(d0, d1) {
			rc <- Chg{Type: Meta, Time: metat, D: d1}
		}
		return nil
	}
	if dirMetaChanged(d0, d1) {
		rc <- Chg{Type: Meta, Time: metat, D: d1}
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
					dbg.Warn("%s: file ignored (%s)",
						c1.D["path"], c1.D["err"])
				}
				continue
			}
			c1.came(rc)
			continue
		}
		if err1 != nil {
			if c0.D["err"] != "" {
				if c1.D["err"] != "pruned" {
					dbg.Warn("%s: file ignored (%s)",
						c0.D["path"], c0.D["err"])
				}
				continue
			}
			dels = append(dels, c0.D)
			ok := rc <- Chg{Type: Del, Time: metat, D: c0.D}
			if !ok {
				return nil
			}
			continue
		}
		changes(c0, c1, d1time, rc)
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

// NB: for removals to synchronize ok, the tree synchronized should be brought up to date
// with respect to its old version, so it gets "rm" entries as needed to record remove times.
//
func sync(f0, f1 *File, meta0t, meta1t time.Time, pullc, pushc chan<- Chg) error {
	d0 := f0.D
	d1 := f1.D
	if d0["rm"] != "" && d1["rm"] != "" {
		return nil
	}
	if d0["path"] != d1["path"] {
		return fmt.Errorf("path '%s' does not match '%s'", d0["path"], d1["path"])
	}
	if d0["err"] != "" || d1["err"] != "" {
		if d0["err"] != "pruned" && d1["err"] != "pruned" {
			dbg.Warn("%s: file ignored (%s)", d0["path"], d0["err"])
		}
		return nil
	}
	d0time := d0.Time("mtime")
	d1time := d1.Time("mtime")
	d, nd := d0, d1
	f, nf := f0, f1
	dtime := d1time
	metat := meta1t
	cc := pullc
	if d0time.After(d1time) {
		d, nd = nd, d
		f, nf = nf, f
		dtime = d0time
		metat = meta0t
		cc = pushc
	}
	if d["rm"] != "" {
		f.came(cc)
		return nil
	}
	if d1["rm"] != "" {
		cc <- Chg{Type: Del, Time: dtime, D: d}
		return nil
	}
	if d["type"] != nd["type"] {
		cc <- Chg{Type: DirFile, Time: dtime, D: nd}
		return nil
	}
	if d0["type"] != "d" {
		if dataChanged(d, nd) {
			cc <- Chg{Type: Data, Time: dtime, D: nd}
		} else if metaChanged(d, nd) {
			cc <- Chg{Type: Meta, Time: metat, D: nd}
		}
		return nil
	}
	if dirMetaChanged(d, nd) {
		cc <- Chg{Type: Meta, Time: metat, D: nd}
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
	for _, n := range names {
		c0, err0 := f0.Walk1(n)
		c1, err1 := f1.Walk1(n)
		if err0 != nil {
			// If we didn't see a d["rm"] entry for this, assume it came.
			c1.came(pullc)
			continue
		}
		if err1 != nil {
			// If we didn't see a d["rm"] entry for this, assume it came.
			c0.came(pushc)
			continue
		}
		sync(c0, c1, d0time, d1time, pullc, pushc)
	}
	return nil
}
