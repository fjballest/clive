package main

import (
	"clive/cmd"
	"fmt"
	"strings"
	"strconv"
	"errors"
	"os"
	"io"
)

struct xFd {
	fd *os.File
	io, path string
}

struct pFd {
	r, w *os.File
}

// Execution environment for nodes.
// We use unix processes to run clive commands, so for now we
// use actual file descriptors for cmd IO.
// The IO environment is named in clive, 0, 1, 2 are "in", "out", "err",
// other names can be passed using environment variables that map
// the name to the unix file descriptor.
struct xEnv {
	fds []xFd
	closes []io.Closer
}

func newEnv() *xEnv {
	return &xEnv{
		fds: []xFd{
			xFd{fd: os.Stdin, io: "in", path: "in"},
			xFd{fd: os.Stdout, io: "out", path: "out"},
			xFd{fd: os.Stderr, io: "err", path: "err"},
		},
	}
}

func (x *xEnv) dup() *xEnv {
	ne := &xEnv{
		fds: make([]xFd, len(x.fds)),
	}
	for i := range x.fds {
		ne.fds[i] = x.fds[i]
	}
	return ne
}

// run a top-level command
func (nd *Nd) run() error {
	if nd == nil {
		return nil
	}
	nprintf("cmd:\n%s\n", dnd{nd})
	defer nprintf("cmd done\n")
	if dry || yylex.nerrors > 0 {
		cmd.Dprintf("cmd not run, errors\n")
		yylex.nerrors = 0
		return nil
	}
	x := newEnv()
	// bgpipe or func
	switch nd.typ {
	case Npipe:
		return nd.runPipe(x)
	case Nsrc:
		return nd.runSrc(x)
	case Nfunc:
		return nd.runFunc(x)
	default:
		panic(fmt.Errorf("run: bad type %s", nd.typ))
	}
	return nil
}

func (nd *Nd) chk(k ...NdType) {
	if nd == nil {
		panic("chk: nil node")
	}
	for i := 0; i < len(k); i++ {
		if k[i] == nd.typ {
			cmd.Dprintf("chk %s...\n", nd.typ)
			return
		}
	}
	panic(fmt.Errorf("not %v; type %v", k, nd.typ))
}

func (nd *Nd) runSrc(x *xEnv) error {
	nd.chk(Nsrc)
	return nil
}

func (nd *Nd) runFunc(x *xEnv) error {
	nd.chk(Nfunc)
	return nil
}

// make xEnvs for pipe children
func (nd *Nd) mkChildEnvs(x *xEnv) (cxs []*xEnv, pcloses []*os.File, err error) {
	nc := len(nd.Child)
	cxs = make([]*xEnv, nc)
	pipes := map[string]pFd{}
	defer func() {
		if err != nil {
			for _, fd := range pcloses {
				fd.Close()
			}
			pcloses = nil
			return
		}
	}()
	for i, c := range nd.Child {
		cx := x.dup()
		cxs[i] = cx
		for _, r := range c.Redirs {
			paths, err := r.Child[0].expand1(x)
			if err != nil {
				cmd.Warn("expand: %s", err)
				return nil, pcloses, err
			}
			path := paths[0]
			kind, cname := r.Args[0], r.Args[1]
			var fd xFd
			var osfd *os.File
			// TODO: Use zx to rely files
			//	pro: we can avoid using sshfs
			//	con: we'd read the file before the command reads it.
			//	just use a pipe and be careful not to die because of
			//	writes in closed pipes.
			switch kind {
			case "<":
				osfd, err = os.Open(path)
				if err != nil {
					cmd.Warn("redir: %s", err)
					return nil, pcloses, err
				}
			case ">":
				osfd, err = os.Create(path)
				if err != nil {
					cmd.Warn("redir: %s", err)
					return nil, pcloses, err
				}
			case ">>":
				osfd, err = os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
				if err != nil {
					cmd.Warn("redir: %s", err)
					return nil, pcloses, err
				}
			case "<|", ">|":
				p, ok := pipes[path]
				if !ok {
					p.r, p.w, err = os.Pipe()
					if err != nil {
						cmd.Warn("pipe: %s", err)
						return nil, pcloses, err
					}
					pipes[path] = p
				}
				if kind[0] == '>' {
					osfd = p.w
				} else {
					osfd = p.r
				}
			}
			fd = xFd{osfd, cname, path}
			cx.fds = append(cx.fds, fd)
			cx.closes = append(cx.closes, fd.fd)
			pcloses = append(pcloses, fd.fd)
		}
	}
	return cxs, pcloses, nil
}

