package nchan

import (
	"bufio"
	"bytes"
	"clive/dbg"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"testing"
	"time"
)

/*
	As of today:
	go test -bench .
	BenchmarkZWByteRaw            	20000000	       136 ns/op
	BenchmarkZWMsgsRaw            	10000000	       178 ns/op
	BenchmarkZWByteBIO            	20000000	       134 ns/op
	BenchmarkZWMsgsBIO            	10000000	       201 ns/op
	BenchmarkZRByteRaw            	  200000	      9589 ns/op
	BenchmarkZRMsgsRaw            	10000000	       154 ns/op
	BenchmarkZRByteBIO            	  200000	      7679 ns/op
	BenchmarkZRMsgsBIO            	10000000	       160 ns/op
	BenchmarkIOByteRaw            	 2000000	       921 ns/op
	BenchmarkIOByteBIO            	 2000000	       936 ns/op
	BenchmarkIOMsgsRaw            	 1000000	      1608 ns/op
	BenchmarkIOMsgsBIO            	 1000000	      1160 ns/op
	BenchmarkOSByteRaw            	   50000	     43557 ns/op
	BenchmarkOSByteBIO            	   50000	     43763 ns/op
	BenchmarkOSMsgsRaw            	 1000000	      2566 ns/op
	BenchmarkOSMsgsBIO            	 1000000	      1505 ns/op

	Using a WriteMsgsTo that relies on a select of data and flushc:
	BenchmarkZWByteRaw            	20000000	       134 ns/op
	BenchmarkZWMsgsRaw            	 5000000	       307 ns/op
	BenchmarkZWByteBIO            	20000000	       134 ns/op
	BenchmarkZWMsgsBIO            	 5000000	       328 ns/op
	BenchmarkZRByteRaw            	  200000	      9487 ns/op
	BenchmarkZRMsgsRaw            	10000000	       160 ns/op
	BenchmarkZRByteBIO            	  200000	      7429 ns/op
	BenchmarkZRMsgsBIO            	10000000	       169 ns/op
	BenchmarkIOByteRaw            	 2000000	       920 ns/op
	BenchmarkIOByteBIO            	 2000000	       937 ns/op
	BenchmarkIOMsgsRaw            	 1000000	      1704 ns/op
	BenchmarkIOMsgsBIO            	 1000000	      1269 ns/op
	BenchmarkOSByteRaw            	   50000	     44397 ns/op
	BenchmarkOSByteBIO            	   50000	     44627 ns/op
	BenchmarkOSMsgsRaw            	 1000000	      2749 ns/op
	BenchmarkOSMsgsBIO            	 1000000	      1729 ns/op

	go test -bench . '-benchtime=4s' '-cpuprofile=/tmp/prof.out'
	win go tool pprof nchan.test /tmp/prof.out
	(pprof) top20 -cum
	Total: 13409 samples
	       0   0.0%   0.0%     9476  70.7% runtime.gosched0
	       0   0.0%   0.0%     3810  28.4% System
	      17   0.1%   0.1%     2621  19.5% clive/nchan.ReadBytesFrom
	    2356  17.6%  17.7%     2356  17.6% runtime.usleep
	    2172  16.2%  33.9%     2177  16.2% syscall.Syscall
	      83   0.6%  34.5%     2117  15.8% clive/nchan.WriteMsgsTo
	     123   0.9%  35.4%     2055  15.3% runtime.mallocgc
	      43   0.3%  35.8%     1988  14.8% runtime.makeslice
	       2   0.0%  35.8%     1945  14.5% makeslice1
	       6   0.0%  35.8%     1943  14.5% runtime.cnewarray
	      31   0.2%  36.0%     1937  14.4% cnew
	     141   1.1%  37.1%     1881  14.0% clive/nchan.ReadMsgsFrom
	    1788  13.3%  50.4%     1788  13.3% runtime.mach_semaphore_signal
	       0   0.0%  50.4%     1721  12.8% clive/bufs.New
	       0   0.0%  50.4%     1685  12.6% runtime.gc
	       0   0.0%  50.4%     1684  12.6% runtime.starttheworld
	       0   0.0%  50.4%     1683  12.6% runtime.mach_semrelease
	       0   0.0%  50.4%     1683  12.6% runtime.notewakeup
	       0   0.0%  50.4%     1683  12.6% runtime.semawakeup
	       0   0.0%  50.4%     1487  11.1% os.(*File).Write

*/

