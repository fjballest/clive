package rfs

import (
	"clive/dbg"
	"clive/nchan"
	"clive/net/auth"
	"clive/zx"
	"fmt"
	"os"
	"time"
)

type handler func(*Msg, <-chan []byte, chan<- []byte)

/*
	a server for a set of zx.Trees.
*/
type Srv  {
	t       []zx.Tree
	ro      bool
	c       *nchan.Mux
	ops     map[MsgId]handler
	Tag     string
	Debug   bool
	dprintf dbg.PrintFunc
	ai      *auth.Info

	donec chan bool
	tid   int

	Pings bool	// set if you issue ping requests to auto close if tout.
}

const (
	RO = true
	RW = false
)

var (
	Verb    bool
	vprintf = dbg.FlagPrintf(os.Stdout, &Verb)
	idc     = make(chan int)
)

func init() {
	go func() {
		for id := 1; ; id++ {
			idc <- id
		}
	}()
}

/*
	Serve some trees through the given connection.
	The connection should no longer be used by the caller.

	The auth info is that provided by the auth protocol performend in con, and may be nil.
*/
func Serve(tag string, con nchan.Conn, ai *auth.Info, ronly bool, t ...zx.Tree) *Srv {
	if len(t) == 0 {
		return nil
	}
	if ai != nil {
		nt := make([]zx.Tree, 0, len(t))
		for i := 0; i < len(t); i++ {
			if st, ok := t[i].(zx.ServerTree); ok {
				ci := &zx.ClientInfo{Tag: con.Tag, Ai: ai, Id: <-idc}
				xt, err := st.ServerFor(ci)
				if err != nil {
					dbg.Warn("%s: server tree: %s", t[i].Name(), err)
					continue
				}
				nt = append(nt, xt)
			} else if at, ok := t[i].(zx.AuthTree); ok {
				xt, err := at.AuthFor(ai)
				if err != nil {
					dbg.Warn("%s: auth tree: %s", t[i].Name(), err)
					continue
				}
				nt = append(nt, xt)
			} else {
				dbg.Warn("%s: no auth", t[i].Name())
				nt = append(nt, t[i])
			}
		}
		t = nt
	}
	c := nchan.NewMux(con, false)
	s := &Srv{
		t:     t,
		c:     c,
		ro:    ronly,
		Tag:   tag,
		ai:    ai,
		donec: make(chan bool),
	}
	s.dprintf = dbg.FlagPrintf(os.Stdout, &s.Debug)
	s.ops = map[MsgId]handler{
		Tstat:      s.stat,
		Tget:       s.get,
		Tput:       s.put,
		Tmkdir:     s.mkdir,
		Tmove:      s.move,
		Tremove:    s.remove,
		Tremoveall: s.removeall,
		Twstat:     s.wstat,
		Tfind:      s.find,
		Tfindget:   s.findget,
		Tfsys:      s.fsys,
	}
	go s.loop()
	return s
}

func (s *Srv) Close(e error) {
	close(s.c.In, e)
}

func (s *Srv) Wait() {
	<-s.donec
}

const (
	logIn  = "logged in"
	logOut = "logged out"
)

// trees that implement this interface get calls made to report
// logs in/out for dialing users.
type LogInOutTree interface {
	LogIn(who string)
	LogOut(who string)
}

func (xs *Srv) logUsr(what string) {
	if xs.ai == nil {
		vprintf("%s none %s\n", xs.Tag, what)
		return
	}
	id := fmt.Sprintf("%s %s as %s", xs.Tag, xs.ai.SpeaksFor, xs.ai.Uid)
	if xs.ai != nil {
		vprintf("%s: %s %s\n", os.Args[0], id, what)
	} else {
		vprintf("%s: %s none as none %s\n", os.Args[0], id, what)
	}
	for _, t := range xs.t {
		if lt, ok := t.(LogInOutTree); ok {
			if what == logIn {
				lt.LogIn(id)
			} else {
				lt.LogOut(id)
			}
		}
	}
}

func (xs *Srv) loop() {
	s := &Srv{}
	*s = *xs // Everything should be r/o or shared, and s.tid is local
	c := s.c
	xs.logUsr(logIn)
	defer xs.logUsr(logOut)
	s.dprintf("%s loop started\n", xs.Tag)
	doselect {
	case <-time.After(60*time.Second):
		if xs.Pings {
			break
		}
	case x, ok := <-c.In:
		if !ok {
			break
		}
		// c.Debug = s.Debug	// too verbose
		// one req
		go func() {
			hdr, ok := <-x.In
			if !ok {
				close(x.Out, cerror(c.In))
				return
			}
			msg, err := UnpackMsg(hdr)
			if err != nil {
				s.dprintf("msg: %s", err)
				close(x.In, err)
				close(x.Out, err)
				close(c.In, err)
				return
			}
			if op, ok := s.ops[msg.Op]; ok {
				s.dprintf("%s<- %s\n", s.Tag, msg)
				op(msg, x.In, x.Out)
				return
			} else {
				s.dprintf("%s <-???- %s\n", s.Tag, msg)
			}
			close(x.Out, "request not implemented or unknown request")
		}()
	}
	c.Close(nil)
	close(xs.donec)
}

