package ql

import (
	"os/exec"
	"clive/dbg"
	"clive/app"
	"clive/app/nsutil"
	"runtime/debug"
	"errors"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
	"bytes"
	"fmt"
	"clive/zx"
	"os"
)

type xNd {
	*xEnv
	pi, po	chan interface{}	// temps to setup pipes
	bg bool
}

func (x *xNd) xprintf(fmts string, arg ...interface{}) {
	if x.debugX {
		app.Eprintf(fmts, arg...)
	}
}

// pipe nodes are async if the first arg is a string with their tag
func (nd *Nd) isAsync() bool {
	return nd.Kind == Npipe && len(nd.Args) > 0 && len(nd.Args[0]) > 0
}

// Run() is called by the parser to execute top-level nodes of the AST,
// this calls run() which corresponds to cmd in grammar.
// run executes pipes and vars.
// pipes may be async and only their children may have redirections.
func (x *xCmd) Run(nd *Nd) {
	if nd == nil || nd.Kind == Nnop || x.nerrors > 0 {
		return
	}
	if (x.dry || x.debugX) && !x.Debug {
		app.Eprintf("run %s\n", nd.sprint())
	}
	if !x.dry {
		xnd := &xNd{xEnv: x.xEnv}
		xnd.runCmd(nd)
	}
}

func (x *xNd) runCmd(nd *Nd) error {
	x.xprintf("runCmd %s\n", nd)
	switch nd.Kind {
	case Nnop:
	case Nset:	
		return x.setVar(nd)
	case Npipe:
		return x.runPipe(nd, nil)
	case Ncond:
		return x.runCond(nd)
	default:
		dbg.Fatal("ql bug: runCmd nd kind %s\n", nd.Kind)
	}
	return nil
}

// Nval, Nlen, Njoin
func (x *xNd) expandVar(nd *Nd) ([]string, error) {
	x.xprintf("expandVar %s\n", nd)
	if nd.Kind != Nval && nd.Kind != Nlen && nd.Kind != Njoin {
		dbg.Fatal("ql expandVar bug: type %s", nd.Kind)
	}
	if len(nd.Args) == 0 {
		return nil, errors.New("no args")
	}
	i := ""
	if len(nd.Args) > 1 {
		i = nd.Args[1]
	}
	words, err := GetEnvAt(nd.Args[0], i)
	switch nd.Kind {
	case Nlen:
		words = []string{strconv.Itoa(len(words))}
	case Njoin:
		words = []string{strings.Join(words, " ")}
	}
	x.xprintf("expand: %v\n", words)
	return words, err
}

func (x *xNd) expandApp(nd *Nd) ([]string, error) {
	x.xprintf("expandApp %s\n", nd)
	if nd.Kind != Napp {
		app.Fatal("ql expandApp bug")
	}
	if len(nd.Child) != 2 {
		return nil, errors.New("no args")
	}
	l0, err := x.expand(nd.Child[0])
	if err != nil {
		return nil, err
	}
	l1, err := x.expand(nd.Child[1])
	if err != nil {
		return nil, err
	}
	names := []string{}
	switch {
	case len(l0) == len(l1):
		for i := 0; i < len(l1); i++ {
			names = append(names, l0[i]+l1[i])
		}
	case len(l0) == 1:
		for i := 0; i < len(l1); i++ {
			names = append(names, l0[0]+l1[i])
		}
	case len(l1) == 1:
		for i := 0; i < len(l0); i++ {
			names = append(names, l0[i]+l1[0])
		}
	default:
		names = append(l0, l1...)
	}
	return names, nil
}

func cmdOut(c chan interface{}) (string, error) {
	var b bytes.Buffer
	for m := range c {
		if m, ok := m.([]byte); ok {
			b.Write(m)
		}
	}
	return b.String(), cerror(c)
}

func cmdRaw(c chan interface{}) ([]string, error) {
	var o []string
	for m := range c {
		if m, ok := m.([]byte); ok {
			o = append(o, string(m))
		}
	}
	return o, cerror(c)
}

func cmdOuts(c chan interface{}) ([]string, error) {
	s, err := cmdOut(c)
	outs := strings.Fields(s)
	return outs, err
}

