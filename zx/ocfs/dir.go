package ocfs

import (
	"clive/bufs"
	"clive/dbg"
	"clive/net/auth"
	"clive/zx"
	"clive/zx/pred"
	"crypto/sha1"
	"fmt"
	"io"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

/*
	Cached directory entry (and file data)
*/
type Dir  {
	// inmutable
	z    *Cfs
	path string // cached from d
	mode uint   // cached from d
	sync.RWMutex
	mct    time.Time       // time of the last stat, zt if invalidated
	dct    time.Time       // time of the last data read, zt if invalidated
	d      zx.Dir          // attributes
	parent *Dir            // parent directory
	child  map[string]*Dir // children, nil if not yet read
	data   *bufs.Blocks    // cached data for files, nil if not yet read
	ghost  bool            // the file is no longer in the tree.
	epoch  int
}

var zt time.Time

type byName []*Dir

func (b byName) Len() int {
	return len(b)
}

func (b byName) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b byName) Less(i, j int) bool {
	return b[i].d["name"] < b[j].d["name"]
}

func (zd *Dir) String() string {
	return "cfs:" + zd.path
}

// Make a new Dir form a zd.Dir
func (z *Cfs) newDir(d zx.Dir) *Dir {
	zd := &Dir{
		z:     z,
		d:     d,
		path:  d["path"],
		mode:  uint(d.Uint64("mode")&0777),
		epoch: *z.epoch,
	}
	if zd.mode == 0 {
		zd.mode = 0777
	}
	return zd
}

func (zd *Dir) zprintf(op string, args ...interface{}) {
	zd.z.zprintf(zd.path, op, args...)
}

func (zd *Dir) dprintf(op string, args ...interface{}) {
	zd.z.dprintf(zd.path, op, args...)
}

func (zd *Dir) cprintf(op string, args ...interface{}) {
	zd.z.cprintf(zd.path, op, args...)
}

// Dump a debug representation of the tree rooted at zd into w, with lvl indent.
func (zd *Dir) DumpTo(w io.Writer, lvl int) {
	zd.RLock()
	tabs := strings.Repeat("    ", lvl)
	s := ""
	if zd.ghost {
		s = " ghost"
	}
	fmt.Fprintf(w, "%s%s %s %s %s", tabs, zd.path, zd.d["type"], zd.d["mode"], s)
	fmt.Fprintf(w, " sz %s", zd.d["size"])

	x := zd.z.Debug
	zd.z.Debug = false
	if zd.staleMeta() {
		fmt.Fprintf(w, " mstale")
	}
	if zd.dct == zt {
		fmt.Fprintf(w, " dinval")
	}
	zd.z.Debug = x

	if zd.d["type"] == "d" {
		if zd.data != nil {
			fmt.Fprintf(w, " DATA")
		}
		if zd.child == nil {
			fmt.Fprintf(w, " unread")
		} else {
			fmt.Fprintf(w, " child[%d]", len(zd.children()))
		}
	} else {
		if zd.child != nil {
			fmt.Fprintf(w, " CHILD")
		}
		if zd.data == nil {
			fmt.Fprintf(w, " unread")
		} else {
			fmt.Fprintf(w, " data[%d]", zd.data.Len())
		}
	}
	if zd.d["path"] != zd.path {
		fmt.Fprintf(w, " PATH<%s>", zd.d["path"])
	}
	if path.Base(zd.path) != zd.d["name"] {
		fmt.Fprintf(w, " NAME<%s>", zd.d["name"])
	}
	fmt.Fprintf(w, "\n")
	cl := zd.children()
	zd.RUnlock()
	sort.Sort(byName(cl))
	for _, c := range cl {
		if c.parent != zd {
			fmt.Fprintf(w, "BAD PARENT")
		}
		c.DumpTo(w, lvl+1)
	}
}

func (zd *Dir) canWstat(ai *auth.Info, nd zx.Dir) error {
	isowner := ai==nil || zd.d["Uid"]=="" || zd.d["Uid"]==ai.Uid
	canwrite := zd.canWrite(ai)
	for k, v := range nd {
		if v==zd.d[k] || ai!=nil && ai.Uid=="elf" {
			continue // no change really
		}
		switch k {
		case "mode":
			if !isowner {
				return fmt.Errorf("mode: %s", dbg.ErrPerm)
			}
		case "mtime", "size":
			if !canwrite {
				return fmt.Errorf("mtime: %s", dbg.ErrPerm)
			}
		case "type", "name", "Wuid":
			return fmt.Errorf("%s: %s", k, dbg.ErrPerm)
		case "Uid":
			if !ai.InGroup("sys") {
				return fmt.Errorf("%s: %s", k, dbg.ErrPerm)
			}
		case "Gid":
			if !ai.InGroup(v) && !ai.InGroup("sys") {
				return fmt.Errorf("%s: %s", k, dbg.ErrPerm)
			}
		default:
			if len(k)>0 && k[0]>='A' && k[0]<='Z' &&
				!canwrite && !isowner {
				return fmt.Errorf("mtime: %s", dbg.ErrPerm)
			}
		}
	}
	return nil
}

