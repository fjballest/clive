package wr

import (
	"clive/app"
	"clive/app/nsutil"
	"clive/sre"
	"fmt"
	"html"
	"io"
	"strconv"
	"strings"
)

const (
	CSS      = `http://lsub.org/ls/class.css` // CSS used for html output
	MAN      = `http://lsub.org/sys/man`      // base url for man pages in output
	TEMPLATE = `/zx/usr/web/sys/man/TEMPLATE` // template for clive man pages
)

var cliveMan bool
var sect string

type htmlFmt struct {
	lvl  int
	ps   int
	fnts []int
	*par
	outfig string

	ups        bool // hacks for clive man
	hasSeeAlso bool // hacks for clive man
}

func escHtml(s string) string {
	ns := ""
	noesc := false
	w := ""
	for _, r := range s {
		switch {
		case r == 1:
			noesc = true
			if w != "" {
				ns += html.EscapeString(w)
				w = ""
			}
			continue
		case r == 2:
			noesc = false
			continue
		case noesc:
			ns += string(r)
		default:
			w += string(r)
		}
	}
	if w != "" {
		ns += html.EscapeString(w)
	}
	return ns
}

func (f *htmlFmt) citeMan(cite string) bool {
	rg, _ := sre.Match(mrexp, cite)
	if len(rg) < 3 {
		return false
	}
	pg := rg[1]
	sec := rg[2]
	f.printParCmd(`<a href="` + MAN + `/` + sec + `/` + pg + `.html">` + cite + `</a>`)
	return true
}

func (f *htmlFmt) wrText(e *Elem) {
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
		x := e.Data
		if f.ups {
			x = strings.ToUpper(x)
		}
		if len(toks) == 1 {
			f.printParCmd(`<a href="`, e.Data, `">`, x, "</a>")
		} else {
			f.printParCmd(`<a href="`, toks[1], `">`,
				html.EscapeString(toks[0]), "</a>")
		}
		return
	case Kcite:
		if f.citeMan(e.Data) {
			return
		}
		e.Data = "[" + e.Data + "]"
	case Kbib:
		nbs := strings.Split(e.Data, ",")
		if len(nbs) == 0 {
			nbs = append(nbs, "XXX")
		}
		f.printParCmd(`[<a href="#bib`+nbs[0]+`">`, nbs[0], `</a>`)
		for _, nb := range nbs[1:] {
			f.printParCmd(`,<a href="#bib`+nb+`">`, nb, `</a>`)
		}
		f.printParCmd("]")
		return
	case Kcref:
		f.printParCmd(`<a href="#lst`+e.Data+`">`, e.Data, `</a>`)
		return
	case Keref:
		f.printParCmd(`<a href="#eqn`+e.Data+`">`, e.Data, `</a>`)
		return
	case Ktref:
		f.printParCmd(`<a href="#tbl`+e.Data+`">`, e.Data, `</a>`)
		return
	case Kfref:
		f.printParCmd(`<a href="#fig`+e.Data+`">`, e.Data, `</a>`)
		return
	case Ksref:
		nb := strings.Replace(e.Data, ".", "x", -1)
		f.printParCmd(`<a href="#sec`+nb+`">`, e.Data, `</a>`)
		return
	}
	x := e.Data
	if f.ups {
		x = strings.ToUpper(x)
	}
	f.printPar(x)
	for _, c := range e.Textchild {
		f.wrText(c)
	}
}

var hfnts = map[Kind]string{
	Kit:    `em`,
	Kbf:    `b`,
	Ktt:    `code`,
	Kitend: "/em",
	Kbfend: "/b",
	Kttend: "/code",
}

var hhdrs = map[Kind]string{
	Khdr1: "h2",
	Khdr2: "h3",
	Khdr3: "h3",
}

var hlst = map[Kind]string{
	Kitemize:     "ul",
	Kenumeration: "ol",
	Kdescription: "dl",
}

func (f *htmlFmt) wrFnt(e *Elem) {
	if e.Inline {
		f.printParCmd("<", hfnts[e.Kind], ">")
	} else {
		f.printCmd("<%s>\n", hfnts[e.Kind])
	}
}

var hszs = map[int]string{
	-5: "tiny",
	-4: "tiny",
	-3: "scriptsize",
	-2: "footnotesize",
	-1: "small",
	0:  "normalsize",
	1:  "large",
	2:  "Large",
	3:  "LARGE",
	4:  "huge",
	5:  "Huge",
}

func (f *htmlFmt) fntSz(d string) {
	if len(d) == 0 {
		return
	}
	n, _ := strconv.Atoi(d)
	f.fnts = append(f.fnts, n)
	nf := len(f.fnts)
	if nf > 1 && f.fnts[nf-1] == -f.fnts[nf-2] {
		// close a font size
		if n < 0 {
			f.printParCmd(`</big>`)
		} else {
			f.printParCmd(`</small>`)
		}
		f.fnts = f.fnts[:nf-2]
		return
	}
	// open a font size
	if n > 0 {
		f.printParCmd(`<big>`)
	} else {
		f.printParCmd(`<small>`)
	}
	f.fnts = append(f.fnts, n)
}

