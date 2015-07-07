/*
	Ql builtin and external xpr command.
	calculate expressions
*/
package xp

import (
	"bufio"
	"clive/cmd"
	"clive/cmd/opt"
	"errors"
	"fmt"
	"sort"
	"strings"
)

type xCmd  {
	quiet bool
	*cmd.Ctx
	*opt.Flags
}

func (x *xCmd) expr(s string) (err error) {
	defer func() {
		if x := recover(); x != nil {
			err = fmt.Errorf("failed: %s", x)
		}
	}()
	var v yySymType
	l := newLex(s)
	if debugLex {
		for c := l.Lex(&v); c != 0; c = l.Lex(&v) {
		}
		return nil
	}
	yyParse(l)
	if !x.quiet {
		x.Printf("%v\n", result)
	}
	return nil
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("expr")
	debugLex = false
	bhelp := false
	x.NewFlag("F", "report known functions and exit", &bhelp)
	x.NewFlag("D", "debug", &debugLex)
	x.NewFlag("q", "do not print values as they are evaluated", &x.quiet)
	x.Argv0 = argv[0]
	args, err := x.Parse(argv)
	if err != nil {
		x.Usage(x.Stderr)
		return err
	}
	cmd.MkNS()
	if bhelp {
		fns := []string{}
		for k := range funcs {
			fns = append(fns, k)
		}
		sort.Sort(sort.StringSlice(fns))
		for _, b := range fns {
			x.Printf("%s\n", b)
		}
		return nil
	}
	if len(args) != 0 {
		expr := strings.Join(args, " ")
		return x.expr(expr)
	}
	var sts error
	scn := bufio.NewScanner(x.Stdin)
	for scn.Scan() {
		ln := scn.Text()
		if ln == "" {
			continue
		}
		if err := x.expr(ln); err != nil {
			sts = errors.New("errors")
			x.Warn("'%s': %s", ln, err)
		}
	}
	if err := scn.Err(); err != nil {
		x.Warn("%s", err)
		sts = errors.New("errors")
	}
	if x, ok := result.(bool); sts==nil && ok {
		if x {
			return nil
		}
		return errors.New("false")
	}
	return sts
}
