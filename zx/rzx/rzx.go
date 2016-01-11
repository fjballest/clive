/*
	Remote ZX access
*/
package rzx

import (
	"clive/ch"
	"clive/dbg"
	"clive/net"
	"clive/net/auth"
	"clive/zx"
	"crypto/tls"
	"fmt"
	"sort"
	"strings"
	"sync"
)

struct clients {
	sync.Mutex
	set map[string]string
}

struct Server {
	*dbg.Flag
	*sync.Mutex
	fs      map[string]zx.Fs // file trees served
	addr    string           // where served
	rdonly  bool
	noauth  bool
	inc     <-chan *ch.Mux
	endc    chan bool
	clients *clients
	// when we auth a user, we make a new copy of the Server
	// struct, with local copies of everything that's not a pointer,
	// and a new ai for the user.
	// If you are more fields, and are to be shared among all users
	// make sure they are references
}

func (c *clients) add(addr, uid string) {
	c.Lock()
	c.set[addr] = uid
	c.Unlock()
}

func (c *clients) del(tag string) {
	c.Lock()
	delete(c.set, tag)
	c.Unlock()
}

func (c *clients) list() []string {
	c.Lock()
	defer c.Unlock()
	out := make([]string, 0, len(c.set))
	for k, v := range c.set {
		out = append(out, fmt.Sprintf("%s %s", v, k))
	}
	sort.Sort(sort.StringSlice(out))
	return out
}

func (c *clients) String() string {
	cls := c.list()
	if len(cls) == 0 {
		return "none"
	}
	return strings.Join(cls, "\nuser ")
}

func (s *Server) String() string {
	return s.addr
}

func (s *Server) mkaddr(d zx.Dir, fsys string) {
	old := d["addr"]
	p := strings.LastIndexByte(old, '!')
	if p < 0 {
		p = 0
	} else {
		p++
	}
	d["addr"] = fmt.Sprintf("zx!%s!%s!%s", s.addr, fsys, old[p:])
}

func newServer(addr string, tc *tls.Config, ro bool) (*Server, error) {
	inc, endc, err := net.MuxServe(addr, tc)
	if err != nil {
		return nil, err
	}
	s := &Server{
		Flag:    &dbg.Flag{},
		Mutex:   &sync.Mutex{},
		inc:     inc,
		endc:    endc,
		addr:    addr,
		rdonly:  ro,
		fs:      map[string]zx.Fs{},
		clients: &clients{set: map[string]string{}},
	}
	s.Tag = addr
	go s.loop()
	return s, nil
}

// Start a read-write server at the given address.
func NewServer(addr string, tlscfg ...*tls.Config) (*Server, error) {
	var tc *tls.Config
	if len(tlscfg) > 0 {
		tc = tlscfg[0]
	}
	return newServer(addr, tc, false)
}

// Start a read-only server at the given address.
func NewROServer(addr string, tlscfg ...*tls.Config) (*Server, error) {
	var tc *tls.Config
	if len(tlscfg) > 0 {
		tc = tlscfg[0]
	}
	return newServer(addr, tc, true)
}

// Disable auth in server
func (s *Server) NoAuth() {
	s.noauth = true
}

interface flagAdder {
	Add(name string, vp face{})
	AddRO(name string, vp face{})
}

// Serve fs with the given tree name.
func (s *Server) Serve(name string, fs zx.Fs) error {
	s.Lock()
	defer s.Unlock()
	if s.fs[name] != nil {
		return fmt.Errorf("%s: %s already served", s.addr, name)
	}
	s.fs[name] = fs
	if ffs, ok := fs.(flagAdder); ok {
		ffs.AddRO("server rdonly", &s.rdonly)
		ffs.AddRO("server noauth", &s.noauth)
		ffs.AddRO("server addr", &s.addr)
		ffs.AddRO("user", s.clients)
	}
	dbg.Warn("%s: serving %s...", s, fs)
	return nil
}

func (s *Server) tree(name string) zx.Fs {
	s.Lock()
	defer s.Unlock()
	return s.fs[name]
}

func (s *Server) trees(c ch.Conn, m *Msg, fs zx.Fs) error {
	ts := []string{}
	s.Lock()
	for t := range s.fs {
		ts = append(ts, t)
	}
	s.Unlock()
	for s := range s.fs {
		if ok := c.Out <- s; !ok {
			return cerror(c.Out)
		}
	}
	return nil
}