func (x *xNd) expandInHerePipeblk(nd *Nd) ([]string, error) {
	x.xprintf("expandInHerePipeblk %s\n", nd)
	if nd.Kind != Ninblk && nd.Kind != Nrawinblk && nd.Kind != Nsingleinblk && nd.Kind != Npipeblk {
		dbg.Fatal("ql expandInHerePipeblk bug")
	}
	if len(nd.Child) != 1 || nd.Child[0].Kind != Npipe {
		dbg.Fatal("ql expand child is %s", nd.Child[0].Kind)
	}
	outc := make(chan interface{})
	if err := x.runPipe(nd.Child[0], outc); err != nil {
		return nil, err
	}
	switch nd.Kind {
	case Npipeblk:
		n := app.AddIO(outc)
		x.xprintf("pipeblk outc %p\n", outc)
		return []string{fmt.Sprintf("#%d", n)}, nil
	case Nrawinblk:
		o, err := cmdRaw(outc)
		return o, err
	case Nsingleinblk:
		o, err := cmdOut(outc)
		return []string{o}, err
	case Ninblk:
		return cmdOuts(outc)
	}
	return nil, nil
}

// Expand names at this node.
func (x *xNd) expand(nd *Nd) (names []string, err error) {
	switch nd.Kind {
	case Nname:
		nargs := make([]string, len(nd.Args))
		copy(nargs, nd.Args)
		return nargs, nil
	case Nval, Nlen, Njoin:
		return x.expandVar(nd)
	case Napp:
		return x.expandApp(nd)
	case Ninblk, Nrawinblk, Nsingleinblk, Npipeblk:
		return x.expandInHerePipeblk(nd)
	default:
		return nil, nil
	}
}

// Expand names in children of this nd.
// Names are Nname, Nval, Njoin, Napp, Ninblk, Nrawinblk, Nsingleinblk, Npipeblk, Nlen
// and can be used in Nnames, Nexec, Nset (also in Napp)
func (x *xNd) expandChildren(nd *Nd) ([]string, error) {
	x.xprintf("expand %s\n", nd)
	if nd.Kind != Nnames && nd.Kind != Nexec && nd.Kind != Nfor && nd.Kind != Nset && nd.Kind != Napp {
		dbg.Fatal("ql expand children bug: type %s", nd.Kind)
	}
	names := []string{}
	for i := 0; i < len(nd.Child); i++ {
		cnames, err := x.expand(nd.Child[i])
		if err != nil {
			return nil, err
		}
		names = append(names, cnames...)
	}
	return names, nil
}
	
func (x *xNd) setVar(nd *Nd) error {
	x.xprintf("setVar %s\n", nd)
	if nd.Kind != Nset || len(nd.Args) == 0 || len(nd.Args[0]) == 0 {
		dbg.Fatal("ql setVar bug")
	}
	vname := nd.Args[0]
	// maps
	if len(nd.Child) > 0 && nd.Child[0].Kind == Nset {
		if vname == "prompt" || vname == "path" {
			app.Warn("%s is not a map", vname)
		}
		m := GetEnvMap(vname)
		for _, c := range nd.Child {
			if len(c.Args) < 2 {
				continue
			}
			if len(c.Args[1]) == 0 {
				delete(m, c.Args[0])
			} else {
				m[c.Args[0]] = c.Args[1]
			}
		}
		SetEnvMap(vname, m)
		return nil
	}
	vals, err := x.expandChildren(nd)
	if err != nil {
		app.Warn("%s", err)
		return nil
	}
	sts := ""
	switch {
	case vname == "path" && len(vals) == 0:
		app.Warn("won't reset path")
		return errors.New("won't reset path")
	case vname == "prompt"&& len(vals) != 2:
		app.Warn("prompt must have two values")
		return errors.New("prompt must have two values")
	case vname == "status":
		sts = strings.Join(vals, " ")
		vals = []string{sts}
	}
	if len(nd.Args) > 1 {
		SetEnvAt(vname, nd.Args[1], strings.Join(vals, " "))
	} else {
		SetEnvList(vname, vals...)
	}
	switch vname {
	case "path":
		x.setPath()
	case "prompt":
		x.setPrompt(vals[0], vals[1])
	case "status":
		if sts != "" {
			return errors.New(sts)
		}
	}
	return nil
}