// children may be cmd, block, for, while, cond, set
func (nd *Nd) runPipe(x *xEnv) error {
	nd.chk(Npipe)
	cxs, pcloses, err := nd.mkChildEnvs(x)
	if err != nil {
		return err
	}
	for i, c := range nd.Child {
		switch c.typ {
		case Ncmd:
			err = c.runCmd(cxs[i])
		case Nblock:
			err = c.runBlock(cxs[i])
		case Nfor:
			err = c.runFor(cxs[i])
		case Nwhile:
			err = c.runWhile(cxs[i])
		case Ncond:
			err = c.runCond(cxs[i])
		case Nset:
			err = c.runSet(cxs[i])
		case Nsetmap:
			err = c.runSet(cxs[i])
		default:
			panic(fmt.Errorf("run: bad pipe child type %s", c.typ))
		}
		if err != nil {
			break
		}
	}
	for _, fd := range pcloses {
		fd.Close()
	}
	return err
}

func (nd *Nd) varLen() (int, error) {
	nd.chk(Nlen)
	if len(nd.Args) != 1 {
		panic("bad Nlen arg list")
	}
	// XXX: take a look to $nd.Args[0] and see if it's a list or a map or what
	sz := 1
	return sz, nil
}

func (nd *Nd) varValue(x *xEnv) (names []string) {
	nd.chk(Nval, Nsingle)
	if len(nd.Args) != 1 {
		panic("bad var node args")
	}
	switch len(nd.Child) {
	case 0:	// $a
		// XXX: get $a names
		names = []string{"$" + nd.Args[0]}
	case 1:	// $a[b]
		c := nd.Child[0]
		names, err := c.expand1(x)
		if err != nil {
			cmd.Warn("expand: %s", err)
			break
		}
		if len(names) != 1 {
			cmd.Warn("$%s[...]: not a single index name", nd.Args[0])
			break
		}
		// XXX: get $a names
		// XXX: get element with index c.Args[0] 
		names = []string{"$" + nd.Args[0] + "[" + names[0] + "]" }
	default:
		panic("bad Nvar children list")
	}
	if nd.typ == Nsingle {
		name := strings.Join(names, " ")
		names = []string{name}
	}
	return names
}

func (nd *Nd) appNames(x *xEnv) (names []string) {
	nd.chk(Napp)
	if len(nd.Child) != 2 {
		panic("bad app node children")
	}
	left, err := nd.Child[0].expand(x)
	if err != nil {
		cmd.Warn("expand: append: %s", err)
		return nil
	}
	right, err := nd.Child[1].expand(x)
	if err != nil {
		cmd.Warn("expand: append: %s", err)
		return nil
	}
	if len(left) == 0 {
		return right
	}
	if len(right) == 0 {
		return left
	}
	if len(left) == 1 {
		for i := 0; i < len(right); i++ {
			right[i] = left[0] + right[i]
		}
		return right
	}
	if len(right) == 1 {
		for i := 0; i < len(left); i++ {
			left[i] += right[0]
		}
		return left
	}
	if len(left) != len(right) {
		cmd.Warn("expand: different list lengths")
		return nil
	}
	for i := 0; i < len(left); i++ {
		left[i] += right[i]
	}
	return left
}

func (nd *Nd) expandIO(x *xEnv) ([]string, error) {
	nd.chk(Nioblk)
	// Either <{...} or <[names]{....} or >[name]{....}
	// The children is a block, including perhaps redirs.
	if len(nd.Args) == 0 || len(nd.Args) > 2 {
		panic("bad ioblk arg list")
	}
	if len(nd.Args) == 1 {
		// XXX: run and read all the output and
		// then collect the names
		nd.runBlock(x) // but for i/o
		return nil, fmt.Errorf("<{} not yet implemented")
	}
	switch nd.Args[0] {
	case ">":
		// XXX start the cmd setting up an out chan into it
		// and return its name
		
		nd.runBlock(x) // but for i/o
		return nil, fmt.Errorf("<{} not yet implemented")
	case "<":
		// XXX start the cmd setting up an in chan from it
		// and return its name
		nd.runBlock(x) // but for i/o
		return nil, fmt.Errorf(">{} not yet implemented")
	default:
		panic("bad ioblk arg")
	}

}