var Printf = dbg.FuncPrintf(os.Stdout, testing.Verbose)
var xxx bool

func Example() {
	// With our go compiler we can send error
	// indications through chans:
	c := make(chan string, 2)
	c <- "hi"
	c <- " there"
	close(c, "oops")

	// and receive them
	for x := range c {
		Printf("got msg %s\n", x)
		// we can stop the sender if we don't want more.
		if xxx {
			close(c, "had an error")
			break
		}
	}
	if err := cerror(c); err != nil {
		Printf("got error %s\n", err)
	}
}

func ExampleConn() {
	var fd io.ReadWriteCloser

	c := NewConn(fd, 0, nil, nil)
	c.Out <- []byte("hi there")
	c.Out <- []byte("sending out this")
	msg := <-c.In
	Printf("got %s\n", string(msg))
	close(c.Out, "bye: had an error!")
	close(c.In, "don't want more")
	Printf("error from c?: %v", cerror(c.In))
}

type plain struct {
	c   chan bool
	ops string
}

func (p *plain) Read(b []byte) (int, error) {
	return len(b), nil
}

func (p *plain) Write(b []byte) (int, error) {
	p.ops += "w"
	return len(b), nil
}

func (p *plain) Flush() error {
	p.ops += "!"
	return nil
}

func (p *plain) Close() error {
	p.ops += "x"
	close(p.c)
	return nil
}

type dont struct {
	*plain
}

func (f dont) DontFlush() {
}

func TestLines(t *testing.T) {
	lns := [][]byte{
		[]byte("line"), []byte("oneπ\n"),
		[]byte("\n"),
		[]byte("line two\n\nlineππ four"),
		[]byte("\nfive"),
		[]byte("is"),
		[]byte("aπ"),
		[]byte("long line\n"),
	}
	out := []string{
		`lineoneπ` + "\n",
		`` + "\n",
		`line two` + "\n",
		`` + "\n",
		`lineππ four` + "\n",
		`fiveisaπlong line` + "\n",
		`π` + "\n",
		`and a last one`,
	}
	pi := []byte("π\nand a last one")
	lns = append(lns, pi[:1], pi[1:])
	bc := make(chan []byte, len(lns))
	for _, b := range lns {
		bc <- b
	}
	close(bc)
	lc := Lines(bc, '\n')
	res := []string{}
	for l := range lc {
		Printf("\t`%s`,\n", l)
		res = append(res, string(l))
	}
	Printf("sts %v\n", cerror(lc))
	if len(res) != len(out) {
		t.Fatal("bad len")
	}
	for i := 0; i < len(res); i++ {
		if res[i] != out[i] {
			t.Fatal("bad output")
		}
	}
}

func TestBuffering(t *testing.T) {
	buf1 := &plain{make(chan bool, 1), ""}
	var x = Buffering
	Buffering = true
	defer func() {
		Buffering = x
	}()
	c := NewConn(buf1, 0, nil, nil)
	for i := 0; i < 3; i++ {
		c.Out <- []byte("hi")
	}
	close(c.Out)
	close(c.In)
	<-buf1.c
	Printf("ops is %s\n", buf1.ops)
	if buf1.ops != "ww!ww!ww!x" {
		t.Logf("bad ops. was '%s'\n", buf1.ops)
		t.Fail()
	}

	buf2 := dont{&plain{make(chan bool, 1), ""}}
	c = NewConn(buf2, 0, nil, nil)
	for i := 0; i < 3; i++ {
		c.Out <- []byte("hi")
	}
	close(c.Out)
	close(c.In)
	<-buf2.c
	Printf("ops is %s\n", buf2.ops)
	if buf2.ops != "wwwwwwx" {
		t.Logf("bad ops. was '%s'\n", buf2.ops)
		t.Fail()
	}
}

