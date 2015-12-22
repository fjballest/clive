package zx

import (
	"testing"
	"clive/dbg"
)

var (
	debug bool
	printf = dbg.FlagPrintf(&debug)
)

func TestPaths(t *testing.T) {
	debug = testing.Verbose()

	prefs := [...]string {"", "/", "/a", "/b", "/a/b", "..", "z", "/c/d", "", "/a/b/..", "/a/b/../..", "/a/b/../../.."}
	suffs := [...]string{"", "", "/", "", "/b", "", "", "", "", "/", "", ""}
	rsuffs := [...]string {"", "/", "/a", "/b", "/a/b", "", "", "/c/d", "", "/a", "/", "/"}

	r := Suffix("/a/b", "/a")
	printf("suff /a/b /a %q\n", r)
	if r != "/b" {
		t.Fatalf("bad suffix")
	}
	for i, p := range prefs {
		r := Suffix(p, "/a")
		printf("suff %q %q ->  %q\n", "/a", p, r)
		if suffs[i] != r {
			t.Fatalf("bad suffix")
		}
		r = Suffix(p, "/")
		printf("suff %q %q ->  %q\n", "/", p, r)
		if rsuffs[i] != r {
			t.Fatalf("bad suffix")
		}
		r = Suffix(p, "")
		printf("suff '' %q ->  %q\n", p, r)
		if r != "" {
			t.Fatalf("bad suffix")
		}
	}
}
