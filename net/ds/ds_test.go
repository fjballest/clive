package ds

import (
	"clive/dbg"
	"clive/nchan"
	"clive/net/fifo"
	"fmt"
	"os"
	"testing"
)

/*
	As of today:
	go test -bench .
	BenchmarkPipeRaw              	  500000	      3304 ns/op
	BenchmarkPipeBIO              	 1000000	      2362 ns/op

	compare to
	BenchmarkFifoRaw              	   10000	    196737 ns/op
	BenchmarkFifoBIO              	   10000	    205755 ns/op


*/

var printf = dbg.FuncPrintf(os.Stdout, testing.Verbose)

func useConn(c nchan.Conn) {
}

func ExampleDial() {
	// dial the ns service on the fifo network.
	c, err := Dial("fifo!*!ns")
	if err != nil {
		printf("had error %s\n", err)
		return
	}
	// You may use now c.In and c.Out to use the Conn.
	useConn(c)

	// dial the ns service on any known network.
	// pipe and fifo are tried out first.
	c, err = Dial("*!*!ns")
	if err != nil {
		printf("had error %s\n", err)
		return
	}
	// You may use now c.In and c.Out to use the Conn.
	useConn(c)
}

func TestBadTcpDial(t *testing.T) {
	c, err := Dial("tcp!blah!zx")
	if err == nil {
		t.Fatal("did not fail")
	}
	printf("failed with err %s\n", err)
	close(c.In)
	close(c.Out)
}

func TestFifoDial(t *testing.T) {
	defer os.RemoveAll(fifo.Dir)

	hs, hc := fifo.NewChanHandler()
	// This is an echo server from inb to outb.
	ec := make(chan error, 1)
	go func() {
		h := <-hc
		if h == nil {
			ec <- nil
			return
		}
		defer close(h.Out)
		for m := range h.In {
			if ok := h.Out <- m; !ok {
				ec <- fmt.Errorf("send: %v", cerror(h.Out))
				return
			}
			printf("srv: msg %s\n", string(m))
		}
		printf("srv: done %v\n", cerror(h.In))
		close(h.Out, cerror(h.In))
		ec <- nil
	}()

	s := fifo.New("test", "ftest", hs)
	s.Verbose = testing.Verbose()
	s.Debug = s.Verbose
	if err := s.Serve(); err != nil {
		t.Fatalf("serve: %v", err)
	}

	cc, err := Dial("fifo!*!ftest")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		cc.Out <- []byte(fmt.Sprintf("<%d>", i))
		msg := string(<-cc.In)
		printf("got %s back\n", msg)
		if msg != fmt.Sprintf("<%d>", i) {
			t.Fatal("msg does not match")
		}
	}
	close(cc.In)
	close(cc.Out)
	s.Stop(true)
	if err := <-ec; err != nil {
		t.Fatal(err)
	}
}

