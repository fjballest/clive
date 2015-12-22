/*
	Evaluate expressions.
*/
package xp

import (
	"clive/app"
	"clive/app/opt"
	"clive/dbg"
	"clive/zx"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type xCmd struct {
	*opt.Flags
	*app.Ctx

	result interface{}
	quiet  bool
}

func (x *xCmd) expr(s string) (result interface{}, err error) {
	defer func() {
		if x := recover(); x != nil {
			result = nil
			err = fmt.Errorf("failed: %s", x)
		}
	}()
	var v yySymType
	l := newLex(s)
	if debugLex {
		for c := l.Lex(&v); c != 0; c = l.Lex(&v) {
		}
		return nil, nil
	}
	yyParse(l)
	return l.result, nil
}

func (x *xCmd) xp(in chan interface{}) error {
	out := app.Out()
	d := zx.Dir{"uname": "stdin"}
	var sts, err error
	var res interface{}
	nln := 0
	doselect {
	case s := <-x.Sig:
		app.Dprintf("got sig %s\n", s)
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-in:
		if !ok {
			app.Dprintf("eof\n")
			break
		}
		switch m := m.(type) {
		case []byte:
			e := strings.TrimSpace(string(m))
			app.Dprintf("got %T '%s'\n", m, e)
			nln++
			if e == "" {
				continue
			}
			res, err = x.expr(e)
			if err != nil {
				app.Warn("%s:%d: %s", d["uname"], nln, err)
				sts = err
			}
			if !x.quiet {
				if t, ok := res.(time.Time); ok {
					res = t.Format(opt.TimeFormat)
				}
				ok = app.Printf("%v\n", res) == nil
			}
		case zx.Dir:
			d = m
			ok = out <- m
			nln = 0
		default:
			app.Dprintf("got %T\n", m)
			ok = out <- m
		}
		if !ok {
			app.Exits(cerror(out))
		}
	}
	if sts == nil {
		sts = cerror(in)
	}
	if sts != nil {
		return sts
	}
	if x, ok := res.(bool); ok && !x {
		return errors.New("false")
	}
	return nil
}

func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("[expr]")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("L", "debug lex", &debugLex)
	x.NewFlag("Y", "debug yacc", &debugYacc)
	bhelp := false
	x.NewFlag("F", "report known functions and exit", &bhelp)
	defer func() {
		debugLex = false
		debugYacc = false
	}()
	x.NewFlag("q", "do not print values as they are evaluated", &x.quiet)
	args, err := x.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}
	if bhelp {
		fns := []string{}
		for k := range funcs {
			fns = append(fns, k)
		}
		sort.Sort(sort.StringSlice(fns))
		for _, b := range fns {
			app.Printf("%s\n", b)
		}
		return
	}
	in := app.In()
	if len(args) != 0 {
		in = make(chan interface{}, 1)
		in <- []byte(strings.Join(args, " ")+"\n")
		close(in)
	}
	in = app.Lines(in)
	app.Exits(x.xp(in))
}
