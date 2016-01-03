package cmd

import (
	"clive/zx"
	fpath "path"
	"strings"
	"errors"
)

// Issue a find for these names ("filename,predicate")
// If no predicate is given, then "depth<1" is used.
// eg. /a -> just /a; /a, -> subtree at /a
// Errors are reported by sending an error.
// The chan error status is not nil if there's an error.
// The Upath attribute in the dir entries returned mimics the paths given
// in the names.
// The Rpath attribute in the dir entries provide a path relative to the one
// specified by the user.
func Dirs(names ...string) chan interface{} {
	ns := NS()
	rc := make(chan interface{})
	go func() {
		var err error
		for _, name := range names {
			if len(name) > 0 && name[0] == '#' {
				d := zx.Dir{"path": name, "name": name,
					"Upath": name, "type": "c"}
				rc <- d
				continue
			}
			toks := strings.SplitN(name, ",", 2)
			if toks[0] == "" {
				toks[0] = "."
			}
			if len(toks) == 1 {
				toks = append(toks, "0")
			}
			toks[0] = fpath.Clean(toks[0])
			name = AbsPath(toks[0])
			Dprintf("getdirs: find %s %s\n", name, toks[1])
			dc := ns.Find(name, toks[1], "/", "/", 0)
			for d := range dc {
				if d == nil {
					break
				}
				d["Upath"] = d["path"]
				if toks[0] != name && zx.HasPrefix(d["path"], name) {
					u := fpath.Join(toks[0], zx.Suffix(d["path"], name))
					d["Upath"] = u
				}
				d["Rpath"] = zx.Suffix(d["path"], name)
				if d["err"] != "" {
					if d["err"] != "pruned" {
						err = errors.New(d["err"])
						rc <- err
					}
					continue
				}
				if ok := rc <- d; !ok {
					close(dc, cerror(rc))
					return
				}
			}
			if derr := cerror(dc); derr != nil {
				err = derr
				if ok := rc <- err; !ok {
					return
				}
			} else {
				close(dc) // in case a null was sent but no error
			}
		}
		close(rc, err)
	}()
	return rc
}

// Like Dirs(), but sends also file contents
func Files(names ...string) chan interface{} {
	ns := NS()
	rc := make(chan interface{})
	go func() {
		var err error
		for _, name := range names {
			if len(name) > 0 && name[0] == '#' {
				d := zx.Dir{"path": name, "name": name,
					"Upath": name, "type": "c"}
				rc <- d
				continue
			}
			toks := strings.SplitN(name, ",", 2)
			if toks[0] == "" {
				toks[0] = "."
			}
			if len(toks) == 1 {
				toks = append(toks, "0")
			}
			toks[0] = fpath.Clean(toks[0])
			name = AbsPath(toks[0])
			Dprintf("getfiles: findget %s %s\n", name, toks[1])
			dc := ns.FindGet(name, toks[1], "/", "/", 0)
			for m := range dc {
				d, ok := m.(zx.Dir)
				if !ok {
					if ok := rc <- m; !ok {
						close(dc, cerror(rc))
						return
					}
					continue
				}
				d["Upath"] = d["path"]
				if toks[0] != name && zx.HasPrefix(d["path"], name) {
					u := fpath.Join(toks[0], zx.Suffix(d["path"], name))
					d["Upath"] = u
				}
				d["Rpath"] = zx.Suffix(d["path"], name)
				if d["err"] != "" {
					if d["err"] != "pruned" {
						err = errors.New(d["err"])
						rc <- err
					}
					continue
				}
				if ok := rc <- d; !ok {
					close(dc, cerror(rc))
					return
				}
			}
			if derr := cerror(dc); derr != nil {
				err = derr
				if ok := rc <- err; !ok {
					return
				}
			} else {
				close(dc) // in case a null was sent but no error
			}
		}
		close(rc, err)
	}()
	return rc
}