var hcaps = map[Kind]string{
	Kfig:  "Figure",
	Kpic:  "Figure",
	Ktbl:  "Table",
	Keqn:  "Eqn.",
	Kcode: "Listing",
}

func (f *htmlFmt) wrCaption(e *Elem) {
	if e.Caption == nil {
		f.printCmd("<b>%s %s.</b>", hcaps[e.Kind], e.Nb)
	} else {
		f.printCmd("<b>%s %s:</b> <em>", hcaps[e.Kind], e.Nb)
		f.wrText(e.Caption)
		f.printParCmd(`</em>`)
	}
}

func (f *htmlFmt) wrElems(els ...*Elem) {
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
			f.closePar()
			f.printParCmd(`<a name="` + llbl[e.Kind] +
				strings.Replace(e.Nb, ".", "x", -1) + `"></a>`)
			f.printParCmd("<" + hhdrs[e.Kind] + ">")
			if e.Nb != "" && !cliveMan {
				f.printPar(e.Nb, ".")
				f.printPar(" ")
			}
			f.ups = cliveMan
			f.hasSeeAlso = false
			if f.ups && strings.ToLower(e.Data) == "see also" {
				f.hasSeeAlso = true
			}
			f.wrText(e)
			f.ups = false
			f.printParCmd("</" + hhdrs[e.Kind] + ">")
			f.closePar()
		case Kpar:
			f.printCmd("<p>\n")
		case Kbr:
			f.printParCmd(`<br>`)
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
			f.printCmd(pref + `<p><ul style="list-style:none;">` + "\n")
			f.wrElems(e.Child...)
			f.printCmd(pref + `</ul><p>` + "\n")
		case Kitemize, Kenumeration, Kdescription:
			f.printCmd(pref+"<%s>\n", hlst[e.Kind])
			f.wrElems(e.Child...)
			f.printCmd(pref+"</%s>\n", hlst[e.Kind])
		case Kname:
			f.closePar()
			f.printParCmd(`<dt>`)
			f.wrText(e)
			f.printParCmd("</dt><dd>\n")
			f.wrElems(e.Child...)
			f.printCmd(pref + "</dd>\n")
		case Kitem, Kenum:
			f.closePar()
			f.printParCmd(`<li>`)
			f.wrText(e)
			f.closePar()
		case Kverb, Ksh:
			f.printCmd(pref + `<code><pre>` + "\n")
			e.Data = indentVerb(e.Data, f.i0, f.tab)
			f.printCmd("%s", html.EscapeString(e.Data))
			f.printCmd(pref + `</pre></code>` + "\n")
		case Ktext, Kurl, Kbib, Kcref, Keref, Ktref, Kfref, Ksref, Kcite:
			f.wrText(e)
		case Kfig:
			f.printCmd(pref + "<p>\n")
			f.printCmd(pref + "<hr>\n<center>\n")
			s := strings.TrimSpace(e.Data)
			if strings.HasSuffix(s, ".eps") {
				s = epstopdf(s)
			}
			f.printCmd(pref + `<a name="` + llbl[e.Kind] + e.Nb + `"></a>` + "\n")
			f.printCmd(pref+"<img src=%s></img>\n", s)
			f.printCmd(pref + "</center>\n")
			f.wrCaption(e)
			f.printCmd(pref + "<hr><p>\n")
		case Kcode:
			f.printCmd(pref + "<p>\n")
			f.printCmd(pref + "<hr>\n\n")
			f.printCmd(pref + `<a name="` + llbl[e.Kind] + e.Nb + `"></a>` + "\n")
			f.printCmd(pref + `<code><pre>` + "\n")
			e.Data = indentVerb(e.Data, f.i0, f.tab)
			f.printCmd("%s", html.EscapeString(e.Data))
			f.printCmd(pref + `</pre></code>` + "\n")
			f.wrCaption(e)
			f.printCmd(pref + "<hr><p>\n")
		case Kpic, Keqn:
			f.printCmd(pref + "<p>\n")
			f.printCmd(pref + "<hr>\n<center>\n")
			f.printCmd(pref + `<a name="` + llbl[e.Kind] + e.Nb + `"></a>` + "\n")
			pfn := e.pic(f.outfig)
			f.printCmd(pref + `<img src="` + pfn + `"></img>`)
			f.printCmd(pref + "</center>\n")
			f.wrCaption(e)
			f.printCmd(pref + "<hr><p>\n")
		case Ktbl:
			f.printCmd(pref + "<p>\n")
			f.printCmd(pref + "<hr>\n<center>\n")
			f.printCmd(pref + `<a name="` + llbl[e.Kind] + e.Nb + `"></a>` + "\n")
			f.lvl++
			f.i0, f.in = pref+f.tab, pref+f.tab
			f.wrTbl(e.Tbl)
			f.lvl--
			f.printCmd(pref + "</center>\n")
			f.wrCaption(e)
			f.printCmd(pref + "<hr><p>\n")
		}
	}
	f.closePar()
}

