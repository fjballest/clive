package srv

import (
	"bytes"
	"clive/dbg"
	"clive/nchan"
	"clive/net/auth"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

var Printf = dbg.FuncPrintf(os.Stdout, testing.Verbose)

func ExampleNew() {
	// we might disable TLS (enabled by default).
	auth.TLSenable(false)

	// create a handler
	hndlr, clic := NewChanHandler()

	// receive client connections and do echo for them.
	go func() {
		h := <-clic
		for m := range h.In {
			Printf("srv: got msg %s\n", string(m))
			if ok := h.Out <- m; !ok {
				dbg.Fatal("send: %v", cerror(h.Out))
			}
		}
		Printf("srv: done with status %v\n", cerror(h.In))
		close(h.Out, cerror(h.In))
	}()

	// start the service on the local port 8082.
	s := New("test", "tcp", "", "8082", hndlr)
	if err := s.Serve(); err != nil {
		dbg.Fatal("serve: %s", err)
	}
}

type th {}

func (h th) HandleCli(c net.Conn, endc chan bool) {
	io.Copy(c, c)
	c.Close()
}

func TestSrv(t *testing.T) {
	auth.TLSenable(false)
	Verbose = testing.Verbose()
	Debug = Verbose

	var h th

	s := New("test", "tcp", "", "8081", h)
	if err := s.Serve(); err != nil {
		t.Fatal(err)
	}
	addr, err := net.ResolveTCPAddr("tcp", "[::1]:8081")
	if err != nil {
		t.Fatal(err)
	}
	donec := make(chan int, 15)
	for i := 0; i < 15; i++ {
		go func() {
			c, err := net.DialTCP("tcp", nil, addr)
			if err != nil {
				t.Fatal(err)
			}
			ec := make(chan int)
			s1 := fmt.Sprintf("hi %d\n", i)
			s2 := fmt.Sprintf("there %d\n", i)
			go func() {
				fmt.Fprintf(c, "%s", s1)
				fmt.Fprintf(c, "%s", s2)
				c.CloseWrite()
				ec <- 0
			}()
			var buf bytes.Buffer
			io.Copy(&buf, c)
			dat := string(buf.Bytes())
			if dat != s1+s2 {
				t.Fatalf("got %s", dat)
			}
			c.CloseRead()
			<-ec
			donec <- i
		}()
	}
	for i := 0; i < 15; i++ {
		<-donec
	}
	s.Stop(true)
}

func parseAddr(addr string) (net, mach, svc string) {
	args := strings.Split(addr, "!")
	switch len(args) {
	case 0:
		return // invalid fmt
	case 1:
		return "fifo", "*", args[0]
	case 2:
		return "tcp", args[0], args[1]
	default:
		return args[0], args[1], args[2]
	}
}
func Dial(naddr string) (nchan.Conn, error) {
	netw := "tcp"
	toks := strings.SplitN(naddr, "!", 2)
	addr, svc := toks[0], toks[1]
	taddr, err := net.ResolveTCPAddr(netw, addr+":"+svc)
	if err != nil {
		return nchan.Conn{}, err
	}
	c, err := net.DialTCP(netw, nil, taddr)
	if err != nil {
		return nchan.Conn{}, err
	}
	if auth.TLSclient != nil {
		tc := tls.Client(c, auth.TLSclient)
		return nchan.NewConn(tc, 5, nil, nil), nil
	}
	return nchan.NewConn(c, 5, nil, nil), nil
}

func TestSrvChan(t *testing.T) {
	auth.TLSenable(false)
	Verbose = testing.Verbose()
	Debug = Verbose

	hs, hc := NewChanHandler()

	// This is an echo server from inb to outb.
	go func() {
		h := <-hc
		defer close(h.Out)
		for m := range h.In {
			if ok := h.Out <- m; !ok {
				t.Fatalf("send: %v", cerror(h.Out))
				break
			}
			Printf("srv: msg %s\n", string(m))
		}
		Printf("srv: done %v\n", cerror(h.In))
		close(h.Out, cerror(h.In))
	}()

	s := New("test", "tcp", "", "8082", hs)
	if err := s.Serve(); err != nil {
		t.Fatalf("serve: %v", err)
	}

	// client: send an error and several int/string messages.
	ch, err := Dial("[::1]!8082")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	ec := make(chan int, 1)
	go func() {
		for i := 0; i < 10; i++ {
			Printf("cli: send %v\n", i)
			m := []byte(fmt.Sprintf("<%d>", i))
			if ok := ch.Out <- m; !ok {
				t.Fatalf("send: %s", err)
				break
			}
		}
		close(ch.Out)
		ec <- 0
	}()
	n := 0
	for m := range ch.In {
		if string(m) != fmt.Sprintf("<%d>", n) {
			t.Fatalf("bad echo %s", string(m))
		}
		n++
	}
	if n != 10 {
		t.Fatalf("got %d msgs and expected 10", n)
	}
	<-ec
	s.Stop(true)
}

func TestTLSSrvChan(t *testing.T) {
	auth.TLSenable(true)
	Verbose = testing.Verbose()
	Debug = Verbose

	hs, hc := NewChanHandler()

	// This is an echo server from inb to outb.
	go func() {
		h := <-hc
		defer close(h.Out)
		for m := range h.In {
			if ok := h.Out <- m; !ok {
				t.Fatalf("send: %v", cerror(h.Out))
				break
			}
			Printf("srv: msg %s\n", string(m))
		}
		Printf("srv: done %v\n", cerror(h.In))
		close(h.Out, cerror(h.In))
	}()

	s := New("test", "tcp", "", "8083", hs)
	if err := s.Serve(); err != nil {
		t.Fatalf("serve: %s", err)
	}

	// client: send several string messages.
	ch, err := Dial("[::1]!8083")
	if err != nil {
		t.Fatalf("dial: %s", err)
	}
	ec := make(chan int, 1)
	go func() {
		for i := 0; i < 10; i++ {
			Printf("cli: send %v\n", i)
			m := []byte(fmt.Sprintf("<%d>", i))
			if ok := ch.Out <- m; !ok {
				t.Fatalf("send: %s", err)
				break
			}
		}
		close(ch.Out)
		ec <- 0
	}()
	n := 0
	for m := range ch.In {
		if string(m) != fmt.Sprintf("<%d>", n) {
			t.Fatalf("bad echo %s", string(m))
		}
		n++
		if n == 10 {
			// TLS can't closeread/write, so we must
			// stop when we want and can't just close ch.Out
			// in the client sender process.
			close(ch.In)
			close(ch.Out)
		}
	}
	if n != 10 {
		t.Fatalf("got %d msgs and expected 10", n)
	}
	<-ec
	s.Stop(true)
}

func TestSrvChanClose(t *testing.T) {
	auth.TLSenable(false)
	Verbose = testing.Verbose()
	Debug = Verbose

	hs, hc := NewChanHandler()

	// This is an echo server from inb to outb closing its
	// input after echoing the 3rd message.
	go func() {
		h := <-hc
		i := 0
		for m := range h.In {
			if ok := h.Out <- m; !ok {
				t.Fatalf("send: %v", cerror(h.Out))
				break
			}
			Printf("srv: msg %s\n", string(m))
			if i++; i == 2 {
				close(h.Out, "closing")
				break
			}
		}
		Printf("srv: done %v\n", cerror(h.In))
		close(h.Out, cerror(h.In))
	}()

	s := New("test", "tcp", "", "8084", hs)
	if err := s.Serve(); err != nil {
		t.Fatal(err)
	}

	// client: several string messages but check we couldn't
	// send all messages.
	ch, err := Dial("[::1]!8084")
	if err != nil {
		t.Fatal(err)
	}
	ec := make(chan error, 1)
	go func() {
		for i := 0; i < 10; i++ {
			Printf("cli: send %v\n", i)
			m := []byte(fmt.Sprintf("<%d>", i))
			if ok := ch.Out <- m; !ok {
				ec <- fmt.Errorf("send: %s", cerror(ch.Out))
				return
			}
			time.Sleep(100*time.Millisecond)
		}
		close(ch.Out)
		ec <- nil
	}()
	n := 0
	for m := range ch.In {
		if string(m) != fmt.Sprintf("<%d>", n) {
			t.Fatalf("bad echo %s", string(m))
		}
		Printf("cli echo %s\n", string(m))
		n++
	}
	if err := cerror(ch.In); err==nil || err.Error()!="closing" {
		t.Fatalf("bad error %v", err)
	}
	if err := <-ec; err != nil {
		t.Fatal(err)
	}
	if n > 2 {
		t.Fatalf("got too much")
	}
	s.Stop(true)
}
