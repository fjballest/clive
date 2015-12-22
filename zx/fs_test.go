package zx

import (
	"clive/dbg"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"
)

/*
	As of today:
	go test -bench Dir

	BenchmarkNewDir               	 5000000	       746 ns/op
	BenchmarkDirPack              	 5000000	       440 ns/op
	BenchmarkUnpackDir            	 2000000	       933 ns/op
	BenchmarkDirGetPath           	50000000	        41.2 ns/op
	BenchmarkDirSetPath           	50000000	        52.0 ns/op
	BenchmarkDirGetTime           	20000000	       109 ns/op
	BenchmarkDirSetTime           	 5000000	       497 ns/op
	BenchmarkDirGetSize           	50000000	        61.5 ns/op
	BenchmarkDirSetSize           	10000000	       227 ns/op
	BenchmarkStatNewDir           	  500000	      3241 ns/op

	Before optimizing:

	Using bytes.Buffer and Fprintf in Dir.String() gave
	BenchmarkDirString            	  500000	      5163 ns/op

	Using strings and += gave
	BenchmarkDirString            	  500000	      5077 ns/op

	Using []byte and append is even faster.

*/

const tdir = "/tmp/zx_test"

var (
	printf = dbg.FuncPrintf(os.Stdout, testing.Verbose)
	xt     int64
)

func touch(path string) {
	tm := time.Unix(xt/1e9, xt%1e9)
	os.Chtimes(path, tm, tm)
	xt += 1e9
}

