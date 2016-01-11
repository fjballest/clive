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

// Remote zx client
struct Fs {
	*dbg.Flag
	*zx.Flags
	Verb  bool
	addr  string
	ai    *auth.Info
	trees map[string]bool
	fsys  string
	m     *ch.Mux
}

type ddir zx.Dir

func (d ddir) String() string {
	return zx.Dir(d).LongFmt()
}

var (
	dials   = map[string]*Fs{}
	dialslk sync.Mutex
	_fs     zx.FullFs = &Fs{}
)

func (fs *Fs) String() string {
	return fs.Tag
}

func dialed(addr string) (*Fs, bool) {
	dialslk.Lock()
	defer dialslk.Unlock()
	fs, ok := dials[addr]
	return fs, ok
}

// return network!host!port!tree from addr.
// 	host -> tcp!host!zx!main
//	host!port -> tcp!host!port!main
//	net!host!port -> net!host!port!main
func FillAddr(addr string) string {
	toks := strings.Split(addr, "!")
	switch len(toks) {
	case 1:
		return fmt.Sprintf("tcp!%s!zx!main", toks[0])
	case 2:
		return fmt.Sprintf("tcp!%s!%s!main", toks[0], toks[1])
	case 3:
		return fmt.Sprintf("%s!main", addr)
	default:
		return addr
	}
}

func splitaddr(addr string) (string, string) {
	n := strings.LastIndexByte(addr, '!')
	if n < 0 {
		panic("bad address")
	}
	return addr[:n], addr[n+1:]
}

// addr is completed if needed using FillAddr()
// The previously dialed addresses are cached and the
// old connections are returned.
// Different fsys names are considered different dials.
func Dial(addr string, tlscfg ...*tls.Config) (*Fs, error) {
	var tc *tls.Config
	if len(tlscfg) > 0 {
		tc = tlscfg[0]
	}
	addr = FillAddr(addr)
	if fs, ok := dialed(addr); ok {
		return fs, nil
	}
	addr, fsys := splitaddr(addr)
	m, err := net.MuxDial(addr, tc)
	if err != nil {
		return nil, err
	}
	fs := &Fs{
		Flag:  &dbg.Flag{},
		Flags: &zx.Flags{},
		addr:  addr,
		m:     m,
		trees: map[string]bool{},
		fsys:  fsys,
	}
	fs.Tag = "rfs"
	fs.Flags.Add("debug", &fs.Debug)
	fs.Flags.Add("verbdebug", &fs.Verb)
	call := fs.m.Rpc()
	ai, err := auth.AtClient(call, "", "zx")
	if err != nil {
		if !strings.Contains(err.Error(), "auth disabled") {
			m.Close()
			return nil, fmt.Errorf("%s: %s", addr, err)
		}
		dbg.Warn("%s: %s", addr, err)
	}
	fs.ai = ai
	err = fs.getTrees()
	if err != nil {
		m.Close()
		return nil, err
	}
	if !fs.trees[fsys] {
		m.Close()
		return nil, fmt.Errorf("no fsys '%s' found in server", fsys)
	}
	dialslk.Lock()
	dials[addr] = fs
	dialslk.Unlock()
	go func() {
		<-m.Hup
		dialslk.Lock()
		delete(dials, addr)
		dialslk.Unlock()
	}()
	return fs, nil
}

func (fs *Fs) Close() error {
	fs.m.Close()
	return nil
}

func (fs *Fs) getTrees() error {
	c := fs.m.Rpc()
	m := &Msg{Op: Ttrees, Fsys: "main"}
	fs.Dprintf("->%s\n", m)
	if ok := c.Out <- m; !ok {
		err := cerror(c.Out)
		close(c.In, err)
		return err
	}
	close(c.Out)
	for m := range c.In {
		fs.Dprintf("<-%s\n", m)
		if s, ok := m.(string); !ok {
			err := ErrBadMsg
			close(c.In, err)
			return err
		} else {
			fs.trees[s] = true
		}
	}
	fs.trees["main"] = true	// by convention
	return cerror(c.In)
}

