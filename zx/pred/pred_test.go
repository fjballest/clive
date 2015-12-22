package pred

import (
	"clive/dbg"
	"clive/zx"
	"os"
	"path"
	"testing"
)

/*
	go test -bench .
	BenchmarkNew                  	 1000000	      1453 ns/op
	BenchmarkPredString           	 5000000	       320 ns/op

	go test -bench New -cpuprofile /tmp/prof.out
	win go tool pprof pred.test /tmp/prof.out
*/

var printf = dbg.FuncPrintf(os.Stdout, testing.Verbose)

type top struct {
	s   string
	err bool
	out string
}

var preds = []top{
	{"name", true, ""},
	{"!name", true, ""},
	{"(name)", true, ""},
	{"t", false, "true"},
	{"4", false, "depth <= 4"},
	{"mtime==555", false, "mtime == 555"},
	{"mtime==555|name!=", true, ""},
	{"mtime==555|name!='*.x'", false, "mtime == 555 | name != *.x"},
	{"(mtime==555|name!='*.x')&mtime == 555", false, "(mtime == 555 | name != *.x) & mtime == 555"},
	{"!(mtime==555|name!='*.x')&!mtime == 555", false, "!(mtime == 555 | name != *.x) & !mtime == 555"},
	{"name=b&mtime>4", false, "name = b & mtime > 4"},
	{"path~*.c", false, "path ~ *.c"},
	{"prune", false, "prune"},
	{"true", false, "true"},
}

func TestPred(t *testing.T) {
	for i := range preds {
		printf("parse '%s'\n", preds[i].s)
		p, err := New(preds[i].s)
		if err != nil {
			printf("err %s\n", err)
			if !preds[i].err {
				t.Fatal(err)
			}
			continue
		}
		printf("%s\n", p)
		if preds[i].err {
			t.Fatal("did parse")
		}
		if preds[i].out != p.String() {
			printf("[%#v]\n", p)
			t.Fatal("wrong output " + p.String())
		}
		np, err := New(preds[i].out)
		if err != nil {
			t.Fatal(err)
		}
		if np.String() != preds[i].out {
			t.Fatal("reparse didnt match for " + np.String())
		}
	}
}

func TestPrune(t *testing.T) {
	debug = testing.Verbose()
	d := zx.Dir{
		"path":  "/a/b",
		"name":  "b",
		"type":  "d",
		"mode":  "0755",
		"spath": "/a/b",
		"tpath": "/tmp/lfs_test",
		"proto": "lfs",
	}
	pr := `(path = "/a/b" | path = "/d") & prune | type = d`
	p, err := New(pr)
	if err != nil {
		t.Fatalf("parse %s", err)
	}
	t.Logf("eval %s\n", p)
	m, prune, err := p.EvalAt(d, 0)
	t.Logf("match %v %v %v", m, prune, err)
	if m != false || prune != true || err != nil {
		t.Fatalf("bad eval")
	}
}

func TestName(t *testing.T) {
	debug = testing.Verbose()
	d := zx.Dir{
		"path":  "/a/b",
		"name":  "b",
		"type":  "d",
		"mode":  "0755",
		"spath": "/a/b",
		"tpath": "/tmp/lfs_test",
		"proto": "lfs",
	}
	preds := []string{
		`name=b`,
		`! name=b`,
		`name!=b`,
	}
	matches := []bool{true, false, false}
	prunes := []bool{false, true, true}
	for i, pr := range preds {
		p, err := New(pr)
		if err != nil {
			t.Fatalf("parse %s", err)
		}
		t.Logf("eval %s\n", p.DebugString())
		m, prune, err := p.EvalAt(d, 0)
		t.Logf("match %v prune %v sts %v", m, prune, err)
		if err != nil || m != matches[i] || prune != prunes[i] {
			t.Logf("wrong result %v %v %v", err, m, prune)
			t.Fail()
		}
	}
}

type mTest struct {
	pred, path      string
	matches, prunes bool
}

func TestMatch(t *testing.T) {
	debug = testing.Verbose()
	d := zx.Dir{
		"path":  "/a/b",
		"name":  "b",
		"type":  "d",
		"mode":  "0755",
		"spath": "/a/b",
		"tpath": "/tmp/lfs_test",
		"proto": "lfs",
	}
	preds := []mTest{
		mTest{`~*.c`, `/a/b`, false, false},
		mTest{`~*.c`, `/a/b/.c`, true, false},
		mTest{`~*.c`, `/a/b/c`, false, false},
		mTest{`~*.c`, `/.c`, true, false},
		mTest{`~/*.c`, `/.c`, true, false},
		mTest{`~/*.c`, `/.cd`, false, true},
		mTest{`~/*b`, `/a/b`, false, true},
		mTest{`~/*/b/*/*/b`, `/a/b`, false, false},
		mTest{`~/*/c/*/*/b`, `/a/b`, false, true},
	}
	for _, pr := range preds {
		p, err := New(pr.pred)
		if err != nil {
			t.Fatalf("parse %s", err)
		}
		t.Logf("eval %s\n", p.DebugString())
		d["path"] = pr.path
		d["name"] = path.Base(pr.path)
		m, prune, err := p.EvalAt(d, 0)
		t.Logf("match %v prune %v sts %v", m, prune, err)
		if err != nil || m != pr.matches || prune != pr.prunes {
			t.Logf("wrong result %v %v %v", m, prune, err)
			t.Fail()
		}
	}
}

func BenchmarkNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		id := i % len(preds)
		New(preds[id].s)
	}
}

func BenchmarkPredString(b *testing.B) {
	b.StopTimer()
	p := []*Pred{}
	for i := 0; i < len(preds); i++ {
		x, err := New(preds[i].s)
		if err == nil {
			p = append(p, x)
		}
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		id := i % len(p)
		p[id].String()
	}

}
