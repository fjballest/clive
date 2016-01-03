package net

import (
	"testing"
	"os"
	"crypto/tls"
	"clive/dbg"
	"time"
)

type as struct {
	addr, net, mach, svc string
}

func TestParseAddr(t *testing.T) {
	addrs := []as{
		{"", "*", "*", "zx"},
		{"foo", "*", "foo", "zx"},
		{"!svc", "tcp", "*", "svc"},
		{"*!svc", "tcp", "*", "svc"},
		{"mach!svc", "tcp", "mach", "svc"},
		{"what!*!svc", "what", "*", "svc"},
		{"!*!svc", "*", "*", "svc"},
		{"*!foo!svc", "*", "foo", "svc"},
		{"tcp!*!svc", "tcp", "*", "svc"},
		{"tcp!*!svc!x", "tcp", "*", "svc"},
	}
	for _, a := range addrs {
		n, m, s := ParseAddr(a.addr)
		t.Logf("\t\t{%q, %q, %q, %q},\n", a.addr, n, m, s)
		if n != a.net || m != a.mach || s != a.svc {
			t.Fatalf("was not %v", a)
		}
	}
}

func TestIsLocal(t *testing.T) {
	addrs := []string{
		"127.0.0.1",
		"[::1]",
		"localhost",
	}
	for _, a := range addrs {
		if !IsLocal(a) {
			t.Fatalf("%s is not local", a)
		}
	}
}

func testConn(t *testing.T, addr string, clicfg, srvcfg *tls.Config) {
	verb := testing.Verbose()
	printf := dbg.FlagPrintf(&verb)
	printf("serving...\n")
	sc, ec, err := Serve(addr, srvcfg)
	if err != nil {
		t.Fatal(err)
	}
	donec := make(chan bool)
	go func() {
		for cc := range sc {
			printf("new conn %s\n", cc.Tag)
			for m := range cc.In {
				printf("new msg %s: %v\n", cc.Tag, m)
				cc.Out <- m
			}
			err := cerror(cc.In)
			printf("gone conn %s %v\n", cc.Tag, err)
			close(cc.Out, err)
		}
		close(donec)
	}()
	go func() {
		<-ec
		printf("serve done: %v\n", cerror(ec))
	}()
	printf("dialing...\n")
	c, err := Dial(addr, clicfg)
	if err != nil {
		t.Fatal(err)
	}
	printf("now talking...\n")
	reqs := [...]string{"hi there", "again", "and again"}
	for _, r := range reqs {
		if ok := c.Out <- r; !ok {
			t.Fatalf("send: %v", cerror(c.Out))
		}
		if rep, ok := <-c.In; !ok {
			t.Fatalf("send: %v", cerror(c.In))
		} else if rep.(string) != r {
			t.Fatal("bad reply")
		}
	}
	close(c.Out, "oops")
	// When using tls, we don't have half close and must close it directly,
	// which means we don't get the closing error.
	if clicfg == nil {
		if m, ok := <-c.In; !ok {
			t.Fatal("didn't get the err echo")
		} else if m, ok := m.(error); !ok || m.Error() != "oops" {
			t.Fatal("didn't get the close error")
		}
	}
	if m, ok := <-c.In; ok {
		t.Fatal("could recv %v", m)
	}
	if clicfg == nil && cerror(c.In) != nil {
		t.Fatalf("bad error %v", cerror(c.In))
	}
	printf("closing...\n")
	close(ec)
	<-donec
}

func TestUnixConn(t *testing.T) {
	os.Remove("/tmp/clive.6667")
	os.Args[0] = "net.test"
	testConn(t, "unix!local!6667", nil, nil)
}

func TestTCPConn(t *testing.T) {
	os.Args[0] = "net.test"
	testConn(t, "tcp!local!6667", nil, nil)
}

