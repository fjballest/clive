package main

import (
	"bytes"
	"clive/cmd"
	"clive/dbg"
	"errors"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"fmt"
)

/*
	These are tools to execute Nd's once they are parsed.
	The main entry point is Nd.Exec()
	Nds to be executed get a NdExec attacched and then
	an xFOO method gets called to execute it.

	The convention is that before calling xFoo() we must mkexec for it, and
	then it must call redirs and call closeall before returning.

	xFoo() methods return before doing the work, doing it in the bg.
	the wait method may be called to wait for a node to execute.
*/

type NdIO  {
	Stdin          io.Reader
	Stdout, Stderr io.Writer
}

type NdExec  {
	Tag         string
	NdIO                      // default I/O for the node.
	x           *exec.Cmd     // for external commands
	waitc       chan error    // to wait for execution of a node
	intrc       chan bool     // closed to ask for command interruption
	closefds    []io.Closer   // to be closed after waiting for this.
	infd, outfd io.ReadCloser // aux used by xPipe
	extrafds	[]*os.File
	args        []string
}

type Bg  {
	seq  int
	Nds  map[*Nd]bool
	Tags map[string]int
	sync.Mutex
	waits map[string][]chan bool
}

var (
	bg = &Bg{
		Nds:   map[*Nd]bool{},
		Tags:  map[string]int{},
		waits: map[string][]chan bool{},
	}
	debugExec bool
	xprintf   = dbg.FlagPrintf(os.Stderr, &debugExec)

	intrlk      sync.Mutex
	intrc       chan bool // closed upon interrupts
	Interrupted bool      // set to true upon interrupts
	IntrExits   bool      // set to true when running scripts
	ErrIntr     = errors.New("interrupted")
)

func init() {
	intrlk.Lock()
	intrc = make(chan bool)
	intrlk.Unlock()
	dbg.AtIntr(func() bool {
		dprintf("*** SIGINT\n")
		intrlk.Lock()
		close(intrc)
		intrc = make(chan bool)
		Interrupted = true
		intrlk.Unlock()
		return !IntrExits
	})
}

func (bg *Bg) Add(nd *Nd) {
	bg.Lock()
	var tag string
	if len(nd.Args)>0 && len(nd.Args[0])>0 {
		tag = nd.Args[0]
	} else {
		bg.seq++
		tag = "%" + strconv.Itoa(bg.seq)
	}
	nd.Tag = tag
	xprintf("bg add %s %s\n", nd.Tag, nd)
	bg.Tags[tag] = bg.Tags[tag] + 1
	bg.Nds[nd] = true
	bg.Unlock()
}

func (bg *Bg) Del(nd *Nd, err error) {
	xprintf("bg del %s\n", nd)
	bg.Lock()
	defer bg.Unlock()
	bg.Tags[nd.Tag] = bg.Tags[nd.Tag] - 1
	if bg.Tags[nd.Tag] <= 0 {
		delete(bg.Tags, nd.Tag)
	}
	delete(bg.Nds, nd)
	ws := bg.waits[nd.Tag]
	delete(bg.waits, nd.Tag)
	if len(bg.Tags) == 0 {
		ws = append(ws, bg.waits[""]...)
		delete(bg.waits, "")
	}
	for _, w := range ws {
		xprintf("wait done\n")
		close(w, err)
	}
}

// the chan is closed when no async jobs remain
// the chan error is set to the command sts
func (bg *Bg) Wait(tag string) chan bool {
	c := make(chan bool)
	bg.Lock()
	defer bg.Unlock()
	if tag!="" && bg.Tags[tag]==0 {
		xprintf("wait done\n")
		close(c, errors.New("job exited or not found"))
	} else if len(bg.Nds) == 0 {
		xprintf("wait done\n")
		close(c, errors.New("no async job"))
	} else {
		bg.waits[tag] = append(bg.waits[tag], c)
	}
	return c
}

