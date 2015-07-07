/*
	FUSE service loop, from bazil.org.

	This was once bazil.org/fuse/fs/serve.go, see bazil.org/fuse/LICENSE.
	This version has been heavily changed to clean it up and to make it fit our needs.
*/
package fuse

import (
	"clive/dbg"
	"clive/x/bazil.org/fuse"
	"clive/x/bazil.org/fuse/fuseutil"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
)

// An Intr is a channel that signals that a request has been interrupted.
// Being able to receive from the channel means the request has been
// interrupted.
type Intr chan bool

type Server  {
	FS FS

	// Function to send debug log messages to. If nil, use fuse.Debug.
	// Note that changing this or fuse.Debug may not affect existing
	// calls to Serve.
	dprintf dbg.PrintFunc
}

type serveConn  {
	meta       sync.Mutex
	fs         FS
	req        map[fuse.RequestID]*serveRequest
	node       []*serveNode
	handle     []*serveHandle
	freeNode   []fuse.NodeID
	freeHandle []fuse.HandleID
	nodeGen    uint64
}

type serveRequest  {
	Request fuse.Request
	Intr    Intr
}

type serveNode  {
	inode uint64
	node  Node
	refs  uint64
}

type serveHandle  {
	handle   Handle
	readData []byte // used to cached packed dir entries
	nodeID   fuse.NodeID
}

type nodeRef interface {
	nodeRef() *NodeRef
}

var (
	attrValidTime  = 5*time.Second
	entryValidTime = 5*time.Second
	startTime      = time.Now()
	Debug          bool
	dprintf        = dbg.FlagPrintf(os.Stderr, &Debug)

	uid, gid uint32
)

// nodeRef is only ever accessed while holding serveConn.meta
func (n *NodeRef) nodeRef() *NodeRef {
	return n
}

/*
	Set the fuse protocol cache timeouts for attributes and entries
*/
func SetTimeOuts(attrvalid, entryvalid time.Duration) {
	attrValidTime = attrvalid
	entryValidTime = entryvalid
}

func (Intr) String() string { return "fuse.Intr" }

func nodeAttr(n Node) (attr fuse.Attr, err error) {
	x, xerr := n.Attr()
	if xerr != nil {
		err = xerr
		return
	}
	attr = *x
	if attr.Nlink == 0 {
		attr.Nlink = 1
	}
	if attr.Atime.IsZero() {
		attr.Atime = startTime
	}
	if attr.Mtime.IsZero() {
		attr.Mtime = startTime
	}
	if attr.Ctime.IsZero() {
		attr.Ctime = startTime
	}
	if attr.Crtime.IsZero() {
		attr.Crtime = startTime
	}
	if attr.Uid == 0 {
		attr.Uid = uid
	}
	if attr.Gid == 0 {
		attr.Gid = gid
	}
	return
}

// Serve serves the FUSE connection by making calls to the methods
// of fs and the Nodes and Handles it makes available.  It returns only
// when the connection has been closed or an unexpected error occurs.
func (s *Server) Serve(c *fuse.Conn) error {
	sc := serveConn{
		fs:  s.FS,
		req: map[fuse.RequestID]*serveRequest{},
	}

	root, err := sc.fs.Root()
	if err != nil {
		return fmt.Errorf("cannot obtain root node: %v", syscall.Errno(err.(fuse.Errno)).Error())
	}
	// make sure we get inodes
	attr, err := root.Attr()
	if err!=nil || attr.Inode==0 {
		return errors.New("root does not have a valid inode number")
	}
	sc.node = append(sc.node, nil, &serveNode{inode: 1, node: root, refs: 1})
	sc.handle = append(sc.handle, nil)

	for {
		req, err := c.ReadRequest()
		if err != nil {
			if err == io.EOF {
				dprintf("server: EOF\n")
				break
			}
			dprintf("server: %s\n", err)
			return err
		}

		go sc.serve(req)
	}
	return nil
}

// Serve serves a FUSE connection with the default settings. See
// Server.Serve.
func Serve(c *fuse.Conn, fs FS) error {
	server := Server{
		FS:      fs,
		dprintf: dbg.FlagPrintf(os.Stderr, &Debug),
	}
	return server.Serve(c)
}

func (sn *serveNode) attr() (attr fuse.Attr, err fuse.Error) {
	attr, err = nodeAttr(sn.node)
	if err != nil {
		return
	}
	if attr.Inode == 0 {
		attr.Inode = sn.inode
	}
	if attr.Uid == 0 {
		attr.Uid = uid
	}
	if attr.Gid == 0 {
		attr.Gid = gid
	}
	return
}

