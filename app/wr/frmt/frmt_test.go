package frmt

import (
	"testing"
	"clive/dbg"
	"os"
	"strings"
)

var dprintf = dbg.FuncPrintf(os.Stderr, testing.Verbose)

func TestWords(t *testing.T) {
	txt := []string{` a te`, `xt wit`, `h`, `
  some`,`
spaces `,`and words `,
	}
	sc, rc := Words()
	go func() {
		for _, t := range txt {
			sc <- t
		}
		close(sc)
	}()
	words := []string{}
	for w := range rc {
		ws := string(w)
		dprintf("word [%s]\n", ws)
		words = append(words, ws)
	}
	xw := strings.Join(words, "|")
	dprintf("out = %#v\n", xw)
	if xw != " |a| |text| |with|\n  |some|\n|spaces| |and| |words| " {
		t.Fatalf("bad out")
	}
}

func TestFmt(t *testing.T) {
	txt := []string{` a te`, `xt with


  some`,`
spaces `,`and words and.a.very.long.word here.`,
	}
	sc, wc := Words()
	go func() {
		for _, t := range txt {
			sc <- t
		}
		close(sc)
	}()
	dprintf("ln [0123456789]\n")
	lnc := Fmt(wc, 10, false, NoBlankLines)
	lns := []string{}
	for w := range lnc {
		ln := string(w)
		dprintf("ln [%s]\n", ln)
		lns = append(lns, ln)
	}
	x := strings.Join(lns, "|")
	dprintf("out = %#v\n", x)
	if x != "a text|with some|spaces and|words|and.a.very.long.word|here." {
		t.Fatalf("bad out")
	}
}

func Test2Fmt(t *testing.T) {
	txt := []string{` a te`, `xt w th
  so e`,`
sp ces `,`and words and.a.very.long.word 


here.`,
	}
	sc, wc := Words()
	go func() {
		for _, t := range txt {
			sc <- t
		}
		close(sc)
	}()
	dprintf("ln [0123456789]\n")
	lnc := Fmt(wc, 10, true, NoBlankLines)
	lns := []string{}
	for w := range lnc {
		ln := string(w)
		dprintf("ln [%s]\n", ln)
		lns = append(lns, ln)
	}
	x := strings.Join(lns, "|")
	dprintf("out = %#v\n", x)
	if false && x != "a  text  w|th so e sp|ces    and|words|and.a.very.long.word|here." {
		t.Fatalf("bad out")
	}
}

func TestAllFmt(t *testing.T) {
	txt := []string{`
 a te`, `xt with


  some`,`
spaces `,`and 

words and.a.very.long.word here.`,
	}
	sc, wc := Words()
	go func() {
		for _, t := range txt {
			sc <- t
		}
		close(sc)
	}()
	dprintf("ln [0123456789]\n")
	lnc := Fmt(wc, 10, false, AllBlankLines)
	lns := []string{}
	for w := range lnc {
		ln := string(w)
		dprintf("ln [%s]\n", ln)
		lns = append(lns, ln)
	}
	x := strings.Join(lns, "|")
	dprintf("out = %#v\n", x)
	if x != "|a text|with|||some|spaces and||words|and.a.very.long.word|here." {
		t.Fatalf("bad out")
	}

}

func TestOneFmt(t *testing.T) {
	txt := []string{`
 a te`, `xt with


  some`,`
spaces `,`and 

words and.a.very.long.word here.`,
	}
	sc, wc := Words()
	go func() {
		for _, t := range txt {
			sc <- t
		}
		close(sc)
	}()
	dprintf("ln [0123456789]\n")
	lnc := Fmt(wc, 10, false, OneBlankLine)
	lns := []string{}
	for w := range lnc {
		ln := string(w)
		dprintf("ln [%s]\n", ln)
		lns = append(lns, ln)
	}
	x := strings.Join(lns, "|")
	dprintf("out = %#v\n", x)
	if x != "|a text|with||some|spaces and||words|and.a.very.long.word|here." {
		t.Fatalf("bad out")
	}

}

func TestROneFmt(t *testing.T) {
	txt := []string{`
 a te`, `xt with


  some`,`
spaces `,`and 

words and.a.very.long.word here.`,
	}
	sc, wc := Words()
	go func() {
		for _, t := range txt {
			sc <- t
		}
		close(sc)
	}()
	dprintf("ln [0123456789]\n")
	lnc := Fmt(wc, 10, true, OneBlankLine)
	lns := []string{}
	for w := range lnc {
		ln := string(w)
		dprintf("ln [%s]\n", ln)
		lns = append(lns, ln)
	}
	x := strings.Join(lns, "|")
	dprintf("out = %#v\n", x)
	if x != "|a     text|with||some|spaces and||words|and.a.very.long.word|here." {
		t.Fatalf("bad out")
	}

}