func (nd *Nd) mkExec(i io.Reader, o, e io.Writer, args ...string) {
	intrlk.Lock()
	nd.NdExec = &NdExec{
		NdIO:  NdIO{i, o, e},
		waitc: make(chan error, 1),
		intrc: intrc,
	}
	intrlk.Unlock()
	if (nd.Kind==Npipe || nd.Kind==Npipeblk) && len(nd.Args)>0 && len(nd.Args[0])>0 {
		fd, err := os.Open(os.DevNull)
		if err != nil {
			dbg.Fatal("can't open /dev/null: %s", err)
		}
		nd.Stdin = fd
		nd.closefds = append(nd.closefds, fd)
	}
	nd.args = args
}

func (nd *Nd) extraFds(parent *Nd) {
	if parent != nd && parent.NdExec != nil && len(parent.extrafds) > 0 {
		nd.extrafds = append(nd.extrafds, parent.extrafds...)
	}
}

func (nd *Nd) wait() error {
	return <-nd.waitc
}

func (nd *Nd) async() bool {
	return nd==nil || (nd.Kind==Npipe &&
		len(nd.Args)>0 && len(nd.Args[0])>0)
}

func setsts(err error) {
	if err == nil {
		os.Setenv("status", "")
	} else {
		os.Setenv("status", err.Error())
	}
}

// called from yacc, to execute a top-level node.
func (nd *Nd) Exec() error {
	if nd==nil || nd.Kind==Nnone || nd.Kind==Nnop {
		return nil
	}
	var err error
	switch nd.Kind {
	case Npipe:
		nd.mkExec(os.Stdin, os.Stdout, os.Stderr, Argv...)
		err = nd.xPipe()
		if err == nil {
			if nd.async() {
				bg.Add(nd)
				go func() {
					err := nd.wait()
					bg.Del(nd, err)
				}()
			} else {
				err = nd.wait()
			}
		}
	case Nset:
		nd.mkExec(os.Stdin, os.Stdout, os.Stderr, Argv...)
		nd.xSet()
		err = nd.wait()
	default:
		dbg.Warn("%s not yet implemented", nd.Kind)
		err = dbg.ErrBug
	}
	setsts(err)
	return err
}

func closeAll(fds ...io.Closer) {
	for _, fd := range fds {
		if fd != nil {
			fd.Close()
		}
	}
}

func (nd *Nd) closeAll() {
	if nd==nil || nd.NdExec==nil {
		return
	}
	nd.infd = nil
	nd.outfd = nil
	closeAll(nd.closefds...)
	nd.closefds = nil
}

// previous checks must make sure redirs make sense and do not conflict.
func (nd *Nd) redirs() error {
	var (
		xi     io.Reader
		xo, xe io.Writer
		xerr   error
	)
	for _, r := range nd.Redirs {
		switch r.Name {
		case "":
		case "=":
			if r.From == 1 {
				o := xe
				if o == nil {
					o = nd.Stderr
				}
				xo = o
			} else {
				o := xo
				if o == nil {
					o = nd.Stdout
				}
				xe = o
			}
		case "|":
			if r.From == 0 {
				xi = nd.infd
				break
			}
			rfd, wfd, err := os.Pipe()
			if err != nil {
				xerr = err
			}
			if r.From == 1 {
				xo = wfd
			} else {
				xe = wfd
			}
			nd.outfd = rfd
			nd.closefds = append(nd.closefds, wfd)
		default:
			if r.From == 0 {
				fd, err := os.Open(r.Name)
				if err != nil {
					xerr = err
					dbg.Warn("%s: %s", r.Name, err)
				}
				xi = fd
				nd.closefds = append(nd.closefds, fd)
				break
			}
			var fd *os.File
			var err error
			if r.App {
				flag := os.O_WRONLY | os.O_APPEND
				fd, err = os.OpenFile(r.Name, flag, 0664)
			} else {
				fd, err = os.Create(r.Name)
			}
			if err != nil {
				xerr = err
				dbg.Warn("%s: %s", r.Name, err)
			}
			if r.From == 1 {
				xo = fd
			} else {
				xe = fd
			}
			nd.closefds = append(nd.closefds, fd)
		}
	}
	if xi == nil {
		xi = nd.Stdin
	}
	if xo == nil {
		xo = nd.Stdout
	}
	if xe == nil {
		xe = nd.Stderr
	}
	nd.Stdin, nd.Stdout, nd.Stderr = xi, xo, xe
	if nd.interrupted() {
		xerr = errors.New("interrupted")
	}
	return xerr
}

