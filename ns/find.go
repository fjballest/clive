package ns

import (
	"clive/zx"
	"clive/zx/pred"
	"fmt"
	fpath "path"
)

// Add to pred to exclude suffixes of name from find.
func (ns *NS) exclSuffixes(name string, fpred *pred.Pred) *pred.Pred {
	var xpreds []*pred.Pred
	for _, p := range ns.pref {
		if p.name != name && zx.HasPrefix(p.name, name) {
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

// f.name is the entire path being walked.
// f.walked is what we walked so far, perhaps a suffix of f.name during search.
// f.p is a prefix of the name where we are finding, perhaps exactly at name.
// f.pred is f.upred with prunes for all suffixes mounted.
struct finder {
	ns           *NS
	name         string        // where the find starts, as given by the user
	upred        *pred.Pred    // predicate, as given by the user
	spref, dpref string        // replace spref with dpref in resulting paths
	depth        int           // depth at name is this
	c            chan<- zx.Dir // send replies here
	gc           chan<- face{} // send replies here

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
	pname := fpath.Base(f.walked)
	searching := zx.HasPrefix(f.name, f.walked)
	f.ns.vprintf("fnd:\t\tfind name %s walked %s sp %s dp %s depth %d searching %v\n",
		f.name, f.walked, f.spref, f.dpref, f.depth, searching)
	if !d.IsFinder() {
		f.ns.vprintf("fnd:\t\tnot a finder\n")
		// it's ok to find it if it's just the name where we are finding.
		if f.name == f.walked || !searching {
			d["name"] = pname
			d["path"] = f.walked
			v, _, _ := f.pred.EvalAt(d, f.depth)
			if v {
				if ok := f.gc <- d; !ok {
					return cerror(f.gc)
				}
			}
		}
		return nil
	}

	rf, err := DirFs(d)
	if err != nil {
		f.ns.vprintf("fnd:\t\tdir fs: %s\n", err)
		return err
	} else {
		f.ns.Dprintf("ns dialed %s\n", d.SAddr())
	}
	spath := d.SPath()
	r, ok := rf.(zx.FindGetter)
	if !ok {
		f.ns.vprintf("fnd:\t\tdir fs: not a findgetter\n")
		return fmt.Errorf("%s: not a findgetter", d["path"])
	}
	pending := "/"
	if searching {
		pending = zx.Suffix(f.name, f.walked)
	}
	sname := fpath.Join(spath, pending)
	f.ns.vprintf("fnd:\t\tfind(%s %q %s %s %d)\n",
		sname, f.pred, spath, f.walked, f.depth)
	rgc := r.FindGet(sname, f.pred.String(), spath, f.walked, f.depth)
	for rg := range rgc {
		rd, ok := rg.(zx.Dir)
		if !ok {
			f.ns.vprintf("fnd: fwd msg type %T\n", rg)
			if ok := f.gc <- rg; !ok {
				close(rgc, cerror(f.gc))
				break
			}
			continue
		}
		if rd["name"] == "/" {
			rd["name"] = pname
		}

		// if it's a directory pruned (a mounted suffix), recur now for it.
		// note we get pruned errors also for depth < N predicates.
		rpath := rd["path"]
		np := f.suffs[rpath]
		if np != nil && np.name != f.p.name {
			nf := &finder{}
			*nf = *f
			nf.p = np
			nf.pred = nf.spreds[rpath]
			nf.walked = rpath // == p.name
			nf.depth = 0
			if nf.walked != nf.name && zx.HasPrefix(nf.walked, nf.name) {
				els := zx.Elems(zx.Suffix(nf.walked, nf.name))
				nf.depth = len(els)
			}
			for _, nd := range np.dupDirs() {
				f.ns.vprintf("fnd:\t\trecur at %s\n", nd)
				if err := nf.find1get(nd); err != nil {
					nd["err"] = err.Error()
					if ok := f.gc <- nd; !ok {
						close(rgc, cerror(f.gc))
						return cerror(rgc)
					}
				}
			}
			delete(f.suffs, rpath) // don't repeat the find
			continue
		}
		if f.spref != f.dpref {
			cpath := rd["path"]
			suff := zx.Suffix(cpath, f.spref)
			rd["path"] = fpath.Join(f.dpref, suff)
		}
		if ok := f.gc <- rd; !ok {
			close(rgc, cerror(f.gc))
			break
		}
	}
	return cerror(rgc)
}

// f.find for one mount point
func (f *finder) find1(d zx.Dir) error {
	pname := fpath.Base(f.walked)
	searching := zx.HasPrefix(f.name, f.walked)
	f.ns.vprintf("fnd:\t\tfind1 name %s walked %s sp %s dp %s depth %d searching %v\n",
		f.name, f.walked, f.spref, f.dpref, f.depth, searching)
	if !d.IsFinder() {
		f.ns.vprintf("fnd:\t\tnot a finder: %s\n", d)
		// it's ok to find it if it's just the name where we are finding.
		if f.name == f.walked || !searching {
			d["name"] = pname
			d["path"] = f.walked
			v, _, _ := f.pred.EvalAt(d, f.depth)
			if v {
				f.c <- d
			}
		}
		return nil
	}

	rf, err := DirFs(d)
	if err != nil {
		f.ns.vprintf("fnd:\t\tdir fs: %s\n", err)
		return err
	}
	spath := d.SPath()
	r, ok := rf.(zx.Finder)
	if !ok {
		f.ns.vprintf("fnd:\t\tdir fs: not a finder\n")
		return fmt.Errorf("%s: not a finder", d["path"])
	}

	pending := "/"
	if searching {
		pending = zx.Suffix(f.name, f.walked)
	}
	sname := fpath.Join(spath, pending)
	f.ns.vprintf("fnd:\t\tfind(%s %q %s %s %d)\n",
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
		if np != nil && np.name != f.p.name {
			nf := &finder{}
			*nf = *f
			nf.p = np
			nf.pred = nf.spreds[rpath]
			nf.walked = rpath // == p.name
			nf.depth = 0
			if nf.walked != nf.name && zx.HasPrefix(nf.walked, nf.name) {
				els := zx.Elems(zx.Suffix(nf.walked, nf.name))
				nf.depth = len(els)
			}
			for _, nd := range np.dupDirs() {
				f.ns.vprintf("fnd:\t\trecur at %s\n", nd)
				if err := nf.find1(nd); err != nil {
					nd["err"] = err.Error()
					if ok := f.c <- nd; !ok {
						close(rc, cerror(f.c))
						return cerror(rc)
					}
				}
			}
			delete(f.suffs, rpath) // don't repeat the find
			continue
		}
		if f.spref != f.dpref {
			cpath := rd["path"]
			suff := zx.Suffix(cpath, f.spref)
			rd["path"] = fpath.Join(f.dpref, suff)
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
			ns.vprintf("fnd:\t\tprefix %s of name %s\n", fp.name, f.name)
			f.pred = ns.exclSuffixes(fp.name, f.upred)
			f.p = fp
		}
	}
	// no prefix, no such name (even if there are suffixes!).
	if f.p == nil {
		ns.lk.RUnlock()
		ns.vprintf("fnd:\t%d %s: failed\n", f.depth, f.name)
		return zx.ErrNotExist
	}
	ns.vprintf("fnd:\tdepth %d pref %s for %s\n", f.depth, f.p.name, f.name)

	// Collect all suffixes of name to issue further finds later on,
	// and record their adjusted predicates to exclude their suffixes on their finds.
	f.suffs = map[string]*prefix{}
	f.spreds = map[string]*pred.Pred{}
	for _, xp := range ns.pref {
		if f.p != xp && zx.HasPrefix(xp.name, f.name) {
			f.suffs[xp.name] = xp
			f.spreds[xp.name] = ns.exclSuffixes(xp.name, f.upred)
		}
	}
	ns.lk.RUnlock()

	// Start one find at a time starting with p, considering that we walked
	// already p's name.
	f.walked = f.p.name
	ds := f.p.dupDirs()
	for _, d := range ds {
		ns.vprintf("fnd:\t\tmnt %s: %s\n", f.p.name, d.LongFmt())
		var err error
		if f.gc != nil {
			err = f.find1get(d)
		} else {
			err = f.find1(d)
		}
		// It's hard to do it right here.
		// If there's just one it's better to declare the find as failed
		// but if there are more, we should report the errors in-place
		// and thus have to send dir entries with the err attr set.
		if len(ds) == 1 {
			return err
		}
		if err != nil {
			d["err"] = err.Error()
			if f.gc != nil {
				if ok := f.gc <- d; !ok {
					return cerror(f.gc)
				}
			} else {
				if ok := f.c <- d; !ok {
					return cerror(f.c)
				}
			}
		}
	}
	return nil
}

// Implementation of the Finder.Find operation.
// Issues finds to all involved mount points, starting at the longest prefix that is a prefix of
// name (perhaps name itself).
// As it gets entries, it will issue further finds for those mount points that are suffixes of
// any directory found.
// As a result, if a find is issued at /path, and a suffix of path is in a mount point that
// no longer has a directory entry at the FS mounted at path, no finds are issued for the
// second.
// That is, suffixes might be "disconnected" from the prefix if their names can't be reached from there.
// But you can still issue finds for those prefixes (and any suffix path).
func (ns *NS) Find(name string, fpred string, spref, dpref string, depth0 int) <-chan zx.Dir {
	c := make(chan zx.Dir)
	go ns.findget(name, fpred, spref, dpref, depth0, c, nil)
	return c
}

// Implementation of the Finder.FindGet operation.
// See also Find for a description of the find requests issued.
func (ns *NS) FindGet(name string, fpred string, spref, dpref string, depth0 int) <-chan face{} {
	gc := make(chan face{})
	go ns.findget(name, fpred, spref, dpref, depth0, nil, gc)
	return gc
}

func (ns *NS) findget(name string, fpred string, spref, dpref string, depth0 int, c chan zx.Dir, gc chan face{}) {
	var err error
	var x *pred.Pred
	if spref != "" || dpref != "" {
		spref, err = zx.UseAbsPath(spref)
		if err == nil {
			dpref, err = zx.UseAbsPath(dpref)
		}
	}
	if err == nil {
		name, err = zx.UseAbsPath(name)
	}
	if err == nil {
		x, err = pred.New(fpred)
	}
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
	ns.vprintf("fnd:\t%s %s d%d\n", fndr.name, fndr.upred, fndr.depth)
	err = fndr.find()
	close(c, err)
	close(gc, err)
}
