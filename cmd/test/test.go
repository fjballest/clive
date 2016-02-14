/*
	Help for testing commands
*/
package test

import (
	"bytes"
	"clive/dbg"
	"clive/zx/fstest"
	"os"
	"os/exec"
	"strings"
	"testing"
)

struct Run {
	Line     string
	Out, Err string
	Fails    bool
	Ok       bool // if set, out, err and fails are ignored
}

var tdir = "/tmp/cmdtest"

func Cmds(t *testing.T, runs []Run) {
	for _, r := range runs {
		o, e, fails := Cmd(t, r.Line)
		if r.Ok {
			continue
		}
		if fails {
			if !r.Fails {
				t.Fatal("command did fail")
			}
		} else {
			if r.Fails {
				t.Fatal("command didn't fail")
			}
		}
		if strings.TrimSpace(r.Out) != strings.TrimSpace(o) {
			t.Fatal("bad output")
		}
		if strings.TrimSpace(r.Err) != strings.TrimSpace(e) {
			t.Fatal("bad error output")
		}
	}
}

// Installs the command, for use before running the tests.
func InstallCmd(t *testing.T) {
	x := exec.Command("go", "install", "-v")
	if err := x.Run(); err != nil {
		t.Fatal(err)
	}
}

// Run cmd and make sure that the out and err streams are as given.
// The command runs at /tmp/cmdtest where zx/fstest.MkTree has created
// our usual file testing tree, in case the command needs input.
// It is run using "sh -c 'cd /tmp/cmdtest; <cmd>' ".
// go install is run before running the command.
func Cmd(t *testing.T, cmd string) (cout, cerr string, fails bool) {
	os.RemoveAll(tdir)
	fstest.MkTree(t, tdir)
	defer os.RemoveAll(tdir)
	var bout, berr bytes.Buffer
	if testing.Verbose() {
		dbg.Printf("run %s\n", cmd)
	}
	x := exec.Command("sh", "-c", "cd "+tdir+"; "+cmd)
	x.Stdout = &bout
	x.Stderr = &berr
	if err := x.Start(); err != nil {
		t.Fatal(err)
	}
	if err := x.Wait(); err != nil {
		fails = true
	}
	cout = bout.String()
	cerr = berr.String()
	if testing.Verbose() {
		dbg.Printf("out <%s>\n", cout)
		dbg.Printf("err <%s>\n", cerr)
	}
	return cout, cerr, fails
}
