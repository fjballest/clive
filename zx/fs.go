/*
	Clive ZX file system tools.

	The service in ZX is split in two parts:

	- A finder interface, to find directory entries

	- A tree interface, to operate on directory entries found.

	This package provides support for directory entries, and packages in subdirectories
	provide different ZX file systems for Clive.

	The main interfaces are Tree, RWTree, and File, although most operations
	are performed on the Dir type. All operations in interfaces are designed to
	enable streaming, and thus return results through channels, including error indicaitons.
	In some common cases there are helper functions that can be used to issue
	procedure-call-like calls, blocking until the response arrived.
*/
package zx

import (
	"clive/net/auth"
	"io"
)

// REFERENCE(x): cmd/zx, command to export file trees

// REFERENCE(x): ns, finders and name spaces

const (
	// size arg for Tree.Get: get everything from the file.
	All = -1

	// off arg for Tree.Put: append to file
	App = -1
)

/*
	These are conventional attribute names. These contants are seldom used,
	in favor of the strings naming attributes, but are kept here as a documentation
	for conventions in ZX.

	By convention, attributes "type", "mode", "size", "name", and "mtime" are always
	expected to persist in the file system.

	Attributes starting with [A-Z] may persist if their file systems are configured to
	do so.

	Other lower-case attributes are regarded as temporary
	meta data for the directory entry
	and are used to reach the resource it describes.

	Attributes mode, Uid, and Gid are used to perform permission checking,
	usually by zx/cfs or a similar zx implementation. See the clive/net/auth
	package for a description of expected semantics.
*/
const (
	Size  = "size"  // number of entries in directories, size for files
	Name  = "name"  // file name
	Fpath = "path"  // file path as seen by client
	Addr  = "addr"  // where to reach the server for the directory entry
	Spath = "spath" // path for the resource in the server
	Mtime = "mtime" // mod time since epoch (ns)
	Mode  = "mode"  // permissions (octal, 0777 bits must be unix bits).
	Type  = "type"  // "-": file, "d": dir, ...
	Rm    = "rm"    // whiteout (if set, the file is removed)
	Proto = "proto" // protocols spoken by server
	Err   = "err"   // error for the operation performed at this entry.

	Uid  = "Uid"  // user id or file owner
	Gid  = "Gid"  // group id or file group
	Wuid = "Wuid" // who is guilty if you don't like the data
	Sum  = "Sum"  // hash for the file data
)

/*
	A Dir, or directory entry, identifices a file or a resource in the system.
	It is a set of attribute/value pairs, including some conventional attributes
	like "name", "size", etc.

	Directory entries are self-describing in many cases, and include the address
	and resource path as known by the server as extra attributes. Thus, programs can
	operate on streams of Dir entries and ask each entry to perform an operation on the
	resource it described.

	The purpose of very important interfaces in the system, like ns.Finder and
	zx.Tree is to operate on Dirs.
*/
type Dir map[string]string

/*
	Results from FindGets: A Dir and a chan for regular file data.
*/
type DirData struct {
	Dir   Dir           // Dir found
	Datac <-chan []byte // Data or nil
}

/*

	A finder is an entity that knows how to find files (directory entries, actually).
	Its purpose is to provide navigation on a tree of resources (files or names).

	Refer to zx/pred to learn about predicates given to Find, and to zx.Dir to
	learn about directory entries.

	The interface is intented to permit streaming of requests, hence most results
	are sent through channels, including error indications.

	Name spaces should implement this interface.
*/
type Finder interface {
	// Select the file tree to be navigated
	Fsys(name string) <-chan error

	// Navigate the tree starting at path to find files matching the predicate
	// pred. Found entries are sent through the returned channel.
	// Those with errors are decorated with an "err" attribute indicating the error,
	// and they might not match the predicate given to find, if they convey just the error.
	//
	// The server must consider that the path for a file /a/b/c is actually /x/b/c if
	// spref is /a and dpref is /y. That is, spref must be replaced with dpref in paths
	// before evaluating the predicate. This is used to evaluate paths as seen by the users
	// without having to rewrite the predicates at each mount point.
	//
	// The depth for the path given, once walked, starts at depth0 (and not 0), again,
	// to avoid predicate rewriting on mount points.
	//
	Find(path, pred string, spref, dpref string, depth0 int) <-chan Dir

	// Do a Find(path, pred, spref, dpref, depth0 )
	// and get the Dirs and data for all matching files.
	// In the wire, the series of messages is one packed Dir per found entry followed
	// (for regular files) by the file []byte messages carrying the data (terminated with an
	// empty []byte.
	// The caller must receive everything from the Datac in the structure received
	// before receiving anything else from the DirData channel.
	// If the receiver closes DirData.Datac for a received DirData, it is understood
	// as an indication of discarding further data for that file, but other DirDatas
	// are still being sent to the resulting channel.
	FindGet(path, pred string, spref, dpref string, depth0 int) <-chan DirData
}

