package nspace

import (
	"clive/dbg"
	"clive/nchan"
	"clive/zx"
	"clive/zx/pred"
	"fmt"
	"path"
	"strings"
)

// Add to pred to exclude suffixes of name from find.
func (ns *Tree) exclSuffixes(name string, fpred *pred.Pred) *pred.Pred {
	var xpreds []*pred.Pred
	for _, p := range ns.pref {
		if p.name!=name && zx.HasPrefix(p.name, name) {
			x, _ := pred.New(fmt.Sprintf("path=%q", p.name))
			xpreds = append(xpreds, x)
		}
	}
	if len(xpreds) > 0 {
		xpred := xpreds[0]
		if len(xpreds) > 1 {
			xpred = pred.Or(xpreds...)
		}
		prune, _ := pred.New("prune")
		if fpred == nil {
			fpred, _ = pred.New("true")
		}
		fpred = pred.Or(pred.And(xpred, prune), fpred)
	}
	return fpred
}

func isfinder(d zx.Dir) bool {
	if d == nil {
		return false
	}
	proto := d["proto"]
	if (strings.Contains(proto, "finder") || strings.Contains(proto, "zx")) &&
		d["addr"]!="" {
		return true
	}
	return strings.Contains(proto, "lfs") || strings.Contains(proto, "proc")
}

// f.name is the entire path being walked.
// f.walked is what we walked so far, perhaps a suffix of f.name during search.
// f.p is a prefix of the name where we are finding, perhaps exactly at name.
// f.pred is f.upred with prunes for all suffixes mounted.
type finder  {
	ns           *Tree
	name         string            // where the find starts, as given by the user
	upred        *pred.Pred        // predicate, as given by the user
	spref, dpref string            // replace spref with dpref in resulting paths
	depth        int               // depth at name is this
	c            chan<- zx.Dir     // send replies here
	gc           chan<- zx.DirData // send replies here

	p      *prefix    // where currently finding
	pred   *pred.Pred // to match dir entries at/under p
	walked string     // so far

	suffs  map[string]*prefix
	spreds map[string]*pred.Pred
}

func (p *prefix) dupDirs() []zx.Dir {
	p.ns.lk.RLock()
	defer p.ns.lk.RUnlock()
	dirs := make([]zx.Dir, 0, len(p.mnt))
	for _, d := range p.mnt {
		dirs = append(dirs, d.Dup())
	}
	return dirs
}

// f.findget for one mount point
func (f *finder) find1get(d zx.Dir) error {
	pname := path.Base(f.walked)
	searching := zx.HasPrefix(f.name, f.walked)
	f.ns.dfprintf("fnd:\t\tfind name %s walked %s sp %s dp %s depth %d searching %v\n",
		f.name, f.walked, f.spref, f.dpref, f.depth, searching)
	if !isfinder(d) {
		f.ns.dfprintf("fnd:\t\tnot a finder\n")
		// it's ok to find it if it's just the name where we are finding.
		if f.name==f.walked || !searching {
			d["name"] = pname
			d["path"] = f.walked
			v, _, _ := f.pred.EvalAt(d, f.depth)
			if v {
				f.gc <- zx.DirData{Dir: d}
			}
		}
		return nil
	}
	spath := d["spath"]
	if spath == "" {
		spath = "/"
	}

	r, err := zx.RWDirTree(d)
	if err != nil {
		f.ns.dfprintf("fnd:\t\tdir tree: %s\n", err)
		return err
	}

	pending := "/"
	if searching {
		pending = zx.Suffix(f.name, f.walked)
	}
	sname := zx.Path(spath, pending)
	f.ns.dfprintf("fnd:\t\tfind(%s %q %s %s %d)\n",
		sname, f.pred, spath, f.walked, f.depth)
	rgc := r.FindGet(sname, f.pred.String(), spath, f.walked, f.depth)
	for rg := range rgc {
		if rg.Datac == nil {
			rg.Datac = nchan.Null
		}
		rd := rg.Dir
		if rd["name"] == "/" {
			rd["name"] = pname
		}

		// if it's a directory pruned (a mounted suffix), recur now for it.
		// note we get pruned errors also for depth < N predicates.
		rpath := rd["path"]
		np := f.suffs[rpath]
		if np!=nil && np.name!=f.p.name {
			nf := &finder{}
			*nf = *f
			nf.p = np
			nf.pred = nf.spreds[rpath]
			nf.walked = rpath // == p.name
			nf.depth = 0
			if nf.walked!=nf.name && zx.HasPrefix(nf.walked, nf.name) {
				els := zx.Elems(zx.Suffix(nf.walked, nf.name))
				nf.depth = len(els)
			}
			for _, nd := range np.dupDirs() {
				f.ns.dfprintf("fnd:\t\trecur at %s\n", nd)
				if err := nf.find1get(nd); err != nil {
					nd["err"] = err.Error()
					f.gc <- zx.DirData{Dir: nd}
				}
			}
			delete(f.suffs, rpath)  // don't repeat the find
			close(rg.Datac, "done") // shouldn't be needed, but just in case
			continue
		}
		if f.spref != f.dpref {
			cpath := rd["path"]
			suff := zx.Suffix(cpath, f.spref)
			rd["path"] = zx.Path(f.dpref, suff)
		}
		if ok := f.gc <- rg; !ok {
			close(rgc, cerror(f.gc))
			break
		}
		// The receiver must receive everything or we'll get stuck in the next recv from rgc.
	}
	return cerror(rgc)
}