func TestLocal(t *testing.T) {
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

type as struct {
	addr, net, mach, svc string
}

func ExampleParseAddr() {
	net, mach, svc := ParseAddr("*!localhost!8080")
	fmt.Printf("%s %s %s\n", net, mach, svc)
	net, mach, svc = ParseAddr("fifo!*!zx!/path")
	fmt.Printf("%s %s %s\n", net, mach, svc)
	// Outputs:
	// * localhost 8080
	// fifo * zx
}

func TestParseAddr(t *testing.T) {
	addrs := []as{
		{"", "*", "*", "*"},
		{"foo", "*", "*", "foo"},
		{"svc", "*", "*", "svc"},
		{"!svc", "tcp", "*", "svc"},
		{"*!svc", "tcp", "*", "svc"},
		{"mach!svc", "tcp", "mach", "svc"},
		{"what!*!svc", "what", "*", "svc"},
		{"!*!svc", "*", "*", "svc"},
		{"*!foo!svc", "*", "foo", "svc"},
		{"tcp!*!svc", "tcp", "*", "svc"},
		{"tcp!*!svc!x", "tcp", "*", "svc"},
	}
	newrun := false
	for _, a := range addrs {
		n, m, s := ParseAddr(a.addr)
		printf("\t\t{%q, %q, %q, %q},\n", a.addr, n, m, s)
		if !newrun && (n != a.net || m != a.mach || s != a.svc) {
			t.Fatalf("was not %v", a)
		}
	}
}

func TestServeFifoNet(t *testing.T) {
	fifo.Dir = "/tmp/dsfifotest"
	os.RemoveAll(fifo.Dir)
	defer os.RemoveAll(fifo.Dir)

	hc, ec, err := Serve("test", "fifo!*!ftest")
	if err != nil {
		t.Fatal(err)
	}

	// one echo server
	donec := make(chan error, 1)
	go func() {
		h := <-hc
		if h == nil {
			donec <- nil
			return
		}
		defer close(h.Out)
		for m := range h.In {
			if ok := h.Out <- m; !ok {
				donec <- fmt.Errorf("send: %v", cerror(h.Out))
				return
			}
			printf("srv: msg %s\n", string(m))
		}
		printf("srv: done %v\n", cerror(h.In))
		close(h.Out, cerror(h.In))
		donec <- nil
	}()

	cc, err := Dial("ftest")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		cc.Out <- []byte(fmt.Sprintf("<%d>", i))
		msg := string(<-cc.In)
		printf("got %s back\n", msg)
		if msg != fmt.Sprintf("<%d>", i) {
			t.Fatal("msg does not match")
		}
	}
	close(cc.In)
	close(cc.Out)
	close(ec, "done")
	if err := <-donec; err != nil {
		t.Fatal(err)
	}
	// TODO: sleep and make sure services are gone.
	// I checked this out by hand but it's not in the test.
}

func TestServePipeNet(t *testing.T) {
	os.Args[0] = "test"
	// fifo is not used, but just in case we have a bug.
	fifo.Dir = "/tmp/dsfifotest"
	os.RemoveAll(fifo.Dir)
	defer os.RemoveAll(fifo.Dir)

	hc, ec, err := Serve("test", "pipe!*!ftest")
	if err != nil {
		t.Fatal(err)
	}

	// one echo server
	donec := make(chan error, 1)
	go func() {
		h := <-hc
		if h == nil {
			donec <- nil
			return
		}
		defer close(h.Out)
		for m := range h.In {
			if ok := h.Out <- m; !ok {
				donec <- fmt.Errorf("send: %v", cerror(h.Out))
				return
			}
			printf("srv: msg %s\n", string(m))
		}
		printf("srv: done %v\n", cerror(h.In))
		close(h.Out, cerror(h.In))
		donec <- nil
	}()

	cc, err := Dial("pipe!*!ftest")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		cc.Out <- []byte(fmt.Sprintf("<%d>", i))
		msg := string(<-cc.In)
		printf("got %s back\n", msg)
		if msg != fmt.Sprintf("<%d>", i) {
			t.Fatal("msg does not match")
		}
	}
	close(cc.In)
	close(cc.Out)
	close(ec, "done")
	if err := <-donec; err != nil {
		t.Fatal(err)
	}
	/* TODO: sleep and make sure services are gone.
	   I checked this out by hand but it's not in the test.
	time.Sleep(time.Second)
	printf("pipes: %v\n", pipes)
	*/
}

func serveAClient(c *nchan.Conn) {
}

func ExampleServe() {
	// name our service and indicate which TCP port such name has.
	DefSvc("ftest", "6666")

	clic, ec, err := Serve("test", "*!*!ftest")
	if err != nil {
		printf("couldn't serve: %s", err)
		return
	}
	// Serve the first three clients one after another
	for i := 0; i < 3; i++ {
		cli := <-clic
		if cli == nil {
			printf("server network error: %s\n", cerror(clic))
			return
		}
		// cli.In and cli.Out are the chans from/to the client
		// you might close any of them if you want, perhaps with an error.
		printf("serving client %s\n", cli.Tag)
		serveAClient(cli)
	}
	// Terminate operation
	close(ec, "I'm done")
}