func (c *serveConn) saveNode(inode uint64, node Node) (id fuse.NodeID, gen uint64) {
	c.meta.Lock()
	defer c.meta.Unlock()

	var ref *NodeRef
	if nodeRef, ok := node.(nodeRef); ok {
		ref = nodeRef.nodeRef()

		if ref.id != 0 {
			// dropNode guarantees that NodeRef is zeroed at the same
			// time as the NodeID is removed from serveConn.node, as
			// guarded by c.meta; this means sn cannot be nil here
			sn := c.node[ref.id]
			sn.refs++
			return ref.id, ref.generation
		}
	}

	sn := &serveNode{inode: inode, node: node, refs: 1}
	if n := len(c.freeNode); n > 0 {
		id = c.freeNode[n-1]
		c.freeNode = c.freeNode[:n-1]
		c.node[id] = sn
		c.nodeGen++
	} else {
		id = fuse.NodeID(len(c.node))
		c.node = append(c.node, sn)
	}
	gen = c.nodeGen
	if ref != nil {
		ref.id = id
		ref.generation = gen
	}
	return
}

func (c *serveConn) saveHandle(handle Handle, nodeID fuse.NodeID) (id fuse.HandleID) {
	c.meta.Lock()
	shandle := &serveHandle{handle: handle, nodeID: nodeID}
	if n := len(c.freeHandle); n > 0 {
		id = c.freeHandle[n-1]
		c.freeHandle = c.freeHandle[:n-1]
		c.handle[id] = shandle
	} else {
		id = fuse.HandleID(len(c.handle))
		c.handle = append(c.handle, shandle)
	}
	c.meta.Unlock()
	return
}

type nodeRefcountDropBug  {
	N    uint64
	Refs uint64
	Node fuse.NodeID
}

func (n *nodeRefcountDropBug) String() string {
	return fmt.Sprintf("bug: trying to drop %d of %d references to %v", n.N, n.Refs, n.Node)
}

func (c *serveConn) dropNode(id fuse.NodeID, n uint64) (forget bool) {
	c.meta.Lock()
	defer c.meta.Unlock()
	snode := c.node[id]

	if snode == nil {
		// this should only happen if refcounts kernel<->us disagree
		// *and* two ForgetRequests for the same node race each other;
		// this indicates a bug somewhere
		dprintf("%v\n", nodeRefcountDropBug{N: n, Node: id})

		// we may end up triggering Forget twice, but that's better
		// than not even once, and that's the best we can do
		return true
	}

	if n > snode.refs {
		dprintf("%v\n", nodeRefcountDropBug{N: n, Refs: snode.refs, Node: id})
		n = snode.refs
	}

	snode.refs -= n
	if snode.refs == 0 {
		c.node[id] = nil
		if nodeRef, ok := snode.node.(nodeRef); ok {
			ref := nodeRef.nodeRef()
			*ref = NodeRef{}
		}
		c.freeNode = append(c.freeNode, id)
		return true
	}
	return false
}

func (c *serveConn) dropHandle(id fuse.HandleID) {
	c.meta.Lock()
	c.handle[id] = nil
	c.freeHandle = append(c.freeHandle, id)
	c.meta.Unlock()
}

type missingHandle  {
	Handle    fuse.HandleID
	MaxHandle fuse.HandleID
}

func (m missingHandle) String() string {
	return fmt.Sprint("missing handle", m.Handle, m.MaxHandle)
}

// Returns nil for invalid handles.
func (c *serveConn) getHandle(id fuse.HandleID) (shandle *serveHandle) {
	c.meta.Lock()
	defer c.meta.Unlock()
	if id < fuse.HandleID(len(c.handle)) {
		shandle = c.handle[uint(id)]
	}
	if shandle == nil {
		dprintf("%v\n", missingHandle{
			Handle:    id,
			MaxHandle: fuse.HandleID(len(c.handle)),
		})
	}
	return
}

type request  {
	Op      string
	Request *fuse.Header
	In      interface{} `json:",omitempty"`
}

func (r request) String() string {
	return fmt.Sprintf("<- %s", r.In)
}

type logResponseHeader  {
	ID fuse.RequestID
}

func (m logResponseHeader) String() string {
	return fmt.Sprintf("ID=%#x", m.ID)
}