func (f *htmlFmt) wrTbl(rows [][]string) {
	if len(rows) < 2 || len(rows[0]) < 2 || len(rows[1]) < 2 {
		return
	}
	f.printCmd("<table border=\"1\">\n")
	rows = rows[1:]
	rows[0][0] = ""
	for i, r := range rows {
		f.printCmd("<tr>\n")
		for j, c := range r {
			s := html.EscapeString(c)
			if i == 0 || j == 0 {
				f.printCmd("<td><b>%s</b></td>\n", s)
			} else {
				f.printCmd("<td>%s</td>\n", s)
			}
		}
		f.printCmd("</tr>\n")
	}
	f.printCmd("</table>\n")
}

func (f *htmlFmt) wrBib(refs []string) {
	if len(refs) == 0 {
		return
	}
	f.printCmd("<p>\n")
	if !cliveMan {
		f.printCmd("<p><h3>References</h3>\n<hr>\n")
	} else if !f.hasSeeAlso {
		f.printCmd("<p><h2>SEE ALSO</h2>\n<hr>\n")
	} else {
		f.printCmd("<p><h3>External references</h3>\n\n")
	}
	f.printCmd("<p><ol>\n")
	f.i0 = f.tab
	f.in = f.tab
	for i, r := range refs {
		k := fmt.Sprintf("bib%d", i+1)
		f.printParCmd(`<li> <a name="` + k + `"></a>`)
		f.printPar(fmt.Sprintf("%s", r))
		f.printParCmd("</li><p> ")
		f.closePar()
	}
	f.printCmd("<p></ol>\n")
	f.printCmd("<hr><p>\n")
}

func (f *htmlFmt) run(t *Text) {
	els := t.Elems
	if cliveMan {
		if sect != "doc" {
			f.printCmd(`<b><a href="` + MAN + `">User's manual</a>.</b>` + "\n")
			f.printCmd(`<b><a href="` + MAN + `/` + sect + `">Section ` + sect + `</a>.</b>` + "\n")
		}
	} else {
		f.printCmd(`<html>
<meta http-equiv="Content-Type" content="text/html; charset=UTF-8">
<head>
<link rel="stylesheet" type="text/css" href="` + CSS + `" />
`)
		if len(els) > 0 && els[0].Kind == Ktitle {
			s := html.EscapeString(els[0].Data)
			f.printCmd("<title>%s</title>\n</head>\n", s)
		} else {
			f.printCmd("\n</head>\n")
		}
		f.printCmd("<body>\n")
		f.printCmd("<div id=\"container\" class=\"Container\">\n")
		f.printCmd("<div id=\"content\" class=\"Content\">\n")
	}
	n := 0
	for len(els) > 0 && els[0].Kind == Ktitle {
		switch n {
		case 0:
			f.printParCmd("<h2>")
			f.wrText(els[0])
			f.printParCmd("</h2>")
			f.closePar()
		default:
			f.printParCmd("<b>")
			f.wrText(els[0])
			f.printParCmd("</b><br>")
			f.closePar()
		}
		n++
		els = els[1:]
	}
	f.printCmd("<hr>\n<p>\n\n")
	f.wrElems(els...)
	f.wrBib(t.bibrefs)
	f.printCmd("<p>\n<hr><p>\n\n")
	if !cliveMan {
		f.printCmd("</div></div>\n")
		f.printCmd("</body>\n</html>\n")
	} else if sect != "doc" {
		f.printCmd(`<b><a href="` + MAN + `">User's manual</a>.</b>` + "\n")
		f.printCmd(`<b><a href="` + MAN + `/` + sect + `">Section ` + sect + `</a>.</b>` + "\n")
	}
}

// html writer
func wrhtml(t *Text, wid int, out io.Writer, outfig string) {
	f := &htmlFmt{
		par:    &par{fn: escHtml, out: out, wid: wid, tab: "    "},
		outfig: outfig,
	}
	var tmpl []string
	if cliveMan {
		dat, err := nsutil.GetAll(TEMPLATE)
		if err != nil {
			app.Warn("%s", err)
		} else {
			tmpl = strings.Split(string(dat), "\n")
		}
		for len(tmpl) > 0 {
			ln := tmpl[0]
			tmpl = tmpl[1:]
			fmt.Fprintf(out, "%s\n", ln)
			if strings.Contains(ln, `div id="content" class="Content"`) {
				break
			}
		}
	}
	f.run(t)
	for _, ln := range tmpl {
		fmt.Fprintf(out, "%s\n", ln)
	}
}
