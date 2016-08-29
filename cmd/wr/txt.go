package main

import (
	"clive/sre"
	"fmt"
	"io"
	"strings"
	"unicode"
)

struct txtFmt {
	lvl int
	*par
	hasSeeAlso bool // hacks for clive man
}

type fltFun func(string) string

func indentVerb(s, pref, tab string) string {
	ns := ""
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for _, ln := range lines {
		ln = strings.TrimRightFunc(ln, unicode.IsSpace)
		if ln == "" {
			ns += "\n"
		} else {
			ns += pref + ln + "\n"
		}
	}
	return strings.Replace(ns, "\t", tab, -1)
}

const mrexp = `^([a-zA-Z.0-9]+)\(([0-9]+)\)$`

func (f *txtFmt) wrText(e *Elem) {
	if e == nil {
		return
	}
	if e.Nb != "" && !cliveMan {
		f.printPar(e.Nb, " ")
	}
	switch e.Kind {
	case Kurl:
		toks := strings.SplitN(e.Data, "|", 2)
		if len(toks) == 1 {
			e.Data = "[" + e.Data + "]"
		} else {
			e.Data = toks[0] + " [" + toks[1] + "]"
		}
	case Kcite:
		rg, _ := sre.Match(mrexp, e.Data)
		if len(rg) == 3 {
			break
		}
		fallthrough
	case Kbib:
		e.Data = "[" + e.Data + "]"
	case Knref:
		e.Data = "(" + e.Data + ")"
	case Kcref, Keref, Ktref, Kfref, Ksref:
	}
	f.printPar(e.Data)
	for _, c := range e.Textchild {
		f.wrText(c)
	}
}

var cop = ""

func (f *txtFmt) wrElems(els ...*Elem) {
	nb := 0
	inchap := false
	pref := strings.Repeat(f.tab, f.lvl)
	f.lvl++
	defer func() {
		f.lvl--
	}()
	for _, e := range els {
		f.i0, f.in = pref, pref
		f.fn = nil
		if e.Kind == Kchap {
			inchap = true
		}
		switch e.Kind {
		case Kcop:
			cop = e.Data
		case Kfont, Kit, Kbf, Ktt, Kitend, Kbfend, Kttend:
			if f.sc != nil && !e.Inline {
				f.printPar(" ")
			}
		case Kchap, Khdr1, Khdr2, Khdr3:
			f.closePar()
			f.hasSeeAlso = false
			if cliveMan && strings.ToLower(e.Data) == "see also" {
				f.hasSeeAlso = true
			}
			if cliveMan && e.Kind != Khdr3 {
				f.fn = strings.ToUpper
			}
			if strings.ToLower(e.Data) == "abstract" && inchap {
				e.Data = ""
			}
			f.newPar()
			f.wrText(e)
			f.closePar()
		case Kpar:
			f.printCmd("\n")
		case Kbr:
			f.closePar()
		case Kindent, Kitemize, Kenumeration, Kdescription:
			f.closePar()
			nb = 0
			f.wrElems(e.Child...)
		case Kname:
			f.closePar()
			npref := pref
			if len(npref) > 2 {
				npref = npref[2:]
			}
			f.i0, f.in = npref, pref
			f.newPar()
			f.wrText(e)
			f.closePar()
			f.wrElems(e.Child...)
		case Kitem, Kenum:
			f.closePar()
			npref := pref
			if len(npref) > 2 {
				npref = npref[2:]
			}
			s := fmt.Sprintf("%s* ", npref)
			if e.Kind == Kenum {
				nb++
				s = fmt.Sprintf("%s%d. ", npref, nb)
			}
			f.i0, f.in = s, pref
			f.newPar()
			f.wrText(e)
		case Kverb, Ksh:
			f.closePar()
			if e.Kind == Kverb && e.Tag != "" {
				tg := indentVerb("["+e.Tag+"]", pref, f.tab)
				f.printCmd("%s", tg)
			}
			e.Data = indentVerb(e.Data, pref, f.tab)
			f.printCmd("%s", e.Data)
		case Kfoot:
			// printed at the end.
		case Ktext, Kurl, Kbib, Kcref, Keref, Knref, Ktref, Kfref, Ksref, Kcite:
			f.wrText(e)
		case Kfig, Kpic, Kgrap:
			if e.Kind == Kpic || e.Kind == Kgrap {
				e.Data = "pic drawing"
			} else {
				e.Data = strings.TrimSpace(e.Data)
			}
			f.closePar()
			xpref := pref + f.tab
			s := e.Data
			f.printCmd("%s[%s]\n", xpref+f.tab, s)
			if e.Caption == nil {
				f.printCmd("%s%s %s.\n\n", xpref, labels[e.Kind], e.Nb)
				break
			}
			f.i0, f.in = xpref, xpref
			f.newPar()
			f.printPar(labels[e.Kind]+" ", e.Nb, ": ")
			f.wrText(e.Caption)
			f.closePar()
		case Ktbl:
			f.closePar()
			f.lvl += 2
			f.wrTbl(e.Tbl)
			f.lvl -= 2
			xpref := pref + f.tab
			if e.Caption == nil {
				f.printCmd("%s%s %s.\n\n", xpref, labels[e.Kind], e.Nb)
				break
			}
			f.i0, f.in = xpref, xpref
			f.newPar()
			f.printPar(labels[e.Kind]+" ", e.Nb, ": ")
			f.wrText(e.Caption)
			f.closePar()
		case Keqn:
			f.closePar()
			xpref := pref + f.tab
			s := "eqn data"
			f.printCmd("%s[%s]\n", xpref+f.tab, s)
			if e.Caption == nil {
				f.printCmd("%s%s %s.\n\n",
					xpref, labels[e.Kind], e.Nb)
				break
			}
			f.i0, f.in = xpref, xpref
			f.newPar()
			f.printPar(labels[e.Kind]+" ", e.Nb, ": ")
			f.wrText(e.Caption)
			f.closePar()
		case Kcode:
			xpref := pref + f.tab
			e.Data = indentVerb(e.Data, xpref+f.tab, f.tab)
			f.closePar()
			f.printCmd("%s", e.Data)
			if e.Caption == nil {
				f.printCmd("%s%s %s.\n\n", xpref, labels[e.Kind], e.Nb)
				break
			}
			f.i0, f.in = xpref, xpref
			f.newPar()
			f.printPar(labels[e.Kind]+" ", e.Nb, ": ")
			f.wrText(e.Caption)
			f.closePar()
		}
	}
	f.closePar()
}