func (fs *Fs) Trees() []string {
	ts := []string{}
	for t := range fs.trees {
		ts = append(ts, t)
	}
	sort.Sort(sort.StringSlice(ts))
	return ts
}

func (fs *Fs) Fsys(name string) (*Fs, error) {
	nfs := &Fs{}
	*nfs = *fs
	nfs.fsys = name
	if name == "main" || fs.trees[name] {
		return nfs, nil
	}
	return nil, fmt.Errorf("no fsys '%s'", name)
}

func (fs *Fs) dircall(p string, m *Msg) chan zx.Dir {
	rc := make(chan zx.Dir, 1)
	go func() {
		c := fs.m.Rpc()
		fs.Dprintf("->%s\n", m)
		if ok := c.Out <- m; !ok {
			err := cerror(c.Out)
			close(c.In, err)
			return
		}
		close(c.Out)
		m := <-c.In
		err := cerror(c.In)
		if err == nil {
			if d, ok := m.(zx.Dir); ok {
				fs.Dprintf("<-%s\n", ddir(d))
				rc <- d
			} else {
				err = ErrBadMsg
			}
		} else {
			fs.Dprintf("<-%v\n", err)
		}
		close(rc, err)
		close(c.In, err)
	}()
	return rc
}

func (fs *Fs) Stat(p string) <-chan zx.Dir {
	m := &Msg{Op: Tstat, Fsys: fs.fsys, Path: p}
	return fs.dircall(p, m)
}

func (fs *Fs) Wstat(p string, d zx.Dir) <-chan zx.Dir {
	m := &Msg{Op: Twstat, Fsys: fs.fsys, Path: p, D: d.Dup()}
	return fs.dircall(p, m)
}

func (fs *Fs) errcall(m *Msg) chan error {
	rc := make(chan error, 1)
	go func() {
		c := fs.m.Rpc()
		fs.Dprintf("->%s\n", m)
		if ok := c.Out <- m; !ok {
			err := cerror(c.Out)
			close(c.In, err)
			return
		}
		close(c.Out)
		<-c.In
		err := cerror(c.In)
		if err == nil {
			fs.Dprintf("<- ok\n")
		} else {
			fs.Dprintf("<- %v\n", err)
		}
		close(c.In, err)
		rc <- err
		close(rc, err)
	}()
	return rc
}

func (fs *Fs) Remove(p string) <-chan error {
	m := &Msg{Op: Tremove, Fsys: fs.fsys, Path: p}
	return fs.errcall(m)
}

func (fs *Fs) RemoveAll(p string) <-chan error {
	m := &Msg{Op: Tremoveall, Fsys: fs.fsys, Path: p}
	return fs.errcall(m)
}

func (fs *Fs) Move(from, to string) <-chan error {
	m := &Msg{Op: Tmove, Fsys: fs.fsys, Path: from, To: to}
	return fs.errcall(m)
}

func (fs *Fs) Link(oldp, newp string) <-chan error {
	m := &Msg{Op: Tlink, Fsys: fs.fsys, Path: newp, To: oldp}
	return fs.errcall(m)
}

func (fs *Fs) Get(p string, off, count int64) <-chan []byte {
	rc := make(chan []byte, 1)
	go func() {
		c := fs.m.Rpc()
		m := &Msg{Op: Tget, Fsys: fs.fsys, Path: p, Off: off, Count: count}
		fs.Dprintf("->%s\n", m)
		if ok := c.Out <- m; !ok {
			err := cerror(c.Out)
			close(c.In, err)
			return
		}
		close(c.Out)
		for m := range c.In {
			m, ok := m.([]byte)
			if !ok {
				fs.Dprintf("<- %v\n", m)
				err := ErrBadMsg
				close(c.In, err)
				close(rc, err)
				break
			} else {
				if fs.Verb {
					fs.Dprintf("<- [%d]bytes\n", len(m))
				}
				if ok := rc <- m; !ok {
					close(c.In, cerror(rc))
					break
				}
			}
		}
		err := cerror(c.In)
		if err != nil {
			fs.Dprintf("<-%s\n", err)
		}
		close(rc, err)
	}()
	return rc
}

