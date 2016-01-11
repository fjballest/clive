package auth

import (
	"clive/ch"
	"clive/dbg"
	"encoding/binary"
	"testing"
	"clive/net"
)

var debug = testing.Verbose()
var printf = dbg.FlagPrintf(&debug)

func ExampleAtClient() {
	var c ch.Conn

	// Assume c comes from dialing a server.
	// Authenticate c for domain lsub, to speak the finder
	// and zx protocols.
	ai, err := AtClient(c, "lsub", "finder", "zx")
	if err != nil {
		printf("auth failed: %s\n", err)
		return
	}
	printf("user %s speaking for %s",
		ai.Uid, ai.SpeaksFor)
	printf("protocols understood: %v", ai.Proto)
	printf("auth ok? %v", ai.Ok)
}

func ExampleAtServer() {
	var c ch.Conn

	// Assume we are listening for connections and get
	// c as a fresh connection from a client.

	// Authenticate c for domain lsub, to speak the finder
	// and zx protocols. Assume we dialed.
	ai, err := AtServer(c, "lsub", "finder", "zx")
	if err != nil {
		printf("auth failed: %s\n", err)
		return
	}
	printf("user %s speaking for %s",
		ai.Uid, ai.SpeaksFor)
	printf("protocols understood: %v", ai.Proto)
	printf("auth ok? %v", ai.Ok)
}

func TestAuth(t *testing.T) {
	debug = testing.Verbose()
	// Do it a few times...
	for i := 0; i < 3; i++ {
		c1, c2 := ch.NewPipePair(5)
		ec := make(chan error, 1)
		go func() {
			_, err := AtClient(c1, "", "foo")
			ec <- err
		}()
		if _, err := AtServer(c2, "", "foo"); err != nil {
			t.Fatal(err)
		}
		if err := <-ec; err != nil {
			t.Fatal(err)
		}
		c1.Out <- []byte("hi")
		c2.Out <- []byte("there")
		d := string((<-c2.In).([]byte))
		if string(d) != "hi" {
			t.Fatal("bad msg")
		}
		d = string((<-c1.In).([]byte))
		if string(d) != "there" {
			t.Fatal("bad msg")
		}
	}
}

func TestBadAuth(t *testing.T) {
	debug = testing.Verbose()
	c1, c2 := ch.NewPipePair(5)
	go func() {
		var nb [8]byte
		binary.LittleEndian.PutUint64(nb[0:], 33)
		c1.Out <- nb[:]
		c1.Out <- nb[:]
	}()
	_, err := AtClient(c2, "", "foo")
	if err == nil {
		t.Fatal("didn't fail")
	}
	printf("err is %v\n", err)
	if cerror(c2.In) == nil || cerror(c2.Out) == nil {
		t.Fatal("chans are ok")
	}
}

func TestMuxAuth(t *testing.T) {
	addr := "unix!local!6679"
	debug = testing.Verbose()
	printf("serving...\n")
	mc, ec, err := net.MuxServe(addr)
	if err != nil {
		t.Fatal(err)
	}
	donec := make(chan bool)
	failed := false
	go func() {
		for mx := range mc {
			mx.Debug = testing.Verbose()
			printf("new muxed client %q\n", mx.Tag)
			mx := mx
			c := <- mx.In
			printf("new muxed conn %s\n", c.Tag)
			ai, err := AtServer(c, "", "foo")
			if err != nil {
				printf("auth failed at server: %s\n", err)
				failed = true
			} else {
				printf("server ai %v\n", ai)
			}
			close(c.In)
			close(c.Out)
		}
		close(donec)
	}()
	go func() {
		<-ec
		printf("serve mux done: %v\n", cerror(ec))
	}()
	printf("dialing...\n")
	mx, err := net.MuxDial(addr)
	if err != nil {
		t.Fatal(err)
	}
	printf("now talking...\n")
	call := mx.Rpc()
	ai, err := AtClient(call, "", "foo")
	if err != nil {
		t.Fatalf("auth failed with %s", err)
	}
	if failed {
		t.Fatal("server auth failed")
	}
	printf("client ai %v\n", ai)
	close(call.Out)
	<-call.In
	printf("closing...\n")
	close(ec)
	<-donec
}

func TestMuxDisabledAuth(t *testing.T) {
	addr := "unix!local!6789"
	debug = testing.Verbose()
	printf("serving...\n")
	mc, ec, err := net.MuxServe(addr)
	if err != nil {
		t.Fatal(err)
	}
	donec := make(chan bool)
	failed := false
	go func() {
		for mx := range mc {
			mx.Debug = testing.Verbose()
			printf("new muxed client %q\n", mx.Tag)
			mx := mx
			c := <- mx.In
			printf("new muxed conn %s\n", c.Tag)
			ai, err := NoneAtServer(c, "", "foo")
			if err == nil || err.Error() != "auth disabled" {
				printf("auth failed at server: %s\n", err)
				failed = true
			} else {
				printf("server ai %v\n", ai)
			}
			close(c.In)
			close(c.Out)
		}
		close(donec)
	}()
	go func() {
		<-ec
		printf("serve mux done: %v\n", cerror(ec))
	}()
	printf("dialing...\n")
	mx, err := net.MuxDial(addr)
	if err != nil {
		t.Fatal(err)
	}
	printf("now talking...\n")
	call := mx.Rpc()
	ai, err := AtClient(call, "", "foo")
	if err == nil || err.Error() != "auth disabled" {
		t.Fatalf("auth failed with %s", err)
	}
	if failed {
		t.Fatal("server auth failed")
	}
	printf("client ai %v\n", ai)
	close(call.Out)
	<-call.In
	printf("closing...\n")
	close(ec)
	<-donec
}

