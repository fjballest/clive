package ostest

import (
	"bytes"
	"clive/zx"
	"io/ioutil"
	"math/rand"
	"os"
)

type fsOp int

const (
	oCreate fsOp = iota
	oMkdir
	oRemove
	oWrite
	oWstat
	oRead // Keep as the first read-only op
	oStat
	oMax
	oFirst = oCreate
)

const (
	nOps     = 200
	maxSeek  = 15 * 1024
	maxWrite = 1024
	osdir    = "/tmp/fstest_dir"
)

var paths = []string{
	"/1",
	"/a",
	"/a/b",
}

var calls = map[fsOp]func(Fataler, string, string) bool{
	oCreate: fcreate,
	oRemove: fremove,
	oWrite:  fwrite,
	oMkdir:  fmkdir,
	oWstat:  fwstat,
	oRead:   fread,
	oStat:   fstat,
}

var counts = map[fsOp]int{}

var wbuf [maxWrite]byte

func init() {
	for i := 0; i < len(wbuf); i++ {
		wbuf[i] = byte(i)
	}
}

func (o fsOp) String() string {
	switch o {
	case oCreate:
		return "create"
	case oMkdir:
		return "mkdir"
	case oRemove:
		return "remove"
	case oWrite:
		return "write"
	case oWstat:
		return "wstat"
	case oRead:
		return "read"
	case oStat:
		return "stat"
	default:
		return "unknown"
	}
}

func fcreate(t Fataler, p1, p2 string) bool {
	err1 := ioutil.WriteFile(p1, []byte(p1), 0643)
	err2 := ioutil.WriteFile(p2, []byte(p1), 0643)
	if err1 != nil && err2 != nil {
		t.Logf("create %s fails", p1)
		return false
	}
	if err1 != nil || err2 != nil {
		t.Fatalf("errors in create: %v vs %v", err1, err2)
	}
	t.Logf("create %s ok", p1)
	return true
}

func fremove(t Fataler, p1, p2 string) bool {
	err1 := os.Remove(p1)
	err2 := os.Remove(p2)
	if err1 != nil && err2 != nil {
		t.Logf("remove %s fails", p1)
		return false
	}
	if err1 != nil || err2 != nil {
		t.Fatalf("errors in remove: %v vs %v", err1, err2)
	}
	t.Logf("remove %s ok", p1)
	return true
}

func fwrite(t Fataler, p1, p2 string) bool {
	fd1, err1 := os.OpenFile(p1, os.O_WRONLY, 0)
	if fd1 != nil {
		defer fd1.Close()
	}
	fd2, err2 := os.OpenFile(p2, os.O_WRONLY, 0)
	if fd2 != nil {
		defer fd2.Close()
	}
	if err1 != nil && err2 != nil {
		t.Logf("write %s fails", p1)
		return false
	}
	if err1 != nil || err2 != nil {
		t.Fatalf("errors in remove: %v vs %v", err1, err2)
	}
	pos := rand.Intn(maxSeek)
	sz := rand.Intn(len(wbuf))
	n1, err1 := fd1.WriteAt(wbuf[:sz], int64(pos))
	n2, err2 := fd2.WriteAt(wbuf[:sz], int64(pos))
	if n1 != n2 {
		t.Fatalf("write: %d vs %d bytes", n1, n2)
	}
	if err1 != nil && err2 != nil {
		t.Logf("write %s fails", p1)
		return false
	}
	if err1 != nil || err2 != nil {
		t.Fatalf("errors in write: %v vs %v", err1, err2)
	}
	t.Logf("write %s %d [%d] ok", p1, pos, sz)
	return true
}

func fmkdir(t Fataler, p1, p2 string) bool {
	err1 := os.MkdirAll(p1, 0750)
	err2 := os.MkdirAll(p2, 0750)
	if err1 != nil && err2 != nil {
		t.Logf("mkdir %s fails", p1)
		return false
	}
	if err1 != nil || err2 != nil {
		t.Fatalf("errors in mkdir: %v vs %v", err1, err2)
	}
	t.Logf("mkdir %s ok", p1)
	return true
}

func fwstat(t Fataler, p1, p2 string) bool {
	err1 := os.Chmod(p1, 0760)
	err2 := os.Chmod(p2, 0760)
	if err1 != nil && err2 != nil {
		t.Logf("wstat %s fails", p1)
		return false
	}
	if err1 != nil || err2 != nil {
		t.Fatalf("errors in wstat: %v vs %v", err1, err2)
	}
	t.Logf("wstat %s ok", p1)
	return true
}