func (nd *Nd) interrupted() bool {
	select {
	case <-nd.intrc:
		return true
	default:
		return false
	}
}

func (nd *Nd) xPipe() error {
	xprintf("x %s\n", nd)
	if len(nd.Child) == 0 {
		xprintf("x %s done\n", nd)
		nd.closeAll()
		nd.waitc <- nil
		return nil
	}
	for i := 0; i < len(nd.Child); i++ {
		switch nd.Child[i].Kind {
		case Nexec, Nif, Nfor, Nforblk, Ncmds, Nwhile:
		default:
			dbg.Fatal("bug: pipe child is not executable")
		}
	}
	// NB. you can't wait for an xpipe; don't use nd.waitc
	if err := nd.redirs(); err != nil {
		xprintf("x %s done\n", nd)
		nd.closeAll()
		nd.waitc <- err
		return err
	}
	go func() {
		var prev io.ReadCloser
		var err error
		for i, c := range nd.Child {
			if nd.interrupted() {
				err = errors.New("interrupted")
				nd.Child = nd.Child[:i]
				break
			}
			c.mkExec(nd.Stdin, nd.Stdout, nd.Stderr, nd.args...)
			c.extraFds(nd)
			if i > 0 {
				c.infd = prev // c.redirs() will use it.
			}
			switch c.Kind {
			case Nexec:
				err = c.xExec()
			case Nif:
				err = c.xIf()
			case Nfor:
				err = c.xFor()
			case Nwhile:
				err = c.xWhile()
			case Nforblk:
				err = c.xForBlk()
			case Ncmds:
				err = c.xCmds()
			}
			prev = c.outfd // c.redirs() will set it.
			if err != nil {
				nd.Child = nd.Child[:i]
				break
			}
		}
		for _, c := range nd.Child {
			xprintf("x wait for %s...\n", c)
			err = c.wait()
			xprintf("x wait for %s: sts %v\n", c, err)
			closeAll(c.infd, c.outfd)
		}
		xprintf("x %s done\n", nd)
		nd.closeAll()
		nd.waitc <- err
	}()
	return nil
}

// both for Ninblk and Nhereblk
func (nd *Nd) xInBlk() ([]string, error) {
	xprintf("x %s\n", nd)
	if len(nd.Child) == 0 {
		return []string{}, nil
	}
	if len(nd.Child)>1 || nd.Child[0].Kind!=Npipe {
		dbg.Fatal("inblk child bug; it assumes a single pipe in block")
		return []string{}, dbg.ErrBug
	}
	c := nd.Child[0]
	fd, w, _ := os.Pipe()
	c.mkExec(nd.Stdin, w, nd.Stderr, nd.args...)
	c.extraFds(nd)
	c.closefds = append(c.closefds, w)
	c.Args = nil // don't & it
	c.xPipe()
	var b bytes.Buffer
	io.Copy(&b, fd)
	fd.Close()
	xprintf("x %s done\n", nd)
	if nd.Kind == Nhereblk {
		return []string{b.String()}, nil
	}
	words := strings.Fields(b.String())
	return words, nil
}