// Redirections happen only in children of Npipe.
// & happens only in Npipe.
// No other nodes have to deal with that.
// If outc is not nil then stdout for the pipe should be sent there.
func (x *xNd) runPipe(nd *Nd, outc chan interface{}) error {
	x.xprintf("runPipe %s\n", nd)
	async := nd.isAsync() || outc != nil
	cxs := make([]*xNd, len(nd.Child))
	for i := range cxs {
		nx := &xNd{xEnv: x.xEnv, bg: async}
		cxs[i] = nx
		if i < len(nd.Child)-1 {
			nx.po = make(chan interface{})
		} else if outc != nil {
			nx.po = outc
		}
		if i > 0 {
			nx.pi = cxs[i-1].po
		} else if async {
			nx.pi = app.Null
		}
	}
	var wc chan bool
	dflag := app.AppCtx().Debug
	for i := range nd.Child {
		c := nd.Child[i]
		xc := make(chan chan bool, 1)
		if !async && c.runsHere() {
			x.xprintf("run here %s\n", c)
			err := cxs[i].runSimplerdr(c)
			wait := make(chan bool)
			close(wait, err)
			xc <- wait
		} else {
			go func(i int) {
				defer app.Exiting()
				defer func() {
					if x := recover(); x != nil && x != "fatal" {
						app.Warn("panic: %v", x)
						debug.PrintStack()
						app.Exits("panic")
					}
				}()
				nc := app.New()
				nc.Debug = dflag
				app.DupIO()
				if async && i == 0 {
					app.SetIO(app.Null, 0)
				}
				app.DupEnv()
				if !c.isCmd("cd") {
					app.DupDot()
				}
				if x.bg {
					app.Bg()	// make it ignore intrs.
				}
				x.xprintf("run %s\n%s\n", c, nc.Sprint())
				xc <- nc.Wait
				err := cxs[i].runSimplerdr(c)
				app.Exits(err)
			}(i)
		}
		wc = <-xc
	}
	if async {
		if outc == nil {
			x.addWait(nd.Args[0], wc)
		}
		return nil
	}
	<-wc
	err := cerror(wc)
	if err == nil {
		app.SetEnv("status", "")
	} else {
		app.SetEnv("status", err.Error())
	}
	x.xprintf("status %s -> '%s'\n", nd, app.GetEnv("status"))
	return err
}

func recvc(c <-chan []byte) chan interface{} {
	ic := make(chan interface{})
	go func() {
		for x := range c {
			ic <- x
		}
		close(ic, cerror(c))
	}()
	return ic
}

func sendc(c chan<- []byte) chan interface{} {
	ic := make(chan interface{})
	go func() {
		for x := range ic {
			if b, ok := x.([]byte); ok {
				c <- b
			}
		}
		close(c, cerror(ic))
	}()
	return ic
}

// Called for each pipe child in its context to setup redirs for the command.
func (x *xNd) redirs(nd *Nd) error {
	if len(nd.Redirs) == 0 {
		return nil
	}
	x.xprintf("redirs %s\n", nd)
	defer func() {
		x.xprintf("redirs-> %s\n", app.AppCtx().Sprint())
	}()
	// First open all files. That's easy to undo now if there are errors.
	ins := []<-chan []byte{}
	outs := []chan<- []byte{}

	var err error
	for _, r := range nd.Redirs {
		if r.Name != "" && r.Name != "=" && r.Name != "|" {
			if r.From == 0 {
				in := nsutil.Get(r.Name, 0, zx.All, "")
				ins = append(ins, in)
			} else {
				out := make(chan []byte)
				if r.App {
					nsutil.Put(r.Name, nil, -1, out, "")
				} else {
					nsutil.Put(r.Name, zx.Dir{"mode":"0664"}, 0, out, "")
				}
				outs = append(outs, out)
				// defer error checking until we actually write
			}
		}
	}
	if err != nil {
		for _, i := range ins {
			close(i, err)
		}
		for _, o := range outs {
			close(o, err)
		}
		return err
	}
	for _, r := range nd.Redirs {
		switch r.Name {
		case "":
			app.Warn("redir with null name")
		case "=":
			nfrom, nto := 1, 2		// dup 2 into 1
			if r.From == 2 {		// dup 1 into 2
				nfrom, nto = 2,1
			} else if r.From != 1 {
				app.Fatal("ql redirs dup bug")
			}
			app.CopyIO(nto, nfrom)
		case "|":
			c := x.pi
			if r.From != 0 {
				c = x.po
			}
			if c == nil {
				c = app.Null
			}
			app.SetIO(c, r.From)
		default:
			var c chan interface{}
			if r.From == 0 {
				c = recvc(ins[0])
				ins = ins[1:]
			} else {
				c = sendc(outs[0])
				outs = outs[1:]
			}
			app.SetIO(c, r.From)
		}
	}
	return nil
}

func (nd *Nd) isCmd(cmd ...string) bool {
	if nd.Kind != Nexec || len(nd.Child) == 0 {
		return false
	}
	c := nd.Child[0]
	for _, cname := range cmd {
		if c.Kind == Nname && len(c.Args) > 0 && c.Args[0] == cname {
			return true
		}
	}
	return false
}