func (s *Server) stat(c ch.Conn, m *Msg, fs zx.Fs) error {
	d, err := zx.Stat(fs, m.Path)
	if err == nil {
		s.mkaddr(d, m.Fsys)
		c.Out <- d
	}
	return err
}

func (s *Server) get(c ch.Conn, m *Msg, fs zx.Fs) error {
	xfs, ok := fs.(zx.Getter)
	if !ok {
		return zx.ErrBug
	}
	d, err := zx.Stat(fs, m.Path)
	if err != nil {
		return err
	}
	isdir := d["type"] == "d"
	rc := xfs.Get(m.Path, m.Off, m.Count)
	for x := range rc {
		if isdir {
			_, d, err := zx.UnpackDir(x)
			if err == nil {
				s.mkaddr(d, m.Fsys)
				x = d.Bytes()
			}
		}
		if ok := c.Out <- x; !ok {
			err := cerror(c.Out)
			close(rc, err)
			return err
		}
	}
	return cerror(rc)
}

func (s *Server) put(c ch.Conn, m *Msg, fs zx.Fs) error {
	if s.rdonly {
		return fmt.Errorf("%s: %s", s.addr, zx.ErrRO)
	}
	xfs, ok := fs.(zx.Putter)
	if !ok {
		return zx.ErrBug
	}
	ic := make(chan []byte)
	if m.D["type"] == "d" {
		close(ic)
	} else {
		go func() {
			for m := range c.In {
				switch m := m.(type) {
				case []byte:
					ok := ic <- m
					if !ok {
						close(c.In, cerror(ic))
						break
					}
				default:
					err := ErrBadMsg
					close(c.In, err)
					close(ic, err)
					break
				}
			}
			close(ic, cerror(c.In))
		}()
	}
	rc := xfs.Put(m.Path, m.D, m.Off, ic)
	rd := <-rc
	if err := cerror(rc); err != nil {
		return err
	}
	s.mkaddr(rd, m.Fsys)
	if ok := c.Out <- rd; !ok {
		return cerror(c.Out)
	}
	return nil
}

func (s *Server) move(c ch.Conn, m *Msg, fs zx.Fs) error {
	if s.rdonly {
		return fmt.Errorf("%s: %s", s.addr, zx.ErrRO)
	}
	xfs, ok := fs.(zx.Mover)
	if !ok {
		return zx.ErrBug
	}
	return <-xfs.Move(m.Path, m.To)
}

func (s *Server) link(c ch.Conn, m *Msg, fs zx.Fs) error {
	if s.rdonly {
		return fmt.Errorf("%s: %s", s.addr, zx.ErrRO)
	}
	xfs, ok := fs.(zx.Linker)
	if !ok {
		return zx.ErrBug
	}
	return <-xfs.Link(m.To, m.Path)
}

func (s *Server) remove(c ch.Conn, m *Msg, fs zx.Fs) error {
	if s.rdonly {
		return fmt.Errorf("%s: %s", s.addr, zx.ErrRO)
	}
	xfs, ok := fs.(zx.Remover)
	if !ok {
		return zx.ErrBug
	}
	if m.Path == "" || m.Path == "/" {
		return fmt.Errorf("%s: won't remove /", s.addr)
	}
	if m.Op == Tremove {
		return <-xfs.Remove(m.Path)
	}
	return <-xfs.RemoveAll(m.Path)
}

func (s *Server) find(c ch.Conn, m *Msg, fs zx.Fs) error {
	xfs, ok := fs.(zx.Finder)
	if !ok {
		return zx.ErrBug
	}
	rc := xfs.Find(m.Path, m.Pred, m.Spref, m.Dpref, m.Depth)
	for d := range rc {
		s.mkaddr(d, m.Fsys)
		if ok := c.Out <- d; !ok {
			err := cerror(c.Out)
			close(rc, err)
			return err
		}
	}
	return cerror(rc)
}

func (s *Server) findget(c ch.Conn, m *Msg, fs zx.Fs) error {
	xfs, ok := fs.(zx.FindGetter)
	if !ok {
		return zx.ErrBug
	}
	rc := xfs.FindGet(m.Path, m.Pred, m.Spref, m.Dpref, m.Depth)
	for x := range rc {
		if d, ok := x.(zx.Dir); ok {
			s.mkaddr(d, m.Fsys)
		}
		if ok := c.Out <- x; !ok {
			err := cerror(c.Out)
			close(rc, err)
			return err
		}
	}
	return cerror(rc)
}

