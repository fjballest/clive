package pf

import (
	"testing"
	"clive/app/lf"
	"clive/app"
	"clive/dbg"
	"os"
	"clive/zx"
	"strings"
)

var (
	debug bool
	dprintf = dbg.FuncPrintf(os.Stdout, testing.Verbose)
)

type test  {
	largs []string
	args []string
	out []string
	fails bool
}

var (
	tests = []test{
		test{
			largs: []string{"lf", "-g", "app/lf,"},
			args: []string{"pf", "-d"},
		},
		test{
			largs: []string{"lf", "-g", "app/lf"},
			args: []string{"pf", "-dp"},
		},
		test{
			largs: []string{"lf", "-g", "app/lf,0", "app/lf/lf.go"},
			args: []string{"pf", "-dl"},
		},
		test{
			largs: []string{"lf", "-g", "app/lf,type=-"},
			args: []string{"pf", "-da"},
		},
		test{
			largs: []string{"lf", "-g", "app/lf,0", "fds", "app/lf/lf.go"},
			args: []string{"pf", "-l"},
			fails: true,
		},
	}
)

func TestPf(t *testing.T) {
	app.New()	// prevent app.Fatal from calling dbg.Fatal
	//app.Debug = testing.Verbose()
	for i := range tests {
		lt := tests[i]
		dprintf("\nrun %v | %v\n", lt.largs, lt.args)
		// lf
		pipe := make(chan interface{})
		out := make(chan interface{})
		go func() {
			c := app.New()
			defer app.Exiting()
			app.DupIO()
			app.SetIO(pipe, 1)
			app.Cd("/zx/sys/src/clive")
			c.Args = lt.largs
			lf.Run()
		}()
		// pf
		go func() {
			c := app.New()
			defer app.Exiting()
			app.DupIO()
			app.SetIO(pipe, 0)
			app.SetIO(out, 1)
			app.Cd("/zx/sys/src/clive")
			c.Args = lt.args
			Run()
		}()
		outs := []string{}
		nbytes := 0
		for x := range out {
			switch x := x.(type) {
			case zx.Dir:
				if nbytes > 0 {
					outs = append(outs, "[]")
				}
				nbytes = 0
				dprintf("got %T %s\n", x, x["upath"])
				outs = append(outs, x["upath"])
			case []byte:
				nbytes += len(x)
				dprintf("got %T [%s]\n", x, x)
			case error:
				if nbytes > 0 {
					outs = append(outs, "[]")
				}
				nbytes = 0
				dprintf("got %T %v\n", x, x)
				outs = append(outs, "err")
			default:
				dprintf("got %T %v\n", x, x)
				t.Fatalf("unexpected type %T", x)
			}
		}
		if nbytes >0 {
			outs = append(outs, "[]")
		}
		err := cerror(out)
		dprintf("got sts %v\n", err)
		if lt.fails && err == nil {
			t.Fatalf("didn't fail")
		}
		if !lt.fails && err != nil {
			t.Fatalf("failed: %s", err)
		}
		if lt.out != nil && strings.Join(lt.out, " ") != strings.Join(outs, " ") {
			t.Fatalf("bad output %#v", outs)
		}
		if lt.out == nil {
			dprintf("out: %#v\n", outs)
		}
	}
}
