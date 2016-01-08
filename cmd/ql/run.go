package main

import (
	"clive/cmd"
	"fmt"
	"strings"
	"strconv"
	"errors"
)

// run a top-level command
func (nd *Nd) run() error {
	if nd == nil {
		return nil
	}
	nprintf("cmd:\n%s\n", dnd{nd})
	if dry || yylex.nerrors > 0 {
		cmd.Dprintf("cmd not run, errors\n")
		yylex.nerrors = 0
		return nil
	}
	// bgpipe or func
	switch nd.typ {
	case Npipe:
		return nd.runPipe()
	case Nsrc:
		return nd.runSrc()
	case Nfunc:
		return nd.runFunc()
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
			cmd.Dprintf("run %s...\n", nd.typ)
			return
		}
	}
	panic(fmt.Errorf("not %v; type %v", k, nd.typ))
}

func (nd *Nd) runSrc() error {
	nd.chk(Nsrc)
	return nil
}

func (nd *Nd) runFunc() error {
	nd.chk(Nfunc)
	return nil
}

// children may be cmd, block, for, while, cond, set
func (nd *Nd) runPipe() error {
	nd.chk(Npipe)
	var err error
	for _, c := range nd.Child {
		switch c.typ {
		case Ncmd:
			err = c.runCmd()
		case Nblock:
			err = c.runBlock()
		case Nfor:
			err = c.runFor()
		case Nwhile:
			err = c.runWhile()
		case Ncond:
			err = c.runCond()
		case Nset:
			err = c.runSet()
		case Nsetmap:
			err = c.runSet()
		default:
			panic(fmt.Errorf("run: bad pipe child type %s", c.typ))
		}
		if err != nil {
			break
		}
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

func (nd *Nd) varValue() (names []string) {
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
		names, err := c.expand1()
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

func (nd *Nd) appNames() (names []string) {
	nd.chk(Napp)
	if len(nd.Child) != 2 {
		panic("bad app node children")
	}
	left, right := nd.Child[0], nd.Child[1]
	left, err := left.expand()
	if err != nil {
		cmd.Warn("expand: append: %s", err)
		return nil
	}
	right, err = right.expand()
	if err != nil {
		cmd.Warn("expand: append: %s", err)
		return nil
	}
	if len(left.Args) == 0 {
		return right.Args
	}
	if len(right.Args) == 0 {
		return left.Args
	}
	if len(left.Args) == 1 {
		for i := 0; i < len(right.Args); i++ {
			right.Args[i] = left.Args[0] + right.Args[i]
		}
		return right.Args
	}
	if len(right.Args) == 1 {
		for i := 0; i < len(left.Args); i++ {
			left.Args[i] += right.Args[0]
		}
		return left.Args
	}
	if len(left.Args) != len(right.Args) {
		cmd.Warn("expand: different list lengths")
		return nil
	}
	for i := 0; i < len(left.Args); i++ {
		left.Args[i] += right.Args[i]
	}
	return left.Args
}

func (nd *Nd) expandIO() ([]string, error) {
	nd.chk(Nioblk)
	// Either <{...} or <[names]{....} or >[name]{....}
	// The children is a block, including perhaps redirs.
	if len(nd.Args) == 0 || len(nd.Args) > 2 {
		panic("bad ioblk arg list")
	}
	if len(nd.Args) == 1 {
		// XXX: run and read all the output and
		// then collect the names
		nd.runBlock() // but for i/o
		return nil, fmt.Errorf("<{} not yet implemented")
	}
	switch nd.Args[0] {
	case ">":
		// XXX start the cmd setting up an out chan into it
		// and return its name
		
		nd.runBlock() // but for i/o
		return nil, fmt.Errorf("<{} not yet implemented")
	case "<":
		// XXX start the cmd setting up an in chan from it
		// and return its name
		nd.runBlock() // but for i/o
		return nil, fmt.Errorf(">{} not yet implemented")
	default:
		panic("bad ioblk arg")
	}

}

func (nd *Nd) expand1() (nargs []string, err error) {
	nd.chk(Nname, Napp, Nlen, Nval, Nsingle, Nioblk)
	switch nd.typ {
	case Nname:
		nargs = nd.Args
	case Napp:
		nargs = nd.appNames()
	case Nlen:
		n, err := nd.varLen()
		if err != nil {
			return nil, err
		}
		nargs = []string{strconv.Itoa(n)}
	case Nval, Nsingle:
		nargs = nd.varValue()
	case Nioblk:
		nargs, err = nd.expandIO()
	default:
		panic(fmt.Errorf("expand1: bad names child type %s", nd.typ))
	}
	return nargs, err
}

// expand names: children can be name, app, len, single, val, ioblnk
func (nd *Nd) expand() (*Nd, error) {
	nd.chk(Nnames)
	xnd := *nd
	xnd.Child = nil
	for _, c := range nd.Child {
		nargs, err := c.expand1()
		if err != nil {
			return nil, err
		}
		xnd.Args = append(xnd.Args, nargs...)
	}
	nprintf("expanded: %v\n", xnd.Args)
	return &xnd, nil
}

func (nd *Nd) runCmd() error {
	nd.chk(Ncmd)
	if len(nd.Child) != 2 {
		panic("bad Ncmd children")
	}
	names, redirs := nd.Child[0], nd.Child[1]
	names, err := names.expand()
	if err != nil {
		cmd.Warn("expand: %s", err)
		return err
	}
	args := names.Args
	cmd.VWarn("run: %s", dnames(args))
	_ = redirs
	return nil
}

// block cmds are pipes or sources, and there's at least one cmd
// and a final redirs children.
func (nd *Nd) runBlock() error {
	nd.chk(Nblock, Nioblk)
	if len(nd.Child) < 2 {
		panic("bad block children")
	}
	cmds, redirs := nd.Child[:len(nd.Child)-1], nd.Child[len(nd.Child)-1]
	_ = redirs
	for _, c := range cmds {
		var err error
		switch c.typ {
		case Npipe:
			err = c.runPipe()
		case Nsrc:
			err = c.runSrc()
		default:
			panic(fmt.Errorf("runblock: bad child type %s", c.typ))
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// 
func (nd *Nd) runFor() error {
	nd.chk(Nfor)
	if len(nd.Child) != 3 {
		panic("bad for children")
	}
	names, blk, redirs := nd.Child[0], nd.Child[1], nd.Child[2]
	_ = redirs
	names, err := names.expand()
	if err != nil {
		return err
	}
	if len(names.Args) == 0 {
		cmd.Warn("missing for variable name")
		return fmt.Errorf("no variable name")
	}
	name := names.Args[0]
	values := names.Args[1:]
	if len(values) == 0 {
		// XXX: collect names from the input
	}
	for _, v := range values {
		// XXX: set variable $name to $v
		_, _ = name, v
		err = blk.runBlock()
	}
	return err
}

func (nd *Nd) runWhile() error {
	nd.chk(Nwhile)
	if len(nd.Child) != 3 {
		panic("bad for children")
	}
	pipe, blk, redirs := nd.Child[0], nd.Child[1], nd.Child[2]
	_ = redirs
	// XXX: for now we run the block once
	i := 0
	for {
		if err := pipe.runPipe(); err != nil {
			return err
		}
		// XXX: if status is not ok
		if i++; i > 1 {
			break
		}
		break
		if err := blk.runBlock(); err != nil {
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
func (nd *Nd) runOr() error {
	nd.chk(Nor)
	if len(nd.Child) == 0 {
		panic("bad or children")
	}
	for i, c := range nd.Child {
		var err error
		switch c.typ {
		case Npipe:
			err = c.runPipe()
		case Nsrc:
			err = c.runSrc()
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
func (nd *Nd) runCond() error {
	nd.chk(Ncond)
	if len(nd.Child) < 2 {
		// at least an or and a redir
		panic("bad cond children")
	}
	ors := nd.Child[:len(nd.Child)-1]
	redirs := nd.Child[len(nd.Child)-1]
	_ = redirs
	for _, or1 := range ors {
		if err := or1.runOr(); err != nil {
			if err == orSuccess {
				err = nil
			}
			return err
		}
	}
	return nil
}

func (nd *Nd) runSet() error {
	nd.chk(Nset, Nsetmap)
	return nil
}