func (nd *Nd) isHereCmd() bool {
	if nd.Kind != Nexec || len(nd.Child) == 0 {
		return false
	}
	c := nd.Child[0]
	return c.Kind == Nname && len(c.Args) > 0 && isHere[c.Args[0]]
}

// called for pipe children to see if they can run within the pipe app context
func (nd *Nd) runsHere() bool {
	if len(nd.Redirs) > 0 {	// thus no pipe with multiple cmds runs here
		return  false
	}
	switch nd.Kind {
	case Nblk, Nfor, Ncond, Nwhile, Nset:
		return true
	case Nteeblk:
	case Nexec:
		return nd.isHereCmd()
	default:
		dbg.Fatal("ql bug: runPipeChild nd kind %s\n", nd.Kind)
	}
	return false
}

// Only for pipe children that do not nd.runsHere()
func (x *xNd) runSimplerdr(nd *Nd) error {
	x.xprintf("runSimplerdr %s\n", nd)
	if err := x.redirs(nd); err != nil {
		return err
	}
	x.xprintf("after redirs %s\n%s\n", nd, app.AppCtx().Sprint())
	switch nd.Kind {
	case Nexec:
		return x.runNames(nd)
	case Nblk:
		return x.runBlk(nd)
	case Nteeblk:
		return x.runTee(nd)
	case Nfor:
		return x.runFor(nd)
	case Ncond:
		return x.runCond(nd)
	case Nwhile:
		return x.runWhile(nd)
	default:
		dbg.Fatal("ql bug: runPipeChild nd kind %s\n", nd.Kind)
	}
	return nil
}

// runs already in the context for the app to exec.
// must dup dot and env if the pipe was async
func (x *xNd) runNames(nd *Nd) error {
	x.xprintf("runNames %s\n", nd)
	if nd.Kind != Nexec {
		dbg.Fatal("ql exec bug")
	}
	argv, err := x.expandChildren(nd)
	if err != nil {
		x.xprintf("expand: %s\n", err)
		return err
	}
	if len(argv) == 0 || len(argv[0]) == 0 {
		x.xprintf("expand: no args\n")
		return nil
	}
	argv0 := argv[0]
	if !isHere[argv0] {
		c := app.AppCtx()
		c.Args = argv
		SetEnvList("argv", argv...)
		c.Debug = false
	}
	bfn := bltin[argv0]
	if bfn != nil {
		// those that don't run here never return
		x.xprintf("bltin %v\n", argv)
		return bfn(x.xEnv, argv...)
	}
	fn := x.funcs[argv0]
	if fn != nil {
		x.xprintf("func %v\n", argv)
		return x.runBlk(fn.Child[0])
	}
	return x.xexec(argv)
}

func (x *xNd) runBlk(nd *Nd) error {
	x.xprintf("runBlk %s\n", nd)
	var err error
	switch nd.Kind {
	case Ninblk, Nrawinblk, Nsingleinblk, Npipeblk, Nblk, Nfunc:
		// Nin/here/pipeblk are called from expands
		// Nfunc is called from exec for funcs.
		for _, c := range nd.Child {
			err = x.runCmd(c)
		}
	default:
		dbg.Fatal("ql bug: runPipeChild nd kind %s\n", nd.Kind)
	}
	return err
}

// all commands in a tee run concurrently and the block ends when all
// commands have ended.
func (x *xNd) runTee(nd *Nd) error {
	x.xprintf("runTee %s\n", nd)
	ics := make([]chan interface{}, len(nd.Child))
	wc := make(chan error, len(nd.Child))
	for i := range ics {
		nx := &xNd{xEnv: x.xEnv}
		ics[i] = make(chan interface{})
		go func(i int) {
			defer app.Exiting()
			app.New()
			app.DupIO()
			app.SetIO(ics[i], 0)
			app.DupDot()
			app.DupEnv()
			err := nx.runCmd(nd.Child[i])
			wc <- err
			app.Exits(nil)
		}(i)
	}
	left := len(nd.Child)
	var err error
	in := app.In()
	doselect{
	case cerr := <-wc:
		if err == nil {
			err = cerr
		}
		if left--; left == 0 {
			break
		}
	case x, ok := <-in:
		if !ok {
			err = cerror(in)
			break
		}
		for i, ic := range ics {
			if ic != nil {
				if ok := ic <- x; !ok {
					ics[i] = nil
				}
			}
		}
	}
	for _, ic := range ics {
		close(ic, err)
	}
	for ; left > 0; left -- {
		cerr := <-wc
		if err == nil {
			err = cerr
		}
	}
	return err
}