func (s *Server) wstat(c ch.Conn, m *Msg, fs zx.Fs) error {
	if s.rdonly {
		return fmt.Errorf("%s: %s", s.addr, zx.ErrRO)
	}
	xfs, ok := fs.(zx.Wstater)
	if !ok {
		return zx.ErrBug
	}
	rc := xfs.Wstat(m.Path, m.D)
	rd := <-rc
	if err := cerror(rc); err != nil {
		return err
	}
	s.mkaddr(rd, m.Fsys)
	if ok := c.Out <- rd; !ok {
		return cerror(c.Out)
	}
	return nil
}

func (s *Server) req(c ch.Conn) {
	var rerr error
	dat, ok := <-c.In
	if !ok {
		rerr = cerror(c.In)
		close(c.In, rerr)
		close(c.Out, rerr)
		return
	}
	switch m := dat.(type) {
	case *Msg:
		s.Dprintf("%s: <- %s\n", c.Tag, m)
		if m.Op == Ttrees {
			rerr = s.trees(c, m, nil)
			break
		}
		fs := s.tree(m.Fsys)
		if fs == nil {
			rerr = fmt.Errorf("no fsys '%s'", m.Fsys)
			break
		}
		switch m.Op {
		case Tstat:
			rerr = s.stat(c, m, fs)
		case Tget:
			rerr = s.get(c, m, fs)
		case Tput:
			rerr = s.put(c, m, fs)
		case Tmove:
			rerr = s.move(c, m, fs)
		case Tremove, Tremoveall:
			rerr = s.remove(c, m, fs)
		case Tfind:
			rerr = s.find(c, m, fs)
		case Tfindget:
			rerr = s.findget(c, m, fs)
		case Twstat:
			rerr = s.wstat(c, m, fs)
		default:
			rerr = fmt.Errorf("unknown msg op %v", m.Op)
		}
	default:
		rerr = fmt.Errorf("unknown msg type %T", m)
	}
	if rerr != nil {
		s.Dprintf("%s: %s\n", c.Tag, rerr)
	}
	close(c.In, rerr)
	close(c.Out, rerr)
}

func (s *Server) authFor(ai *auth.Info) *Server {
	s.Lock()
	defer s.Unlock()
	ns := &Server{}
	*ns = *s
	ns.fs = map[string]zx.Fs{}
	for n, fs := range s.fs {
		if afs, ok := fs.(zx.Auther); ok {
			fs, err := afs.Auth(ai)
			if err != nil {
				dbg.Warn("%s: user %s: fs auth: %s", s.addr, ai.Uid, err)
			} else {
				ns.fs[n] = fs
			}
		} else {
			ns.fs[n] = fs
		}
	}
	return ns
}

func (s *Server) client(mx *ch.Mux) {
	s.Dprintf("new client %s\n", mx.Tag)
	defer s.Dprintf("gone client %s\n", mx.Tag)
	var ai *auth.Info
	var err error
	for c := range mx.In {
		if c.Out == nil {
			close(c.In, "must issue auth rpc")
			continue
		}
		if s.noauth {
			ai, err = auth.NoneAtServer(c, "", "zx")
			if ai != nil && err != nil && err.Error() == "auth disabled" {
				err = nil
			}
		} else {
			ai, err = auth.AtServer(c, "", "zx")
		}
		if err != nil {
			dbg.Warn("%s: %s: %s", s.addr, mx.Tag, err)
			continue
		}
		break
	}
	s.Dprintf("%s auth as %s\n", mx.Tag, ai.Uid)
	s.clients.add(mx.Tag, ai.Uid)
	ns := s.authFor(ai)
	for c := range mx.In {
		go ns.req(c)
	}
	ns.clients.del(mx.Tag)
}

func (s *Server) loop() {
	doselect {
	case mx, ok := <-s.inc:
		if !ok {
			close(s.endc, cerror(s.inc))
			continue
		}
		go s.client(mx)
	case <-s.endc:
		dbg.Warn("%s: server exiting", s)
		close(s.inc, "exiting")
		break
	}
}

// Terminate the server.
func (s *Server) Close() {
	close(s.endc)
}

// Wait until the server is done
func (s *Server) Wait() error {
	<-s.endc
	return cerror(s.endc)
}