// scan direct children of nd for $ and <{} expansion.
// All resulting children must be names.
func (nd *Nd) names() []string {
	if nd == nil {
		return nil
	}
	names := []string{}
	for _, c := range nd.Child {
		switch c.Kind {
		case Nname:
			names = append(names, c.Args...)
		case Ninblk, Nhereblk:
			if nd.interrupted() {
				break
			}
			// execute nd.child and read its output
			// then build a list of words from its output and
			// return a list of nodes, one for each word.
			c.mkExec(nd.Stdin, nd.Stdout, nd.Stderr, nd.args...)
			c.extraFds(nd)
			out, err := c.xInBlk()
			if err != nil {
				dbg.Warn("%s: %s", nd.Addr, err)
				break
			}
			names = append(names, out...)
		case Npipeblk:
			if nd.interrupted() {
				break
			}
			// execute nd.child with the output sent through a pipe
			// and supply the pipe file name as an argument.
			// then build a list of words from its output and
			// return a list of nodes, one for each word.
			rfd, wfd, err := os.Pipe()
			if err != nil {
				dbg.Warn("%s: %s", nd.Addr, err)
				break
			}
			c.mkExec(nd.Stdin, wfd, nd.Stderr, nd.args...)
			c.extraFds(nd)
			c.closefds = append(c.closefds, wfd)
			nd.closefds = append(nd.closefds, rfd)
			nm := fmt.Sprintf("/dev/fd/%d", 3 + len(nd.extrafds))
			nd.extrafds = append(nd.extrafds, rfd)
			err = c.xCmds()
			if err != nil {
				dbg.Warn("%s: %s", nd.Addr, err)
				rfd.Close()
				wfd.Close()
				break
			}
			names = append(names, nm)
		case Napp:
			if len(c.Child) != 2 {
				dbg.Fatal("bug: names: app child != 2")
			}
			old := c.Child
			defer func() {
				c.Child = old
			}()
			c.mkExec(nd.Stdin, nd.Stdout, nd.Stderr, nd.args...)
			c.extraFds(nd)
			c.Child = old[:1]
			w1 := c.names()
			c.Child = old[1:]
			w2 := c.names()
			if len(w1) == len(w2) {
				for i := 0; i < len(w1); i++ {
					names = append(names, w1[i]+w2[i])
				}
			} else if len(w1) == 1 {
				for i := 0; i < len(w2); i++ {
					names = append(names, w1[0]+w2[i])
				}
			} else if len(w2) == 1 {
				for i := 0; i < len(w1); i++ {
					names = append(names, w1[i]+w2[0])
				}
			} else {
				names = append(names, w1...)
				names = append(names, w2...)
			}
		case Nval, Nlen, Njoin:
			if len(c.Args) == 0 {
				break
			}
			i := ""
			if len(c.Args) > 1 {
				i = c.Args[1]
			}
			var words []string
			if c.Args[0]=="argv" && nd.NdExec!=nil {
				words = ArgVal(nd.args, i)
			} else {
				words = EnvVal(c.Args[0], i)
			}
			if c.Kind == Nlen {
				n := len(words)
				words = []string{strconv.Itoa(n)}
			} else if c.Kind==Njoin && len(words)>0 {
				words = []string{strings.Join(words, " ")}
			}
			names = append(names, words...)
		default:
			dbg.Fatal("names: wrong nd child kind")
		}
	}
	return names
}

