package fifo

import (
	"clive/dbg"
	"clive/nchan"
	"fmt"
	"os"
	"testing"
	"time"
)

/*
	As of today:
	go test -bench .

	BenchmarkFifoRaw              	   10000	    196737 ns/op
	BenchmarkFifoBIO              	   10000	    205755 ns/op

	(times seem to be similar, the variations are within the error.)

	comparte to:
	BenchmarkPipeRaw              	  500000	      3304 ns/op
	BenchmarkPipeBIO              	 1000000	      2362 ns/op

	go test -bench BIO -benchtime 10s -cpuprofile /tmp/prof.out
	win go tool pprof fifo.test /tmp/prof.out
	(pprof) top20 -cum
	Total: 1531 samples
	       0   0.0%   0.0%      657  56.6% System
	       0   0.0%   0.0%      503  43.4% runtime.gosched0
	     340  29.3%  29.3%      340  29.3% runtime.usleep
	       0   0.0%  29.3%      288  24.8% clive/nchan.ReadMsgsFrom
	       0   0.0%  29.3%      288  24.8% clive/nchan.func·004
	       0   0.0%  29.3%      286  24.7% bufio.(*Reader).Read
	       0   0.0%  29.3%      286  24.7% io.ReadAtLeast
	       0   0.0%  29.3%      286  24.7% io.ReadFull
	     266  22.9%  52.2%      266  22.9% runtime.memmove
	     219  18.9%  71.1%      233  20.1% syscall.Syscall
	       0   0.0%  71.1%      213  18.4% clive/nchan.WriteMsgsTo
	       0   0.0%  71.1%      213  18.4% clive/nchan.func·003
	       0   0.0%  71.1%      209  18.0% bufio.(*Reader).fill
	       0   0.0%  71.1%      209  18.0% os.(*File).Read
	       0   0.0%  71.1%      209  18.0% os.(*File).read
	       0   0.0%  71.1%      209  18.0% syscall.Read
	       0   0.0%  71.1%      209  18.0% syscall.read
	     190  16.4%  87.5%      190  16.4% runtime.mach_semaphore_wait
	       0   0.0%  87.5%      189  16.3% bufio.(*Writer).Write
	       0   0.0%  87.5%      189  16.3% clive/nchan.writeMsg

	go test -bench Raw -benchtime 10s -cpuprofile /tmp/prof.out
	Total: 1227 samples
	       0   0.0%   0.0%      617  50.3% System
	       0   0.0%   0.0%      607  49.5% runtime.gosched0
	     442  36.0%  36.0%      496  40.4% syscall.Syscall
	     354  28.9%  64.9%      354  28.9% runtime.usleep
	       0   0.0%  64.9%      305  24.9% clive/nchan.WriteMsgsTo
	       0   0.0%  64.9%      305  24.9% clive/nchan.func·003
	       0   0.0%  64.9%      305  24.9% clive/nchan.writeMsg
	       0   0.0%  64.9%      305  24.9% os.(*File).Write
	       0   0.0%  64.9%      305  24.9% os.(*File).write
	       0   0.0%  64.9%      305  24.9% syscall.Write
	       0   0.0%  64.9%      305  24.9% syscall.write
	      10   0.8%  65.7%      299  24.4% clive/nchan.ReadMsgsFrom
	       0   0.0%  65.7%      299  24.4% clive/nchan.func·004
	       1   0.1%  65.8%      210  17.1% io.ReadFull
	       7   0.6%  66.3%      209  17.0% io.ReadAtLeast
	       2   0.2%  66.5%      202  16.5% os.(*File).Read
	       2   0.2%  66.7%      200  16.3% os.(*File).read
	       6   0.5%  67.2%      198  16.1% syscall.Read
	       1   0.1%  67.2%      192  15.6% syscall.read
	       0   0.0%  67.2%      187  15.2% clive/nchan.writeHdr
*/

var printf = dbg.FuncPrintf(os.Stdout, testing.Verbose)

func serveAClient(c *nchan.Conn) {
}

func ExampleDial() {
	// dial the testsvc on the fifo network.
	cc, err := Dial("testsvc")
	if err != nil {
		dbg.Fatal("dial: %v", err)
	}
	// use cc.In/cc.Out to talk to the server.
	cc.Out <- []byte("hi there")
	close(cc.Out)        // when done
	close(cc.In, "done") // to stop receiving at any time.
}