func (s *Srv) stat(m *Msg, c <-chan []byte, rc chan<- []byte) {
	var d zx.Dir
	dc := s.t[s.tid].Stat(m.Rid)
	d = <-dc
	if d == nil {
		err := cerror(dc)
		s.dprintf("%s-> %s\n", s.Tag, err)
		close(rc, err)
		return
	}
	delete(d, "tpath")
	d["proto"] = "zx"
	s.dprintf("%s-> %s\n", s.Tag, d)
	rc <- d.Pack()
	close(rc)
}

func (s *Srv) get(m *Msg, c <-chan []byte, rc chan<- []byte) {
	dc := s.t[s.tid].Get(m.Rid, m.Off, m.Count, m.Pred)
	if m.Rid == "/" {
	}
	for d := range dc {
		ok := rc <- d
		if !ok {
			close(dc, "hup")
			break
		}
	}
	err := cerror(dc)
	s.dprintf("%s-> sts %v\n", s.Tag, err)
	close(rc, err)
}

func (s *Srv) put(m *Msg, c <-chan []byte, rc chan<- []byte) {
	wt, ok := s.t[s.tid].(zx.RWTree)
	if !ok || s.ro {
		close(rc, dbg.ErrRO)
		// close c instead?
		for x := range c {
			if len(x) == 0 {
				break
			}
		}
		return
	}
	dc := wt.Put(m.Rid, m.D, m.Off, c, m.Pred)
	d := <-dc
	err := cerror(dc)
	s.dprintf("%s-> d %s sts %v\n", s.Tag, d, err)
	if d != nil {
		rc <- d.Pack()
	}
	close(rc, err)
}

func (s *Srv) mkdir(m *Msg, c <-chan []byte, rc chan<- []byte) {
	wt, ok := s.t[s.tid].(zx.RWTree)
	if !ok || s.ro {
		close(rc, dbg.ErrRO)
		return
	}
	err := <-wt.Mkdir(m.Rid, m.D)
	s.dprintf("%s-> sts %v\n", s.Tag, err)
	close(rc, err)
}

func (s *Srv) move(m *Msg, c <-chan []byte, rc chan<- []byte) {
	wt, ok := s.t[s.tid].(zx.RWTree)
	if !ok || s.ro {
		close(rc, dbg.ErrRO)
		return
	}
	err := <-wt.Move(m.Rid, m.To)
	s.dprintf("%s-> sts %v\n", s.Tag, err)
	close(rc, err)
}

func (s *Srv) remove(m *Msg, c <-chan []byte, rc chan<- []byte) {
	wt, ok := s.t[s.tid].(zx.RWTree)
	if !ok || s.ro {
		close(rc, dbg.ErrRO)
		return
	}
	err := <-wt.Remove(m.Rid)
	s.dprintf("%s-> sts %v\n", s.Tag, err)
	close(rc, err)
}

func (s *Srv) removeall(m *Msg, c <-chan []byte, rc chan<- []byte) {
	wt, ok := s.t[s.tid].(zx.RWTree)
	if !ok || s.ro {
		close(rc, dbg.ErrRO)
		return
	}
	err := <-wt.RemoveAll(m.Rid)
	s.dprintf("%s-> sts %v\n", s.Tag, err)
	close(rc, err)
}

func (s *Srv) wstat(m *Msg, c <-chan []byte, rc chan<- []byte) {
	wt, ok := s.t[s.tid].(zx.RWTree)
	if !ok || s.ro {
		close(rc, dbg.ErrRO)
		return
	}
	err := <-wt.Wstat(m.Rid, m.D)
	s.dprintf("%s-> sts %v\n", s.Tag, err)
	close(rc, err)
}

func (s *Srv) find(m *Msg, c <-chan []byte, rc chan<- []byte) {
	dc := s.t[s.tid].Find(m.Rid, m.Pred, m.Spref, m.Dpref, m.Depth)
	for d := range dc {
		if len(d) == 0 {
			break
		}
		delete(d, "tpath")
		if ok := rc <- d.Pack(); !ok {
			close(dc, cerror(rc))
			return
		}
	}
	err := cerror(dc)
	s.dprintf("%s-> sts %v\n", s.Tag, err)
	close(rc, err)
}

func (s *Srv) findget(m *Msg, c <-chan []byte, rc chan<- []byte) {
	gc := s.t[s.tid].FindGet(m.Rid, m.Pred, m.Spref, m.Dpref, m.Depth)
	for g := range gc {
		d := g.Dir
		if len(d) == 0 {
			break
		}
		delete(d, "tpath")
		if ok := rc <- d.Pack(); !ok {
			close(gc, cerror(rc))
			return
		}
		if d["type"]=="-" && d["err"]=="" {
			for data := range g.Datac {
				if len(data) == 0 {
					break
				}
				if ok := rc <- data; !ok {
					close(g.Datac, cerror(rc))
					close(gc, cerror(rc))
					return
				}
			}
			rc <- []byte{}
			close(g.Datac)
		}
	}
	err := cerror(gc)
	s.dprintf("%s-> sts %v\n", s.Tag, err)
	close(rc, err)
}

func (s *Srv) fsys(m *Msg, c <-chan []byte, rc chan<- []byte) {
	if m.Rid == "main" {
		s.tid = 0
		close(rc)
		return
	}
	for i := 0; i < len(s.t); i++ {
		if s.t[i].Name() == m.Rid {
			s.tid = i
			close(rc)
			return
		}
	}
	s.dprintf("%s-> sts no such tree\n", s.Tag)
	close(rc, "no such tree")
}