func (f *txtFmt) wrTbl(rows [][]string) {
	pref := strings.Repeat(f.tab, f.lvl)
	if len(rows) < 2 {
		return
	}
	rows = rows[1:]
	f.printCmd("%s---\n", pref)
	for _, r := range rows {
		f.printCmd("%s", pref)
		for _, c := range r {
			f.printCmd("%s\t", c)
		}
		f.printCmd("\n")
	}
	f.printCmd("%s---\n", pref)
}

func (f *txtFmt) wrBib(refs []string) {
	if len(refs) == 0 {
		return
	}
	if !cliveMan {
		fmt.Fprintf(f.out, "\nREFERENCES\n\n")
	} else if !f.hasSeeAlso {
		fmt.Fprintf(f.out, "\nSEE ALSO\n\n")
	} else {
		fmt.Fprintf(f.out, "\nExternal references\n\n")
	}
	for i, r := range refs {
		f.i0, f.in = "", "  "
		f.newPar()
		f.printPar(fmt.Sprintf("%d. %s", i+1, r))
		f.closePar()
	}
}

func (f *txtFmt) wrFoots(t *Text) {
	foots := t.refs[Kfoot]
	if len(foots) == 0 {
		return
	}
	fmt.Fprintf(f.out, "\nNOTES\n\n")
	for _, ek := range foots {
		e := ek.el
		f.i0, f.in = "", "  "
		f.newPar()
		f.printPar(fmt.Sprintf("%s. ", e.Nb))
		f.wrText(e)
		f.endPar()
	}
}

func (f *txtFmt) run(t *Text) {
	els := t.Elems
	up := strings.ToUpper
	for len(els) > 0 && els[0].Kind == Ktitle {
		f.i0, f.in, f.fn = "", "", up
		f.newPar()
		f.wrText(els[0])
		f.endPar()
		els = els[1:]
		up = nil
	}
	fmt.Fprintf(f.out, "\n")
	f.wrElems(els...)
	f.wrFoots(t)
	f.wrBib(t.bibrefs)
	if cop != "" {
		fmt.Fprintf(f.out, "\n(c)  %s\n", cop);
	}
}

// plain text writer (for man)
func wrtxt(t *Text, wid int, out io.Writer, outfig string) {
	f := &txtFmt{par: &par{wid: wid, out: out, tab: "    "}}
	f.run(t)
}
