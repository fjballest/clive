/*
	Evaluate expressions.
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

var (
	opts = opt.New("[expr]")
	result interface{}
	quiet  bool
)

func expr(s string) (result interface{}, err error) {
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

func xp(in <-chan interface{}) error {
	out := cmd.Out("out")
	d := zx.Dir{"uname": "stdin"}
	var sts, err error
	var res interface{}
	nln := 0
	for m := range in {
		ok := true
		switch m := m.(type) {
		case []byte:
			e := strings.TrimSpace(string(m))
			cmd.Dprintf("got %T '%s'\n", m, e)
			nln++
			if e == "" {
				continue
			}
			res, err = expr(e)
			if err != nil {
				cmd.Warn("%s:%d: %s", d["uname"], nln, err)
				sts = err
			}
			if !quiet {
				if t, ok := res.(time.Time); ok {
					res = t.Format(opt.TimeFormat)
				}
				if _, err := cmd.Printf("%v\n", res); err != nil {
					ok = false
				}
			}
		case zx.Dir:
			d = m
			ok = out <- m
			nln = 0
		default:
			cmd.Dprintf("got %T\n", m)
			ok = out <- m
		}
		if !ok {
			close(in, cerror(out))
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

func main() {
	c := cmd.AppCtx()
	cmd.UnixIO("err")
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("L", "debug lex", &debugLex)
	opts.NewFlag("Y", "debug yacc", &debugYacc)
	ux := false
	opts.NewFlag("u", "use unix out", &ux)
	bhelp := false
	opts.NewFlag("F", "report known functions and exit", &bhelp)
	opts.NewFlag("q", "do not print values as they are evaluated", &quiet)
	args, err := opts.Parse()
	if err != nil {
		cmd.Warn("%s", err)
		opts.Usage()
	}
	if ux {
		cmd.UnixIO("out")
	}
	if bhelp {
		fns := []string{}
		for k := range funcs {
			fns = append(fns, k)
		}
		sort.Sort(sort.StringSlice(fns))
		for _, b := range fns {
			cmd.Printf("%s\n", b)
		}
		cmd.Exit(nil)
	}
	if len(args) != 0 {
		in := make(chan interface{}, 1)
		in <- []byte(strings.Join(args, " ")+"\n")
		close(in)
		cmd.SetIn("in", in)
	}
	in := cmd.Lines(cmd.In("in"))
	if err := xp(in); err != nil {
		cmd.Exit(err)
	}
}