func TestGoTLS(t *testing.T) {
	os.Args[0] = "net.test"
	ccfg, err := TLSCfg("/Users/nemo/.ssh/client")
	if err != nil {
		t.Skip("no certs found, TLS test skip")
	}
	scfg, err := TLSCfg("/Users/nemo/.ssh/server")
	if err != nil {
		t.Skip("no certs found, TLS test skip")
	}
	l, err := tls.Listen("tcp", ":6669", scfg)
	if err != nil {
		t.Fatal(err)
	}
	wc := make(chan bool, 1)
	go func() {
		a, err := l.Accept()
		if err != nil {
			panic("XXX")
		}
		buf := make([]byte, 1024)
		n, _ := a.Read(buf)
		a.Write(buf[:n])
		a.Close()
		l.Close()
		wc <- true
	}()
	fd, err := tls.Dial("tcp", "127.0.0.1:6669", ccfg)
	if err != nil {
		t.Fatal(err)
	}
	fd.Write([]byte("hi there"))
	buf := make([]byte, 1024)
	n, _ := fd.Read(buf)
	t.Logf("did read %s", string(buf[:n]))
	fd.Close()
	<-wc
}

func TestTLSConn(t *testing.T) {
	os.Args[0] = "net.test"
	ccfg, err := TLSCfg("/Users/nemo/.ssh/client")
	if err != nil {
		t.Skip("no certs found, TLS test skip")
	}
	scfg, err := TLSCfg("/Users/nemo/.ssh/server")
	if err != nil {
		t.Skip("no certs found, TLS test skip")
	}
	testConn(t, "tcp!127.0.0.1!6669", ccfg, scfg)
}

func testMux(t *testing.T, addr string, clicfg, srvcfg *tls.Config) {
	verb := testing.Verbose()
	printf := dbg.FlagPrintf(&verb)
	printf("serving...\n")
	mc, ec, err := MuxServe(addr, srvcfg)
	if err != nil {
		t.Fatal(err)
	}
	donec := make(chan bool)
	go func() {
		for mx := range mc {
			mx.Debug = verb
			printf("new muxed client %q\n", mx.Tag)
			mx := mx
			go func() {
				for c := range mx.In {
					printf("new muxed conn %s\n", c.Tag)
					for m := range c.In {
						printf("new msg %s: %v\n", c.Tag, m)
						if c.Out != nil {
							c.Out <- m
						}
					}
					err := cerror(c.In)
					printf("gone conn %s %v\n", c.Tag, err)
					close(c.Out, "oops")
				}
				printf("gone client %q %v\n", mx.Tag, cerror(mx.In))
			}()
		}
		close(donec)
	}()
	go func() {
		<-ec
		printf("serve mux done: %v\n", cerror(ec))
	}()
	printf("dialing...\n")
	mx, err := MuxDial(addr, clicfg)
	if err != nil {
		t.Fatal(err)
	}
	printf("now talking...\n")
	reqs := [...]string{"hi there", "again", "and again"}
	call := mx.Rpc()
	for _, r := range reqs {
		if ok := call.Out <- r; !ok {
			t.Fatalf("send: %v", cerror(call.Out))
		}
		if rep, ok := <-call.In; !ok {
			t.Fatalf("send: %v", cerror(call.In))
		} else if rep.(string) != r {
			t.Fatal("bad reply")
		}
	}
	close(call.Out)
	if m, ok := <-call.In; ok {
		t.Fatal("could get a reply %v", m)
	}
	if e := cerror(call.In); e == nil || e.Error() != "oops" {
		t.Fatalf("bad error %v", cerror(call.In))
	}
	printf("closing...\n")
	close(ec)
	<-donec
}

func TestUnixMux(t *testing.T) {
	os.Remove("/tmp/clive.6667")
	os.Args[0] = "net.test"
	testMux(t, "unix!local!6667", nil, nil)
}

func TestTCPMux(t *testing.T) {
	os.Args[0] = "net.test"
	testMux(t, "tcp!local!6667", nil, nil)
}

func TestTLSMux(t *testing.T) {
	verb := testing.Verbose()
	printf := dbg.FlagPrintf(&verb)
	os.Args[0] = "net.test"
	ccfg, err := TLSCfg("/Users/nemo/.ssh/client")
	if err != nil {
		t.Skip("no certs found, TLS test skip")
	}
	scfg, err := TLSCfg("/Users/nemo/.ssh/server")
	if err != nil {
		t.Skip("no certs found, TLS test skip")
	}
	ClientTLSCfg = ccfg
	ServerTLSCfg = scfg
	testMux(t, "tls!local!6667", nil, nil)
	if false {
		printf("pkill -QUIT net.test\n")
		time.Sleep(60 * time.Second)
	}
}