func TestServeAllNets(t *testing.T) {
	// fifo is not used, but just in case we have a bug.
	fifo.Dir = "/tmp/dsfifotest"
	os.RemoveAll(fifo.Dir)
	defer os.RemoveAll(fifo.Dir)

	DefSvc("ftest", "6666")
	hc, ec, err := Serve("test", "ftest")
	if err != nil {
		t.Fatal(err)
	}

	// four echo servers
	donec := make(chan error, 4)
	for i := 0; i < 4; i++ {
		go func() {
			h := <-hc
			printf("srv: client %v\n", h)
			if h == nil {
				donec <- nil
				return
			}
			defer close(h.Out)
			for m := range h.In {
				if ok := h.Out <- m; !ok {
					donec <- fmt.Errorf("send: %v", cerror(h.Out))
					return
				}
				printf("srv: msg %s\n", string(m))
			}
			printf("srv: done %v\n", cerror(h.In))
			close(h.Out, cerror(h.In))
			donec <- nil
		}()
	}

	cli := func(t *testing.T, cc nchan.Conn) {
		for i := 0; i < 10; i++ {
			cc.Out <- []byte(fmt.Sprintf("<%d>", i))
			msg := string(<-cc.In)
			printf("got %s back\n", msg)
			if msg != fmt.Sprintf("<%d>", i) {
				t.Fatal("msg does not match")
			}
		}
		close(cc.In)
		close(cc.Out)
	}

	addrs := []string{"pipe!*!ftest", "fifo!*!ftest", "tcp!localhost!ftest", "ftest"}
	for _, a := range addrs {
		printf("dialing %s\n", a)
		cc, err := Dial(a)
		if err != nil {
			t.Fatal(err)
		}
		cli(t, cc)
	}
	close(ec, "done")
	for i := 0; i < 4; i++ {
		if err := <-donec; err != nil {
			t.Fatal(err)
		}
	}
}

func TestPeerFifo(t *testing.T) {
	fifo.Dir = "/tmp/dsfifotest"
	os.RemoveAll(fifo.Dir)
	defer os.RemoveAll(fifo.Dir)

	add1c, del1c, h1c, err1 := Peer("test1", "fifo!*!ftest1")
	if err1 != nil {
		t.Fatal(err1)
	}
	add2c, del2c, h2c, err2 := Peer("test2", "fifo!*!ftest2")
	if err1 != nil {
		t.Fatal(err2)
	}

	nc := make(chan int)
	go func() {
		for h := range h1c {
			nc <- 1
			printf("test1: client %v\n", h)
			close(h.Out)
			close(h.In)
		}
	}()
	go func() {
		for h := range h2c {
			nc <- 1
			printf("test2: client %v\n", h)
			close(h.Out)
			close(h.In)
		}
	}()
	add1c <- "fifo!*!ftest2"
	add1c <- "fifo!*!ftest2"
	add2c <- "fifo!*!ftest1"
	add2c <- "fifo!*!ftest1"
	del1c <- "fifo!*!ftest2"

	<-nc
	<-nc

	close(add1c)
	close(add2c)
	// this is not really needed..., but get rid of unused diags.
	close(del1c)
	close(del2c)
	close(h1c)
	close(h2c)
}

