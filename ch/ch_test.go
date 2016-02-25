package ch

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type foo int
type bar int

func (f foo) String() string {
	return "foo"
}

struct tb {
	r io.ReadCloser
	w io.WriteCloser
}

func (b *tb) Write(dat []byte) (int, error) {
	return b.w.Write(dat)
}

func (b *tb) Read(dat []byte) (int, error) {
	return b.r.Read(dat)
}

func (b *tb) CloseWrite() error {
	return b.w.Close()
}

func (b *tb) CloseRead() error {
	return b.r.Close()
}

var out = []byte{0xc, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x1, 0x0, 0x61, 0x20, 0x62, 0x79, 0x74, 0x65, 0x20, 0x61, 0x72, 0x72, 0x61, 0x79, 0x12, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x9a, 0x2, 0x61, 0x6e, 0x6f, 0x74, 0x68, 0x65, 0x72, 0x20, 0x62, 0x79, 0x74, 0x65, 0x20, 0x61, 0x72, 0x72, 0x61, 0x79, 0x8, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x3, 0x0, 0x61, 0x20, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0xe, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x3, 0x0, 0x61, 0x6e, 0x6f, 0x74, 0x68, 0x65, 0x72, 0x20, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x3, 0x0, 0x4, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x4, 0x0, 0x6f, 0x6f, 0x70, 0x73, 0x3, 0x0, 0x0, 0x0, 0x1, 0x0, 0x0, 0x0, 0x2, 0x0, 0x66, 0x6f, 0x6f}