func (nd *Nd) xExec() error {
	xprintf("x %s\n", nd)
	err := nd.redirs()
	if err != nil {
		xprintf("x %s done\n", nd)
		nd.closeAll()
		nd.waitc <- err
		return err
	}
	args := nd.names()
	if len(args) == 0 {
		xprintf("x %s done\n", nd)
		err = errors.New("no name for command")
		nd.closeAll()
		nd.waitc <- err
		return err
	}
	fname := LookCmd(args[0])
	name := args[0]

	fn := bltin[name]
	if fn != nil {
		ctx := cmd.Ctx{
			Stdin:  nd.Stdin,
			Stdout: nd.Stdout,
			Stderr: nd.Stderr,
			Intrc:  nd.intrc,
			Args:   args,
		}
		go func() {
			err := fn(ctx)
			if err != nil {
				ctx.Warn("%s", err)
			}
			nd.closeAll()
			xprintf("x %s done\n", nd)
			nd.waitc <- err
			xprintf("x %s waitc done\n", nd)
		}()
		return nil
	}

	fns := funcs[name]
	if fns != nil {
		bdy := fns.Child[0]
		bdy.mkExec(nd.Stdin, nd.Stdout, nd.Stderr, args[1:]...)
		err = bdy.xCmds()
		if err != nil {
			nd.closeAll()
			nd.waitc <- err
			return err
		}
		go func() {
			err := bdy.wait()
			nd.closeAll()
			xprintf("x %s done\n", nd)
			nd.waitc <- err
		}()
		return nil
	}

	if nd.interrupted() {
		err = errors.New("interrupted")
		nd.closeAll()
		xprintf("x %s done\n", nd)
		nd.waitc <- err
		return err
	}
	nd.x = exec.Command(fname, args[1:]...)
	nd.x.Stdin, nd.x.Stdout, nd.x.Stderr = nd.Stdin, nd.Stdout, nd.Stderr
	nd.x.ExtraFiles = append(nd.x.ExtraFiles, nd.extrafds...)
	err = nd.x.Start()
	if err != nil {
		dbg.Warn("%s: %s", name, err)
		xprintf("x %s done\n", nd)
		nd.closeAll()
		nd.waitc <- err
		return err
	}
	go func() {
		xprintf("x %s done\n", nd)
		errc := make(chan error, 1)
		go func() {
			errc <- nd.x.Wait()
		}()
		var err error
	Loop:
		for {
			select {
			case err = <-errc:
				break Loop
			case <-nd.intrc:
				err = errors.New("interrupted")
				proc := nd.x.Process
				if proc != nil {
					proc.Signal(os.Interrupt)
				} else {
					break Loop
				}
			}
		}
		nd.closeAll()
		nd.waitc <- err
	}()
	return nil
}

func (nd *Nd) xCmds() error {
	xprintf("x %s\n", nd)
	if err := nd.redirs(); err != nil {
		xprintf("x %s done\n", nd)
		nd.closeAll()
		nd.waitc <- err
		return err
	}
	go func() {
		var err error
		for _, c := range nd.Child {
			if nd.interrupted() {
				err = errors.New("interrupted")
				break
			}
			switch c.Kind {
			case Nnone, Nnop:
				continue
			case Npipe:
				c.mkExec(nd.Stdin, nd.Stdout, nd.Stderr, nd.args...)
				c.extraFds(nd)
				c.xPipe()
				err = nil
				if nd.async() {
					bg.Add(c)
					go func() {
						err := c.wait()
						bg.Del(c, err)
					}()
				} else {
					err = c.wait()
				}
				setsts(err)
			case Nset:
				c.mkExec(nd.Stdin, nd.Stdout, nd.Stderr, nd.args...)
				c.extraFds(nd)
				c.xSet()
				err = c.wait()
				setsts(err)
			default:
				dbg.Fatal("xcmds: child bug")
			}

		}
		xprintf("x %s done\n", nd)
		nd.closeAll()
		nd.waitc <- err
	}()
	return nil

}

func (nd *Nd) xForBlk() error {
	xprintf("x %s\n", nd)
	if err := nd.redirs(); err != nil {
		xprintf("x %s done\n", nd)
		nd.closeAll()
		nd.waitc <- err
		return err
	}
	go func() {
		var err error
		for _, c := range nd.Child {
			switch c.Kind {
			case Nnone, Nnop, Npipe:
				continue
			case Nset:
				err = errors.New("cannot set a variable in this block")
				xprintf("x %s done\n", nd)
				nd.closeAll()
				nd.waitc <- err
				return
			default:
				dbg.Fatal("xforblk: child bug")
			}
		}
		wchild := []*Nd{}
		tee := &Tee{In: nd.Stdin}
		for _, c := range nd.Child {
			if nd.interrupted() {
				err = errors.New("interrupted")
				break
			}
			switch c.Kind {
			case Nnone, Nnop:
				continue
			case Npipe:
				c.mkExec(tee.New(), nd.Stdout, nd.Stderr, nd.args...)
				c.extraFds(nd)
				c.xPipe()
				err = nil
				if nd.async() {
					bg.Add(c)
					go func() {
						err := c.wait()
						bg.Del(c, err)
					}()
				} else {
					wchild = append(wchild, c)
				}
			}
		}
		if xerr := tee.IO(); xerr != nil {
			xprintf("x %s: tee: %s\n", nd, xerr)
		}
		for _, wc := range wchild {
			err = wc.wait()
		}
		setsts(err)
		xprintf("x %s done\n", nd)
		nd.closeAll()
		nd.waitc <- err
	}()
	return nil

}

