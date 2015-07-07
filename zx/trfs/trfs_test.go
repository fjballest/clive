package trfs

import (
	"clive/dbg"
	"clive/zx"
	"clive/zx/mfs"
	"clive/zx/fstest"
	"clive/net/auth"
	"os"
	"testing"
)

const tdir = "/tmp/mfs_test"

var (
	printf = dbg.FuncPrintf(os.Stdout, testing.Verbose)
	moreverb = false
)

func testfn(t *testing.T, fn func(t fstest.Fataler, fss ...zx.Tree)) {
	fstest.Repeats = 1
	fs, err := mfs.New("example mfs")
	if err != nil {
		dbg.Fatal("lfs: %s", err)
	}
	xfs, _:= fs.AuthFor(&auth.Info{Uid: dbg.Usr, SpeaksFor: dbg.Usr, Ok: true})
	tr := New(xfs)
	tc := make(chan string)
	dc := make(chan bool)
	go func() {
		for x := range tc {
			printf("%s\n", x)
		}
		dc <-true
	}()
	fstest.MkZXTree(t, tr)
	fs.Dbg = testing.Verbose()
	if fs.Dbg {
		defer fs.Dump(os.Stdout)
	}
	tr.C = tc
	if fn != nil {
		fn(t, tr)
	}
	tr.Close(nil)
	close(tc)
	<-dc
}

func TestInitFiles(t *testing.T) {
	testfn(t, nil)
}

func TestStats(t *testing.T) {
	testfn(t, fstest.Stats)
}

func TestGets(t *testing.T) {
	testfn(t, fstest.Gets)
}

func TestFinds(t *testing.T) {
	testfn(t, fstest.Finds)
}

func TestFindGets(t *testing.T) {
	testfn(t, fstest.FindGets)
}

func TestPuts(t *testing.T) {
	testfn(t, fstest.Puts)
}

func TestMkdirs(t *testing.T) {
	testfn(t, fstest.Mkdirs)
}

func TestRemoves(t *testing.T) {
	testfn(t, fstest.Removes)
}

func TestWstats(t *testing.T) {
	testfn(t, fstest.Wstats)
}


func TestAsAFile(t *testing.T) {
	testfn(t, fstest.AsAFile)
}

func TestParse(t *testing.T) {
	ops := []string {
		`->put[32] /a/b path:/a/b mode:0777`,
		`cfs->put[32] /a/b path:/a/b mode:0777`,
	}
	for _, op := range ops {
		p := Parse(op)
		printf("%s\n", p)
		if p.String() != op {
			t.Fatalf("wrong op")
		}
	}
}
