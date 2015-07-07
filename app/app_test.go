package app

import (
	"testing"
	"time"
	"os"
	"clive/dbg"
)

// Example app usage.
func ExampleGo() {
	fn := func() {
		dprintf("this is a new ctx")
		Exits(nil)
	}
	ctx := Go(fn, "argv0", "arg1", "arg2")
	dprintf("new ctx id is %d\n", ctx.Id)
	<-ctx.Wait
	dprintf("app status is %v\n", ctx.Sts)
}

// Example app usage.
func ExampleNew() {
	// This is the main "program" of the app.
	defer Exiting()
	New()
	// ... new procs inherit the ctxt and might call
	Fatal("oops")
}

func TestSigs(t *testing.T) {
	t.Skip("this is not a test")
	Debug = testing.Verbose()
	defer dprintf("A\n")
	defer Exiting()
	defer dprintf("B\n")
	rc := make(chan bool)
	c := New()
	Handle(All, func(sig string) bool {
		if sig != "exit" {
			return false
		}
		dprintf("ctx %d is exiting\n", c.Id)
		return true
	})
	Handle(All, func(sig string) bool {
		if sig != "intr" {
			return false
		}
		dprintf("ctx %d is intred\n", c.Id)
		return true
	})
	go func() {
		c := New()
		defer Exiting()
		Handle(All, func(sig string) bool {
			if sig != "exit" {
				return false
			}
			dprintf("ctx %d is exiting\n", c.Id)
			return true
		})
		defer func() {
			dprintf("id %d exited %v\n", c.Id, Exited())
		}()
		time.Sleep(60*time.Second)
		rc <- true
		Fatal()
		dprintf("hi\n")
	}()
	<-rc
	defer func() {
		dprintf("id %d exited %v\n", c.Id, Exited())
	}()
	dprintf("id %d exited %v\n", c.Id, Exited())
	Fatal(nil)
	dprintf("hi\n")
}

func TestPrints(t *testing.T) {
	t.Skip("this is not a test")
	Debug = testing.Verbose()
	defer Exiting()
	rc := make(chan bool)
	c := New()
	c.Args = []string{"proc1"}
	go func() {
		c := New()
		c.Args = []string{"proc2"}
		defer Exiting()
		Printf("print2\n")
		Warn("warn2\n")
		rc <- true
		Fatal()
		dprintf("end2\n")
	}()
	Warn("warn\n")
	<-rc
	os.Stdin, _ = os.Open("app_test.go")
	SetIO(OSIn(), 0)
	in := In()
	for i := 0; i < 3; i++ {
		x := <- in
		if x, ok := x.([]byte); ok {
			dprintf("got [%s]\n", string(x))
		}
	}
	dprintf("in sts %v\n", cerror(in))
	Printf("print\n")
	Fatal("fatal")
	dprintf("end\n")
}

func TestWait(t *testing.T) {
	os.Args[0] = "app.test"
	t.Skip("this is not a test")
	Debug = testing.Verbose()
	defer Exiting()
	c := New()
	c.Args = []string{"proc1"}
	wc := make(chan *Ctx)
	Printf("getting ctx\n")
	go func() {
		c := New()
		c.Args = []string{"proc2"}
		defer Exiting()
		defer dbg.Warn("D2")
		wc <- c
		Printf("print2\n")
		Fatal("ouch")
		dprintf("end2\n")
	}()
	cc := <-wc
	Printf("got ctx\n")
	<-cc.Wait
	Warn("cc sts %v\n", cerror(cc.Wait))
	Fatal("fatal")
	dprintf("end\n")
}

type pr int

func (pr) Prompt() string {
	return "> "
}

var prompt pr

func TestReadIntr(t *testing.T) {
	t.Skip("this is not a test")
	Debug = testing.Verbose()
	defer Exiting()
	rc := make(chan bool)
	os.Stdin, _ = os.Open("/dev/tty")
	c := New()
	c.Args = []string{"proc1"}
	go func() {
		c := New()
		c.Args = []string{"proc2"}
		defer Exiting()
		Printf("print2\n")
		Warn("warn2\n")
		rc <- true
		Fatal()
		dprintf("end2\n")
	}()
	Warn("warn\n")
	<-rc
	SetIO(PromptIn(prompt), 0)
	in := In()
	for i := 0; i < 5; i++ {
		x := <- in
		if x, ok := x.([]byte); ok {
			dprintf("got [%s]\n", string(x))
		}
		if x, ok := x.(error); ok {
			dprintf("err %s\n", x)
		}
	}
	dprintf("in sts %v\n", cerror(in))
	Printf("print\n")
	Fatal("fatal")
	dprintf("end\n")
}