func TestMsgs(t *testing.T) {
	var buf bytes.Buffer
	b1 := []byte("a byte array")
	b2 := Ign{666, []byte("another byte array")}
	s1 := "a string"
	s2 := "another string"
	var b3 []byte
	var s3 string
	var e1 error
	var f foo
	var b bar
	e2 := errors.New("oops")
	n, err := WriteMsg(&buf, 1, b1)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("+%d\tsz = %d\n", n, buf.Len())
	n, err = WriteMsg(&buf, 1, b2)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("+%d\tsz = %d\n", n, buf.Len())
	n, err = WriteMsg(&buf, 1, s1)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("+%d\tsz = %d\n", n, buf.Len())
	n, err = WriteMsg(&buf, 1, s2)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("+%d\tsz = %d\n", n, buf.Len())
	n, err = WriteMsg(&buf, 1, b3)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("+%d\tsz = %d\n", n, buf.Len())
	n, err = WriteMsg(&buf, 1, s3)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("+%d\tsz = %d\n", n, buf.Len())
	n, err = WriteMsg(&buf, 1, e1)
	if err != ErrDiscarded {
		t.Fatal(err)
	}
	t.Logf("+%d\tsz = %d\n", n, buf.Len())
	n, err = WriteMsg(&buf, 1, e2)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("+%d\tsz = %d\n", n, buf.Len())
	n, err = WriteMsg(&buf, 1, f)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("+%d\tsz = %d\n", n, buf.Len())
	n, err = WriteMsg(&buf, 1, b)
	if err != ErrDiscarded {
		t.Fatal(err)
	}
	t.Logf("+%d\tsz = %d\n", n, buf.Len())
	if bout := buf.Bytes(); !bytes.Equal(out, bout) {
		t.Logf("var out = %#v", bout)
		t.Fatalf("bad encoding")
	}

	outs := []string{
		"22 1 []uint8 [97 32 98 121 116 101 32 97 114 114 97 121] <nil>",
		"28 1 ch.Ign {666 [97 110 111 116 104 101 114 32 98 121 116 101 32 97 114 114 97 121]} <nil>",
		"18 1 string a string <nil>",
		"24 1 string another string <nil>",
		"10 1 []uint8 [] <nil>",
		"10 1 string  <nil>",
		"14 1 *errors.errorString oops <nil>",
		"13 1 ch.Ign {2 [102 111 111]} <nil>",
		"0 0 <nil> <nil> EOF",
	}

	for _, s := range outs {
		n, tag, m, err := ReadMsg(&buf)
		t.Logf("%d %d %T %v %v\n", n, tag, m, m, err)
		if s != "" &&
			s != fmt.Sprintf("%d %d %T %v %v", n, tag, m, m, err) {
			t.Fatal("bad msg")
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestBytes(t *testing.T) {
	MsgSz = 10
	c := make(chan face{}, 512)
	var buf bytes.Buffer
	nb, nm, err := ReadBytes(&buf, c)
	t.Logf("%d %d %v\n", nb, nm, err)
	if nb != 0 || nm != 0 || err != nil {
		t.Fatal("bad null read bytes")
	}
	buf.Write(out)
	nb, nm, err = ReadBytes(&buf, c)
	t.Logf("%d %d %v\n", nb, nm, err)
	enm := (len(out) + MsgSz - 1) / MsgSz
	if nb != int64(len(out)) || nm != enm || err != nil {
		t.Fatal("bad null read bytes")
	}
	close(c)
	var nbuf bytes.Buffer
	nb2, nm2, err := WriteBytes(&nbuf, c)
	if err != nil {
		t.Fatal(err)
	}
	if nb2 != nb || nm2 != nm {
		t.Fatal("bad nb of bytes or msgs")
	}
	if !bytes.Equal(out, nbuf.Bytes()) {
		t.Fatal("bad output")
	}
}

func TestPipe(t *testing.T) {
	MsgSz = 10
	p := NewPipe(512)

	var buf, nbuf bytes.Buffer
	buf.Write(out)
	_, _, err := ReadBytes(&buf, p.Out)
	close(p.Out, err)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = WriteBytes(&nbuf, p.In)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, nbuf.Bytes()) {
		t.Fatal("bad output")
	}

}

func TestPipeErr(t *testing.T) {
	MsgSz = 10
	p := NewPipe(512)

	var buf, nbuf bytes.Buffer
	buf.Write(out)
	_, _, err := ReadBytes(&buf, p.Out)
	if err != nil {
		t.Fatal(err)
	}
	p.Out <- errors.New("oops")
	close(p.Out, "coops")
	_, _, err = WriteBytes(&nbuf, p.In)
	if err == nil || err.Error() != "oops" {
		t.Fatal("didn't fail as expected")
	}
	if !bytes.Equal(out, nbuf.Bytes()) {
		t.Fatal("bad output")
	}

}

func TestPipePair(t *testing.T) {
	MsgSz = 10

	c1, c2 := NewPipePair(512)
	go func() {
		for m := range c2.In {
			if ok := c2.Out <- m; !ok {
				close(c2.In, cerror(c2.Out))
				break
			}
		}
		close(c2.Out, cerror(c2.In))
	}()

	var buf, nbuf bytes.Buffer
	buf.Write(out)
	_, _, err := ReadBytes(&buf, c1.Out)
	close(c1.Out, err)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = WriteBytes(&nbuf, c1.In)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, nbuf.Bytes()) {
		t.Fatal("bad output")
	}
}

func TestPipePairErr(t *testing.T) {
	MsgSz = 10

	c1, c2 := NewPipePair(512)
	go func() {
		for m := range c2.In {
			if ok := c2.Out <- m; !ok {
				close(c2.In, cerror(c2.Out))
				break
			}
		}
		close(c2.Out, cerror(c2.In))
	}()

	var buf, nbuf bytes.Buffer
	buf.Write(out)
	_, _, err := ReadBytes(&buf, c1.Out)
	if err != nil {
		t.Fatal(err)
	}
	c1.Out <- errors.New("oops")
	close(c1.Out, "coops")
	_, _, err = WriteBytes(&nbuf, c1.In)
	if err == nil || err.Error() != "oops" {
		t.Fatal("didn't fail as expected")
	}
	if !bytes.Equal(out, nbuf.Bytes()) {
		t.Fatal("bad output")
	}
}

func TestPipePairHalfClose(t *testing.T) {
	MsgSz = 10

	c1, c2 := NewPipePair(0)
	close(c2.In)
	go func() {
		var buf bytes.Buffer
		buf.Write(out)
		_, _, err := ReadBytes(&buf, c2.Out)
		close(c2.Out, err)
	}()
	i := 0
	for ; i < 100; i++ {
		if ok := c1.Out <- []byte("hi there"); !ok {
			break
		}
	}
	t.Logf("could write %d msgs", i)
	if i == 100 {
		t.Fatalf("could write %d msgs", i)
	}
	var nbuf bytes.Buffer
	_, _, err := WriteBytes(&nbuf, c1.In)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, nbuf.Bytes()) {
		t.Fatal("bad output")
	}
}

func TestConnErr(t *testing.T) {
	MsgSz = 15

	for x := 0; x < 300; x++ {
		fd := &tb{}
		fd.r, fd.w = io.Pipe()
		c := NewConn(fd, 300, nil)

		var obuf, buf, nbuf bytes.Buffer
		for i := 0; i < 100; i++ {
			fmt.Fprintf(&obuf, "<%d>", i)
			fmt.Fprintf(&buf, "<%d>", i)
		}
		c.Out <- errors.New("oops")
		_, _, err := ReadBytes(&buf, c.Out)
		if err != nil {
			t.Fatal(err)
		}
		close(c.Out, "coops")
		_, _, err = WriteBytes(&nbuf, c.In)
		if err == nil || err.Error() != "oops" {
			t.Fatalf("didn't fail as expected: sts %v", err)
		}
		if obuf.String() != nbuf.String() {
			t.Logf("did expect %s", buf.String())
			t.Fatalf("bad output %s", nbuf.String())
		}
	}
}

