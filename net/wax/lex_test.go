package wax

import (
	"clive/dbg"
	"fmt"
	"os"
	"testing"
)

var Printf = dbg.FuncPrintf(os.Stdout, testing.Verbose)

type tst struct {
	txt  string
	toks []tok
	vals []string
}

var tsts = []tst{
	{"", []tok{}, []string{}},
	{"$", []tok{tErr}, []string{}},
	{"$$", []tok{tText}, []string{"$"}},
	{"a$$", []tok{tText}, []string{"a$"}},
	{"a$$a", []tok{tText}, []string{"a$a"}},
	{"a$", []tok{tErr}, []string{""}},
	{"$$a", []tok{tText}, []string{"$a"}},
	{"$a", []tok{tErr}, []string{""}},
	{"$a$", []tok{tId}, []string{"a"}},
	{"x y$a b$xy",
		[]tok{tText, tId, tId, tText},
		[]string{"x y", "a", "b", "xy"},
	},
	{"xy$$$ab$xy$c$",
		[]tok{tText, tId, tText, tId},
		[]string{"xy$", "ab", "xy", "c"},
	},
	{"xy$$$for t in a.b do xx end$",
		[]tok{tText, tFor, tId, tIn, tId, tDot, tId, tDo, tId, tEnd},
		[]string{"xy$", "for", "t", "in", "a", ".", "b", "do", "xx", "end"},
	},
	{"xy$$$if a.c do xx else aa bb end$",
		[]tok{tText, tIf, tId, tDot, tId, tDo, tId, tElse, tId, tId, tEnd},
		[]string{"xy$", "if", "a", ".", "c", "do", "xx", "else", "aa", "bb", "end"},
	},
}

func TestLex(t *testing.T) {
	for _, ti := range tsts {
		l := newLex(ti.txt)
		toks := []tok{}
		vals := []string{}
		Printf("->%s:\n", ti.txt)
		for t := l.next(); t != tEOF; t = l.next() {
			toks = append(toks, t)
			vals = append(vals, l.val())
			Printf("\t%v\t%q\t%v\n", t, l.val(), l.err)
			if t == tErr {
				break
			}
			Printf("%s.%s:\n", string(ti.txt[l.p0:l.p1]), string(ti.txt[l.p1:]))
		}
		etoks := fmt.Sprintf("%v", ti.toks)
		evals := fmt.Sprintf("%v", ti.vals)
		stoks := fmt.Sprintf("%v", toks)
		svals := fmt.Sprintf("%v", vals)
		if etoks != stoks {
			Printf("got: %s\n", stoks)
			Printf("expected: %s\n", etoks)
			t.Fatal("wrong tokens")
		}
		if evals != svals {
			Printf("got: %s\n", svals)
			Printf("expected: %s\n", evals)
			t.Fatal("wrong values")
		}
	}
}