func (fs *Fs) Put(p string, d zx.Dir, off int64, dc <-chan []byte) <-chan zx.Dir {
	rc := make(chan zx.Dir, 1)
	d = d.Dup()
	go func() {
		c := fs.m.Rpc()
		if dc == nil || d["type"] == "d" {
			dc = make(chan []byte)
			close(dc)
		}
		m := &Msg{Op: Tput, Fsys: fs.fsys, Path: p, D: d, Off: off}
		fs.Dprintf("->%s\n", m)
		if ok := c.Out <- m; !ok {
			err := cerror(c.Out)
			close(c.In, err)
			return
		}
		if d["type"] == "d" {
			close(c.Out)
		} else {
			for m := range dc {
				if fs.Verb {
					fs.Dprintf("-> [%d]bytes\n", len(m))
				}
				if ok := c.Out <- m; !ok {
					err := cerror(c.Out)
					close(dc, err)
					close(c.In, err)
					close(rc, err)
					return
				}
			}
			err := cerror(dc)
			if err != nil {
				fs.Dprintf("->%s\n", err)
			}
			close(c.Out, err)
			if err != nil {
				close(c.In, err)
				close(rc, err)
				return
			}
		}
		x := <-c.In
		err := cerror(c.In)
		if err == nil {
			if d, ok := x.(zx.Dir); ok {
				fs.Dprintf("<-%s\n", ddir(d))
				rc <- d
			} else {
				err = ErrBadMsg
			}
		} else {
			fs.Dprintf("<-%s\n", err)
		}
		close(c.In, err)
		close(rc, err)
	}()
	return rc
}

func (fs *Fs) Find(p, fpred, spref, dpref string, depth0 int) <-chan zx.Dir {
	rc := make(chan zx.Dir)
	go func() {
		m := &Msg{Op: Tfind, Fsys: fs.fsys, Path: p,
			Pred: fpred, Spref: spref, Dpref: dpref, Depth: depth0,
		}
		c := fs.m.Rpc()
		fs.Dprintf("->%s\n", m)
		if ok := c.Out <- m; !ok {
			err := cerror(c.Out)
			close(c.In, err)
			return
		}
		close(c.Out)
		for m := range c.In {
			if m, ok := m.(zx.Dir); !ok {
				err := ErrBadMsg
				close(c.In, err)
				close(rc, err)
				break
			} else {
				fs.Dprintf("<-%s\n", ddir(m))
				if ok := rc <- m; !ok {
					close(c.In, cerror(rc))
					break
				}
			}
		}
		err := cerror(c.In)
		if err != nil {
			fs.Dprintf("<-%s\n", err)
		}
		close(rc, err)
	}()
	return rc
}

func (fs *Fs) FindGet(p, fpred, spref, dpref string, depth0 int) <-chan face{} {
	rc := make(chan face{})
	go func() {
		m := &Msg{Op: Tfindget, Fsys: fs.fsys, Path: p,
			Pred: fpred, Spref: spref, Dpref: dpref, Depth: depth0,
		}
		c := fs.m.Rpc()
		fs.Dprintf("->%s\n", m)
		if ok := c.Out <- m; !ok {
			err := cerror(c.Out)
			close(c.In, err)
			return
		}
		close(c.Out)
		for m := range c.In {
			if ok := rc <- m; !ok {
				close(c.In, cerror(rc))
				break
			}
		}
		close(rc, cerror(c.In))
	}()
	return rc
}