func setup(t *testing.T, tdir string) {
	xt = 1
	f1data := []byte("hola\ncaracola\n")
	f2data := []byte("adios\ncara...")
	if err := os.MkdirAll(tdir+"/a/b", 0755); err != nil {
		t.Fatal(err)
	}
	touch(tdir + "/a/b")
	_, err := os.Stat(tdir)
	if err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(tdir+"/.db", f1data, 0644); err != nil {
		t.Fatal(err)
	}
	touch(tdir + "/.db")
	if err := ioutil.WriteFile(tdir+"/f1", f1data, 0644); err != nil {
		t.Fatal(err)
	}
	touch(tdir + "/f1")
	if err := ioutil.WriteFile(tdir+"/a/f2", f2data, 0644); err != nil {
		t.Fatal(err)
	}
	touch(tdir + "/a/f2")
	if err := ioutil.WriteFile(tdir+"/a/f3", f2data, 0644); err != nil {
		t.Fatal(err)
	}
	touch(tdir + "/a/f3")
	touch(tdir + "/a")
	if err := os.MkdirAll(tdir+"/c/d", 0755); err != nil {
		t.Fatal(err)
	}
	touch(tdir + "/c")
	touch(tdir)
	if err := ioutil.WriteFile(tdir+"/c/d/d3", f1data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(tdir+"/c/d/d4", f1data, 0644); err != nil {
		t.Fatal(err)
	}
	touch(tdir + "/c/d/d3")
	touch(tdir + "/c/d/d4")
	touch(tdir + "/c/d")
}

func ExampleIOstats() {
	stats := &IOstats{}
	defer func() {
		stats.Averages()
		fmt.Printf("iostats:\n%s\n", stats)
	}()
	// then run your code calling stats methods here
	// See NewCall for a example of how that code looks like.
}

func ExampleIOstats_NewCall() {
	// stats might be set to non-nil, or nil to avoid stats,
	// assume this is a get call and we are the code replying to it through c.
	var stats *IOstats
	var c chan []byte
	var done bool
	var err error

	sc := stats.NewCall(Sget)
	for !done {
		// produce some reply
		msg := []byte{ /*...*/ }

		// account for the reply and the # of bytes
		sc.Send(int64(len(msg)))

		c <- msg
	}

	close(c, err)
	// account for the reply/error
	sc.End(err != nil)
}

func ExampleNewDir() {
	fi, err := os.Stat("/a/file")
	if err != nil {
		dbg.Fatal(err)
	}
	d := NewDir(fi, 0)
	printf("%s\n", d)
	// could print `path"/a/file", name:"file" type:"d" mode:"0755" size:"3" mtime:"5000000000"`
}

func TestDir(t *testing.T) {
	os.RemoveAll(tdir)
	setup(t, tdir)
	defer os.RemoveAll(tdir)

	fi, err := os.Stat(tdir + "/a")
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	d := NewDir(fi, 3)
	printf("a `%s`\n", d)
	ds := `name:"a" type:"d" mode:"0755" size:"3" mtime:"5000000000"`
	if d.String() != ds {
		t.Logf("unexpected dir stat %s", d)
		t.Fail()
	}
	fi, err = os.Stat(tdir + "/a/f3")
	if err != nil {
		t.Log(err)
		t.Fail()
	}
	d = NewDir(fi, 3)
	printf("f3 `%s`\n", d)
	ds = `name:"f3" type:"-" mode:"0644" size:"13" mtime:"4000000000"`
	if d.String() != ds {
		t.Log("unexpected dir stat")
		t.Fail()
	}
	nd, _ := ParseDirString(ds)
	printf("parsed: %s\n", nd)
	if nd.String() != ds {
		t.Log("unexpected parsed dir stat")
		t.Fail()
	}
	nd["name"] = `One funny "string"
with specials`
	printf("dq `%s`\n", nd)
	nds := `name:"One funny \"string\"\nwith specials" type:"-" mode:"0644" size:"13" mtime:"4000000000"`
	if nd.String() != nds {
		t.Log("unexpected dir quote")
		t.Fail()
	}
	qd, _ := ParseDirString(nds)
	if qd.String() != nds {
		t.Fatal("unexpected parsed dir quote")
	}
	b := nd.Pack()
	nd, b, err = UnpackDir(b)
	if err != nil {
		t.Logf("unpack: %s", err)
		t.Fail()
		return
	}
	if len(b) != 0 {
		t.Logf("extra bytes in dir")
		t.Fail()
		return
	}
	if nd.String() != nds {
		t.Logf("bad unpacked dir string")
		t.Fail()
		return
	}
	ds = `path:"/z3" mode:0600 type:- size:3 mtime:2000000000 name:"z3"`
	pds := `path:"/z3" name:"z3" type:"-" mode:"0600" size:"3" mtime:"2000000000"`
	qd, _ = ParseDirString(ds)
	if qd.String() != pds {
		t.Logf("unexpected parsed dir `%s`", qd)
		t.Fail()
	}

}

func TestSuffix(t *testing.T) {
	ss := []string{"/", "/", "/a/b", "/a/b"}
	sss := []string{"/", "", "b", "/b"}
	for i, s := range ss {
		if !HasSuffix(s, sss[i]) {
			t.Fatalf("%s %s", s, sss[i])
		}
	}
}

func BenchmarkNewDir(b *testing.B) {
	b.StopTimer()
	dents, err := ioutil.ReadDir("/usr/bin")
	if err != nil {
		b.Fatalf("read /usr/bin: %s", err)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		id := i % len(dents)
		NewDir(dents[id], 0)
	}
}

func BenchmarkDirPack(b *testing.B) {
	b.StopTimer()
	dents, err := ioutil.ReadDir("/usr/bin")
	if err != nil {
		b.Fatalf("read /usr/bin: %s", err)
	}
	ds := make([]Dir, len(dents))
	for i := 0; i < len(dents); i++ {
		ds[i] = NewDir(dents[i], 0)
	}
	ss := make([][]byte, len(dents))
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		id := i % len(dents)
		ss[id] = ds[id].Pack()
	}
}