func TestConn(t *testing.T) {
	MsgSz = 100

	for x := 0; x < 300; x++ {
		fd := &tb{}
		fd.r, fd.w = io.Pipe()
		c := NewConn(fd, 300, nil)

		var buf, nbuf bytes.Buffer
		buf.Write(out)
		_, _, err := ReadBytes(&buf, c.Out)
		if err != nil {
			t.Fatal(err)
		}
		close(c.Out, "coops")
		_, _, err = WriteBytes(&nbuf, c.In)
		if err == nil || err.Error() != "coops" {
			t.Fatal("didn't fail as expected")
		}
		if !bytes.Equal(out, nbuf.Bytes()) {
			t.Fatal("bad output")
		}
	}
}

func TestMuxOut(t *testing.T) {
	m1, m2, _ := NewMuxPair()
	m1.Tag = "m1"
	m1.Debug = testing.Verbose()
	printf := m1.Dprintf
	m2.Tag = "m2"
	m2.Debug = testing.Verbose()

	var wg, wg2 sync.WaitGroup
	wg.Add(4)
	wg2.Add(1)

	reqs := [...]string{"hi there", "again", "and again"}
	failed := false
	nCalls := 3
	msrv := func(m *Mux) {
		nc := 0
		for c := range m.In {
			printf("%s call\n", m.Tag)
			n := 0
			// NB: the user should not do this,
			// it should be always receiving from m.In
			for d := range c.In {
				printf("%s req %v\n", m.Tag, d)
				s, ok := d.(string)
				if !ok {
					printf("%s bad req type %T\n", m.Tag, d)
					failed = true
					m.Close()
					break
				}
				if n >= len(reqs) || reqs[n] != s {
					printf("%s req is wrong\n", m.Tag)
					failed = true
					m.Close()
					break
				}
				n++
			}
			printf("%s call done\n", m.Tag)
			if n != len(reqs) {
				printf("%s call: bad nb of reqs %d\n", m.Tag, n)
				failed = true
				m.Close()
			}
			nc++
			if nc == nCalls {
				wg2.Done()
			}
		}
		close(m.In, "I'm done")
		printf("%s done\n", m.Tag)
		wg.Done()
	}
	mwait := func(m *Mux) {
		<-m.Hup
		printf("%s wait done\n", m.Tag)
		wg.Done()
	}
	go msrv(m1)
	go msrv(m2)
	go mwait(m1)
	go mwait(m2)
	for i := 0; i < nCalls; i++ {
		o1 := m1.Out()
		for _, s := range reqs {
			o1.Out <- s
		}
		close(o1.Out)
	}
	wg2.Wait()
	if failed {
		t.Fatal("reqs were wrong")
	}
	m1.Close()
	m2.Close()
	wg.Wait()
}