/*
	This is the interface expected for file trees in the system.
	Note that RWTree further extends this one with write operations.

*/
type Tree interface {
	// return the tree name
	Name() string

	// Return the directory entry for the file at path.
	Stat(path string) chan Dir

	// Terminate the file tree operation, perhaps with an error.
	Close(e error)

	// Retrieve the contents of the file at path.
	// For directories, off and count refer to the number of
	// directory entries, counting from 0.
	// A count of -1 means "everything".
	// Each directory entry is returned as a string with the format
	// produced by Dir.String, The end of the file (or dir) is signaled with
	// an empty message.
	// When pred is not "", it is evaluated at path and the get is performed
	// only if the predicate is true. Depth is always considered as 0.
	Get(path string, off, count int64, pred string) <-chan []byte

	// Find directory entries in this tree. See zx.Finder.
	Find(rid, pred string, spref, dpref string, depth0 int) <-chan Dir

	// Do a Find(path, pred, spref, dpref, depth0 )
	// and get the Dirs and data for all matching files. See zx.Finder.
	// The caller must receive everything from the Datac in the structure received
	// before receiving anything else from the DirData channel.
	// If the receiver closes DirData.Datac for a received DirData, it is understood
	// as an indication of discarding further data for that file, but other DirDatas
	// are still being sent to the resulting channel.
	FindGet(path, pred string, spref, dpref string, depth0 int) <-chan DirData
}

/*
	This is the interface expected for RW file trees in the system.
	Note that RWTree extends the Tree interface.
*/
type RWTree interface {
	Tree

	// Update or create a file at the given path with the attributes
	// found in d and the data sent through dc.
	// If d is nil or d["mode"] is not defined, the file is not
	// created if it does not exist.
	// If d["size"] is defined, the file is truncated to that size before
	// writing the data (even when creating the file).
	// If extra attributes are included in d, they are also updated.
	// If off is < 0 then the new data is appended to the file.
	// Note off<0 makes no sense if d["mode"] is defined.
	// The file mtime and size after the put, or the error is reported
	// through the returned channel.
	// If pred is not "", it is evaluted at the file if it already exists
	// and the put happens only if pred is true. Depth is considered 0.
	Put(path string, d Dir, off int64, dc <-chan []byte, pred string) chan Dir

	// Create a directory at the given path with the attributes found in d.
	Mkdir(path string, d Dir) chan error

	// Move a file or directory from one place to another.
	Move(from, to string) chan error

	// Delete the file or empty directory found at path.
	Remove(path string) chan error

	// Delete the file or directory found at path.
	RemoveAll(path string) chan error

	// Update attributes for the file at path with those from d.
	Wstat(path string, d Dir) chan error
}

/*
	A File records together a Tree and a Dir used within the tree.
*/
type File struct {
	T Tree
	D Dir
}

// Tree with statistics
type StatTree interface {
	Stats() *IOstats
}

// Tree that operates on behalf of a user.
type AuthTree interface {
	// Return a view of the tree that operates on
	// behalf of the user described by ai.
	AuthFor(ai *auth.Info) (Tree, error)
}

// Information provided to ServerTrees per-client
type ClientInfo struct {
	Ai  *auth.Info // auth info for the client
	Tag string     // from the conn to the client
	Id  int        // unique per-client id
}

// Tree that needs to keep per-client data, each call returns
// the version of the tree to be used for a client.
// If the tree provides this interface, AuthFor() is not called.
type ServerTree interface {
	// return the tree to be used for the given client info.
	ServerFor(info *ClientInfo) (Tree, error)
}

type Dumper interface {
	Dump(w io.Writer)
}

type IsCtler interface {
	IsCtl() bool
}