// f.find for one mount point
func (f *finder) find1(d zx.Dir) error {
	pname := path.Base(f.walked)
	searching := zx.HasPrefix(f.name, f.walked)
	f.ns.dfprintf("fnd:\t\tfind name %s walked %s sp %s dp %s depth %d searching %v\n",
		f.name, f.walked, f.spref, f.dpref, f.depth, searching)
	if !isfinder(d) {
		f.ns.dfprintf("fnd:\t\tnot a finder: %s\n", d)
		// it's ok to find it if it's just the name where we are finding.
		if f.name==f.walked || !searching {
			d["name"] = pname
			d["path"] = f.walked
			v, _, _ := f.pred.EvalAt(d, f.depth)
			if v {
				f.c <- d
			}
		}
		return nil
	}
	spath := d["spath"]
	if spath == "" {
		spath = "/"
	}

	r, err := zx.RWDirTree(d)
	if err != nil {
		f.ns.dfprintf("fnd:\t\tdir tree: %s\n", err)
		return err
	}

	pending := "/"
	if searching {
		pending = zx.Suffix(f.name, f.walked)
	}
	sname := zx.Path(spath, pending)
	f.ns.dfprintf("fnd:\t\tfind(%s %q %s %s %d)\n",
		sname, f.pred, spath, f.walked, f.depth)
	rc := r.Find(sname, f.pred.String(), spath, f.walked, f.depth)
	for rd := range rc {
		if rd["name"] == "/" {
			rd["name"] = pname
		}

		// if it's a directory pruned (a mounted suffix), recur now for it.
		// note we get pruned errors also for depth < N predicates.
		rpath := rd["path"]
		np := f.suffs[rpath]
		if np!=nil && np.name!=f.p.name {
			nf := &finder{}
			*nf = *f
			nf.p = np
			nf.pred = nf.spreds[rpath]
			nf.walked = rpath // == p.name
			nf.depth = 0
			if nf.walked!=nf.name && zx.HasPrefix(nf.walked, nf.name) {
				els := zx.Elems(zx.Suffix(nf.walked, nf.name))
				nf.depth = len(els)
			}
			for _, nd := range np.dupDirs() {
				f.ns.dfprintf("fnd:\t\trecur at %s\n", nd)
				if err := nf.find1(nd); err != nil {
					nd["err"] = err.Error()
					f.c <- nd
				}
			}
			delete(f.suffs, rpath) // don't repeat the find
			continue
		}
		if f.spref != f.dpref {
			cpath := rd["path"]
			suff := zx.Suffix(cpath, f.spref)
			rd["path"] = zx.Path(f.dpref, suff)
		}
		if ok := f.c <- rd; !ok {
			close(rc, cerror(f.c))
			break
		}
	}
	return cerror(rc)
}

