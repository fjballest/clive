package zx

import (
	"bytes"
	"clive/net/auth"
)

// A zx file system.
// It must provide at least Stats.
interface Fs {
	// Return the directory entry for the file at path.
	Stat(p string) <-chan Dir
}

const All = -1

// File systems able to find directory entries
interface Finder {
	// Navigate the tree starting at path to find files matching the predicate
	// pred. Found entries are sent through the returned channel.
	// Those with errors are decorated with an "err" attribute indicating the error,
	// and they might not match the predicate given to find, if they convey just the error.
	//
	// The server must consider that the path for a file /a/b/c is actually /x/b/c if
	// spref is /a and dpref is /y. That is, spref is replaced with dpref in paths
	// before evaluating the predicate. This is used to evaluate paths as seen by the users
	// without having to rewrite the predicates at each mount point.
	//
	// The depth for the path given, once walked, starts at depth0 (and not 0), again,
	// to evaluate the pred in entries as seen by the user
	//
	Find(path, pred string, spref, dpref string, depth0 int) <-chan Dir
}

// File systems able to get file contents
interface Getter {
	Fs
	// Retrieve the contents of the file at path.
	// For directories, off and count refer to the number of
	// directory entries, counting from 0.
	// A count of -1 means "everything".
	// Each directory entry is returned as a []byte with the format
	// produced by Dir.Bytes(),
	// The end of the file (or dir) is signaled with an empty message.
	Get(path string, off, count int64) <-chan []byte
}

// File systems able to both find and get directory entries in a single RPC
interface FindGetter {
	// This is like Find(), but streams through the returned channel the file
	// data after each matching Dir.
	// errors during Get()s are also streamed.
	FindGet(path, pred string, spref, dpref string, depth0 int) <-chan face{}
}

// File systems able to put files
interface Putter {
	// Update or create a file at the given path with the attributes
	// found in d and the data sent through dc.
	// If d is nil or d["type"] is not defined, the file is not
	// created if it does not exist; otherwise it is used as it is.
	// "-" is the type for files, and "d" for directories.
	// If d["size"] is defined, the file is truncated to that size before
	// writing the data.
	// If extra attributes are included in d, they are also updated.
	// If off is < 0 then the new data is appended to the file.
	// Note off<0 makes sense even if d["mode"] is defined.
	// The file mtime and size after the put, or the error is reported
	// through the returned channel.
	// If d["type"] is "d", then dc is ignored and a directory is created
	// (unless it already exists, in which case it's ok)
	// If d["type"] is "F", it means "-" but parent directories are
	// created if they do not exist.
	// If d["type"] is "D", it means "d" but parent directories are created
	// if they do not exist.
	Put(path string, d Dir, off int64, dc <-chan []byte) <-chan Dir
}

// File systems able to wstat files
interface Wstater {
	// Update attributes for the file at path with those from d
	// and return the resulting directory entry
	Wstat(path string, d Dir) <-chan Dir
}

// File systems able to remove files
// Removing "/" always fails
interface Remover {
	// Delete the file or empty directory found at path.
	Remove(path string) <-chan error
	// Delete the file or directory found at path.
	RemoveAll(path string) <-chan error
}

// File systems able to move files
interface Mover {
	// Move file src to be at dst
	// If from is to, the op is a nop.
	// Otherwise, it is an error to mv to or from / and /Ctl.
	Move(from, to string) <-chan error
}

// File systems able to link files
interface Linker {
	// Link new to refer to old
	Link(oldp, newp string) <-chan error
}

// File systems that can authenticate a user
interface Auther {
	// returns a new view of the Fs authenticated for ai
	Auth(ai *auth.Info) (Fs, error)
}

// File systems that need/have sync()
interface Syncer {
	Sync() error
}

// Typical file systems with usual read/write ops,
interface RWFs {
	Getter
	Putter
	Wstater
	Remover
}

// Full file systems including find and link
interface FullFs {
	Getter
	Putter
	Wstater
	Remover
	Mover
	Linker
	Finder
	FindGetter
}

// Do a Stat on fs and return the reply now
func Stat(fs Fs, p string) (Dir, error) {
	dc := fs.Stat(p)
	d := <-dc
	return d, cerror(dc)
}

// Get all contents for a file
func GetAll(fs Getter, p string) ([]byte, error) {
	var buf bytes.Buffer
	gc := fs.Get(p, 0, -1)
	for d := range gc {
		buf.Write(d)
	}
	return buf.Bytes(), cerror(gc)
}

// Put all contents for a file, creating it.
func PutAll(fs Putter, path string, data []byte, mode ...string) error {
	m := "0644"
	if len(mode) > 0 {
		m = mode[0]
	}
	c := make(chan []byte, 1)
	c <- data
	close(c)
	d := Dir{"type": "-", "mode": m}
	gc := fs.Put(path, d, 0, c)
	<-gc
	return cerror(gc)
}

// Get all dir entries
func GetDir(fs Getter, p string) ([]Dir, error) {
	ds := make([]Dir, 0, 16)
	var c <-chan []byte
	c = fs.Get(p, 0, All)
	for b := range c {
		_, d, err := UnpackDir(b)
		if err != nil {
			return nil, err
		}
		ds = append(ds, d)
	}
	if err := cerror(c); err != nil {
		return nil, err
	}
	return ds, nil
}