func TestPutString(t *testing.T) {
	b := []byte{}
	b = PutString(nil, "hola")
	b = PutString(b, "cara cola")
	s, nb, err := GetString(b)
	if err != nil {
		t.Logf("get str: %s", err)
		t.Fail()
		return
	}
	if s != "hola" {
		t.Logf("got '%s', not hola", s)
		t.Fail()
		return
	}
	s, nb, err = GetString(nb)
	if err != nil {
		t.Logf("get str: %s", err)
		t.Fail()
		return
	}
	if s != "cara cola" {
		t.Logf("got '%s', not cara cola", s)
		t.Fail()
		return
	}
	if len(nb) != 0 {
		t.Logf("extra bytes in buffer")
		t.Fail()
	}
}

func TestNoBuffering(t *testing.T) {
	buf1 := &plain{make(chan bool, 1), ""}
	var x = Buffering
	Buffering = false
	defer func() {
		Buffering = x
	}()
	c := NewConn(buf1, 0, nil, nil)
	for i := 0; i < 3; i++ {
		c.Out <- []byte("hi")
	}
	close(c.Out)
	close(c.In)
	<-buf1.c
	Printf("ops is %s\n", buf1.ops)
	if buf1.ops != "wwwwwwx" {
		t.Logf("bad ops. was '%s'\n", buf1.ops)
		t.Fail()
	}

	buf2 := dont{&plain{make(chan bool, 1), ""}}
	c = NewConn(buf2, 0, nil, nil)
	for i := 0; i < 3; i++ {
		c.Out <- []byte("hi")
	}
	close(c.Out)
	close(c.In)
	<-buf2.c
	Printf("ops is %s\n", buf2.ops)
	if buf2.ops != "wwwwwwx" {
		t.Logf("bad ops. was '%s'\n", buf2.ops)
		t.Fail()
	}
}

func TestChanSend(t *testing.T) {
	c := make(chan int)
	go func() {
		defer close(c)
		for i := 0; i < 10; i++ {
			ok := c <- i
			if !ok {
				t.Fatal(cerror(c))
			}
		}
	}()
	n := 0
	for {
		d, ok := <-c
		if !ok {
			msg := cerror(c)
			if msg != nil {
				t.Fatalf("didn't get a nil error")
			}
			Printf("receiver %v\n", msg)
			break
		}
		Printf("got %v\n", d)
		n++
	}
	if n != 10 {
		t.Fatalf("got %d msgs, not 10", 10)
	}
}

func TestClosedChanSend(t *testing.T) {
	c := make(chan int, 0)
	x := make(chan int)
	go func() {
		for i := 0; i < 10; i++ {
			ok := c <- i
			if !ok {
				err := cerror(c)
				if err != nil && err.Error() == "no more" {
					x <- 0
					return
				}
				t.Fatal(cerror(c))
			}
		}
		t.Fatal("could send all")
		close(c)
		x <- 0
	}()
	n := 0
	for {
		if n == 5 {
			close(c, "no more")
		}
		d, ok := <-c
		if !ok {
			if n == 5 {
				<-x
				return
			}
			t.Fatalf("recv: %s", cerror(c))
			break
		}
		if testing.Verbose() {
			fmt.Println("got", d)
		}
		n++
	}
	if n == 10 {
		t.Fatalf("got all")
	}
	<-x
}

func ExampleWriteBytesTo() {
	c := make(chan []byte, 0)
	// send some messages through c and close with error.
	go func() {
		for i := 0; i < 10; i++ {
			ok := c <- []byte(fmt.Sprintf("<%d>", i))
			if !ok {
				dbg.Fatal(cerror(c))
			}
		}
		close(c, "oops")
	}()

	// write the msgs to buf. "oops" is the error returned.
	var buf bytes.Buffer
	_, n, err := WriteBytesTo(&buf, c)
	Printf("tot %d err %v\n", n, err)
}