func (zd *Dir) canExec(ai *auth.Info) bool {
	if ai==nil || ai.Uid=="elf" {
		return zd.mode&0111 != 0
	}
	uid := zd.d["Uid"]
	if uid=="" || ai.Uid==uid || ai.Uid=="elf" {
		return zd.mode&0111 != 0
	}
	if ai.InGroup(zd.d["Gid"]) {
		return zd.mode&0011 != 0
	}
	return zd.mode&0001 != 0
}

func (zd *Dir) canWrite(ai *auth.Info) bool {
	if ai==nil || ai.Uid=="elf" {
		return zd.mode&0222 != 0
	}
	uid := zd.d["Uid"]
	if uid=="" || ai.Uid==uid {
		return zd.mode&0222 != 0
	}
	if ai.InGroup(zd.d["Gid"]) {
		return zd.mode&0022 != 0
	}
	return zd.mode&0002 != 0
}

func (zd *Dir) canRead(ai *auth.Info) bool {
	if ai==nil || ai.Uid=="elf" {
		return zd.mode&0444 != 0
	}
	uid := zd.d["Uid"]
	if uid=="" || ai.Uid==uid {
		return zd.mode&0444 != 0
	}
	if ai.InGroup(zd.d["Gid"]) {
		return zd.mode&0044 != 0
	}
	return zd.mode&0004 != 0
}

func (zd *Dir) canWalk(ai *auth.Info) bool {
	return zd.canExec(ai)
}

func (zd *Dir) match(fpred string) error {
	if fpred == "" {
		return nil
	}
	p, err := pred.New(fpred)
	if err != nil {
		return err
	}
	match, _, err := p.EvalAt(zd.d, 0)
	if err != nil {
		return err
	}
	if !match {
		return ErrNoMatch
	}
	return nil
}

// return a copy of the list of children
func (zd *Dir) children() []*Dir {
	if len(zd.child) == 0 {
		return nil
	}
	child := make([]*Dir, 0, len(zd.child))
	var ghosts []string
	for k, v := range zd.child {
		if v.ghost {
			ghosts = append(ghosts, k)
		} else {
			child = append(child, v)
		}
	}
	for _, g := range ghosts {
		delete(zd.child, g)
	}
	return child
}

// compute sum and size for children
func (zd *Dir) newSumSize() {
	names := make([]string, 0, len(zd.child))
	for k, v := range zd.child {
		if !v.ghost {
			names = append(names, k)
		}
	}
	sort.Sort(sort.StringSlice(names))
	h := sha1.New()
	for _, n := range names {
		h.Write([]byte(n))
	}
	sum := h.Sum(nil)
	zd.d["Sum"] = fmt.Sprintf("%040x", sum)
	zd.d["size"] = strconv.Itoa(len(names))
}

// add a child and compute sum and size for children
func (zd *Dir) addChild(elem string, zcd *Dir) {
	zcd.parent = zd
	zd.child[elem] = zcd
	zd.newSumSize()
	zd.wstatAttrs("Sum")
}

// Make sure zd is idle by locking and unlocking everything
func (zd *Dir) wait() {
	zd.Lock()
	defer zd.Unlock()
	for _, c := range zd.child {
		c.wait()
	}
}

// called with zd rlocked.
// unlocks zd when moving down to children.
func (zd *Dir) find(d zx.Dir, p *pred.Pred, spref, dpref string, lvl int, c chan<- zx.Dir, ai *auth.Info) (int, error) {
	nm := 0
	if zd.ghost {
		d["err"] = "file error in find (shouldn't happen)"
		zd.RUnlock()
		c <- d
		return 1, nil
	}
	match, pruned, err := p.EvalAt(d, lvl)
	if pruned {
		zd.RUnlock()
		if !match {
			d["err"] = "pruned"
		}
		c <- d
		return 1, nil
	}
	if err != nil {
		zd.RUnlock()
		return nm, fmt.Errorf("eval: %s", err)
	}
	if d["rm"] != "" {
		zd.RUnlock()
		return 0, nil
	}
	var ds []*Dir
	if d["type"] == "d" {
		if !zd.canWalk(ai) {
			d["err"] = "walk: " + dbg.ErrPerm.Error()
			c <- d
			zd.RUnlock()
			return 1, nil
		}
		ds = zd.children()
		sort.Sort(byName(ds))
	}
	if match {
		zd.RUnlock()
		d["spath"] = d["path"]
		if ok := c <- d; !ok {
			return 1, nil
		}
		nm++
	} else {
		zd.RUnlock()
	}
	for _, zcd := range ds {
		// fmt.Printf("child %s\n", cd)
		zcd.RLock()
		if zcd.d["type"] == "d" {
			if err := zcd.updData(true); err != nil {
				zcd.RUnlock()
				return 0, err
			}
		} else {
			if err := zcd.updMeta(true); err != nil {
				zcd.RUnlock()
				return 0, err
			}
		}
		nd := zcd.d.Dup()
		nd["path"] = zx.Path(d["path"], nd["name"])
		nd["spath"] = nd["path"]
		if spref != dpref {
			suff := zx.Suffix(nd["path"], spref)
			nd["path"] = zx.Path(dpref, suff)
		}
		if err != nil {
			nd["err"] = "read: " + err.Error()
			c <- nd
			nm++
			zcd.RUnlock()
			continue
		}
		cnm, _ := zcd.find(nd, p, spref, dpref, lvl+1, c, ai) // runlocks zcd
		nm += cnm
	}
	return nm, nil
}
