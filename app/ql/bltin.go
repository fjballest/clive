package ql

import (
	"clive/zx"
	"clive/dbg"
	"clive/app"
	"clive/app/opt"
	"clive/app/nsutil"

	"clive/app/ch"
	"clive/app/cnt"
	"clive/app/cols"
	"clive/app/cpm"
	"clive/app/echo"
	"clive/app/flds"
	"clive/app/lf"
	"clive/app/lns"
	"clive/app/pf"
	"clive/app/pwd"
	"clive/app/sleep"
	"clive/app/gr"
	"clive/app/wr"
	"clive/app/frmt"
	"clive/app/diffs"
	"clive/app/jn"
	"clive/app/srt"
	"clive/app/rem"
	"clive/app/mvm"
	"clive/app/trex"
	"clive/app/gp"
	"clive/app/xp"
	"clive/app/hist"
	"clive/app/words"
)

type bFunc func(*xEnv, ...string) error

var (
	// builtins that should run within the ql context
	isHere = map[string]bool{
		"flag": true,
		"exit": true,
		"wait": true,
		"dup": true,
		"new": true,
	}
	// commands besides those that "isHereCmd" for which ones
	// we don't want the lf a b c | pf pipe rewrite, to save a '|'
	noRewrites = []string {
		"type",
		"echo",
		":",
		"lf",
		"gf",
	}
	bltin map[string]bFunc
)

func importRun(fn func()) bFunc {
	return func(*xEnv, ...string) error {
		fn()
		return nil
	}
}

func init() {
	bltin = map[string]bFunc{
		// these run here
		"cd": runCd,
		"flag": runFlag,
		"exit": runExit,
		"wait": runWait,
		"dup": runDup,
		"new": runNew,

		// these run in their own context
		"type": runType,
		"ch": importRun(ch.Run),
		"cnt": importRun(cnt.Run),
		"cols": importRun(cols.Run),
		"cpf": importRun(cpm.Run),
		"cpm": importRun(cpm.Run),
		"echo": importRun(echo.Run),
		"flds": importRun(flds.Run),
		"gf": importRun(lf.Run),
		"gr": importRun(gr.Run),
		"gg": importRun(gr.Run),
		"gv": importRun(gr.Run),
		"gx": importRun(gr.Run),
		"lf": importRun(lf.Run),
		"lns": importRun(lns.Run),
		"pf": importRun(pf.Run),
		"wf": importRun(pf.Run),
		"pwd": importRun(pwd.Run),
		"ql": importRun(Run),
		"sleep": importRun(sleep.Run),
		"wr": importRun(wr.Run),
		"frmt": importRun(frmt.Run),
		"diffs": importRun(diffs.Run),
		"jn": importRun(jn.Run),
		"srt": importRun(srt.Run),
		"rem": importRun(rem.Run),
		"mvf": importRun(mvm.Run),
		"mvm": importRun(mvm.Run),
		"trex": importRun(trex.Run),
		"gp": importRun(gp.Run),
		"xp": importRun(xp.Run),
		":": importRun(xp.Run),
		"hist": importRun(hist.Run),
		"words": importRun(words.Run),

		/* todo

		XXX: $status is not set for function calls
		XXX: must source init file and per-user init file
		XXX: We should evaluate functions before bltins, but
		making sure than within a function any call to that function
		does not use the function, to prevent loops and (sic) recursive calls
		XXX: must make sure that execing blocks and compound commands
		pay attention to intrs.
		*/
	}
}

func runTODO(x *xEnv, argv ...string) error {
	defer app.Exiting()
	app.Fatal("bug: todo: %v", app.Args())
	return nil
}

// runs here
func runCd(x *xEnv, argv ...string) error {
	opts := opt.New("")
	app.Dprintf("cd %v\n", argv)
	args, err := opts.Parse(argv)
	if err != nil {
		opts.Usage()
		return dbg.ErrUsage
	}
	if len(args) != 0 {
		opts.Usage()
		return dbg.ErrUsage
	}
	m, ok := <-app.In()
	if !ok {
		err = cerror(app.In())
		app.Warn("%s", err)
		return err
	}
	d, ok := m.(zx.Dir)
	if !ok {
		b, ok := m.([]byte)
		if !ok {
			app.Warn("%s", dbg.ErrNotDir)
			return dbg.ErrNotDir
		}
		s := string(b)
		d, err = nsutil.Stat(s)
		if err != nil {
			app.Warn("%s: %s", s, err)
			return err
		}
	}
	if d["type"] != "d" {
		app.Warn("%s: %s", d["path"], dbg.ErrNotDir)
		return dbg.ErrNotDir
	}
	app.Cd(d["path"])
	out := app.Out()
	if out != nil {
		out <- d
	}
	return nil
}