func ExampleNew() {
	// create a handler
	hndlr, clic := NewChanHandler()
	// keep on receiving connections for clients...
	go func() {
		for {
			if cli := <-clic; cli != nil {
				// use cli.In/cli.Out to serve the client
				printf("new client %s\n", cli.Tag)
				go serveAClient(cli)
			} else {
				dbg.Fatal("server error: %s\n", cerror(clic))
			}
		}
	}()

	// and start the service...
	s := New("test tag", "testsvc", hndlr)
	s.Verbose = testing.Verbose()
	s.Debug = s.Verbose
	if err := s.Serve(); err != nil {
		dbg.Fatal("serve: %v", err)
	}
}

func TestFifos(t *testing.T) {
	Fifo := "/tmp/zxfifotest"
	os.Remove(Fifo)
	defer os.Remove(Fifo)
	if err := mkfifo(Fifo); err != nil {
		t.Fatalf("mkfifo: %s", err)
	}
	ec := make(chan error, 2)
	go func() {
		fd, err := os.OpenFile(Fifo, os.O_WRONLY, 0600)
		if err != nil {
			ec <- err
			return
		}
		defer fd.Close()
		t.Logf("writing...")
		if _, err := fd.Write([]byte("hola")); err != nil {
			ec <- err
			return
		}
		ec <- nil
	}()
	go func() {
		fd, err := os.OpenFile(Fifo, os.O_RDONLY, 0600)
		if err != nil {
			ec <- err
			return
		}
		defer fd.Close()
		t.Logf("reading...")
		data := make([]byte, 50)
		n, err := fd.Read(data)
		if err != nil {
			ec <- err
			return
		}
		t.Logf("did read %s", data[:n])
		ec <- nil
	}()
	n := 0
	doselect {
	case err := <-ec:
		if err != nil {
			t.Fatalf("got err %s", err)
		}
		n++
		if n == 2 {
			break
		}
	case <-time.After(10*time.Second):
		t.Fatalf("timed out: fifos do not work in this system using go.")
	}
}

func TestSrv(t *testing.T) {
	Dir = "/tmp/fifotest"
	os.RemoveAll(Dir)
	defer os.RemoveAll(Dir)

	hs, hc := NewChanHandler()
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

	s := New("test", "ftest", hs)
	s.Verbose = testing.Verbose()
	s.Debug = s.Verbose
	if err := s.Serve(); err != nil {
		t.Fatalf("serve: %v", err)
	}

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
	close(cc.Out, "err1")
	<-cc.In
	printf("cli done %v\n", cerror(cc.In))
	if cerror(cc.In)==nil || cerror(cc.In).Error()!="err1" {
		t.Fatal("bad sts")
	}
	s.Stop(true)
	if err := <-ec; err != nil {
		t.Fatal(err)
	}
}

func TestSrvClose(t *testing.T) {
	Dir = "/tmp/fifotest"
	os.RemoveAll(Dir)
	defer os.RemoveAll(Dir)

	hs, hc := NewChanHandler()
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

	s := New("test", "ftest", hs)
	s.Verbose = testing.Verbose()
	s.Debug = s.Verbose
	if err := s.Serve(); err != nil {
		t.Fatalf("serve: %v", err)
	}
	cc, err := Dial("ftest")
	if err != nil {
		t.Fatal(err)
	}
	s.Stop(true)
	for i := 0; i < 10; i++ {
		if ok := cc.Out <- []byte(fmt.Sprintf("<%d>", i)); !ok {
			printf("send %s\n", cerror(cc.Out))
		}
		msg := string(<-cc.In)
		printf("got %s back\n", msg)
		if msg != "" {
			t.Fatal("got msgs")
		}
	}
	printf("cli done %v\n", cerror(cc.In))
}

func benchFifo(b *testing.B) {
	b.StopTimer()

	Dir = "/tmp/fifobench"
	os.RemoveAll(Dir)
	defer os.RemoveAll(Dir)

	hs, hc := NewChanHandler()
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
		}
		close(h.Out, cerror(h.In))
		ec <- nil
	}()

	s := New("test", "ftest", hs)
	s.Verbose = testing.Verbose()
	s.Debug = s.Verbose
	if err := s.Serve(); err != nil {
		b.Fatalf("serve: %v", err)
	}

	cc, err := Dial("ftest")
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
	close(cc.Out, "err1")
	<-cc.In
	printf("cli done %v\n", cerror(cc.In))
	if cerror(cc.In)==nil || cerror(cc.In).Error()!="err1" {
		b.Fatal("bad sts")
	}
	s.Stop(true)
	if err := <-ec; err != nil {
		b.Fatal(err)
	}
}

func BenchmarkFifoRaw(b *testing.B) {
	old := nchan.Buffering
	nchan.Buffering = false
	benchFifo(b)
	nchan.Buffering = old
}

func BenchmarkFifoBIO(b *testing.B) {
	old := nchan.Buffering
	nchan.Buffering = true
	benchFifo(b)
	nchan.Buffering = old
}