func fread(t Fataler, p1, p2 string) bool {
	fd1, err1 := os.Open(p1)
	if fd1 != nil {
		defer fd1.Close()
	}
	fd2, err2 := os.Open(p2)
	if fd2 != nil {
		defer fd2.Close()
	}
	if err1 != nil && err2 != nil {
		t.Logf("read %s fails", p1)
		return false
	}
	if err1 != nil || err2 != nil {
		t.Fatalf("errors in read: %v vs %v", err1, err2)
	}
	pos := rand.Intn(maxSeek)
	sz := rand.Intn(len(wbuf))
	var rbuf1, rbuf2 [maxWrite]byte
	n1, err1 := fd1.ReadAt(rbuf1[:sz], int64(pos))
	n2, err2 := fd2.ReadAt(rbuf2[:sz], int64(pos))
	if n1 != n2 {
		t.Fatalf("read: %d vs %d bytes", n1, n2)
	}
	if err1 != nil && err2 != nil {
		t.Logf("read %s fails", p1)
		return false
	}
	if err1 != nil || err2 != nil {
		t.Fatalf("errors in remove: %v vs %v", err1, err2)
	}
	if bytes.Compare(rbuf1[:n1], rbuf2[:n2]) != 0 {
		t.Fatalf("didn't read the same")
	}
	t.Logf("read %s %d [%d] ok", p1, pos, sz)
	return true
}

func fstat(t Fataler, p1, p2 string) bool {
	st1, err1 := os.Stat(p1)
	st2, err2 := os.Stat(p2)
	if err1 != nil && err2 != nil {
		t.Logf("stat %s fails", p1)
		return false
	}
	if err1 != nil || err2 != nil {
		t.Fatalf("errors in stat: %v vs %v", err1, err2)
	}
	if st1.Name() != st2.Name() {
		t.Fatalf("names: %s vs %s", st1.Name(), st2.Name())
	}
	if st1.Mode() != st2.Mode() {
		t.Fatalf("modes: %s vs %s", st1.Mode(), st2.Mode())
	}
	if !st1.IsDir() && st1.Size() != st2.Size() {
		t.Fatalf("sizes: %s vs %s", st1.Size(), st2.Size())
	}
	t.Logf("stat %s ok", p1)
	return true
}

// Perform black box testing of dir1
// by performing random FS ops in it and a read OS dir and comparing the trees
// and the results from the operations.
// There can be at most one invocation of this function at a time.
func AsAFs(t Fataler, dirs ...string) {
	dir1 := dirs[0]
	os.RemoveAll(osdir)
	defer os.RemoveAll(osdir)
	if err := os.Mkdir(osdir, 0775); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	dir1 = dir1 + "/fst"
	os.RemoveAll(dir1)
	if err := os.Mkdir(dir1, 0775); err != nil {
		t.Fatalf("mkdir: %s", err)
	}
	defer os.RemoveAll(dir1)

	opc := make(chan fsOp)
	nops := 0
	for o := oFirst; o < oRead; o++ {
		n := nOps
		if o == oCreate || o == oMkdir {
			n += nOps
		}
		nops += n
		go func(o fsOp, n int) {
			for i := 0; i < n; i++ {
				if ok := opc <- o; !ok {
					break
				}
			}
		}(o, n)
	}
	for o := oRead; o < oMax; o++ {
		go func(o fsOp) {
			nops += nOps
			for i := 0; i < nOps; i++ {
				if ok := opc <- o; !ok {
					break
				}
			}
		}(o)
	}
	t.Logf("%d ops", nops)
	counts = map[fsOp]int{}
	for i := 0; i < nops; i++ {
		// Make an update operation
		op := <-opc
		if op == oRemove && i%2 == 0 {
			continue
		}
		fp := paths[i%len(paths)]
		p1 := zx.Path(dir1, fp)
		p2 := zx.Path(osdir, fp)
		ok := calls[op](t, p1, p2)
		if ok {
			counts[op] = counts[op] + 1
		}
	}
	for o := oFirst; o < oMax; o++ {
		t.Logf("op %v: %d ok", o, counts[o])
	}
	close(opc)
}