func BenchmarkUnpackDir(b *testing.B) {
	b.StopTimer()
	dents, err := ioutil.ReadDir("/usr/bin")
	if err != nil {
		b.Fatalf("read /usr/bin: %s", err)
	}
	ds := make([]Dir, len(dents))
	ss := make([][]byte, len(dents))
	for i := 0; i < len(dents); i++ {
		ds[i] = NewDir(dents[i], 0)
		ss[i] = ds[i].Pack()
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		id := i % len(dents)
		_, _, err := UnpackDir(ss[id])
		if err != nil {
			b.Fatalf("couldn't unpack: %s", err)
		}
	}
}

func BenchmarkDirGetPath(b *testing.B) {
	b.StopTimer()
	dents, err := ioutil.ReadDir("/usr/bin")
	if err != nil {
		b.Fatalf("read /usr/bin: %s", err)
	}
	ds := make([]Dir, len(dents))
	ss := make([]string, len(dents))
	for i := 0; i < len(dents); i++ {
		ds[i] = NewDir(dents[i], 0)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		id := i % len(dents)
		ss[id] = ds[id]["path"]
	}
}

func BenchmarkDirSetPath(b *testing.B) {
	b.StopTimer()
	dents, err := ioutil.ReadDir("/usr/bin")
	if err != nil {
		b.Fatalf("read /usr/bin: %s", err)
	}
	ds := make([]Dir, len(dents))
	for i := 0; i < len(dents); i++ {
		ds[i] = NewDir(dents[i], 0)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		id := i % len(dents)
		ds[id]["path"] = "foo"
	}
}

func BenchmarkDirGetTime(b *testing.B) {
	b.StopTimer()
	dents, err := ioutil.ReadDir("/usr/bin")
	if err != nil {
		b.Fatalf("read /usr/bin: %s", err)
	}
	ds := make([]Dir, len(dents))
	for i := 0; i < len(dents); i++ {
		ds[i] = NewDir(dents[i], 0)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		id := i % len(dents)
		ds[id].Time("mtime")
	}
}

func BenchmarkDirSetTime(b *testing.B) {
	b.StopTimer()
	dents, err := ioutil.ReadDir("/usr/bin")
	if err != nil {
		b.Fatalf("read /usr/bin: %s", err)
	}
	ds := make([]Dir, len(dents))
	for i := 0; i < len(dents); i++ {
		ds[i] = NewDir(dents[i], 0)
	}
	t := time.Now()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		id := i % len(dents)
		ds[id]["mtime"] = fmt.Sprintf("%d", t.UnixNano())
	}
}

func BenchmarkDirGetSize(b *testing.B) {
	b.StopTimer()
	dents, err := ioutil.ReadDir("/usr/bin")
	if err != nil {
		b.Fatalf("read /usr/bin: %s", err)
	}
	ds := make([]Dir, len(dents))
	for i := 0; i < len(dents); i++ {
		ds[i] = NewDir(dents[i], 0)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		id := i % len(dents)
		ds[id].Int64("size")
	}
}

func BenchmarkDirSetSize(b *testing.B) {
	b.StopTimer()
	dents, err := ioutil.ReadDir("/usr/bin")
	if err != nil {
		b.Fatalf("read /usr/bin: %s", err)
	}
	ds := make([]Dir, len(dents))
	for i := 0; i < len(dents); i++ {
		ds[i] = NewDir(dents[i], 0)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		id := i % len(dents)
		ds[id]["size"] = fmt.Sprintf("%d", 666)
	}
}

func BenchmarkStat(b *testing.B) {
	b.StopTimer()
	dents, err := ioutil.ReadDir("/usr/bin")
	if err != nil {
		b.Fatalf("read /usr/bin: %s", err)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		id := i % len(dents)
		_, err := os.Stat("/usr/bin/" + dents[id].Name())
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStatNewDir(b *testing.B) {
	b.StopTimer()
	dents, err := ioutil.ReadDir("/usr/bin")
	if err != nil {
		b.Fatalf("read /usr/bin: %s", err)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		id := i % len(dents)
		fi, err := os.Stat("/usr/bin/" + dents[id].Name())
		if err != nil {
			b.Fatal(err)
		}
		NewDir(fi, 0)
	}
}