func TestWriteBytesTo(t *testing.T) {
	c := make(chan []byte, 0)
	go func() {
		for i := 0; i < 10; i++ {
			ok := c <- []byte(fmt.Sprintf("<%d>", i))
			if !ok {
				t.Fatal(cerror(c))
			}
		}
		close(c)
	}()
	var buf bytes.Buffer
	_, n, err := WriteBytesTo(&buf, c)
	Printf("tot %d err %v\n", n, err)
	str := buf.String()
	if str != "<0><1><2><3><4><5><6><7><8><9>" {
		t.Fatalf("got %s", str)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestWriteErrTo(t *testing.T) {
	c := make(chan []byte, 0)
	go func() {
		for i := 0; i < 10; i++ {
			ok := c <- []byte(fmt.Sprintf("<%d>", i))
			if !ok {
				t.Fatal(cerror(c))
			}
		}
		close(c, "oops")
	}()
	var buf bytes.Buffer
	_, n, err := WriteBytesTo(&buf, c)
	Printf("tot %d err %v\n", n, err)
	str := buf.String()
	if str != "<0><1><2><3><4><5><6><7><8><9>" {
		t.Fatalf("got %s", str)
	}
	if err == nil || err.Error() != "oops" {
		t.Fatal("didn't get an oops error")
	}
}

func TestReadBytesFrom(t *testing.T) {
	c := make(chan []byte, 0)
	go func() {
		for i := 0; i < 10; i++ {
			ok := c <- []byte(fmt.Sprintf("<%d>", i))
			if !ok {
				t.Fatal(cerror(c))
			}
		}
		close(c)
	}()
	var buf bytes.Buffer
	_, n, err := WriteBytesTo(&buf, c)
	Printf("tot %d err %v\n", n, err)
	nc := make(chan []byte, 10)
	_, n, err = ReadBytesFrom(&buf, nc)
	close(nc, err)
	Printf("tot %d err %v\n", n, err)
	str := ""
	for d := range nc {
		str += string(d)
	}
	if str != "<0><1><2><3><4><5><6><7><8><9>" {
		t.Fatalf("got %s", str)
	}
	if err != nil {
		t.Fatal(err)
	}
}

type Err struct {
	Msg string
}

func pipeStrsTo(w io.Writer, c chan string) error {
	enc := gob.NewEncoder(w)
	for {
		s, ok := <-c
		if !ok {
			if cerror(c) != nil {
				e := Err{cerror(c).Error()}
				err := enc.Encode(e)
				if err != nil {
					return err
				}
				return errors.New(e.Msg)
			}
			return nil
		}
		err := enc.Encode(Err{})
		if err != nil {
			close(c, err.Error())
			return err
		}
		if err := enc.Encode(s); err != nil {
			close(c, err.Error())
			return err
		}
	}
}

func pipeStrsFrom(r io.Reader, c chan string) error {
	dec := gob.NewDecoder(r)
	for {
		var s string
		var e Err
		err := dec.Decode(&e)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			close(c, err.Error())
			return err
		}
		err = dec.Decode(&s)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			close(c, err.Error())
			return err
		}
		ok := c <- s
		if !ok {
			return cerror(c)
		}
	}
}

func TestPipeTo(t *testing.T) {
	var buf bytes.Buffer
	c := make(chan string, 10)
	for i := 0; i < 10; i++ {
		ok := c <- fmt.Sprintf("<%d>", i)
		if !ok {
			t.Fatal(cerror(c))
		}
	}
	close(c, "oops")
	err := pipeStrsTo(&buf, c)
	if err == nil || err.Error() != "oops" {
		t.Fatal(err)
	}
	errc := make(chan error, 1)
	nc := make(chan string)
	go func() {
		err := pipeStrsFrom(&buf, nc)
		close(nc)
		errc <- err
	}()
	out := ""
	for s := range nc {
		Printf("got %s\n", s)
		out += s
	}
	if out != "<0><1><2><3><4><5><6><7><8><9>" {
		t.Fatalf("got %s", out)
	}
	close(nc)
	e := <-errc
	if e != nil {
		t.Fatal(e)
	}
}

