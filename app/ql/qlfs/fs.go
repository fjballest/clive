/*
	Ql as a file system

	ql/
		env0/	environment
			vars/
				var (contents are $var)
				ns  (textual form for the ns)
				dot (cwd)
			cmd0/	one command
				cmd	script, perhaps just a single command
				in	// stdin
				out	// stdout as of now
				err	// stdout as of now
				pout	// streaming stdout
				perr	// streaming stderr
				sts	// current command status
				wait	// (wait for exit and get) exit status
				sig	// write to post sigs to cmd
			cmd1/
				...
		env1/...

	Each top level dir created is a new environment created as
	a set of name space, environment vars, dot, etc.
	Commands are created as new dirs within a top-level dir. They

		- inherit a copy of env's dot
		- inherit a copy of env's env (vars)
		- start with a new IO set
		- share env's name space

	Writing text into the "cmd" file runs that command using "ql -c cmd".
	Be careful to issue a single write when using FUSE.

	Remove the directory for a command once you are done with it.

	Remove a top-level directory to get dir of that environment and all the
	commands in it.
*/
package qlfs

import (
	"clive/app"
	"clive/dbg"
	"clive/nchan"
	"clive/zx"
	"clive/zx/vfs"
	"fmt"
	"strconv"
	"sync"
)

// Ql commands served as a ZX tree.
type Fs struct {
	*vfs.Fs // Implementor of ZX fs ops.
	root    qRoot
}

type qRoot struct {
	sync.Mutex
	envs map[string]*qEnv
}

// One per environment
type qEnv struct {
	name string
	vars map[string]string
	cmds map[string]*qCmd
	runc chan func()
	sync.Mutex
}

type qVars struct {
	e *qEnv
}

type qVar struct {
	name string
	e    *qEnv
}

type qIO struct {
	msgs []interface{}
	n    int // size in bytes
	c    chan interface{}
	wc   []chan bool
	eof  bool
}

// One per command
type qCmd struct {
	name, txt    string
	e            *qEnv
	in, out, err *qIO
	sync.Mutex
	ctx *app.Ctx
}

type qId int

const (
	qccmd qId = iota
	qcin
	qcout
	qcerr
	qcpout
	qcperr
	qcsts
	qcwait
	qcsig
)

type qGen struct {
	id         qId
	name, mode string
}

type qFile struct {
	qGen
	c *qCmd
}

var (
	cFiles = map[string]qGen{
		"cmd":  qGen{qccmd, "cmd", "0644"},
		"in":   qGen{qcin, "in", "0660"},
		"out":  qGen{qcout, "out", "0440"},
		"err":  qGen{qcerr, "err", "0440"},
		"pout": qGen{qcpout, "pout", "0440"},
		"perr": qGen{qcperr, "perr", "0440"},
		"sts":  qGen{qcsts, "sts", "0440"},
		"sig":  qGen{qcsig, "sig", "0220"},
		"wait": qGen{qcwait, "wait", "0440"},
	}

	// make sure we implement the right interfaces
	_fs  *Fs
	_t   zx.RWTree   = _fs
	_r   zx.Recver   = _fs
	_snd zx.Sender   = _fs
	_g   zx.Getter   = _fs
	_w   zx.Walker   = _fs
	_s   zx.Stater   = _fs
	_a   zx.AuthTree = _fs
	_c   zx.IsCtler  = _fs
)

// Tell fuse that we are virtual for all files
func (r *qRoot) IsCtl() bool {
	return true
}