func TestMuxRpc(t *testing.T) {
	m1, m2, _ := NewMuxPair()
	m1.Tag = "m1"
	m1.Debug = testing.Verbose()
	printf := m1.Dprintf
	m2.Tag = "m2"
	m2.Debug = testing.Verbose()

	var wg, wg2 sync.WaitGroup
	wg.Add(4)
	wg2.Add(1)

	reqs := [...]string{"hi there", "again", "and again"}
	repls := [...]string{"hi there.repl", "again.repl", "and again.repl"}
	failed := false
	nCalls := 15

	msrv := func(m *Mux) {
		nc := 0
		for c := range m.In {
			printf("%s call\n", m.Tag)
			n := 0
			repls := []string{}
			for d := range c.In {
				printf("%s req %v\n", m.Tag, d)
				s, ok := d.(string)
				if !ok {
					printf("%s bad req type %T\n", m.Tag, d)
					failed = true
					m.Close()
					break
				}
				if n >= len(reqs) || reqs[n] != s {
					printf("%s req is wrong\n", m.Tag)
					failed = true
					m.Close()
					break
				}
				n++
				repls = append(repls, s+".repl")
			}
			printf("%s call done: replying\n", m.Tag)
			if n != len(reqs) {
				printf("%s call: bad nb of reqs %d\n", m.Tag, n)
				failed = true
				m.Close()
			}
			nc++
			for _, s := range repls {
				printf("%s repl %v\n", m.Tag, s)
				ok := c.Out <- s
				if !ok {
					printf("%s repl failed: %v\n", m.Tag, cerror(c.Out))
					failed = true
					m.Close()
					break
				}
			}
			printf("%s call done: reply done\n", m.Tag)
			close(c.Out)
			if nc == nCalls {
				wg2.Done()
			}
		}
		close(m.In, "I'm done")
		printf("%s done\n", m.Tag)
		wg.Done()
	}
	mwait := func(m *Mux) {
		<-m.Hup
		printf("%s wait done\n", m.Tag)
		wg.Done()
	}
	go msrv(m1)
	go msrv(m2)
	go mwait(m1)
	go mwait(m2)
	for i := 0; i < nCalls; i++ {
		r := m1.Rpc()
		for _, s := range reqs {
			r.Out <- s
		}
		close(r.Out)
		rs := []string{}
		for s := range r.In {
			printf("got reply %v\n", s)
			rs = append(rs, s.(string))
		}
		printf("reply sts %v\n", cerror(r.In))
		if len(rs) != len(reqs) {
			t.Fatal("bad nb of replies")
		}
		if strings.Join(repls[:], " ") != strings.Join(rs, " ") {
			t.Fatal("bad replies")
		}
	}
	wg2.Wait()
	if failed {
		t.Fatal("reqs were wrong")
	}
	m1.Close()
	m2.Close()
	wg.Wait()
}

func TestMuxFlow(t *testing.T) {
	nbuf = 10
	m1, m2, _ := NewMuxPair()
	m1.Tag = "m1"
	m1.Debug = testing.Verbose()
	printf := m1.Dprintf
	m2.Tag = "m2"
	m2.Debug = testing.Verbose()

	var wg, wg2 sync.WaitGroup
	wg.Add(4)
	nCalls := 50 // at least 2
	nlong := 40000
	nshort := 50
	nrep := 1000
	if testing.Short() {
		nshort = 5
		nCalls = 10
	}
	wg2.Add(nCalls - 1)
	syncc := make(chan bool, 1)
	failed := false
	msrv := func(m *Mux) {
		nc := 0
		for c := range m.In {
			c := c
			go func(nc int) {
				printf("%s call\n", m.Tag)
				n := 0
				for d := range c.In {
					printf("%s req %d\n", m.Tag, n)
					if nc == 0 && n == 0 {
						syncc <- true
					}
					if nc == 0 && n == 5 {
						printf("%s slow flow reader...\n", m.Tag)
						time.Sleep(300 * time.Second)
						return
					}
					s, ok := d.(string)
					if !ok {
						printf("%s bad req type %T\n", m.Tag, d)
						failed = true
						m.Close()
						break
					}
					xs := fmt.Sprintf("msg.%d", n)
					xs = strings.Repeat(xs, nrep)
					if s != xs {
						printf("%s req was not <%s>\n", m.Tag,
							fmt.Sprintf("msg.%d", n))
						failed = true
						m.Close()
						break
					}
					n++
				}
				printf("%s call done\n", m.Tag)
				xn := nlong
				if nc > 0 {
					xn = nshort
				}
				if n != xn {
					printf("%s call: bad nb of reqs %d\n", m.Tag, n)
					failed = true
					m.Close()
				}
				if nc > 0 {
					wg2.Done()
				}
			}(nc)
			nc++
		}
		close(m.In, "I'm done")
		printf("%s done\n", m.Tag)
		wg.Done()
	}
	mwait := func(m *Mux) {
		<-m.Hup
		printf("%s wait done\n", m.Tag)
		wg.Done()
	}
	go msrv(m1)
	go msrv(m2)
	go mwait(m1)
	go mwait(m2)
	o1 := m1.Out()
	o1.Out <- strings.Repeat(fmt.Sprintf("msg.%d", 0), nrep)
	<-syncc
	go func() {
		for j := 1; j < nlong; j++ {
			o1.Out <- strings.Repeat(fmt.Sprintf("msg.%d", j), nrep)
		}
		close(o1.Out)
	}()
	for i := 1; i < nCalls; i++ {
		o1 := m1.Out()
		for j := 0; j < nshort; j++ {
			o1.Out <- strings.Repeat(fmt.Sprintf("msg.%d", j), nrep)
		}
		close(o1.Out)
	}
	wg2.Wait()
	if failed {
		t.Fatal("reqs were wrong")
	}
	m1.Close()
	m2.Close()
	wg.Wait()
}
