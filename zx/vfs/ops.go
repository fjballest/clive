package vfs

import (
	"clive/zx"
)

// All files must implement at least this.
type File interface {
	Stat() (zx.Dir, error)
}

// For directories, to permit walking to children.
type Walker interface {
	Walk(elem string) (File, error)
	Getdir() ([]string, error)
}

// At root, to notify of closes of the tree.
type Closer interface {
	Close(err error)
}

// To make Gets return something other than zero bytes.
type Getter interface {
	Get(off, count int64, c chan<- []byte) error
}

// To permit file creation and file writes.
// For creations, the parent Putter must also be a Walker
type Putter interface {
	// Like zx.Put, but called on the parent with the name of the child
	// to create the child, or called in the child with "" name  otherwise.
	// Must always return the child where the put happen and/or error
	Put(name string, d zx.Dir, off int64, c <-chan []byte) error
}

// At root, to accept Ctl requests
type Ctler interface {
	// return true if it's a known ctl and error
	PutCtl(s string) (bool, error)
}

// To permit creation of directories
type Mkdirer interface {
	Walker
	Mkdir(name string, d zx.Dir) error
}

// To permit wstats
// Note that this also means truncates.
// If not implemented, all wstats succeed and are ignored if Put is also
// implemented (otherwise all of them fail).
type Wstater interface {
	Wstat(d zx.Dir) error
}

// Implement to permit moves.
// The parent dir of the target must be a Walker.
type Mover interface {
	Walker
	Move(child File, elem string, todir File, nelem string) error
}

// To permit removes
type Remover interface {
	Walker
	Remove(child File, elem string, all bool) error
}

func GetBytes(dat []byte, off, count int64, c chan<- []byte) error {
	if off >= int64(len(dat)) {
		return nil
	}
	dat = dat[int(off):]
	if nd := int64(len(dat)); count > nd {
		count = nd
	}
	if count < 0 {
		count = int64(len(dat))
	}
	c <- dat[:int(count)]
	return nil
}