func New(name string) (*Fs, error) {
	t := &Fs{
		root: qRoot{envs: map[string]*qEnv{}},
	}
	var err error
	t.Fs, err = vfs.New(name, &t.root)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (r *qRoot) String() string {
	return "/"
}

func (r *qRoot) Stat() (zx.Dir, error) {
	r.Lock()
	n := len(r.envs)
	r.Unlock()
	return zx.Dir{
		"name": "/",
		"type": "d",
		"mode": "0755",
		"size": strconv.Itoa(n + 1), // +1 for /Ctl in vfs
	}, nil
}

func (r *qRoot) Walk(elem string) (vfs.File, error) {
	r.Lock()
	defer r.Unlock()
	e := r.envs[elem]
	if e != nil {
		return e, nil
	}
	return nil, fmt.Errorf("/: %s: %s", elem, dbg.ErrNotExist)
}

func (r *qRoot) Getdir() ([]string, error) {
	r.Lock()
	defer r.Unlock()
	var ns []string
	for e := range r.envs {
		ns = append(ns, e)
	}
	return ns, nil
}

func (r *qRoot) Mkdir(name string, d zx.Dir) error {
	r.Lock()
	defer r.Unlock()
	if r.envs[name] != nil {
		return fmt.Errorf("/: %s: %s", name, dbg.ErrExists)
	}
	ne := newEnv(name)
	r.envs[name] = ne
	app.Dprintf("new env %s: %d vars\n", ne, len(ne.vars))
	return nil
}

func (r *qRoot) Remove(chld vfs.File, name string, all bool) error {
	r.Lock()
	defer r.Unlock()
	re := r.envs[name]
	if re == nil {
		return fmt.Errorf("/: %s: %s", name, dbg.ErrNotExist)
	}
	delete(r.envs, name)
	go re.removed()
	return nil
}

func (e *qEnv) String() string {
	return "/" + e.name
}

func (e *qEnv) Stat() (zx.Dir, error) {
	e.Lock()
	n := len(e.cmds)
	e.Unlock()
	return zx.Dir{
		"name": e.name,
		"type": "d",
		"mode": "0755",
		"size": strconv.Itoa(n + 1), // +1 for vars
	}, nil
}

func (e *qEnv) Walk(elem string) (vfs.File, error) {
	if elem == "vars" {
		return &qVars{e: e}, nil
	}
	e.Lock()
	defer e.Unlock()
	c := e.cmds[elem]
	if c != nil {
		return c, nil
	}
	return nil, fmt.Errorf("%s: %s: %s", e, elem, dbg.ErrNotExist)
}

func (e *qEnv) Getdir() ([]string, error) {
	ns := []string{"vars"}
	e.Lock()
	defer e.Unlock()
	for c := range e.cmds {
		ns = append(ns, c)
	}
	return ns, nil
}

func (e *qEnv) Remove(chld vfs.File, name string, all bool) error {
	if name == "vars" {
		return fmt.Errorf("%s: vars: %s", e, dbg.ErrPerm)
	}
	e.Lock()
	defer e.Unlock()
	rc := e.cmds[name]
	if rc == nil {
		return fmt.Errorf("%s: %s: %s", e, name, dbg.ErrNotExist)
	}
	delete(e.cmds, name)
	go rc.removed()
	return nil
}

func (e *qEnv) Mkdir(name string, d zx.Dir) error {
	if name == "vars" {
		return fmt.Errorf("%s: vars: %s", e, dbg.ErrExists)
	}
	return e.newCmd(name)
}

func (vs *qVars) String() string {
	return fmt.Sprintf("/%s/vars", vs.e.name)
}

func (vs *qVars) Stat() (zx.Dir, error) {
	vs.e.Lock()
	n := len(vs.e.vars)
	vs.e.Unlock()
	return zx.Dir{
		"name": "vars",
		"type": "d",
		"mode": "0755",
		"size": strconv.Itoa(n),
	}, nil
}

func (vs *qVars) Walk(elem string) (vfs.File, error) {
	vs.e.Lock()
	defer vs.e.Unlock()
	_, ok := vs.e.vars[elem]
	if !ok {
		return nil, fmt.Errorf("%s: %s: %s", vs, elem, dbg.ErrNotExist)
	}
	return &qVar{elem, vs.e}, nil
}

func (vs *qVars) Getdir() ([]string, error) {
	ns := []string{}
	vs.e.Lock()
	defer vs.e.Unlock()
	for k := range vs.e.vars {
		ns = append(ns, k)
	}
	return ns, nil
}

func (e *qEnv) setEnv(n, v string) {
	e.runc <- func() {
		app.SetEnv(n, v)
		if n == "ns" {
			app.NewNS(nil)
		} else if n == "dot" {
			app.NewDot(v)
		}
	}
}

func (vs *qVars) Remove(chld vfs.File, name string, all bool) error {
	vs.e.Lock()
	defer vs.e.Unlock()
	_, ok := vs.e.vars[name]
	if !ok {
		return fmt.Errorf("%s: %s: %s", vs, name, dbg.ErrNotExist)
	}
	delete(vs.e.vars, name)
	vs.e.setEnv(name, "")
	return nil
}

func (vs *qVars) Put(name string, d zx.Dir, off int64, c <-chan []byte) error {
	if name == "" {
		return fmt.Errorf("%s: %s", vs, dbg.ErrIsDir)
	}
	vs.e.Lock()
	defer vs.e.Unlock()
	v, err := nchan.String(c)
	if err != nil {
		return err
	}
	vs.e.vars[name] = v
	vs.e.setEnv(name, v)
	return nil
}

func (v *qVar) String() string {
	return fmt.Sprintf("/%s/vars/%s", v.e.name, v.name)
}

func (v *qVar) Stat() (zx.Dir, error) {
	v.e.Lock()
	n := len(v.e.vars[v.name])
	v.e.Unlock()
	return zx.Dir{
		"name": v.name,
		"type": "-",
		"mode": "0644",
		"size": strconv.Itoa(n),
	}, nil
}

func (v *qVar) Put(name string, d zx.Dir, off int64, c <-chan []byte) error {
	v.e.Lock()
	defer v.e.Unlock()
	val, err := nchan.String(c)
	if err != nil {
		return err
	}
	v.e.vars[v.name] = val
	v.e.setEnv(v.name, val)
	return nil
}

func (v *qVar) Get(off, count int64, c chan<- []byte) error {
	v.e.Lock()
	dat := []byte(v.e.vars[v.name])
	v.e.Unlock()
	return vfs.GetBytes(dat, off, count, c)
}

func (c *qCmd) String() string {
	return fmt.Sprintf("/%s/%s", c.e.name, c.name)
}

func (c *qCmd) Stat() (zx.Dir, error) {
	return zx.Dir{
		"name": c.name,
		"type": "d",
		"mode": "0755",
		"size": "11",
	}, nil
}

func (c *qCmd) Walk(elem string) (vfs.File, error) {
	c.Lock()
	defer c.Unlock()
	f, ok := cFiles[elem]
	if !ok {
		return nil, fmt.Errorf("%s: %s: %s", c, elem, dbg.ErrNotExist)
	}
	return &qFile{f, c}, nil
}

func (c *qCmd) Getdir() ([]string, error) {
	ns := []string{}
	for k := range cFiles {
		ns = append(ns, k)
	}
	return ns, nil
}

func (f *qFile) String() string {
	return fmt.Sprintf("/%s/%s/%s", f.c.e.name, f.c.name, f.name)
}

func (io *qIO) Len() int {
	if io == nil {
		return 0
	}
	return io.n
}

func (f *qFile) Stat() (zx.Dir, error) {
	n := 0
	switch f.id {
	case qccmd:
		n = len(f.c.txt)
	case qcin:
		n = f.c.in.Len()
	case qcout, qcpout:
		n = f.c.out.Len()
	case qcerr, qcperr:
		n = f.c.err.Len()
	case qcsts:
		n = len(f.c.status())
	}
	d := zx.Dir{
		"name": f.name,
		"type": "-",
		"mode": f.mode,
		"size": strconv.Itoa(n),
	}
	return d, nil
}

func (f *qFile) Put(name string, d zx.Dir, off int64, c <-chan []byte) error {
	switch f.id {
	case qcout, qcerr, qcpout, qcperr, qcwait, qcsts:
		return fmt.Errorf("%s: %s", f, dbg.ErrPerm)
	case qcin:
		if d["mode"] != "" {
			f.c.clearIn() // ignored if already started
		}
		return f.c.putIn(off, c)
	case qccmd:
		s, err := nchan.String(c)
		if err != nil {
			return err
		}
		return f.c.start(s)
	case qcsig:
		s, err := nchan.String(c)
		if err != nil {
			return err
		}
		f.c.post(s)
		return nil
	}
	return fmt.Errorf("%s: %s", f, dbg.ErrBug)
}

func (f *qFile) Get(off, count int64, c chan<- []byte) error {
	switch f.id {
	case qcin:
		return f.c.getIn(off, count, c)
	case qccmd:
		return vfs.GetBytes([]byte(f.c.txt), off, count, c)
	case qcout:
		return f.c.getOut(off, count, c)
	case qcpout:
		return f.c.getPout(off, count, c)
	case qcerr:
		return f.c.getErr(off, count, c)
	case qcperr:
		return f.c.getPerr(off, count, c)
	case qcwait:
		f.c.wait()
		fallthrough
	case qcsts:
		sts := f.c.status()
		return vfs.GetBytes([]byte(sts), off, count, c)
	case qcsig:
		return nil
	}
	return fmt.Errorf("%s: %s", f, dbg.ErrBug)
}