type response  {
	Op      string
	Request logResponseHeader
	Out     interface{} `json:",omitempty"`
	// Errno contains the errno value as a string, for example "EPERM".
	Errno string `json:",omitempty"`
	// Error may contain a free form error message.
	Error string `json:",omitempty"`
}

func (r response) errstr() string {
	s := r.Errno
	if r.Error != "" {
		// prefix the errno constant to the long form message
		s = s + ": " + r.Error
	}
	return s
}

func (r response) String() string {
	switch {
	case r.Errno!="" && r.Out!=nil:
		return fmt.Sprintf("-> %s error=%s %s", r.Request, r.errstr(), r.Out)
	case r.Errno != "":
		return fmt.Sprintf("-> %s error=%s", r.Request, r.errstr())
	case r.Out != nil:
		// make sure (seemingly) empty values are readable
		switch r.Out.(type) {
		case string:
			return fmt.Sprintf("-> %s %q", r.Request, r.Out)
		case []byte:
			return fmt.Sprintf("-> %s [% x]", r.Request, r.Out)
		default:
			return fmt.Sprintf("-> %s %s", r.Request, r.Out)
		}
	default:
		return fmt.Sprintf("-> %s", r.Request)
	}
}

type logMissingNode  {
	MaxNode fuse.NodeID
}

func opName(req fuse.Request) string {
	t := reflect.Indirect(reflect.ValueOf(req)).Type()
	s := t.Name()
	s = strings.TrimSuffix(s, "Request")
	return s
}

type logLinkRequestOldNodeNotFound  {
	Request *fuse.Header
	In      *fuse.LinkRequest
}

func (m *logLinkRequestOldNodeNotFound) String() string {
	return fmt.Sprintf("In LinkRequest (request %#x), node %d not found", m.Request.Hdr().ID, m.In.OldNode)
}

type renameNewDirNodeNotFound  {
	Request *fuse.Header
	In      *fuse.RenameRequest
}

func (m *renameNewDirNodeNotFound) String() string {
	return fmt.Sprintf("In RenameRequest (request %#x), node %d not found", m.Request.Hdr().ID, m.In.NewDir)
}