func TestReader(t *testing.T) {
	c := make(chan []byte, 0)
	go func() {
		for i := 0; i < 10; i++ {
			ok := c <- []byte(fmt.Sprintf("<%d>", i))
			if !ok {
				t.Fatal(cerror(c))
			}
		}
		close(c)
	}()
	r := Reader(c)
	data, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "<0><1><2><3><4><5><6><7><8><9>" {
		t.Fatalf("got %s", string(data))
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWriter(t *testing.T) {
	c := make(chan []byte, 0)
	go func() {
		w := Writer(c)
		for i := 0; i < 10; i++ {
			if _, err := w.Write([]byte(fmt.Sprintf("<%d>", i))); err != nil {
				t.Fatal(cerror(c))
			}
		}
		w.Close()
	}()
	out := ""
	for x := range c {
		s := string(x)
		Printf("got %s\n", s)
		out += s
	}
	if out != "<0><1><2><3><4><5><6><7><8><9>" {
		t.Fatalf("got %s", out)
	}
}

func xWriteMsgsTo(w io.Writer, c <-chan []byte, fc chan<- int) (int64, error) {
	if Buffering {
		bw := bufio.NewWriter(w)
		w = bw
	}
	_, n, err := WriteMsgsTo(w, c)
	return n, err
}

func TestMsgs(t *testing.T) {
	c := make(chan []byte, 0)
	var buf bytes.Buffer
	go func() {
		if ok := c <- []byte{}; !ok {
			t.Fatal(cerror(c))
		}
		for i := 0; i < 10; i++ {
			if ok := c <- []byte(fmt.Sprintf("<%d>", i)); !ok {
				t.Fatal(cerror(c))
			}
		}
		close(c)
	}()
	tot, err := xWriteMsgsTo(&buf, c, nil)
	Printf("wrote tot %d err %v\n", tot, err)
	if tot != 118 {
		t.Fatalf("didnt' write 118 bytes but %d", tot)
	}
	if err != nil {
		t.Fatal(err)
	}
	c1 := make(chan []byte, 0)
	go func() {
		_, tot, err := ReadMsgsFrom(&buf, c1)
		Printf("tot %d err %v\n", tot, err)
		if err != nil {
			t.Fatal(err)
		}
		close(c1, err)
	}()
	out := ""
	i := -1
	for x := range c1 {
		s := string(x)
		Printf("got '%s'\n", s)
		if i < 0 {
			if len(x) != 0 {
				t.Fatal("empty msg expected")
			}
		} else if s != fmt.Sprintf("<%d>", i) {
			t.Fatalf("didn't want %s", s)
		}
		i++
		out += s
	}
	if out != "<0><1><2><3><4><5><6><7><8><9>" {
		t.Fatalf("got '%s'", out)
	}
}

func TestMsgErr(t *testing.T) {
	c := make(chan []byte, 0)
	var buf bytes.Buffer
	go func() {
		if ok := c <- []byte{}; !ok {
			t.Fatal(cerror(c))
		}
		for i := 0; i < 10; i++ {
			if ok := c <- []byte(fmt.Sprintf("<%d>", i)); !ok {
				t.Fatal(cerror(c))
			}
		}
		close(c, "oops")
	}()
	tot, err := xWriteMsgsTo(&buf, c, nil)
	Printf("wrote tot %d err %v\n", tot, err)
	if tot != 118 {
		t.Fatalf("didnt' write 118 bytes but %d", tot)
	}
	if err == nil || err.Error() != "oops" {
		t.Fatalf("bad err msg %v", err)
	}
	c1 := make(chan []byte, 0)
	go func() {
		_, tot, err := ReadMsgsFrom(&buf, c1)
		Printf("tot %d err %v\n", tot, err)
		if err == nil || err.Error() != "oops" {
			t.Fatalf("bad err msg %v", err)
		}
		close(c1, err)
	}()
	out := ""
	i := -1
	for x := range c1 {
		s := string(x)
		Printf("got '%s'\n", s)
		if i < 0 {
			if len(x) != 0 {
				t.Fatal("empty msg expected")
			}
		} else if s != fmt.Sprintf("<%d>", i) {
			t.Fatalf("didn't want %s", s)
		}
		i++
		out += s
	}
	if err := cerror(c1); err == nil || err.Error() != "oops" {
		t.Fatalf("bad err msg %v", err)
	}
	if out != "<0><1><2><3><4><5><6><7><8><9>" {
		t.Fatalf("got '%s'", out)
	}
}

func TestPipe(t *testing.T) {
	c := NewPipe(0)
	go func() {
		for i := 0; i < 5; i++ {
			if ok := c.Out <- []byte(fmt.Sprintf("<%d>", i)); !ok {
				t.Fatal(cerror(c.Out))
			}
		}
		close(c.Out, "done")
	}()
	i := 0
	for x := range c.In {
		if string(x) != fmt.Sprintf("<%d>", i) {
			t.Fatalf("did receive %s", string(x))
		}
		i++
	}
	if i != 5 {
		t.Fatalf("did receive %d messages", i)
	}
	e := cerror(c.In)
	if e == nil {
		t.Fatal("error expected")
	}
	if e.Error() != "done" {
		t.Fatal("wrong error %v", e)
	}
}

func TestPipeErr(t *testing.T) {
	c := NewPipe(0)
	go func() {
		i := 0
		for x := range c.In {
			if string(x) != fmt.Sprintf("<%d>", i) {
				t.Fatalf("did receive %s", string(x))
			}
			i++
			if i == 3 {
				close(c.In, "oops")
				break
			}
		}
	}()
	for i := 0; i < 10; i++ {
		if ok := c.Out <- []byte(fmt.Sprintf("<%d>", i)); !ok {
			if i == 5 {
				if cerror(c.Out).Error() != "oops" {
					t.Fatalf("bad error %v", cerror(c.Out))
				}
				break
			}
			t.Fatal(cerror(c.Out))
		}
		if i >= 5 {
			t.Fatalf("could send for i %d", i)
		}
	}
}

var (
	m2msgs = []string{
		`new m2 in 0 rpc true`,
		`m2:0 m xxxx`,
		`m2:0 m yyyy`,
		`m2:0 closed <nil>`,
		`new m2 in 1 rpc false`,
		`m2:1 closed <nil>`,
		`new m2 in 2 rpc false`,
		`m2:2 m hola`,
		`m2:2 m caracola`,
		`new m2 in 3 rpc false`,
		`m2:3 m hi`,
		`m2:3 m there`,
		`m2:3 closed <nil>`,
		`m2:2 closed oops`,
		`m2 closed <nil>`,
	}

	m1msgs = []string{
		`new m1 in 0 out false`,
		`m1:0 m 0`,
		`m1:0 closed <nil>`,
		`new m1 in 1 out false`,
		`m1:1 m 2`,
		`m1:1 closed <nil>`,
		`new m1 in 2 out false`,
		`m1:2 m 1`,
		`m1:2 closed <nil>`,
		`m1 closed <nil>`,
	}

	rmsgs = []string{
		`reply a rep`,
		`reply ly going`,
		`reply closed repl.err.`,
	}
	mlk sync.Mutex
)

func msg(lst *[]string, fmts string, args ...interface{}) {
	Printf(fmts, args...)
	s := fmt.Sprintf(fmts, args...)
	if s[len(s)-1] == '\n' {
		s = s[:len(s)-1]
	}
	mlk.Lock()
	*lst = append(*lst, s)
	mlk.Unlock()
}

func ExampleMux_Out() {
	var c Conn

	// create a mux for a dialer
	m1 := NewMux(c, true)

	// sending a request when we have errors
	req := m1.Out()
	req <- []byte("hi")
	req <- []byte("thread")
	close(req, "oops! had an error")

	// sending a request with no errors
	req2 := m1.Out()
	req2 <- []byte("hi")
	close(req2)
}

func ExampleMux_Rpc() {
	var c Conn

	// create a mux for a dialer
	m1 := NewMux(c, true)

	// sending a request when we have errors
	reqc, repc := m1.Rpc()
	go func() {
		for r := range repc {
			Printf("got reply %s\n", string(r))
		}
		Printf("reply error sts is %v\n", cerror(repc))
	}()
	reqc <- []byte("hi")
	reqc <- []byte("thread")
	close(reqc)

}

func checkmsgs(t *testing.T, tag string, outs []string, msgs []string) {
	for i := 0; i < len(outs); i++ {
		Printf("out %d: <%s>\n", i, outs[i])
		if i >= len(msgs) {
			t.Fatalf("unexpected output")
		}
		if outs[i] != msgs[i] {
			t.Fatalf("wrong msg <%s>", msgs[i])
		}
	}
	if len(outs) < len(msgs) {
		t.Fatalf("didn't receive <%s>", msgs[len(outs)])
	}
}

func TestMux(t *testing.T) {
	t.Skip("not yet")
	c1, c2 := NewConnPipe(0)
	var out1, out2, outr []string
	c1.Tag = "m1"
	c2.Tag = "m2"
	m1 := NewMux(c1, true)
	m2 := NewMux(c2, false)
	m1.Debug = testing.Verbose()
	m2.Debug = testing.Verbose()
	mc := make(chan int)
	go func() {
		i := 0
		wc := make(chan int)
		for x := range m1.In {
			msg(&out1, "new m1 in %d out %v\n", i, x.Out != nil)
			go func(i int, x Conn) {
				for im := range x.In {
					msg(&out1, "m1:%d m %s\n", i, string(im))
				}
				msg(&out1, "m1:%d closed %v\n", i, cerror(x.In))
				close(x.Out)
				wc <- 0
			}(i, x)
			i++
		}
		msg(&out1, "m1 closed %v\n", cerror(m1.In))
		for ; i > 0; i-- {
			<-wc
		}
		mc <- 0
	}()
	go func() {
		i := 0
		wc := make(chan int)
		for x := range m2.In {
			msg(&out2, "new m2 in %d rpc %v\n", i, x.Out != nil)
			go func(i int, x Conn) {
				isrpc := false
				for im := range x.In {
					isrpc = isrpc || string(im) == "yyyy"
					msg(&out2, "m2:%d m %s\n", i, string(im))
				}
				msg(&out2, "m2:%d closed %v\n", i, cerror(x.In))
				if isrpc {
					x.Out <- []byte("a rep")
					x.Out <- []byte("ly going")
				}
				close(x.Out, "repl.err.")
				wc <- 0
			}(i, x)
			i++
		}
		msg(&out2, "m2 closed %v\n", cerror(m2.In))
		for ; i > 0; i-- {
			<-wc
		}
		mc <- 0
	}()
	rc, rr := m1.Rpc()
	go func() {
		for m := range rr {
			msg(&outr, "reply %s\n", string(m))
		}
		msg(&outr, "reply closed %v\n", cerror(rr))
		mc <- 0
	}()
	rc <- []byte("xxxx")
	rc <- []byte("yyyy")
	close(rc)
	<-mc
	go func() {
		for i := 0; i < 3; i++ {
			oc2 := m2.Out()
			oc2 <- []byte(fmt.Sprintf("%d", i))
			close(oc2)
		}
	}()
	oc := m1.Out()
	close(oc)
	oc = m1.Out()
	oc2 := m1.Out()
	oc <- []byte("hola")
	oc2 <- []byte("hi")
	oc <- []byte("caracola")
	oc2 <- []byte("there")
	close(oc, "oops")
	close(oc2)
	m1.Out() // not used, not closed
	time.Sleep(time.Second)
	m1.Close(nil)
	m2.Close(nil)
	<-mc
	<-mc
	Printf("all closed\n")
	checkmsgs(t, "mux 1", out1, m1msgs)
	checkmsgs(t, "mux 2", out2, m2msgs)
	checkmsgs(t, "rpc", outr, rmsgs)
}

type nw struct{}

func (nw) Write(data []byte) (int, error) {
	return len(data), nil
}

func (nw) Close() error {
	return nil
}

func (nw) Read(b []byte) (int, error) {
	return len(b), nil
}

func benchZW(b *testing.B, ospipe, bufio, msgs bool) {
	c := make(chan []byte)
	var null nw
	var bytes [128]byte
	Buffering = bufio

	if msgs {
		go xWriteMsgsTo(null, c, nil)
	} else {
		go WriteBytesTo(null, c)
	}
	for i := 0; i < b.N; i++ {
		c <- bytes[:]
	}
	close(c)
}

func BenchmarkZWByteRaw(b *testing.B) {
	benchZW(b, false, false, false)
}

func BenchmarkZWMsgsRaw(b *testing.B) {
	benchZW(b, false, false, true)
}

func BenchmarkZWByteBIO(b *testing.B) {
	benchZW(b, false, true, false)
}

func BenchmarkZWMsgsBIO(b *testing.B) {
	benchZW(b, false, true, true)
}

func benchZR(b *testing.B, ospipe, bufio, msgs bool) {
	c := make(chan []byte)
	Buffering = bufio
	var null nw
	if msgs {
		go ReadMsgsFrom(null, c)
	} else {
		go ReadBytesFrom(null, c)
	}
	for i := 0; i < b.N; i++ {
		<-c
	}
}

func BenchmarkZRByteRaw(b *testing.B) {
	benchZR(b, false, false, false)
}

func BenchmarkZRMsgsRaw(b *testing.B) {
	benchZR(b, false, false, true)
}

func BenchmarkZRByteBIO(b *testing.B) {
	benchZR(b, false, true, false)
}

func BenchmarkZRMsgsBIO(b *testing.B) {
	benchZR(b, false, true, true)
}

func benchReadWrite(b *testing.B, ospipe, bufio, msgs bool) {
	Buffering = bufio
	cr := make(chan []byte)
	cw := make(chan []byte)
	var r io.ReadCloser
	var w io.WriteCloser
	if ospipe {
		r, w, _ = os.Pipe()
	} else {
		r, w = io.Pipe()
	}
	var bytes [128]byte
	go func() {
		for i := 0; i < b.N || !msgs; i++ {
			cw <- bytes[:]
		}
		cw <- nil
		close(cw)
	}()
	if msgs {
		go xWriteMsgsTo(w, cw, nil)
		go ReadMsgsFrom(r, cr)
	} else {
		go WriteBytesTo(w, cw)
		go ReadBytesFrom(r, cr)
	}
	for i := 0; i < b.N; i++ {
		<-cr
	}
}

func BenchmarkIOByteRaw(b *testing.B) {
	benchReadWrite(b, false, false, false)
}

func BenchmarkIOByteBIO(b *testing.B) {
	benchReadWrite(b, false, true, false)
}

func BenchmarkIOMsgsRaw(b *testing.B) {
	benchReadWrite(b, false, false, true)
}

func BenchmarkIOMsgsBIO(b *testing.B) {
	benchReadWrite(b, false, true, true)
}

func BenchmarkOSByteRaw(b *testing.B) {
	benchReadWrite(b, true, false, false)
}

func BenchmarkOSByteBIO(b *testing.B) {
	benchReadWrite(b, true, true, false)
}

func BenchmarkOSMsgsRaw(b *testing.B) {
	benchReadWrite(b, true, false, true)
}

func BenchmarkOSMsgsBIO(b *testing.B) {
	benchReadWrite(b, true, true, true)
}

func BenchmarkSelect(b *testing.B) {
	b.StopTimer()
	c := make(chan int, 1)
	d := make(chan bool)
	b.StartTimer()
	c <- 1
	for i := 0; i < b.N; i++ {
		select {
		case <-c:
			c <- 1
		case <-d:
		}
	}
}

func BenchmarkProcSelect(b *testing.B) {
	b.StopTimer()
	c := make(chan int, 1)
	b.StartTimer()
	c <- 1
	for i := 0; i < b.N; i++ {
		d := make(chan bool)
		x := make(chan int, 1)
		go func() {
			<-c
			x <- 1
		}()
		go func() {
			<-d
			x <- 2
		}()
		<-x
		c <- 1
		close(d)
		close(c)
	}
}

func BenchmarkNoSelect(b *testing.B) {
	b.StopTimer()
	c := make(chan int, 1)
	b.StartTimer()
	c <- 1
	for i := 0; i < b.N; i++ {
		<-c
		c <- 1
	}
}
