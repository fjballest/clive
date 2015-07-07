package wr

import (
	"io"
	"fmt"
	"strings"
	"strconv"
	"os/exec"
	"bytes"
	"clive/app"
//	"clive/app/nsutil"
	"path"
)

type texFmt {
	lvl int
	ps int
	*par
	outfig string
}

const lspecial = `&_$\%{}#`

func escTex(s string) string {
	ns := ""
	noesc := false
	for _, r := range s {
		switch {
		case r == 1:
			noesc = true
			continue
		case r == 2:
			noesc = false
			continue
		case noesc:
		case strings.ContainsRune(lspecial, r):
			ns += `\`
		}
		ns += string(r)
	}
	return ns
}

var pic2eps = `pic | tbl | eqn | groff -ms -m pspic | ps2eps | epstopdf -f -o=`

var figk = map[Kind] string {
	Kfig: "pic",
	Kpic: "pic",
	Keqn: "eqn",
}
var figstart = map[Kind] string {
	Kpic: ".PS",
	Keqn: ".EQ",
}
var figend = map[Kind] string {
	Kpic: ".PE",
	Keqn: ".EN",
}

// pipe the pic data into pic2eps and return the path to the eps file for the pic.
func (e *Elem) pic(outfig string) string {
	outf := fmt.Sprintf("%s.%s%s", path.Base(outfig), figk[e.Kind], e.Nb)
	outf = strings.Replace(outf, ".", "_", -1) + ".pdf"
	cmd := exec.Command("sh", "-c", pic2eps+outf)
	var b bytes.Buffer
	b.WriteString(figstart[e.Kind]+"\n")
	b.WriteString(e.Data)
	b.WriteString(figend[e.Kind]+"\n")
	cmd.Stdin = &b
	err := cmd.Run()
	if err != nil {
		app.Warn("mkpic: ", err)
		return "none.png"
	}
	app.Warn("pic %s", outf)
	return outf
}

func epstopdf(fn string) string {
	if strings.HasSuffix(fn, ".pdf") {
		return fn
	}
	outf := fn
	if strings.HasSuffix(outf, ".eps") {
		outf = outf[:len(outf)-4]
	}
	outf += ".pdf"
	cmd := exec.Command("epstopdf", "-o", outf, fn)
	err := cmd.Run()
	if err != nil {
		app.Warn("epstopdf: ", err)
		return "none.png"
	}
	app.Warn("pic %s", outf)
	return outf
}

func (f *texFmt) wrText(e *Elem) {
	if e == nil {
		return
	}
	switch e.Kind {
	case Khdr1, Khdr2, Khdr3:
	default:
		if e.Nb != "" {
			f.printPar(e.Nb, " ")
		}
	}
	switch e.Kind {
	case Kit, Kbf, Ktt, Kitend, Kbfend, Kttend:
		f.wrFnt(e)
	case Kfont:
		f.fntSz(e.Data)
	case Kurl:
		toks := strings.SplitN(e.Data, "|", 2)
		if len(toks) == 1 {
			f.printParCmd(`\verb|`+e.Data+`|`)
		} else {
			f.printPar(toks[0]+" ")
			f.printParCmd(`\verb|`+toks[1]+`|`)
		}
	case Kbib:
		nbs := strings.Split(e.Data, ",")
		if len(nbs) == 0 {
			nbs = append(nbs, "XXX")
		}
		e.Data = `\cite{bib`+nbs[0]
		for _, nb := range nbs[1:] {
			e.Data += ",bib"+nb
		}
		e.Data += "}"
		f.printParCmd(e.Data)
	case Kcref:
		f.printParCmd(`\ref{lst`+e.Data+`}`)
	case Keref:
		f.printParCmd(`\ref{eqn`+e.Data+`}`)
	case Ktref:
		f.printParCmd(`\ref{tbl`+e.Data+`}`)
	case Kfref:
		f.printParCmd(`\ref{fig`+e.Data+`}`)
	case Ksref:
		nb := strings.Replace(e.Data, ".", "x", -1)
		f.printParCmd(`\ref{sec`+nb+`}`)
	case Kcite:
		e.Data = "[" + e.Data + "]"
		f.printPar(e.Data)
	default:
		f.printPar(e.Data)
		for _, c := range e.Textchild {
			f.wrText(c)
		}
	}
}

var ilfnts = map[Kind]string {
	Kit: `\textit{`,
	Kbf: `\textbf{`,
	Ktt: `\texttt{`,
	Kitend: "}",
	Kbfend: "}",
	Kttend: "}",
}

var lfnts = map[Kind]string {
	Kit: `\textit{%`,
	Kbf: `\textbf{%`,
	Ktt: `\texttt{%`,
	Kitend: "}%",
	Kbfend: "}%",
	Kttend: "}%",
}

var lhdrs = map[Kind]string {
	Khdr1: "section",
	Khdr2: "subsection",
	Khdr3: "subsubsection",
}

var llst = map[Kind]string {
	Kindent: "itemize",
	Kitemize: "itemize",
	Kenumeration: "enumerate",
	Kdescription: "description",
}

func (f *texFmt) wrFnt(e *Elem) {
	if e.Inline {
		f.printParCmd(ilfnts[e.Kind])
	} else {
		f.printCmd("%s\n", lfnts[e.Kind])
	}
}

var lszs = map[int] string {
	-5: "tiny",
	-4: "tiny",
	-3: "scriptsize",
	-2: "footnotesize",
	-1: "small",
	0: "normalsize",
	1: "large",
	2: "Large",
	3: "LARGE",
	4: "huge",
	5: "Huge",
}

func (f *texFmt) fntSz(d string) {
	if len(d) == 0 {
		return
	}
	n, _ := strconv.Atoi(d)
	f.ps += n
	s := lszs[f.ps]
	if s == "" {
		s = lszs[0]
	}
	f.printParCmd(`\`+s + ` `)
}

func (f *texFmt) wrCaption(e *Elem) {
	f.printParCmd(`\caption{`)
	if e.Caption != nil {
		f.wrText(e.Caption)
	}
	f.printParCmd(`\label{` + llbl[e.Kind] + e.Nb + `}`)
	f.printParCmd(`}`)
}

var llbl = map[Kind] string {
	Kfig: "fig",
	Kpic: "fig",
	Kcode: "lst",
	Keqn: "eqn",
	Ktbl: "tbl",
	Khdr1: "sec",
	Khdr2: "sec",
	Khdr3: "sec",
}
func (f *texFmt) wrElems(els ...*Elem) {
	inabs := false
	pref := strings.Repeat(f.tab, f.lvl)
	f.lvl++
	defer func() {
		f.lvl--
	}()
	for _, e := range els {
		f.i0, f.in = pref, pref
		switch e.Kind {
		case Kit, Kbf, Ktt, Kitend, Kbfend, Kttend:
			f.wrFnt(e)
		case Kfont:
			f.fntSz(e.Data)
		case Khdr1, Khdr2, Khdr3:
			if inabs {
				f.printCmd(`\end{abstract}`+"\n\n")
				inabs = false
			}
			if strings.ToLower(e.Data) == "abstract" {
				f.printCmd(`\begin{abstract}`+"\n")
				inabs = true
				break
			}
			f.closePar()
			f.printParCmd("\\", lhdrs[e.Kind], "{")
			f.wrText(e)
			f.printParCmd("}")
			f.closePar()
			f.printCmd(pref + `\label{` + llbl[e.Kind] +
				strings.Replace(e.Nb, ".", "x", -1) + `}` + "\n")
		case Kpar:
			f.printCmd("\n")
			if inabs {
				f.printCmd(`\end{abstract}`+"\n")
				inabs = false
			}
		case Kbr:
			f.printParCmd(`\\`)
			f.closePar()
		case Kindent:
			// If it contains just a fig, pic, or tbl, then
			// skip this level and jump to the child
			if len(e.Child) == 1 || len(e.Child) == 2 && e.Child[1].Kind == Kpar {
				switch e.Child[0].Kind {
				case Kfig, Kpic, Keqn, Ktbl, Kcode:
					f.wrElems(e.Child...)
					continue
				}
			}
			fallthrough
		case Kitemize, Kenumeration, Kdescription:
			f.closePar()
			f.printCmd(pref+`\begin{`+llst[e.Kind]+`}`+"\n")
			if e.Kind == Kindent {
				f.printCmd(pref+`\item[]`+"\n")
			}
			f.wrElems(e.Child...)
			f.printCmd(pref+`\end{`+llst[e.Kind]+`}`+"\n")
		case Kname:
			f.closePar()
			f.printParCmd(`\item[`)
			f.wrText(e)
			f.printParCmd(`]`)
			f.closePar()
			f.wrElems(e.Child...)
		case Kitem, Kenum:
			f.closePar()
			f.printCmd("\n")
			f.printParCmd(`\item `)
			f.wrText(e)
		case Kverb, Ksh:
			f.printCmd(pref+`\begin{verbatim}`+"\n")
			e.Data = indentVerb(e.Data, f.i0, f.tab)
			f.printCmd("%s", e.Data)
			f.printCmd(pref+`\end{verbatim}`+"\n")
		case Ktext, Kurl, Kbib, Kcref, Keref, Ktref, Kfref, Ksref, Kcite:
			f.wrText(e)
		case Kfig, Kpic, Kcode, Keqn:
			f.printCmd(pref + `\begin{figure}`+"\n")
			f.printCmd(pref + `\centering`+"\n")
			switch e.Kind{
			case Kpic:
				fn := e.pic(f.outfig)
				f.printCmd("%s\n", pref + f.tab + `\includegraphics{`+ fn + "}")
			case Kfig:
				fn := strings.TrimSpace(e.Data)
				if strings.HasSuffix(fn, ".eps") {
					fn = epstopdf(fn)
				}
				f.printCmd("%s\n", pref + f.tab + `\includegraphics{`+ fn + "}")
			case Keqn:
				fn := e.pic(f.outfig)
				f.printCmd("%s\n", pref + f.tab + `\includegraphics{`+ fn + "}")
			case Kcode:
				xpref := pref+f.tab
				f.printCmd(xpref+`\begin{verbatim}`+"\n")
				f.printCmd("%s\n", indentVerb(e.Data, xpref+f.tab, f.tab))
				f.printCmd(xpref+`\end{verbatim}`+"\n")
			}
			f.closePar()
			f.wrCaption(e)
			f.printCmd(pref + `\end{figure}`+"\n")
		case Ktbl:
			f.closePar()
			f.printCmd(pref+`\begin{table}`+"\n")
			f.printCmd(pref + `\centering`+"\n")
			f.lvl++
			f.i0, f.in = pref+f.tab, pref+f.tab
			f.wrTbl(e.Tbl)
			f.lvl--
			f.wrCaption(e)
			f.printCmd(pref+`\end{table}`+"\n")
		}
	}
	f.closePar()
}

func (f *texFmt) wrTbl(rows [][]string) {
	if len(rows)<2 || len(rows[0])<2 || len(rows[1])<2 {
		return
	}
	rfmt := rows[0]
	rows = rows[1:]
	tfmt := ""
	rfmt[0] = "|l"
	for _, r := range rfmt {
		tfmt += "|" + r
	}
	tfmt += "|"
	f.printCmd(f.i0+`\begin{tabular}{` + tfmt + `}\hline`+"\n")
	rows[0][0] = ""
	for i, r := range rows {
		f.printCmd(f.i0+f.tab)
		for j, c := range r {
			if j > 0 {
				f.printCmd("\t&")
			}
			f.printCmd("%s", escTex(c))
		}
		if i < len(rows)-1 {
			f.printCmd(`\\ \hline`+"\n")
		} else {
			f.printCmd(`\\`+"\n")
		}
	}
	f.printCmd(f.i0+f.tab+`\hline`+"\n")
	f.printCmd(f.i0+`\end{tabular}`+"\n")
}

func (f *texFmt) wrBib(refs []string) {
	if len(refs) == 0 {
		return
	}
	f.printCmd(`\begin{thebibliography}{50}`+"\n")
	f.i0 = f.tab
	f.in = f.tab
	for i, r := range refs {
		k := fmt.Sprintf("bib%d", i+1)
		f.printCmd(`\bibitem{`+ k + `} `)
		f.printPar(r)
		f.closePar()
	}
	f.printCmd(`\end{thebibliography}`+"\n")
}

func (f *texFmt) run(t *Text) {
	f.printCmd("%s\n", `% Compile with latex, not pdflatex (or pic circles wont show up)`)
	f.printCmd("%s\n", `% If there are problems with figures, remember they should be eps or pic`)
	f.printCmd(`\documentclass[a4paper]{article}`+"\n")
	f.printCmd(`\usepackage{graphicx}`+"\n")
	f.printCmd(`\usepackage[utf8]{inputenc}`+"\n")
	els := t.Elems
	n := 0
	for len(els)>0 && els[0].Kind==Ktitle {
		switch n {
		case 0:
			f.printParCmd("\\title{")
			f.wrText(els[0])
			f.printParCmd("}")
			f.closePar()
		case 1:
			f.printParCmd("\\author{")
			f.wrText(els[0])
		default:
			f.printParCmd(`\\`)
			f.closePar()
			f.wrText(els[0])
		}
		n++
		els = els[1:]
	}
	if n > 0 {
		f.printParCmd("}\n")
	}
	f.printCmd("\n\\begin{document}\n")
	f.printCmd("\n\\maketitle{}\n")
	f.wrElems(els...)
	f.wrBib(t.bibrefs)
	f.printCmd("\n\\end{document}\n")
}

// (la)tex writer
func wrtex(t *Text, wid int, out io.Writer, outfig string) {
	f := &texFmt{
		par: &par{fn: escTex, out: out, wid: wid, tab: "    "},
		outfig: outfig,
	}
	f.run(t)
}