func TestCloseFifoNet(t *testing.T) {
	fifo.Dir = "/tmp/dsfifotest"
	os.RemoveAll(fifo.Dir)
	defer os.RemoveAll(fifo.Dir)

	hc, ec, err := Serve("test", "fifo!*!ftest")
	if err != nil {
		t.Fatal(err)
	}

	// one echo server
	donec := make(chan error, 1)
	go func() {
		h := <-hc
		if h == nil {
			donec <- nil
			return
		}
		defer close(h.Out)
		for m := range h.In {
			if ok := h.Out <- m; !ok {
				donec <- fmt.Errorf("send: %v", cerror(h.Out))
				return
			}
			printf("srv: msg %s\n", string(m))
		}
		printf("srv: done %v\n", cerror(h.In))
		close(h.Out, cerror(h.In))
		donec <- nil
	}()

	cc, err := Dial("ftest")
	if err != nil {
		t.Fatal(err)
	}
	close(ec, "done")

	for i := 0; i < 10; i++ {
		cc.Out <- []byte(fmt.Sprintf("<%d>", i))
		msg := string(<-cc.In)
		printf("got %s back\n", msg)
		if msg != "" && i == 9 {
			t.Fatal("got msgs")
		}
	}
	close(cc.In)
	close(cc.Out)
	if err := <-donec; err != nil {
		t.Fatal(err)
	}
	// TODO: sleep and make sure services are gone.
	// I checked this out by hand but it's not in the test.
}

func TestCloseFifoMux(t *testing.T) {
	fifo.Dir = "/tmp/dsfifotest"
	os.RemoveAll(fifo.Dir)
	defer os.RemoveAll(fifo.Dir)

	hc, ec, err := Serve("test", "fifo!*!ftest")
	if err != nil {
		t.Fatal(err)
	}

	// one echo server
	donec := make(chan error, 1)
	go func() {
		h := <-hc
		if h == nil {
			donec <- nil
			return
		}
		m := nchan.NewMux(*h, false)
		for x := range m.In {
			for m := range x.In {
				if ok := h.Out <- m; !ok {
					donec <- fmt.Errorf("send: %v", cerror(h.Out))
					return
				}
				printf("srv: msg %s\n", string(m))
			}
		}
		printf("srv: done %v\n", cerror(h.In))
		m.Close(cerror(m.In))
		donec <- nil
	}()

	cc, err := Dial("ftest")
	if err != nil {
		t.Fatal(err)
	}
	m := nchan.NewMux(cc, true)
	close(ec, "done")
	m.Debug = true
	for i := 0; i < 10; i++ {
		q, a := m.Rpc()
		ok := q <- []byte(fmt.Sprintf("<%d>", i))
		if !ok {
			printf("send %s\n", cerror(q))
		}
		close(q)
		msg := string(<-a)
		printf("got %s back\n", msg)
		if msg != "" {
			t.Fatal("got msgs")
		}
	}
	if err := <-donec; err != nil {
		t.Fatal(err)
	}
	// TODO: sleep and make sure services are gone.
	// I checked this out by hand but it's not in the test.
}

func benchPipe(b *testing.B) {
	b.StopTimer()
	os.Args[0] = "test"
	// fifo is not used, but just in case we have a bug.
	fifo.Dir = "/tmp/dsfifotest"
	os.RemoveAll(fifo.Dir)
	defer os.RemoveAll(fifo.Dir)

	hc, ec, err := Serve("test", "pipe!*!ftest")
	if err != nil {
		b.Fatal(err)
	}

	// one echo server
	go func() {
		h := <-hc
		if h == nil {
			ec <- nil
			return
		}
		defer close(h.Out)
		for m := range h.In {
			if ok := h.Out <- m; !ok {
				break
			}
		}
		close(h.Out, cerror(h.In))
		ec <- nil
	}()

	cc, err := Dial("pipe!*!ftest")
	if err != nil {
		b.Fatal(err)
	}
	msg := make([]byte, 128)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		cc.Out <- msg
		xmsg := <-cc.In
		if len(xmsg) != len(msg) {
			b.Fatal("msg does not match")
		}
	}
	b.StopTimer()
	close(cc.In)
	close(cc.Out)
}

func BenchmarkPipeRaw(b *testing.B) {
	old := nchan.Buffering
	nchan.Buffering = false
	benchPipe(b)
	nchan.Buffering = old
}

func BenchmarkPipeBIO(b *testing.B) {
	old := nchan.Buffering
	nchan.Buffering = true
	benchPipe(b)
	nchan.Buffering = old
}