func (x *xNd) runFor(nd *Nd) error {
	x.xprintf("runFor %s\n", nd)
	if len(nd.Child) != 2 {
		app.Fatal("ql for bug")
	}
	hd, bdy := nd.Child[0], nd.Child[1]
	names, err := x.expandChildren(hd)
	if err != nil {
		return nil
	}
	if nd.IsGet && len(names) == 1 {
		vname := names[0]
		c := app.AppCtx()
		in := app.In()
		doselect {
		case <-c.Sig:
			break
		case m, ok := <- in:
			if !ok {
				break
			}
			if b, ok := m.([]byte); ok {
				app.SetEnv(vname, strings.TrimSpace(string(b)))
				xerr := x.runBlk(bdy)
				if err == nil {
					err = xerr
				}
			}
		}
		return err
	}
	if len(names) < 2 || len(names[0]) == 0 {
		return nil
	}
	vname := names[0]
	for _, val := range names[1:] {
		app.SetEnv(vname, val)
		xerr := x.runBlk(bdy)
		if err == nil {
			err = xerr
		}
	}
	return err
}

func (x *xNd) runCond(nd *Nd) error {
	x.xprintf("runCond %s\n", nd)
	var err error
	for _, cor := range nd.Child {
		if cor.Kind != Ncond {
			dbg.Fatal("cond children are not conds")
		}
		n := 0
		for _, cand := range cor.Child {
			n++
			if err = x.runPipe(cand, nil); err != nil {
				break
			}
		}
		if n == len(cor.Child) {
			break
		}
	}
	return err
}

func (x *xNd) runWhile(nd *Nd) error {
	x.xprintf("runWhile %s\n", nd)
	if len(nd.Child) != 2 {
		dbg.Fatal("ql while bug")
	}
	cond,bdy := nd.Child[0],nd.Child[1]
	for x.runPipe(cond, nil) == nil {
		x.runBlk(bdy)
	}
	return nil
}

func (x *xNd) xexec(argv []string) error {
	x.xprintf("xexec %v\n", argv)
	if len(argv) < 1 || len(argv[0]) < 1 {
		return errors.New("no command name")
	}
	argv0 := argv[0]
	xcmd := exec.Command(argv0, argv[1:]...)
	if p := x.lookCmd(argv0); p != "" {
		xcmd.Path = p
	}
	xcmd.Dir = app.Dot()
	for k, v := range app.Env() {
		xcmd.Env = append(xcmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	in := app.In()
	var err error
	if in != app.Null {
		xcmd.Stdin = app.InReader()
	}
	ofn := func(ofd io.ReadCloser, out chan interface{}, dc chan bool) error {
		for {
			b := make([]byte, 8192)
			n, err := ofd.Read(b)
			if err != nil {
				if err == io.EOF {
					err = nil
				}
				dc <- true
				return err
			}
			if ok := out <- b[:n]; !ok {
				dc <- true
				return cerror(out)
			}
		}
	}
	out := app.Out()
	var ofd io.ReadCloser
	donec := make(chan bool , 2)
	if out != app.Null {
		ofd, err = xcmd.StdoutPipe()
		if err != nil {
			return err
		}
		go ofn(ofd, out, donec)
	} else {
		xcmd.Stdout = ioutil.Discard
		donec <- true
	}
	errc := app.Err()
	var efd io.ReadCloser
	if errc != app.Null {
		efd, err = xcmd.StderrPipe()
		if err != nil {
			return err
		}
		go ofn(efd, errc, donec)
	} else {
		xcmd.Stderr = ioutil.Discard
		donec <- true
	}

	// TODO: #n names for unix
	// xcmd.ExtraFiles = ...add one fd per extra chan

	if err := xcmd.Start(); err != nil {
		app.Warn("%s: %s", argv[0], err)
		return err
	}
	ndone := 2
	ctx := app.AppCtx()
	doselect {
	case <-donec:
		ndone--
		if ndone == 0 {
			break
		}
	case s  := <-ctx.Sig:
		if s == "intr" {
			if !x.bg {
				xcmd.Process.Signal(os.Interrupt)
			}
		} else {
			xcmd.Process.Signal(os.Kill)
		}
	}
	// Go StdoutPipe/StderrPipe requires to read it all before
	// calling wait.
	err = xcmd.Wait()
	return err
}

