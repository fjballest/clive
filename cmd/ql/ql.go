/*
	Clive's shell
*/
package main

import (
	"io"
	"clive/ns"
	"clive/zx"
	"clive/cmd"
	"clive/cmd/opt"
	"errors"
)

type inRdr struct {
	name string
	inc  <-chan interface{}
	left []rune
}


var (
	yylex *lex
	ldebug, ydebug, nddebug, dry bool

	yprintf = cmd.FlagPrintf(&ydebug)
	nprintf = cmd.FlagPrintf(&nddebug)

	opts = opt.New("")
	parseErr = errors.New("parse error")
)

func (ir *inRdr) Name() string {
	return ir.name
}

func (ir *inRdr) ReadRune() (r rune, size int, err error) {
	for len(ir.left) == 0 {
		x, ok := <-ir.inc
		if !ok {
			err = cerror(ir.inc)
			if err == nil {
				err = io.EOF
			}
			return 0, 0, err
		}
		if err, ok = x.(error); ok {
			return 0, 0, err
		}
		if d, ok := x.(zx.Dir); ok {
			ir.name = d["Upath"]
			continue
		}
		if b, ok := x.([]byte); ok {
			ir.left = []rune(string(b))
		}
		break
	}
	r = ir.left[0]
	ir.left = ir.left[1:]
	return
}

func justLex() {
	var lval yySymType
	for {
		t := yylex.lex(&lval)
		if t == 0 {
			break
		}
		cmd.Dprintf("tok %s\n", tokstr(t, &lval))
	}
	cmd.Exit()
}

func parse() (err error) {
	defer func() {
		if e := recover(); e != nil {
			if e == parseErr {
				cmd.Dprintf("parse error\n")
				err = e.(error)
				return
			}
			panic(err)
		}
	}()
	yyParse(yylex)
	if yylex.nerrors > 0 && !yylex.interactive {
		return parseErr
	}
	return nil
}

func main() {
	ns.AddLfsPath("/", nil)
	cmd.UnixIO()
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("x", "print commands as they are run", &c.Verb)
	opts.NewFlag("L", "debug lex", &ldebug)
	opts.NewFlag("Y", "debug yacc", &ydebug)
	opts.NewFlag("N", "debug nodes", &nddebug)
	opts.NewFlag("n", "dry run", &dry)
	args, err := opts.Parse()
	if err != nil {
		opts.Usage()
	}
	if len(args) != 0 {
		cmd.SetIO("in", cmd.Files(args...))
	}
	c.Debug = c.Debug || ldebug || ydebug || nddebug
	nddebug = nddebug || ydebug
	in := &inRdr{name: "in", inc: cmd.IO("in")}
	yylex = newLex(in)
	if ldebug {
		cmd.Warn("debug lex")
		justLex()	// does not return
	}
	if ydebug  {
		yylex.interactive = true
		cmd.Warn("debug yacc")
	}
	if err := parse(); err != nil {
		cmd.Fatal(err)
	}
	cmd.Exit()
}
