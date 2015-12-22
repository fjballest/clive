package fstest

import (
	"io/ioutil"
	"os"
	"testing"
)

func BenchmarkOSStats(b *testing.B) {
	tdir := "/tmp/osbench"
	b.StopTimer()
	defer func() {
		b.StopTimer()
		RmTree(b, tdir)
	}()
	RmTree(b, tdir)
	MkTree(b, tdir)
	b.StartTimer()
	for bi := 0; bi < b.N; bi++ {
		i := bi % len(StatTests)
		st := StatTests[i]
		if st.Fails {
			continue
		}
		_, err := os.Stat(tdir + "/" + st.Path)
		if err != nil {
			b.Fatalf("stat %s: %s", st.Path, err)
		}
	}
}

func BenchmarkOSGets(b *testing.B) {
	tdir := "/tmp/osbench"
	b.StopTimer()
	defer func() {
		b.StopTimer()
		RmTree(b, tdir)
	}()
	RmTree(b, tdir)
	MkTree(b, tdir)
	b.StartTimer()
	for bi := 0; bi < b.N; bi++ {
		dat, err := ioutil.ReadFile(tdir + "/2")
		if len(dat) != 31658 || err != nil {
			b.Fatalf("bad read total")
		}
	}
}

func BenchmarkOSPuts(b *testing.B) {
	tdir := "/tmp/osbench"
	b.StopTimer()
	defer func() {
		b.StopTimer()
		RmTree(b, tdir)
	}()
	RmTree(b, tdir)
	MkTree(b, tdir)
	var buf [1024]byte
	copy(buf[0:], "hola")
	os.Remove(tdir + "/nfb")
	b.StartTimer()
	for bi := 0; bi < b.N; bi++ {
		fd, err := os.Create(tdir + "/nfb")
		if err != nil {
			b.Fatalf("creat %s", err)
		}
		for i := 0; i < 128; i++ {
			_, err := fd.Write(buf[:])
			if err != nil {
				b.Fatalf("write %s", err)
			}
		}
		if fd.Close() != nil {
			b.Fatalf("close %s", err)
		}
	}
}