func (nd *Nd) xIf() error {
	xprintf("x %s\n", nd)
	if len(nd.Child)<2 || len(nd.Child)%2!=0 {
		dbg.Fatal("if bug")
	}
	if err := nd.redirs(); err != nil {
		xprintf("x %s done\n", nd)
		nd.closeAll()
		nd.waitc <- err
		return err
	}
	go func() {
		var err error
		for i := 0; i < len(nd.Child)-1; i += 2 {
			if nd.interrupted() {
				err = errors.New("interrupted")
				break
			}
			cond, body := nd.Child[i], nd.Child[i+1]
			if cond != nil {
				cond.mkExec(nd.Stdin, nd.Stdout, nd.Stderr, nd.args...)
				cond.extraFds(nd)
				err = cond.xPipe()
				if err == nil {
					err = cond.wait()
				}
				if err != nil {
					continue
				}
			}
			body.mkExec(nd.Stdin, nd.Stdout, nd.Stderr, nd.args...)
			body.extraFds(nd)
			err = body.xCmds()
			if err == nil {
				err = body.wait()
			}
			break
		}
		nd.closeAll()
		nd.waitc <- err
	}()
	return nil
}

func (nd *Nd) xWhile() error {
	xprintf("x %s\n", nd)
	if len(nd.Child) != 2 {
		dbg.Fatal("while bug")
	}
	if err := nd.redirs(); err != nil {
		xprintf("x %s done\n", nd)
		nd.closeAll()
		nd.waitc <- err
		return err
	}
	go func() {
		var err error
		cond, body := nd.Child[0], nd.Child[1]
		for {
			if nd.interrupted() {
				err = errors.New("interrupted")
				break
			}
			if cond != nil {
				cond.mkExec(nd.Stdin, nd.Stdout, nd.Stderr, nd.args...)
				cond.extraFds(nd)
				err = cond.xPipe()
				if err == nil {
					err = cond.wait()
				}
				if err != nil {
					break
				}
			}
			body.mkExec(nd.Stdin, nd.Stdout, nd.Stderr, nd.args...)
			body.extraFds(nd)
			err = body.xCmds()
			if err == nil {
				err = body.wait()
			}
		}
		nd.closeAll()
		nd.waitc <- err
	}()
	return nil
}

func (nd *Nd) xFor() error {
	xprintf("x %s\n", nd)
	if len(nd.Child) != 2 {
		dbg.Fatal("for bug")
	}
	if err := nd.redirs(); err != nil {
		xprintf("x %s done\n", nd)
		nd.closeAll()
		nd.waitc <- err
		return err
	}
	go func() {
		iter, body := nd.Child[0], nd.Child[1]
		nd.Child = iter.Child
		args := nd.names()
		nd.Child = []*Nd{iter, body}
		if len(args) < 2 {
			xprintf("x %s done\n", nd)
			nd.closeAll()
			nd.waitc <- nil
			return
		}
		vname := args[0]
		var err error
		for args = args[1:]; len(args) > 0; args = args[1:] {
			if nd.interrupted() {
				err = errors.New("interrupted")
				break
			}
			body.mkExec(nd.Stdin, nd.Stdout, nd.Stderr, nd.args...)
			body.extraFds(nd)
			os.Setenv(vname, args[0])
			err = body.xCmds()
			if err == nil {
				err = body.wait()
			}
		}
		nd.closeAll()
		xprintf("x %s done\n", nd)
		nd.waitc <- err
	}()
	return nil
}
