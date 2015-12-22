package bufs

import (
	"bytes"
	"clive/bufs/rwtest"
	"clive/dbg"
	"io"
	"os"
	"testing"
)

/*
	Writes writes N bytes one by one
	ReadWrite does the same and then reads one bytes at a time

	Initial code.
	BenchmarkWrite                	  200000	    191115 ns/op
	BenchmarkReadWrite            	  100000	    210732 ns/op

	After adding a fast path for sequential appends/reads:

	BenchmarkWrite                	10000000	       209 ns/op
	BenchmarkReadWrite            	10000000	       192 ns/op


	After allocating blocks with Size bytes by default:

	BenchmarkWrite                	100000000	        15.5 ns/op
	BenchmarkReadWrite            	50000000	        29.1 ns/op
*/

var Printf = dbg.FuncPrintf(os.Stdout, testing.Verbose)

func TestEmpty(t *testing.T) {
	Size = 128
	debug = testing.Verbose()
	var buf Blocks
	var what [100]byte

	if buf.Len() != 0 {
		t.Fatal("wrong len")
	}
	b, _ := buf.Open()
	nr, _ := b.Read(what[:])
	if nr != 0 {
		t.Fatal("didn't get eof")
	}
	if b.Len() != 0 {
		t.Fatal("wrong len")
	}
}

func TestFullReadWrite1(t *testing.T) {
	Size = 128
	debug = testing.Verbose()
	var b Blocks
	what := []byte{0, 1, 2}
	resp := make([]byte, 110*len(what))
	for i := 0; i < 100; i++ {
		nw, err := b.Write(what)
		if nw != len(what) || err != nil {
			t.Fatalf("wrong write #%d sts %v", nw, err)
		}
	}
	if b.Len() != 100*len(what) {
		t.Fatal("wrong length")
	}
	bo, _ := b.Open()
	nr, err := bo.Read(resp[:len(resp)/2])
	if nr != len(resp)/2 || err != nil {
		t.Fatalf("wrong read #%d sts %v", nr, err)
	}
	onr := nr
	nr, err = bo.Read(resp[len(resp)/2:])
	if nr != bo.Len()-onr || err != io.EOF {
		t.Fatalf("wrong read #%d sts %v", nr, err)
	}
	onr += nr
	for i := 0; i < onr; i++ {
		if resp[i] != byte(i%3) {
			t.Fatalf("wrong content at off %d", i)
		}
	}

	var buf bytes.Buffer

	n, err := b.WriteTo(&buf)
	if err != nil {
		t.Fatal("writeto")
	}
	if n != int64(buf.Len()) {
		t.Fatal("writeto len")
	}
	mem := []byte{}
	for i := 0; i < len(b.blks); i++ {
		mem = append(mem, b.blks[i].data...)
	}
	if bytes.Compare(buf.Bytes(), mem) != 0 {
		t.Fatal("writeto bug?")
	}
}

func TestFullReadWrite2(t *testing.T) {
	Size = 128
	debug = testing.Verbose()
	var b Blocks
	what := []byte{0, 1, 2}
	resp := make([]byte, 110*len(what))
	for i := 0; i < 100; i++ {
		nw, err := b.Write(what)
		if nw != len(what) || err != nil {
			t.Fatalf("wrong write #%d sts %v", nw, err)
		}
	}
	if b.Len() != 100*len(what) {
		t.Fatal("wrong length")
	}
	tot := 0
	bo, _ := b.Open()
	for tot < len(resp) {
		var x [7]byte
		nr, err := bo.Read(x[:])
		copy(resp[tot:], x[:nr])
		tot += nr
		if err != nil {
			break
		}
	}
	resp = resp[:tot]
	if len(resp) != 100*len(what) || len(resp) != b.Len() {
		t.Fatal("wrong read total")
	}
	for i := 0; i < len(resp); i++ {
		if resp[i] != byte(i%3) {
			t.Fatalf("wrong content at off %d", i)
		}
	}
}

func TestTruncate(t *testing.T) {
	Size = 128
	debug = testing.Verbose()
	var b Blocks
	b.Write([]byte{1})
	for i := 0; i < 10; i++ {
		b.Truncate(int64(4 * i))
	}
	if b.Len() != 4*9 {
		t.Fatalf("bad length %d", b.Len())
	}
	b.Truncate(0)
	b.Write([]byte{0, 1, 2, 3})
	b.Truncate(2)
	b.Truncate(4)
	if bytes.Compare([]byte{0, 1, 0, 0}, b.Bytes()) != 0 {
		t.Fatalf("bad content")
	}
}

func TestFullReadWriteHole(t *testing.T) {
	Size = 128
	debug = testing.Verbose()
	var b Blocks
	what := []byte{0, 1, 2}
	resp := make([]byte, 110*len(what))
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			continue
		}
		nw, err := b.WriteAt(what, int64(i*len(what)))
		if nw != len(what) || err != nil {
			t.Fatalf("wrong write #%d sts %v", nw, err)
		}
	}
	if b.Len() != 100*len(what) {
		t.Fatalf("wrong length %d", b.Len())
	}
	tot := 0
	bo, _ := b.Open()
	for tot < len(resp) {
		var x [7]byte
		nr, err := bo.Read(x[:])
		copy(resp[tot:], x[:nr])
		tot += nr
		if err != nil {
			break
		}
	}
	resp = resp[:tot]
	if len(resp) != 100*len(what) || len(resp) != b.Len() {
		t.Fatal("wrong read total")
	}
	for i := 0; i < len(resp); i++ {
		nw := i / 3
		if nw%2 == 0 && resp[i] != 0 {
			t.Fatalf("wrong content at off %d", i)
		} else if nw%2 != 0 && resp[i] != byte(i%3) {
			t.Fatalf("wrong content %d at off %d", resp[i], i)
		}
	}
}

func TestAsAFile(t *testing.T) {
	Size = 128
	debug = testing.Verbose()
	var b Blocks
	bo, err := b.Open()
	if err != nil {
		t.Fatalf("open: %s", err)
	}
	rwtest.AsAFile(t, bo, 100, 5*KiB, 128)
}

func BenchmarkWrite(b *testing.B) {
	var buf Blocks
	var what [1]byte
	tot := 0
	for i := 0; i < b.N; i++ {
		nw, _ := buf.Write(what[:])
		tot += nw
	}
	if tot != b.N {
		b.Fatal("wrong counts")
	}
}

func BenchmarkReadWrite(b *testing.B) {
	var buf Blocks
	var what [1]byte
	tot := 0
	for i := 0; i < b.N; i++ {
		nw, _ := buf.Write(what[:])
		tot += nw
	}
	bo, _ := buf.Open()
	for i := 0; i < b.N; i++ {
		nr, _ := bo.Read(what[:])
		tot -= nr
	}
	if tot != 0 {
		b.Fatal("wrong counts")
	}
}
