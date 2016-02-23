package look

import (
	"testing"
)

func TestCmdFor(t *testing.T) {
	debug = testing.Verbose()
	r := &Rule{Rexp: `^([a-zA-Z.]+)\(([0-9]+)\)$`, Cmd: `man \2 \1`}
	s, err := r.CmdFor("foo(1)")
	t.Logf("got %v %v\n", s, err)
	if s != "man 1 foo" {
		t.Fatalf("didn't get the expected match")
	}
}

func TestParse(t *testing.T) {
	debug = testing.Verbose()
	txt := `# example

		#rule set
		^([a-zA-Z.]+)\(([0-9]+)\)$
			man \2 \1
		foo
			bar
	`

	rs, err := ParseRules(txt)
	if err != nil {
		t.Fatalf("err %v", err)
	}
	t.Logf("parsed: %v", rs)
	xs := `^([a-zA-Z.]+)\(([0-9]+)\)$
	man \2 \1
foo
	bar
`
	if xs != rs.String() {
		t.Fatalf("bad rules")
	}
}
