package lf

import (
	"testing"
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
	args []string
	out []string
	fails bool
}

var (
	tests = []test{
		test{
			args: []string{"lf", "app/lf,"},
			out: []string{"app/lf", "app/lf/lf.go", "app/lf/lf_test.go"},
		},
		test{
			args: []string{"lf", "app/lf"},
			out: []string{"app/lf"},
		},
		test{
			args: []string{"lf", "app/lf,0", "app/lf/lf.go"},
			out: []string{"app/lf", "app/lf/lf.go"},
		},
		test{
			args: []string{"lf", "app/lf,type=-"},
			out: []string{"app/lf/lf.go", "app/lf/lf_test.go"},
		},
		test{
			args: []string{"lf", "app/lf,0", "fds", "app/lf/lf.go"},
			out: []string{"app/lf", "app/lf/lf.go"},
			fails: true,
		},
	}
)

func TestLf(t *testing.T) {
	app.New()	// prevent app.Fatal from calling dbg.Fatal
	app.Debug = testing.Verbose()
	for i := range tests {
		lt := tests[i]
		dprintf("run %v\n", lt.args)
		out := make(chan interface{})
		go func() {
			c := app.New()
			c.Args = lt.args
			defer app.Exiting()
			app.DupIO()
			app.SetIO(out, 1)
			app.Cd("/zx/sys/src/clive")
			Run()
		}()
		outs := []string{}
		for x := range out {
			d, ok := x.(zx.Dir)
			if !ok {
				dprintf("got %T %v\n", x, x)
				t.Fatalf("not a dir")
			}
			dprintf("got %T %s\n", d, d["upath"])
			outs = append(outs, d["upath"])
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

type gtest  {
	args []string
	out []string
	fails bool
}

var (
	gets = []gtest{
		gtest{
			args: []string{"lf", "-g", "app/lf"},
			out: []string{"app/lf"},
		},
		gtest{
			args: []string{"lf", "-g", "app/lf,"},
			out: []string{"app/lf", "app/lf/lf.go", "[]", "app/lf/lf_test.go", "[]"},
		},
		gtest{
			args: []string{"lf", "-g", "app/lf,0", "fds", "app/lf/lf.go", "[]"},
			fails: true,
		},
	}
)

func TestLg(t *testing.T) {
	app.New()	// prevent app.Fatal from calling dbg.Fatal
	app.Debug = testing.Verbose()
	for i := range gets {
		lt := gets[i]
		dprintf("run %v\n", lt.args)
		out := make(chan interface{})
		go func() {
			c := app.New()
			defer app.Exiting()
			app.DupIO()
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
				dprintf("got %T [%d]\n", x, len(x))
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
