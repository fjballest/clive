/*
	Ql is Clive's shell.
*/
package ql

import (
	"bytes"
	"clive/app"
	"clive/app/nsutil"
	"clive/app/opt"
	"clive/app/tty"
	"clive/dbg"
	"errors"
	"io"
	"os"
	"path"
	"strings"
	"sync"
)

type xWaits struct {
	waits map[string]map[chan bool]bool
	sync.Mutex
}

type xEnv struct {
	path  []string
	funcs map[string]*Nd
	ps    [2]string
	pslvl int
	*xWaits
	debugX bool // debug execs
	debugL bool // debug lex
	iflag  bool // interactive
}

type xCmd struct {
	*opt.Flags
	*app.Ctx
	dry    bool
	cmdarg string // -c cmd

	lvl, plvl int
	*xEnv
	*lex
}

type inRdr struct {
	inc  chan interface{}
	left []rune
}

func (ir *inRdr) ReadRune() (r rune, size int, err error) {
	if len(ir.left) == 0 {
		x, ok := <-ir.inc
		if !ok {
			err = cerror(ir.inc)
			if err == nil {
				err = io.EOF
			}
			return 0, 0, err
		}
		if b, ok := x.([]byte); ok {
			ir.left = []rune(string(b))
		}
		if err, ok = x.(error); ok {
			return 0, 0, err
		}
	}
	r = ir.left[0]
	ir.left = ir.left[1:]
	return
}

func (x *xWaits) addWait(tag string, c chan bool) {
	x.Lock()
	m := x.waits[tag]
	if m == nil {
		m = map[chan bool]bool{}
	}
	m[c] = true
	x.waits[tag] = m
	x.Unlock()
	go func() {
		<-c
		x.Lock()
		delete(x.waits[tag], c)
		x.Unlock()
	}()

}

func (x *xWaits) wait(tag string) {
	x.Lock()
	m := x.waits[tag]
	delete(x.waits, tag)
	x.Unlock()
	for k := range m {
		<-k
	}
}

func (x *xCmd) promptLvl(lvl int) {
	if lvl != 0 {
		lvl = 1
	}
	x.pslvl = lvl
	x.lex.prompt = x.ps[x.pslvl]
}

func (x *xEnv) setPrompt(ps ...string) {
	switch len(ps) {
	case 0:
		x.ps[0] = "% "
		x.ps[1] = "%     "
	case 1:
		x.ps[0], x.ps[1] = ps[0], ps[0]
	default:
		x.ps[0], x.ps[1] = ps[0], ps[1]
	}
}

func (x *xEnv) setPath() {
	var ps []string
	if p := app.GetEnv("path"); p == "" {
		p = app.GetEnv("PATH")
		if p == "" {
			p = "/bin:/usr/bin"
		}
		ps = strings.SplitN(p, ":", -1)
	} else {
		ps = strings.Fields(p)
	}
	x.path = nil
	for _, d := range ps {
		x.path = append(x.path, strings.SplitN(d, "\b", -1)...)
	}
}

func (x *xEnv) lookCmd(name string) string {
	if strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") ||
		strings.HasPrefix(name, "/") {
		return name
	}
	for _, pd := range x.path {
		nm := path.Join(pd, name)
		if d, err := nsutil.Stat(nm); err == nil {
			if d["type"] != "d" && d.CanExec(nil) {
				return nm
			}
		}
	}
	return ""
}

// Run ql in the current app context.
func Run() {
	waits := make(map[string]map[chan bool]bool)
	x := &xCmd{
		Ctx: app.AppCtx(),
		xEnv: &xEnv{
			funcs:  map[string]*Nd{},
			xWaits: &xWaits{waits: waits},
		},
	}
	xcmdarg := ""
	x.Flags = opt.New("[file {arg}]")
	x.NewFlag("c", "cmd: execute this command w/o rewrites and exit", &x.cmdarg)
	x.NewFlag("x", "cmd: execute this command w/ rewrites and exit", &xcmdarg)
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("L", "debug lex", &x.debugL)
	x.NewFlag("X", "debug execs", &x.debugX)
	x.NewFlag("n", "dry run", &x.dry)
	x.NewFlag("i", "interactive", &x.iflag)
	args, err := x.Parse(x.Args)

	if x.cmdarg != "" {
		x.cmdarg = "{" + x.cmdarg + "}"
	} else {
		x.cmdarg = xcmdarg
	}
	// if In is closed, then we are a unix command: init input and prompt
	x.setPrompt()
	if _, err := app.IOchan(0); err != nil {
		app.SetIO(app.OSIn(), 0)
		x.iflag = tty.IsTTY(os.Stdin)
	}

	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}
	var in inText
	var argv0, iname string
	var argv []string
	if x.cmdarg != "" {
		if x.cmdarg[len(x.cmdarg)-1] != '\n' {
			x.cmdarg += "\n"
		}
		in = bytes.NewBufferString(x.cmdarg)
		argv0 = "ql"
		iname = "flag-c"
		argv = args
		x.iflag = false
	} else if len(args) > 0 {
		argv0 = args[0]
		iname = args[0]
		argv = args[1:]
		dat, err := nsutil.GetAll(iname)
		if err != nil {
			app.Fatal("%s: %s", iname, err)
		}
		in = bytes.NewBuffer(dat)
		x.iflag = false
	} else {
		argv0 = "ql"
		iname = "stdin"
		xin := app.In()
		if xin == nil {
			app.Fatal("no input")
		}
		in = &inRdr{inc: xin}
	}
	x.setPath()
	SetEnvList("argv0", argv0)
	SetEnvList("argv", argv...)
	x.lex = newLex(iname, in)
	x.lex.debug = x.debugL
	x.lex.interactive = x.iflag
	x.lex.prompt = x.ps[0]
	if x.lex.interactive && x.lex.prompt != "" {
		app.Printf("%s", x.lex.prompt)
	}

	for {
		err = x.run()
		if err == dbg.ErrIntr {
			if x.iflag {
				continue
			}
			app.Fatal("interrupted")
		}
		break
	}
	app.Dprintf("ql exiting with %v\n", err)
	app.Exits(err)
}

func (x *xCmd) run() (err error) {
	// if there's a C-c while we are reading,
	// the lexer will panic with ErrIntr,
	// to signal us that we must discard the current
	// input and parsing and start again.
	defer func() {
		x.lvl = 0
		x.nerrors = 0
		if xerr := recover(); xerr != nil {
			if xe, ok := xerr.(error); ok && xe == dbg.ErrIntr {
				err = xe
				return
			}
			panic(xerr)
		}
	}()
	rc := yyParse(x)
	if x.dry {
		return nil
	}
	s := app.GetEnv("status")
	if s != "" {
		err = errors.New(s)
	}
	if err == nil && rc < 0 {
		err = errors.New("errors")
	}
	return err
}
