package run

import (
	"bytes"
	"clive/ch"
	"clive/cmd"
	"clive/dbg"
	"fmt"
	"strings"
	"testing"
	"time"
)

var (
	debug  bool
	printf = dbg.FlagPrintf(&debug)
)

func TestUnixCmd(t *testing.T) {
	debug = testing.Verbose()

	c, err := UnixCmd("seq", "1024")
	if err != nil {
		t.Fatalf("sts %v", err)
	}
	out := ""
	var buf bytes.Buffer
	for i := 1; i <= 1024; i++ {
		fmt.Fprintf(&buf, "%d\n", i)
	}
	for x := range ch.Merge(c.Out, c.Err) {
		switch x := x.(type) {
		case []byte:
			printf("-> [%s]\n", x)
			out += string(x)
		default:
			t.Fatalf("got type %T", x)
		}
	}
	printf("sts %v\n", cerror(c.Err))
	if out != buf.String() {
		t.Fatalf("bad output")
	}
	err = c.Wait()
	printf("sts %v\n", err)
	if err != nil {
		t.Fatalf("did fail")
	}
}

func TestUnixPipe(t *testing.T) {
	debug = testing.Verbose()

	c, err := PipeToUnix("tr", "a-z", "A-Z")
	if err != nil {
		t.Fatalf("sts %v", err)
	}
	go func() {
		for i := 1; i < 1024; i++ {
			c.In <- []byte(fmt.Sprintf("xxxx%d\n", i))
		}
		close(c.In)
	}()
	out := ""
	var buf bytes.Buffer
	for i := 1; i < 1024; i++ {
		fmt.Fprintf(&buf, "XXXX%d\n", i)
	}
	for x := range ch.Merge(c.Out, c.Err) {
		switch x := x.(type) {
		case []byte:
			printf("-> [%s]\n", x)
			out += string(x)
		default:
			t.Fatalf("got type %T", x)
		}
	}
	printf("sts %v\n", cerror(c.Err))
	if out != buf.String() {
		t.Fatalf("bad output")
	}
	err = c.Wait()
	printf("sts %v\n", err)
	if err != nil {
		t.Fatalf("did fail")
	}
}

func TestUnixGroup(t *testing.T) {
	debug = testing.Verbose()

	if testing.Short() {
		t.Skip("long test")
	}
	c, err := UnixCmd("sh", "-c", "for x in `seq 16` ; do sleep 1 ; echo $x; done")
	if err != nil {
		t.Fatalf("sts %v", err)
	}
	out := ""
	var buf bytes.Buffer
	for i := 1; i <= 16; i++ {
		fmt.Fprintf(&buf, "%d\n", i)
	}
	for x := range ch.GroupBytes(ch.Merge(c.Out, c.Err), 2*time.Second, 1024) {
		switch x := x.(type) {
		case []byte:
			printf("-> [%s]\n", x)
			out += string(x)
		default:
			t.Fatalf("got type %T", x)
		}
	}
	printf("sts %v\n", cerror(c.Err))
	if out != buf.String() {
		t.Fatalf("bad output")
	}
	err = c.Wait()
	printf("sts %v\n", err)
	if err != nil {
		t.Fatalf("did fail")
	}
}

func TestCliveCmd(t *testing.T) {
	debug = testing.Verbose()

	c, err := Cmd("eco", "-m", "a", "b", "c")
	if err != nil {
		t.Fatalf("sts %v", err)
	}
	out := []string{}
	for x := range ch.Merge(c.Out, c.Err) {
		switch x := x.(type) {
		case []byte:
			printf("-> [%s]\n", x)
			out = append(out, string(x))
		default:
			t.Fatalf("got type %T", x)
		}
	}
	if strings.Join(out, "|") != "a|b|c" {
		t.Fatalf("bad output")
	}
	err = c.Wait()
	printf("sts %v\n", err)
	if err != nil {
		t.Fatalf("did fail")
	}
}

func TestBadCliveCmd(t *testing.T) {
	debug = testing.Verbose()

	c, err := Cmd("eco", "-?")
	if err != nil {
		t.Fatalf("sts %v", err)
	}
	out := []string{}
	for x := range ch.Merge(c.Out, c.Err) {
		switch x := x.(type) {
		case []byte:
			printf("-> [%s]\n", x)
			out = append(out, string(x))
		default:
			t.Fatalf("got type %T", x)
		}
	}
	err = c.Wait()
	printf("sts %v\n", err)
	if err == nil {
		t.Fatalf("didn't fail")
	}
}

func TestCliveOutChan(t *testing.T) {
	debug = testing.Verbose()
	out2 := make(chan face{})
	adj := func(c *cmd.Ctx) {
		c.ForkEnv()
		c.ForkNS()
		c.ForkDot()
		c.SetOut("out2", out2)
	}
	c, err := CtxCmd(adj, "eco", "-o", "out2", "-m", "a", "b", "c")
	if err != nil {
		t.Fatalf("sts %v", err)
	}
	out := []string{}
	for x := range ch.Merge(c.Out, c.Err, out2) {
		switch x := x.(type) {
		case []byte:
			printf("-> [%s]\n", x)
			out = append(out, string(x))
		default:
			t.Fatalf("got type %T", x)
		}
	}
	err = c.Wait()
	printf("sts %v\n", err)
	if err != nil {
		t.Fatalf("did fail")
	}
}

func TestCliveIn2Chan(t *testing.T) {
	debug = testing.Verbose()
	in := make(chan face{}, 3)
	adj := func(c *cmd.Ctx) {
		c.ForkEnv()
		c.ForkNS()
		c.ForkDot()
		c.SetIn("in2", in)
	}
	c, err := CtxCmd(adj, "eco", "-i", "in2", "-m", "a", "b", "c")
	if err != nil {
		t.Fatalf("sts %v", err)
	}
	for _, m := range []string{"x", "y", "z"} {
		in <- []byte(m)
	}
	close(in)
	out := []string{}
	for x := range ch.Merge(c.Out, c.Err) {
		switch x := x.(type) {
		case []byte:
			printf("-> [%s]\n", x)
			out = append(out, string(x))
		default:
			t.Fatalf("got type %T", x)
		}
	}
	err = c.Wait()
	printf("sts %v\n", err)
	if err != nil {
		t.Fatalf("did fail")
	}
	if strings.Join(out, "|") != "a|b|c|x|y|z" {
		t.Fatalf("bad output")
	}
}

func TestCliveInChan(t *testing.T) {
	debug = testing.Verbose()
	adj := func(c *cmd.Ctx) {
		c.ForkEnv()
		c.ForkNS()
		c.ForkDot()
	}
	c, err := PipeToCtx(adj, "eco", "-i", "in", "-m", "a", "b", "c")
	if err != nil {
		t.Fatalf("sts %v", err)
	}
	for _, m := range []string{"x", "y", "z"} {
		c.In <- []byte(m)
	}
	close(c.In)
	out := []string{}
	for x := range ch.Merge(c.Out, c.Err) {
		switch x := x.(type) {
		case []byte:
			printf("-> [%s]\n", x)
			out = append(out, string(x))
		default:
			t.Fatalf("got type %T", x)
		}
	}
	err = c.Wait()
	printf("sts %v\n", err)
	if err != nil {
		t.Fatalf("did fail")
	}
	if strings.Join(out, "|") != "a|b|c|x|y|z" {
		t.Fatalf("bad output")
	}
}