func (c *serveConn) serve(r fuse.Request) {
	intr := make(Intr)
	req := &serveRequest{Request: r, Intr: intr}

	dprintf("%v\n", request{
		Op:      opName(r),
		Request: r.Hdr(),
		In:      r,
	})
	var node Node
	var snode *serveNode
	c.meta.Lock()
	hdr := r.Hdr()
	if id := hdr.Node; id != 0 {
		if id < fuse.NodeID(len(c.node)) {
			snode = c.node[uint(id)]
		}
		if snode == nil {
			c.meta.Unlock()
			dprintf("%v\n", response{
				Op:      opName(r),
				Request: logResponseHeader{ID: hdr.ID},
				Error:   fuse.ESTALE.ErrnoName(),
				// this is the only place that sets both Error and
				// Out; not sure if i want to do that; might get rid
				// of len(c.node) things altogether
				Out: logMissingNode{
					MaxNode: fuse.NodeID(len(c.node)),
				},
			})
			r.RespondError(fuse.ESTALE)
			return
		}
		node = snode.node
	}
	if c.req[hdr.ID] != nil {
		// This happens with OSXFUSE.  Assume it's okay and
		// that we'll never see an interrupt for this one.
		// Otherwise everything wedges.  TODO: Report to OSXFUSE?
		//
		// TODO this might have been because of missing done() calls
		intr = nil
	} else {
		c.req[hdr.ID] = req
	}
	c.meta.Unlock()

	// Call this before responding.
	// After responding is too late: we might get another request
	// with the same ID and be very confused.
	done := func(resp interface{}) {
		msg := response{
			Op:      opName(r),
			Request: logResponseHeader{ID: hdr.ID},
		}
		if err, ok := resp.(error); ok {
			msg.Error = err.Error()
			if ferr, ok := err.(fuse.ErrorNumber); ok {
				errno := ferr.Errno()
				msg.Errno = errno.ErrnoName()
				if errno == err {
					// it's just a fuse.Errno with no extra detail;
					// skip the textual message for log readability
					msg.Error = ""
				}
			} else {
				msg.Errno = fuse.DefaultErrno.ErrnoName()
			}
		} else {
			msg.Out = resp
		}
		dprintf("%v\n", msg)

		c.meta.Lock()
		delete(c.req, hdr.ID)
		c.meta.Unlock()
	}

Req:
	switch r := r.(type) {
	default:
		// Note: To FUSE, ENOSYS means "this server never implements this request."
		// It would be inappropriate to return ENOSYS for other operations in this
		// switch that might only be unavailable in some contexts, not all.
		done(fuse.ENOSYS)
		r.RespondError(fuse.ENOSYS)

	// FS operations.
	case *fuse.InitRequest:
		uid = r.Header.Uid
		gid = r.Header.Gid
		s := &fuse.InitResponse{
			MaxWrite: 16*1024,
		}
		done(s)
		r.Respond(s)

	case *fuse.StatfsRequest:
		// fake it so the other end always thinks we have resources
		s := &fuse.StatfsResponse{}
		done(s)
		r.Respond(s)

	// Node operations.
	case *fuse.GetattrRequest:
		s := &fuse.GetattrResponse{}
		s.AttrValid = attrValidTime
		a, err := snode.attr()
		if err != nil {
			done(err)
			r.RespondError(err)
			break
		}
		s.Attr = a
		done(s)
		r.Respond(s)

	case *fuse.SetattrRequest:
		s := &fuse.SetattrResponse{}
		n, ok := node.(NodeSetAttrer)
		if !ok {
			done(fuse.EPERM)
			r.RespondError(fuse.EPERM)
			break
		}
		if err := n.SetAttr(r, intr); err != nil {
			done(err)
			r.RespondError(err)
			break
		}
		if s.AttrValid == 0 {
			s.AttrValid = attrValidTime
		}
		a, err := snode.attr()
		if err != nil {
			done(err)
			r.RespondError(err)
			break
		}
		s.Attr = a
		done(s)
		r.Respond(s)

	case *fuse.GetxattrRequest:
		s := &fuse.GetxattrResponse{}
		n, ok := node.(NodeXAttrer)
		if !ok || r.Position>0 {
			done(fuse.ENODATA)
			r.RespondError(fuse.ENODATA)
			break
		}
		v, err := n.Xattr(r.Name)
		if err != nil {
			done(fuse.ENODATA)
			r.RespondError(fuse.ENODATA)
			break
		}
		s.Xattr = v
		done(s)
		r.Respond(s)

	case *fuse.ListxattrRequest:
		s := &fuse.ListxattrResponse{}
		if n, ok := node.(NodeXAttrer); ok && r.Position==0 {
			v := n.Xattrs()
			if len(v) > 0 {
				s.Append(v...)
			}
		}
		done(s)
		r.Respond(s)

	case *fuse.SetxattrRequest:
		n, ok := node.(NodeXAttrer)
		if !ok || r.Position>0 {
			done(fuse.EPERM)
			r.RespondError(fuse.EPERM)
			break
		}
		err := n.Wxattr(r.Name, r.Xattr)
		if err != nil {
			done(fuse.EPERM)
			r.RespondError(fuse.EPERM)
			break
		}
		done(nil)
		r.Respond()

	case *fuse.RemovexattrRequest:
		n, ok := node.(NodeXAttrer)
		if !ok {
			done(fuse.ENOTSUP)
			r.RespondError(fuse.ENOTSUP)
			break
		}
		err := n.Wxattr(r.Name, nil)
		if err != nil {
			done(fuse.EPERM)
			r.RespondError(fuse.EPERM)
			break
		}
		done(nil)
		r.Respond()

	case *fuse.SymlinkRequest:
		done(fuse.EPERM)
		r.RespondError(fuse.EPERM)

	case *fuse.ReadlinkRequest:
		done(fuse.EPERM)
		r.RespondError(fuse.EPERM)

	case *fuse.LinkRequest:
		done(fuse.EPERM)
		r.RespondError(fuse.EPERM)

	case *fuse.RemoveRequest:
		n, ok := node.(NodeRemover)
		if !ok {
			done(fuse.EIO) /// XXX or EPERM?
			r.RespondError(fuse.EIO)
			break
		}
		err := n.Remove(r.Name, intr)
		if err != nil {
			done(err)
			r.RespondError(err)
			break
		}
		done(nil)
		r.Respond()

	case *fuse.AccessRequest:
		done(nil)
		r.Respond()

	case *fuse.LookupRequest:
		var n2 Node
		var err fuse.Error
		s := &fuse.LookupResponse{}
		if n, ok := node.(NodeLookuper); ok {
			n2, err = n.Lookup(r.Name, intr)
		} else {
			done(fuse.ENOENT)
			r.RespondError(fuse.ENOENT)
			break
		}
		if err != nil {
			done(err)
			r.RespondError(err)
			break
		}
		c.saveLookup(s, snode, r.Name, n2)
		done(s)
		r.Respond(s)

	case *fuse.MkdirRequest:
		s := &fuse.MkdirResponse{}
		n, ok := node.(NodeMkdirer)
		if !ok {
			done(fuse.EPERM)
			r.RespondError(fuse.EPERM)
			break
		}
		n2, err := n.Mkdir(r.Name, r.Mode, intr)
		if err != nil {
			done(err)
			r.RespondError(err)
			break
		}
		c.saveLookup(&s.LookupResponse, snode, r.Name, n2)
		done(s)
		r.Respond(s)

	case *fuse.OpenRequest:

		var h2 Handle
		n := node
		hh, err := n.Open(r.Flags, intr)
		if err != nil {
			done(err)
			r.RespondError(err)
			break
		}
		h2 = hh
		// Using DirectIO requires buffers to be page aligned for exec
		// to work on served files.
		// But not using DirectIO means that UNIX stats the file and
		// then reads the data, which means we can't use it for Ctl files.
		flags := fuse.OpenPurgeAttr | fuse.OpenPurgeUBC
		if xh, ok := hh.(HandleIsCtler); ok && xh.IsCtl() {
			flags = fuse.OpenDirectIO
		}
		if r.Flags&3==fuse.OpenFlags(os.O_WRONLY) &&
			runtime.GOOS=="darwin" {
			/*
				This is required for append on osx
			*/
			flags = fuse.OpenPurgeAttr | fuse.OpenPurgeUBC
		}
		s := &fuse.OpenResponse{Flags: flags}
		s.Handle = c.saveHandle(h2, hdr.Node)
		done(s)
		r.Respond(s)

	case *fuse.CreateRequest:
		n, ok := node.(NodeCreater)
		if !ok {
			done(fuse.EPERM)
			r.RespondError(fuse.EPERM)
			break
		}
		flags := fuse.OpenPurgeAttr | fuse.OpenPurgeUBC
		// flags := fuse.OpenDirectIO
		s := &fuse.CreateResponse{OpenResponse: fuse.OpenResponse{Flags: flags}}
		n2, h2, err := n.Create(r.Name, r.Flags, r.Mode, intr)
		if err != nil {
			done(err)
			r.RespondError(err)
			break
		}
		c.saveLookup(&s.LookupResponse, snode, r.Name, n2)
		s.Handle = c.saveHandle(h2, hdr.Node)
		done(s)
		r.Respond(s)

	case *fuse.ForgetRequest:
		forget := c.dropNode(hdr.Node, r.N)
		if forget {
			n, ok := node.(NodePuter)
			if ok {
				n.PutNode()
			}
		}
		done(nil)
		r.Respond()

	// Handle operations.
	case *fuse.ReadRequest:
		shandle := c.getHandle(r.Handle)
		if shandle == nil {
			done(fuse.ESTALE)
			r.RespondError(fuse.ESTALE)
			return
		}
		handle := shandle.handle

		s := &fuse.ReadResponse{Data: make([]byte, 0, r.Size)}
		if r.Dir {
			h := handle
			if shandle.readData == nil {
				dirs, err := h.ReadDir(intr)
				if err != nil {
					done(err)
					r.RespondError(err)
					break
				}
				var data []byte
				for _, dir := range dirs {
					if dir.Inode == 0 {
						err := errors.New("bad inode")
						done(err)
						r.RespondError(err)
						break Req
					}
					data = fuse.AppendDirent(data, dir)
				}
				shandle.readData = data
			}
			fuseutil.HandleRead(r, s, shandle.readData)
			done(s)
			r.Respond(s)
			break
		}
		h := handle
		rdata, err := h.Read(r.Offset, r.Size, intr)
		if err != nil {
			done(err)
			r.RespondError(err)
			break
		}
		s.Data = rdata
		done(s)
		r.Respond(s)

	case *fuse.WriteRequest:
		shandle := c.getHandle(r.Handle)
		if shandle == nil {
			done(fuse.ESTALE)
			r.RespondError(fuse.ESTALE)
			return
		}

		s := &fuse.WriteResponse{}
		if h, ok := shandle.handle.(HandleWriter); ok {
			n, err := h.Write(r.Data, r.Offset, intr)
			if err != nil {
				done(err)
				r.RespondError(err)
				break
			}
			s.Size = n
			done(s)
			r.Respond(s)
			break
		}
		done(fuse.EPERM)
		r.RespondError(fuse.EPERM)

	case *fuse.FlushRequest:
		shandle := c.getHandle(r.Handle)
		if shandle == nil {
			done(fuse.ESTALE)
			r.RespondError(fuse.ESTALE)
			return
		}
		handle := shandle.handle
		h := handle
		if err := h.Close(intr); err != nil {
			done(err)
			r.RespondError(err)
			break
		}
		done(nil)
		r.Respond()

	case *fuse.ReleaseRequest:
		shandle := c.getHandle(r.Handle)
		if shandle == nil {
			done(fuse.ESTALE)
			r.RespondError(fuse.ESTALE)
			return
		}
		handle := shandle.handle

		// No matter what, release the handle.
		c.dropHandle(r.Handle)

		if h, ok := handle.(HandlePuter); ok {
			h.PutHandle()
		}
		done(nil)
		r.Respond()

	case *fuse.DestroyRequest:
		done(nil)
		r.Respond()

	case *fuse.RenameRequest:
		c.meta.Lock()
		var newDirNode *serveNode
		if int(r.NewDir) < len(c.node) {
			newDirNode = c.node[r.NewDir]
		}
		c.meta.Unlock()
		if newDirNode == nil {
			dprintf("%v\n", renameNewDirNodeNotFound{
				Request: r.Hdr(),
				In:      r,
			})
			done(fuse.EIO)
			r.RespondError(fuse.EIO)
			break
		}
		n, ok := node.(NodeRenamer)
		if !ok {
			done(fuse.EPERM)
			r.RespondError(fuse.EPERM)
			break
		}
		err := n.Rename(r.OldName, r.NewName, newDirNode.node, intr)
		if err != nil {
			done(err)
			r.RespondError(err)
			break
		}
		done(nil)
		r.Respond()

	case *fuse.MknodRequest:
		done(fuse.EIO)
		r.RespondError(fuse.EIO)

	case *fuse.FsyncRequest:
		n, ok := node.(NodeFsyncer)
		if !ok {
			// if there's no implementation, we pretend it's done.
			done(nil)
			r.Respond()
			break
		}
		err := n.Fsync(intr)
		if err != nil {
			done(err)
			r.RespondError(err)
			break
		}
		done(nil)
		r.Respond()

	case *fuse.InterruptRequest:
		c.meta.Lock()
		ireq := c.req[r.IntrID]
		if ireq!=nil && ireq.Intr!=nil {
			close(ireq.Intr)
			ireq.Intr = nil
		}
		c.meta.Unlock()
		done(nil)
		r.Respond()

		/*	case *FsyncdirRequest:
				done(ENOSYS)
				r.RespondError(ENOSYS)

			case *GetlkRequest, *SetlkRequest, *SetlkwRequest:
				done(ENOSYS)
				r.RespondError(ENOSYS)

			case *BmapRequest:
				done(ENOSYS)
				r.RespondError(ENOSYS)

			case *SetvolnameRequest, *GetxtimesRequest, *ExchangeRequest:
				done(ENOSYS)
				r.RespondError(ENOSYS)
		*/
	}
}

