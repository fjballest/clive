/*
	Test tools for things that can be read and written.
*/
package rwtest

import (
	"bytes"
	"clive/dbg"
	"io"
	"io/ioutil"
	"math/rand"
)

// Usually testing.T or testing.B
interface Fataler {
	Fatalf(format string, args ...face{})
	Logf(format string, args ...face{})
	Fail()
}

// Objects that can be used in such tests.
interface Object {
	WriteAt([]byte, int64) (int, error)
	ReadAt([]byte, int64) (int, error)
}

// Objects that implement this are tested by comparing the final
// contents of the file.
interface FullyReadable {
	Seek(int64, int) (int64, error)
	Read([]byte) (int, error)
}

// Objects that implemented truncable are also tested for
// truncations and resizes.
interface Resizeable {
	Truncate(int64) error
}

// If the object tested as a file has Truncate, it is also tested.
// Test a rw object by comparing what a real file does and what it does.
// 10% of the operations are resizes (if any) and the rest are half read, half writes.
func AsAFile(t Fataler, rw Object, nops, maxoff, maxsz int) {
	asAFile(t, []Object{rw}, nops, maxoff, maxsz)
}

func AsAConcFile(t Fataler, rw []Object, nops, maxoff, maxsz int) {
	asAFile(t, rw, nops, maxoff, maxsz)
}

func pick(rws []Object) Object {
	n := rand.Intn(len(rws))
	return rws[n]
}

func asAFile(t Fataler, rws []Object, nops, maxoff, maxsz int) {
	rw := rws[0]
	fd, err := ioutil.TempFile("", "rwtest")
	if err != nil {
		t.Fatalf("temp file: %s", err)
	}
	defer fd.Close()
	opc := make(chan int)
	rrw, ok := rw.(Resizeable)
	nrsz := 0
	if ok {
		nrsz = nops / 10
		nops -= nrsz
	}
	nrd := nops / 2
	nwr := nops - nrd
	defer t.Logf("%d reads, %d writes, %d resizes\n", nrd, nwr, nrsz)
	go func() {
		for i := 0; i < nrd; i++ {
			if ok := opc <- 0; !ok {
				break
			}
		}
	}()
	go func() {
		for i := 0; i < nwr; i++ {
			if ok := opc <- 1; !ok {
				break
			}
		}
	}()
	go func() {
		for i := 0; i < nrsz; i++ {
			if ok := opc <- 2; !ok {
				break
			}
		}
	}()
	defer func() {
		close(opc, "done")
	}()
	wdata := make([]byte, maxsz)
	for i := 0; i < maxsz; i++ {
		wdata[i] = byte(i)
	}
	xrw, ok := rw.(FullyReadable)
	for i := 0; i < nops; i++ {
		off := rand.Intn(maxoff)
		sz := rand.Intn(maxsz)
		rw = pick(rws)
		switch <-opc {
		case 0:
			dataf := make([]byte, sz)
			datarw := make([]byte, sz)
			nf, errf := fd.ReadAt(dataf, int64(off))
			nrw, errrw := rw.ReadAt(datarw, int64(off))
			t.Logf("read off %d sz %d -> %d %d", off, len(dataf), nf, nrw)
			if nf != nrw {
				t.Fatalf("didn't read the same")
			}
			if errf == io.EOF {
				errf = nil
			}
			if errrw == io.EOF {
				errrw = nil
			}
			if errf != nil && errrw == nil || errf == nil && errrw != nil {
				t.Logf("file sts %v", errf)
				t.Logf("rw sts %v", errrw)
				t.Fatalf("didn't fail the same in read")
			}
			if bytes.Compare(dataf[:nf], datarw[:nrw]) != 0 {
				s1 := dbg.HexStr(dataf[:nf], 32)
				s2 := dbg.HexStr(datarw[:nrw], 32)
				t.Logf("os %d bytes: %s", nf, s1)
				t.Logf("rw %d bytes: %s", nrw, s2)
				t.Fatalf("didn't read the same content")
			}
		case 1:
			if sz == 0 {
				sz++
			}
			nf, errf := fd.WriteAt(wdata[:sz], int64(off))
			nrw, errrw := rw.WriteAt(wdata[:sz], int64(off))
			t.Logf("write off %d sz %d -> %d %d", off, sz, nf, nrw)
			if nf != nrw {
				t.Fatalf("didn't write the same")
			}
			if errf != nil && errrw == nil || errf == nil && errrw != nil {
				t.Fatalf("didn't fail the same in write")
			}
			if ok {
				compare(t, xrw, fd)
			}
		case 2:
			t.Logf("trunc %d", off)
			if err := fd.Truncate(int64(off)); err != nil {
				t.Fatalf("file resize: %s", err)
			}
			if err := rrw.Truncate(int64(off)); err != nil {
				t.Fatalf("user resize: %s", err)
			}
			if ok {
				compare(t, xrw, fd)
			}
		}
	}

	if !ok {
		return
	}
	// Now compare everything.
	compare(t, xrw, fd)
}

func compare(t Fataler, rw, fd FullyReadable) {
	fdata, err := readall(fd)
	if err != nil {
		t.Fatalf("read all fd: %s", err)
	}
	rwdata, err := readall(rw)
	if err != nil {
		t.Fatalf("read all rw: %s", err)
	}
	if bytes.Compare(fdata, rwdata) != 0 {
		t.Logf("file %d bytes: %s\n", len(fdata), dbg.HexStr(fdata, 32))
		t.Logf("rw %d bytes: %s\n", len(rwdata), dbg.HexStr(rwdata, 32))
		for i := 0; i < len(fdata) && i < len(rwdata); i++ {
			if fdata[i] != rwdata[i] {
				t.Logf("[%d] file %x rw %x\n", i,
					fdata[i], rwdata[i])
				break
			}
		}
		t.Fatalf("files do not match")
	}
}

func readall(rw FullyReadable) ([]byte, error) {
	old, err := rw.Seek(0, 1)
	if err != nil {
		return nil, err
	}
	n, err := rw.Seek(0, 0)
	if n != 0 || err != nil {
		return nil, err
	}
	data, err := ioutil.ReadAll(rw)
	if err != nil {
		return nil, err
	}
	_, err = rw.Seek(old, 0)
	if err != nil {
		return nil, err
	}
	return data, nil
}
