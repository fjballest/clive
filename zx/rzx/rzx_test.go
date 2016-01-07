package rzx

import (
	"clive/ch"
	"clive/net"
	"clive/net/auth"
	"clive/u"
	"clive/zx"
	"clive/zx/fstest"
	"clive/zx/zux"
	"io"
	"os"
	"testing"
)

struct tb {
	r io.ReadCloser
	w io.WriteCloser
}

var (
	tdir = "/tmp/rzxtest"
	ai   = &auth.Info{Uid: u.Uid, SpeaksFor: u.Uid, Ok: true}
)

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

var (
	md   = zx.Dir{"type": "d", "mode": "0755"}
	msgs = [...]*Msg{
		&Msg{Op: Ttrees},
		&Msg{Op: Tstat, Fsys: "main", Path: "/a"},
		&Msg{Op: Tget, Fsys: "main", Path: "/a", Off: -1, Count: 1},
		&Msg{Op: Tput, Fsys: "main", Path: "/a", D: md, Off: -1},
		&Msg{Op: Tmove, Fsys: "main", Path: "/a", To: "/b"},
		&Msg{Op: Tlink, Fsys: "main", Path: "/a", To: "/b"},
		&Msg{Op: Tremove, Fsys: "main", Path: "/a"},
		&Msg{Op: Tremoveall, Fsys: "main", Path: "/a"},
		&Msg{Op: Twstat, Fsys: "main", Path: "/a", D: md},
		&Msg{Op: Tfind, Fsys: "main", Path: "/a",
			Pred: "name=x", Spref: "/", Dpref: "/", Depth: 1},
		&Msg{Op: Tfindget, Fsys: "main", Path: "/a",
			Pred: "name=x", Spref: "/", Dpref: "/", Depth: 1},
	}
	omsgs = [...]string{
		`Ttrees`,
		`Tstat 'main' '/a'`,
		`Tget 'main' '/a' off -1 count 1`,
		`Tput 'main' '/a' off -1 d <type:"d" mode:"0755"> `,
		`Tmove 'main' '/a' to '/b'`,
		`Tlink 'main' '/a' to '/b'`,
		`Tremove 'main' '/a'`,
		`Tremoveall 'main' '/a'`,
		`Twstat 'main' '/a' d <type:"d" mode:"0755"> `,
		`Tfind 'main' '/a' pred 'name=x' spref '/' dpref '/' depth 1`,
		`Tfindget 'main' '/a' pred 'name=x' spref '/' dpref '/' depth 1`,
	}
)

func TestProto(t *testing.T) {
	os.Args[0] = "rzx.test"
	fd := &tb{}
	fd.r, fd.w, _ = os.Pipe()
	p := ch.NewConn(fd, 300, nil)

	for _, m := range msgs {
		p.Out <- m
	}
	for i := range msgs {
		x := <-p.In
		m, ok := x.(*Msg)
		if !ok {
			t.Fatalf("bad msg type %T", x)
		}
		t.Logf("msg %s\n", m)
		if m.String() != omsgs[i] {
			t.Fatal("bad out msg")
		}
	}
}

func runTest(t *testing.T, fn fstest.TestFunc) {
	os.Remove("/tmp/clive.9898")
	defer os.Remove("/tmp/clive.9898")
	os.Args[0] = "rzx.test"
	fstest.Verb = testing.Verbose()
	ccfg, err := net.TLSCfg("/Users/nemo/.ssh/client")
	if err != nil {
		t.Logf("no certs found, no tls conn")
	}
	scfg, err := net.TLSCfg("/Users/nemo/.ssh/server")
	if err != nil || ccfg == nil {
		ccfg = nil
		scfg = nil
		t.Logf("no certs found, no tls conn")
	}
	fstest.MkTree(t, tdir)
	defer os.RemoveAll(tdir)
	fs, err := zux.NewZX(tdir)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := NewServer("unix!local!9898", scfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Serve("tree", fs); err != nil {
		t.Fatal(err)
	}
	rfs, err := Dial("unix!local!9898", ccfg)
	if err != nil {
		t.Fatal(err)
	}
	rfs.Debug = testing.Verbose()
	ts := rfs.Trees()
	t.Logf("trees: %v", ts)
	if len(ts) != 2 || ts[1] != "tree" {
		t.Fatal("bad tree")
	}
	if rfs, err = rfs.Fsys("tree"); err != nil {
		t.Fatal(err)
	}
	if fn != nil {
		fn(t, rfs)
	}
	rfs.Close()

	srv.Close()
}

func TestSrv(t *testing.T) {
	runTest(t, nil)
}

func TestStats(t *testing.T) {
	runTest(t, fstest.Stats)
}

func TestGetCtl(t *testing.T) {
	runTest(t, fstest.GetCtl)
}

func TestGets(t *testing.T) {
	runTest(t, fstest.Gets)
}

func TestFinds(t *testing.T) {
	runTest(t, fstest.Finds)
}

func TestFindGets(t *testing.T) {
	runTest(t, fstest.FindGets)
}

func TestPuts(t *testing.T) {
	runTest(t, fstest.Puts)
}

func TestMkdirs(t *testing.T) {
	runTest(t, fstest.Mkdirs)
}

func TestRemoves(t *testing.T) {
	runTest(t, fstest.Removes)
}

func TestWstats(t *testing.T) {
	runTest(t, fstest.Wstats)
}

func TestAttrs(t *testing.T) {
	runTest(t, fstest.Attrs)
}

func TestMoves(t *testing.T) {
	runTest(t, fstest.Moves)
}

func TestAsAFile(t *testing.T) {
	runTest(t, fstest.AsAFile)
}
