/*
	grep for predicates files in the input
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx"
	"clive/zx/pred"
	"errors"
	"strings"
)

var (
	opts = opt.New("{pred}")
	ux bool
	preds []*pred.Pred
)

func gp(in <-chan interface{}, out chan<- interface{}) error {
	matched := true
	some := false
	ok := true
	for m := range in {
		switch d := m.(type) {
		case zx.Dir:
			cmd.Dprintf("got %T %s\n", m, d["path"])
			matched = false
			depth := len(zx.Elems(d["Rpath"]))
			if depth > 0 {
				depth--
			}
			var err error
			for _, p := range preds {
				matched, _, err = p.EvalAt(d, depth)
				cmd.Dprintf("%s match=%v\n",
					d.Fmt(), matched)
				if matched {
					break
				}
			}
			if err != nil {
				cmd.Warn("%s", err)
			}
			if matched {
				some = true
				ok = out <- d
			}
		default:
			cmd.Dprintf("got %T\n", d)
			if matched {
				some = true
				ok = out <- d
			}
		}
		if !ok {
			close(in, cerror(out))
		}
	}
	if !some {
		return errors.New("no match")
	}
	return cerror(in)
}

// Run gp in the current app context.
func main() {
	c := cmd.AppCtx()
	cmd.UnixIO("err")
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("u", "use unix out", &ux)
	args, err := opts.Parse()
	if err != nil {
		cmd.Warn("%s", err)
		opts.Usage()
	}
	if len(args) == 0 {
		cmd.Warn("missing predicate")
		opts.Usage()
	}
	if ux {
		cmd.UnixIO("out")
	}
	for _, a := range args {
		a = strings.TrimSpace(a)
		p, err := pred.New(a)
		if err != nil {
			cmd.Fatal("pred: <%s>: %s", a, err)
		}
		preds = append(preds, p)
	}
	in := cmd.In("in")
	out := cmd.Out("out")
	cmd.Exit(gp(in, out))
}
