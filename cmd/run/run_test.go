package run

import (
	"testing"
	"clive/ch"
	"clive/dbg"
	"bytes"
	"fmt"
	"time"
)

var (
	debug bool
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
}

func TestUnixPipe(t *testing.T) {
	debug = testing.Verbose()

	in, c, err := PipeToUnix("tr", "a-z", "A-Z")
	if err != nil {
		t.Fatalf("sts %v", err)
	}
	go func() {
		for i := 1; i < 1024; i++ {
			in <- []byte(fmt.Sprintf("xxxx%d\n", i))
		}
		close(in)
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
}

