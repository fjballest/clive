package fuse

import (
	"clive/x/bazil.org/fuse"
	"os"
)

// An FS is the interface required of a file system.
//
// Other FUSE requests can be handled by implementing methods from the
// FS* interfaces, for example FSIniter.
// On normal FUSE filesystems, use Forget of the root Node to
// do actions at unmount time.
interface FS {
	// Root is called to obtain the Node for the file system root.
	Root() (Node, fuse.Error)
}

// A Node is the interface required of a file or directory.
// See the documentation for type FS for general information
// pertaining to all methods.
//
// Other FUSE requests can be handled by implementing methods from the
// Node* interfaces, for example NodeOpener.
interface Node {
	Attr() (*fuse.Attr, fuse.Error)
	Open(fuse.OpenFlags, Intr) (Handle, fuse.Error)
}

// NodeRef can be embedded in a Node to recognize the same Node being
// returned from multiple Lookup, Create etc calls.
//
// Without this, each Node will get a new NodeID, causing spurious
// cache invalidations, extra lookups and aliasing anomalies. This may
// not matter for a simple, read-only filesystem.
struct NodeRef {
	id         fuse.NodeID
	generation uint64
}

interface NodeSetAttrer {
	// Setattr sets the standard metadata for the receiver.
	// EPERM otherwise
	SetAttr(*fuse.SetattrRequest, Intr) fuse.Error
}

interface NodeXAttrer {
	// get the named attribute
	Xattr(name string) ([]byte, fuse.Error)
	// set the named attribute (use nil val to remove)
	Wxattr(name string, val []byte) fuse.Error
	// list named attributes
	Xattrs() []string
}

interface NodeRemover {
	// Remove removes the entry with the given name from
	// the receiver, which must be a directory.  The entry to be removed
	// may correspond to a file (unlink) or to a directory (rmdir).
	Remove(elem string, i Intr) fuse.Error
}

interface NodeLookuper {
	// Lookup looks up a specific entry in the receiver,
	// which must be a directory.  Lookup should return a Node
	// corresponding to the entry.  If the name does not exist in
	// the directory, Lookup should return nil, err.
	//
	// Lookup need not to handle the names "." and "..".
	Lookup(string, Intr) (Node, fuse.Error)
}

interface NodeMkdirer {
	// Create dir name with the given mode
	Mkdir(name string, mode os.FileMode, i Intr) (Node, fuse.Error)
}

interface NodeCreater {
	// Create creates a new directory entry in the receiver, which
	// must be a directory.
	Create(name string, flag fuse.OpenFlags, mode os.FileMode, i Intr) (Node, Handle, fuse.Error)
}

interface NodePuter {
	// Kernel says we can forget node and put it back to where it was.
	PutNode()
}

interface NodeRenamer {
	// Move to the directory node newDir (which is the type implementing Node)
	// so
	Rename(oldelem, newelem string, newDir Node, intr Intr) fuse.Error
}

// TODO this should be on Handle not Node
interface NodeFsyncer {
	Fsync(intr Intr) fuse.Error
}

// A Handle is the interface required of an opened file or directory.
// See the documentation for type FS for general information
// pertaining to all methods.
//
// Other FUSE requests can be handled by implementing methods from the
// Node* interfaces. The most common to implement are
// HandleReader, HandleReadDirer, and HandleWriter.
//
// TODO implement methods: Getlk, Setlk, Setlkw
//
// NB: We do not use DirectIO as the open mode, because that would require
// aligned buffers or execve will fail.
// However, this might affect append. In a previous version, in some cases,
// append would work only with directIO.
interface Handle {
	ReadDir(Intr) ([]fuse.Dirent, fuse.Error)
	Read(off int64, sz int, i Intr) ([]byte, fuse.Error)
	// Close is called each time the file or directory is closed.
	// Because there can be multiple file descriptors referring to a
	// single opened file, Close can be called multiple times.
	Close(Intr) fuse.Error
}

interface HandleWriter {
	Write([]byte, int64, Intr) (int, fuse.Error)
}

interface HandlePuter {
	// Put back the handle to where it was and forget about it.
	PutHandle()
}

// If a handle implements this, and the method returns true,
// FUSE relies on DirectIO for the file.
// That usually prevents exec() from working on the file but on the other hand
// does not let UNIX assume which one is the file size until the file has been read.
// File trees with control files should implement this and return true for ctl files.
interface HandleIsCtler {
	IsCtl() bool
}
