package zx

// part of zx.Tree; see the def. there.
type Namer interface {
	Name() string
}

// part of zx.Tree; see the def. there.
type Stater interface {
	Stat(path string) chan Dir
}

// part of zx.Tree; see the def. there.
type Closer interface {
	Close(e error)
}

// part of zx.Tree; see the def. there.
type Getter interface {
	Get(path string, off, count int64, pred string) <-chan []byte
}

// part of zx.Tree; see the def. there.
type TreeFinder interface {
	Find(rid, fpred, spref, dpref string, depth int) <-chan Dir
}

// part of zx.Tree; see the def. there.
type TreeFindGeter interface {
	FindGet(rid, fpred, spref, dpref string, depth int) <-chan DirData
}

// part of zx.RWTree; see the def. there.
type Putter interface {
	Put(path string, d Dir, off int64, dc <-chan []byte, pred string) chan Dir
}

// part of zx.RWTree; see the def. there.
type Mkdirer interface {
	Mkdir(path string, d Dir) chan error
}

// part of zx.RWTree; see the def. there.
type Mover interface {
	Move(from, to string) chan error
}

// part of zx.RWTree; see the def. there.
type Remover interface {
	Remove(path string) chan error
	RemoveAll(path string) chan error
}

// part of zx.RWTree; see the def. there.
type Wstater interface {
	Wstat(path string, d Dir) chan error
}

type Debugger interface {
	// Return a pointer to the debug flag
	Debug() *bool
}
