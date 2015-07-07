package zx

import (
	"clive/dbg"
	"clive/net/auth"
	"errors"
	"io"
	"fmt"
)

type treeFor  {
	i  interface{}
	rw bool
}

// A Tree that provides nop operations for all not
// implemented by an underlying interface.
// If the argument implements zx.Tree, it's simply returned.
func TreeFor(i interface{}) Tree {
	if t, ok := i.(Tree); ok {
		return t
	}
	return treeFor{i, true}
}

// A RWTree that provides nop operations for all not
// implemented by an underlying interface.
// If the argument implements zx.RWTree, it's simply returned.
func RWTreeFor(i interface{}) RWTree {
	if t, ok := i.(RWTree); ok {
		return t
	}
	return treeFor{i, true}
}

// A RO Tree wrapping a possibly RW tree.
// The returned tree is a RW tree that returns errors for all updates.
func ROTreeFor(i interface{}) RWTree {
	return treeFor{i, false}
}

func (t treeFor) Name() string {
	if x, ok := t.i.(Namer); ok {
		return x.Name()
	}
	return "unamed"
}

func (t treeFor) Stat(path string) chan Dir {
	if x, ok := t.i.(Stater); ok {
		return x.Stat(path)
	}
	c := make(chan Dir)
	close(c, dbg.ErrPerm)
	return c
}

func (t treeFor) Close(e error) {
	if x, ok := t.i.(Closer); ok {
		x.Close(e)
	}
}

func (t treeFor) Get(path string, off, count int64, pred string) <-chan []byte {
	if x, ok := t.i.(Getter); ok {
		return x.Get(path, off, count, pred)
	}
	c := make(chan []byte)
	close(c, dbg.ErrPerm)
	return c
}

func (t treeFor) Find(rid, fpred, spref, dpref string, depth int) <-chan Dir {
	if x, ok := t.i.(TreeFinder); ok {
		return x.Find(rid, fpred, spref, dpref, depth)
	}
	c := make(chan Dir)
	close(c, dbg.ErrPerm)
	return c
}

func (t treeFor) FindGet(rid, fpred, spref, dpref string, depth int) <-chan DirData {
	if x, ok := t.i.(TreeFindGeter); ok {
		return x.FindGet(rid, fpred, spref, dpref, depth)
	}
	c := make(chan DirData)
	close(c, dbg.ErrPerm)
	return c
}

func (t treeFor) err() error {
	if t.rw {
		return dbg.ErrBug
	}
	return dbg.ErrRO
}

func (t treeFor) cerr() chan error {
	e := t.err()
	c := make(chan error, 1)
	c <- e
	close(c, e)
	return c
}

func (t treeFor) Put(path string, d Dir, off int64, dc <-chan []byte, pred string) chan Dir {
	if x, ok := t.i.(Putter); ok && t.rw {
		return x.Put(path, d, off, dc, pred)
	}
	e := t.err()
	close(dc, e)
	c := make(chan Dir)
	close(c, e)
	return c
}

func (t treeFor) Mkdir(path string, d Dir) chan error {
	if x, ok := t.i.(Mkdirer); ok && t.rw {
		return x.Mkdir(path, d)
	}
	return t.cerr()
}

func (t treeFor) Move(from, to string) chan error {
	if x, ok := t.i.(Mover); ok && t.rw {
		return x.Move(from, to)
	}
	return t.cerr()
}

func (t treeFor) Remove(path string) chan error {
	if x, ok := t.i.(Remover); ok && t.rw {
		return x.Remove(path)
	}
	return t.cerr()
}

func (t treeFor) RemoveAll(path string) chan error {
	if x, ok := t.i.(Remover); ok && t.rw {
		return x.RemoveAll(path)
	}
	return t.cerr()
}

func (t treeFor) Wstat(path string, d Dir) chan error {
	if x, ok := t.i.(Wstater); ok && t.rw {
		return x.Wstat(path, d)
	}
	return t.cerr()
}

func (t treeFor) Stats() *IOstats {
	if x, ok := t.i.(StatTree); ok {
		return x.Stats()
	}
	return nil
}

func (t treeFor) AuthFor(ai *auth.Info) (Tree, error) {
	if x, ok := t.i.(AuthTree); ok {
		return x.AuthFor(ai)
	}
	return nil, errors.New("no auth")
}

func (t treeFor) Dump(w io.Writer) {
	if x, ok := t.i.(Dumper); ok {
		x.Dump(w)
		return
	}
	fmt.Fprintf(w, "No dump for %s\n", t)
}