func (f *finder) find() error {
	ns := f.ns
	// Take longest prefix of name with entries, perhaps exactly name.,
	// adding prunes to pred for all of its suffixes.
	ns.lk.RLock()
	f.pred = nil
	var i int
	var fp *prefix
	f.p = nil
	for i = 0; i < len(ns.pref); i++ {
		fp = ns.pref[i]
		if zx.HasPrefix(f.name, fp.name) {
			ns.dfprintf("fnd:\t\tprefix %s of name %s\n", fp.name, f.name)
			f.pred = ns.exclSuffixes(fp.name, f.upred)
			f.p = fp
		}
	}
	// no prefix, no such name (even if there are suffixes!).
	if f.p == nil {
		ns.lk.RUnlock()
		ns.dfprintf("fnd:\t%d %s: failed\n", f.depth, f.name)
		return dbg.ErrNotExist
	}
	ns.dfprintf("fnd:\tdepth %d pref %s for %s\n", f.depth, f.p.name, f.name)

	// Collect all suffixes of name to issue further finds later on,
	// and record their adjusted predicates to exclude their suffixes on their finds.
	f.suffs = map[string]*prefix{}
	f.spreds = map[string]*pred.Pred{}
	for _, xp := range ns.pref {
		if f.p!=xp && zx.HasPrefix(xp.name, f.name) {
			f.suffs[xp.name] = xp
			f.spreds[xp.name] = ns.exclSuffixes(xp.name, f.upred)
		}
	}
	ns.lk.RUnlock()

	// Start one find at a time starting with p, considering that we walked
	// already p's name.
	f.walked = f.p.name
	for _, d := range f.p.dupDirs() {
		ns.dfprintf("fnd:\t\tmnt %s: %s\n", f.p.name, d.Long())
		var err error
		if f.gc != nil {
			err = f.find1get(d)
		} else {
			err = f.find1(d)
		}
		if err != nil {
			d["err"] = err.Error()
			if f.gc != nil {
				f.gc <- zx.DirData{Dir: d}
			} else {
				f.c <- d
			}
		}
	}
	return nil
}

/*
	Implementation of the Finder.Find operation.
*/
func (ns *Tree) Find(name string, fpred string, spref, dpref string, depth0 int) <-chan zx.Dir {
	c := make(chan zx.Dir)
	go ns.findget(name, fpred, spref, dpref, depth0, c, nil)
	return c
}

/*
	Implementation of the Finder.FindGet operation.
*/
func (ns *Tree) FindGet(name string, fpred string, spref, dpref string, depth0 int) <-chan zx.DirData {
	gc := make(chan zx.DirData)
	go ns.findget(name, fpred, spref, dpref, depth0, nil, gc)
	return gc
}

func (ns *Tree) findget(name string, fpred string, spref, dpref string, depth0 int, c chan zx.Dir, gc chan zx.DirData) {
	var err error
	spref, err = zx.AbsPath(spref)
	if err != nil {
		close(c, err)
		close(gc, err)
		return
	}
	dpref, err = zx.AbsPath(dpref)
	if err != nil {
		close(c, err)
		close(gc, err)
		return
	}
	name, err = zx.AbsPath(name)
	if err != nil {
		close(c, err)
		close(gc, err)
		return
	}
	x, err := pred.New(fpred)
	if err != nil {
		close(c, err)
		close(gc, err)
		return
	}
	fndr := &finder{
		ns:    ns,
		name:  name,
		upred: x,
		spref: spref,
		dpref: dpref,
		depth: depth0,
		c:     c,
		gc:    gc,
	}
	ns.dfprintf("fnd:\t%s %s d%d\n", fndr.name, fndr.upred, fndr.depth)
	err = fndr.find()
	close(c, err)
	close(gc, err)
}
