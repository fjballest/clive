package auth

import (
	"clive/dbg"
	"clive/ch"
	"encoding/binary"
	"testing"
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
	c1, c2 := ch.NewPipePair(5)
	go func() {
		var nb [8]byte
		binary.LittleEndian.PutUint64(nb[0:], 33)
		c1.Out <- nb[:]
		c1.Out <- nb[:]
	}()
	_, err := AtClient(c2, "", "foo")
	if err == nil {
		t.Fatal(err)
	}
	printf("err is %v\n", err)
	if cerror(c2.In) == nil || cerror(c2.Out) == nil {
		t.Fatal("chans are ok")
	}
}