func (nd *Nd) expand1(x *xEnv) (nargs []string, err error) {
	nd.chk(Nname, Napp, Nlen, Nval, Nsingle, Nioblk)
	switch nd.typ {
	case Nname:
		nargs = nd.Args
	case Napp:
		nargs = nd.appNames(x)
	case Nlen:
		n, err := nd.varLen()
		if err != nil {
			return nil, err
		}
		nargs = []string{strconv.Itoa(n)}
	case Nval, Nsingle:
		nargs = nd.varValue(x)
	case Nioblk:
		nargs, err = nd.expandIO(x)
	default:
		panic(fmt.Errorf("expand1: bad names child type %s", nd.typ))
	}
	return nargs, err
}

// expand names: children can be name, app, len, single, val, ioblnk
func (nd *Nd) expand(x *xEnv) ([]string, error) {
	nd.chk(Nnames)
	xs := []string{}
	for _, c := range nd.Child {
		nargs, err := c.expand1(x)
		if err != nil {
			return nil, err
		}
		xs = append(xs, nargs...)
	}
	nprintf("expanded: %v\n", xs)
	return xs, nil
}

func (nd *Nd) runCmd(x *xEnv) error {
	nd.chk(Ncmd)
	if len(nd.Child) != 1 {
		panic("bad Ncmd children")
	}
	args, err := nd.Child[0].expand(x)
	if err != nil {
		cmd.Warn("expand: %s", err)
		return err
	}
	cmd.VWarn("run: %s", dnames(args))
	return nil
}

// block cmds are pipes or sources
func (nd *Nd) runBlock(x *xEnv) error {
	nd.chk(Nblock, Nioblk)
	if len(nd.Child) < 1 {
		panic("bad block children")
	}
	for _, c := range nd.Child {
		var err error
		switch c.typ {
		case Npipe:
			err = c.runPipe(x)
		case Nsrc:
			err = c.runSrc(x)
		default:
			panic(fmt.Errorf("runblock: bad child type %s", c.typ))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (nd *Nd) runFor(x *xEnv) error {
	nd.chk(Nfor)
	if len(nd.Child) != 2 {
		panic("bad for children")
	}
	c0, blk := nd.Child[0], nd.Child[1]
	names, err := c0.expand(x)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		cmd.Warn("missing for variable name")
		return fmt.Errorf("no variable name")
	}
	name := names[0]
	values := names[1:]
	if len(values) == 0 {
		// XXX: collect names from the input
	}
	for _, v := range values {
		// XXX: set variable $name to $v
		_, _ = name, v
		err = blk.runBlock(x)
	}
	return err
}

func (nd *Nd) runWhile(x *xEnv) error {
	nd.chk(Nwhile)
	if len(nd.Child) != 2 {
		panic("bad for children")
	}
	pipe, blk := nd.Child[0], nd.Child[1]
	// XXX: for now we run the block once
	i := 0
	for {
		if err := pipe.runPipe(x); err != nil {
			return err
		}
		// XXX: if status is not ok
		if i++; i > 1 {
			break
		}
		break
		if err := blk.runBlock(x); err != nil {
			return err
		}
	}
	return nil
}

var orSuccess = errors.New("or sucessful")

// like a block w/o redirs
// As soon as a child is not sucessful, we stop and return nil
// if the last child does run, we must return orSuccess
// so runCond() knows it has to stop
func (nd *Nd) runOr(x *xEnv) error {
	nd.chk(Nor)
	if len(nd.Child) == 0 {
		panic("bad or children")
	}
	for i, c := range nd.Child {
		var err error
		switch c.typ {
		case Npipe:
			err = c.runPipe(x)
		case Nsrc:
			err = c.runSrc(x)
		default:
			panic(fmt.Errorf("runor: bad child type %s", c.typ))
		}
		if err != nil {
			return err
		}
		if i < len(nd.Child)-1 {
			// XXX: if sts is failure: return nil
		}
	}
	return orSuccess

}

// children are or nodes that are like blocks (w/o redirs)
// and the nd has a final redir child
func (nd *Nd) runCond(x *xEnv) error {
	nd.chk(Ncond)
	if len(nd.Child) == 0 {
		// at least an or
		panic("bad cond children")
	}
	for _, or1 := range nd.Child {
		if err := or1.runOr(x); err != nil {
			if err == orSuccess {
				err = nil
			}
			return err
		}
	}
	return nil
}

func (nd *Nd) runSet(x *xEnv) error {
	nd.chk(Nset, Nsetmap)
	return nil
}