// runs here
func runFlag(x *xEnv, argv ...string) error {
	c := app.AppCtx()
	app.Dprintf("flag %v\n", argv)
	switch len(argv) {
	case 1:
		flgs := ""
		if c.Debug {
			flgs += "D"
		}
		if x.debugX {
			flgs += "X"
		}
		if x.debugL {
			flgs += "L"
		}
		if x.iflag {
			flgs += "i"
		}
		app.Printf("flags %s\n", flgs)
		return nil
	case 2:
	default:
		app.Eprintf("usage: flags [±]flags\n")
		return dbg.ErrUsage
	}
	flg := argv[1]
	set := flg[0] == '+'
	clear := flg[0] != '-'
	if !set && !clear {
		// clear all flags
		c.Debug = false
		x.debugX = false
		x.debugL = false
		// then set those named
		set = true
	} else {
		flg = flg[1:]
	}
	for _, r := range flg {
		switch r {
		case 'D':
			c.Debug = (c.Debug && !clear) || set
		case 'X':
			x.debugX = (x.debugX && !clear) || set
		case 'L':
			x.debugL = (x.debugL && !clear) || set
		case 'i':
			app.Warn("'-i' cannot be changed")
		default:
			app.Warn("unknown flag '%c'", r)
		}
	}
	return nil
}

// runs here
func runExit(x *xEnv, argv ...string) error {
	app.Dprintf("exit %v\n", argv)
	s := ""
	if len(argv) > 1 {
		s = argv[1]
	}
	app.Exits(s)
	return nil
}

// runs here
func runWait(x *xEnv, argv ...string) error {
	opts := opt.New("wait")
	app.Dprintf("wait %v\n", argv)
	args, err := opts.Parse(argv)
	if err != nil {
		opts.Usage()
		return dbg.ErrUsage
	}
	if len(args) == 0 {
		args = []string{"&"}
	}
	if len(args) != 1 {
		opts.Usage()
		return dbg.ErrUsage
	}
	x.wait(args[0])
	return nil
}

// runs here
func runDup(x *xEnv, argv ...string) error {
	opts := opt.New("dup")
	app.Dprintf("dup %v\n", argv)
	args, err := opts.Parse(argv)
	if err != nil {
		opts.Usage()
		return dbg.ErrUsage
	}
	for _,arg := range args {
		switch arg {
		case "ns":
			app.DupNS()
		case "io":
			app.DupIO()
		case "env":
			app.DupEnv()
		case "dot":
			app.DupDot()
		default:
			app.Warn("unknown resource '%s'", arg)
		}
	}
	return nil
}

// runs here
func runNew(x *xEnv, argv ...string) error {
	opts := opt.New("dup")
	app.Dprintf("dup %v\n", argv)
	args, err := opts.Parse(argv)
	if err != nil {
		opts.Usage()
		return dbg.ErrUsage
	}
	for _,arg := range args {
		switch arg {
		case "ns":
			app.NewNS(nil)
		case "io":
			app.NewIO(nil)
		case "env":
			app.NewEnv(nil)
		case "dot":
			app.NewDot("")
		default:
			app.Warn("unknown resource '%s'", arg)
		}
	}
	return nil
}

func (x *xEnv) ntype(nm string) string {
	if bltin[nm] != nil {
		return "builtin"
	}
	if x.funcs[nm] != nil {
		return "func"
	}
	if app.GetEnv(nm) != "" {
		return "env"
	}
	path := x.lookCmd(nm)
	if path != "" {
		path = ": " + path
	}
	return "external" + path
}

func runType(x *xEnv, argv ...string) error {
	defer app.Exiting()
	opts := opt.New("{name}")
	app.Dprintf("type %v\n", argv)
	args, err := opts.Parse(argv)
	if err != nil {
		opts.Usage()
		app.Exits(dbg.ErrUsage)
	}
	for _, n := range args {
		app.Printf("%s: %s\n", n, x.ntype(n))
	}
	app.Exits(nil)
	return nil
}