func (c *serveConn) saveLookup(s *fuse.LookupResponse, snode *serveNode, elem string, n2 Node) {
	var err error
	s.Attr, err = nodeAttr(n2)
	if err != nil {
		return
	}
	if s.Attr.Inode == 0 {
		panic("saveLookup: 0 inode")
	}

	s.Node, s.Generation = c.saveNode(s.Attr.Inode, n2)
	if s.EntryValid == 0 {
		s.EntryValid = entryValidTime
	}
	if s.AttrValid == 0 {
		s.AttrValid = attrValidTime
	}
}

// DataHandle returns a read-only Handle that satisfies reads
// using the given data.
func DataHandle(data []byte) Handle {
	return &dataHandle{data}
}

type dataHandle  {
	data []byte
}

func (d *dataHandle) Read(off int64, sz int, intr Intr) ([]byte, fuse.Error) {
	if off >= int64(len(d.data)) {
		return []byte{}, nil
	}
	// caution: we are not copying into a new slice.
	rd := d.data[int(off):]
	if sz > len(rd) {
		sz = len(rd)
	}
	return rd[:sz], nil
}

func (d *dataHandle) ReadDir(Intr) ([]fuse.Dirent, fuse.Error) {
	dbg.Warn("clive/fuse: BUG: dataHandle ReadDir was called")
	return nil, fuse.EPERM
}

func (d *dataHandle) Close(Intr) fuse.Error {
	return nil
}
