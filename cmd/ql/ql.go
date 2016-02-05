/*
	Clive's shell
*/
package main

import (
	"io"
	"os"
	"clive/zx"
	"clive/cmd"
	"clive/cmd/opt"
	"errors"
	"strings"
	"clive/cmd/tty"
)

type inRdr struct {
	name string
	inc  <-chan interface{}
	left []rune
}


var (
	yylex *lex
	ldebug, ydebug, nddebug, dry, cflag, iflag bool

	intrc <-chan os.Signal

	yprintf = cmd.FlagPrintf(&ydebug)
	nprintf = cmd.FlagPrintf(&nddebug)

	opts = opt.New("[file] ...")
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
	cmd.UnixIO("err")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("x", "print commands as they are run", &c.Verb)
	opts.NewFlag("L", "debug lex", &ldebug)
	opts.NewFlag("Y", "debug yacc", &ydebug)
	opts.NewFlag("N", "debug nodes", &nddebug)
	opts.NewFlag("n", "dry run", &dry)
	opts.NewFlag("c", "run args as a command", &cflag)
	opts.NewFlag("i", "interactive", &iflag)
	noux := false
	opts.NewFlag("u", "do not use unix IO", &noux)
	args := opts.Parse()
	if !noux {
		cmd.UnixIO()
	}
	if cflag {
		if len(args) == 0 {
			cmd.Warn("no args and flag -c")
			opts.Usage()
		}
		c := strings.Join(args, " ")
		if c[len(c)-1] != '\n' {
			c += "\n"
		}
		in := make(chan interface{}, 2)
		in <- zx.Dir{"path": "-c", "Upath": "-c", "type": "c"}
		in <- []byte(c)
		close(in)
		cmd.SetIn("in", in)
	} else if len(args) != 0 {
		cmd.SetIn("in", cmd.Files(args[0]))
	} else {
		iflag = tty.IsTTY(os.Stdin)
	}
	c.Debug = c.Debug || ldebug || ydebug || nddebug
	nddebug = nddebug || ydebug
	cmd.SetEnv("argv0", c.Args[0])
	cmd.SetEnvList("argv", c.Args[1:])
	in := &inRdr{name: "in", inc: cmd.In("in")}
	yylex = newLex(in)
	yylex.interactive = iflag
	if iflag {
		intrc = cmd.HandleIntr()
	} else {
		intrc = make(chan os.Signal)
	}
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
	if sts := cmd.GetEnv("sts"); sts != "" {
		cmd.Exit(sts)
	}
	cmd.Exit()
}
