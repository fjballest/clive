package wr

import (
	"strings"
	"clive/app"
	"clive/app/wr/refs"
	"os/exec"
	"fmt"
	"strconv"
	"unicode"
)


func (s *scan) get() string {
	if s.saved {
		s.saved = false
		s.nb++
		return s.last
	}
	s.nb++
	ln, ok := <-s.lnc
	if len(ln) > 0 && ln[len(ln)-1] == '\n' {
		ln = ln[:len(ln)-1]
	}
	if len(ln) > 0 && ln[0] == '#' {
		return s.get()
	}
	s.last = ln
	s.eof = s.eof || !ok
	return ln
}

func (s *scan) unget() {
	s.saved = true
	s.nb--
}

func (s *scan) skipBlanks() bool {
	some := false
	for !s.eof {
		ln := s.get()
		if len(ln)==0 || ln[0]=='#' {
			continue
		}
		if strings.TrimSpace(ln) != "" {
			s.unget()
			return some
		}
		some = true
	}
	return some
}

func (e *Elem) Warn(fmts string, args ...interface{}) {
	if e != nil && e.fname != "" {
		fmts = "%s:%d: " + fmts
		args = append([]interface{}{e.fname, e.lno}, args...)
	}
	app.Warn(fmts, args...)
}

// return indent lvl, kind of paragraph and data from the first line if any
// Knone is returned for empty or blank lines.
func lookLine(ln string) (int, Kind, string) {
	if strings.TrimSpace(ln) == "" {
		return 0, Kpar, ""
	}
	nt := 0
	for ; nt<len(ln) && ln[nt]=='\t'; nt++ {
	}
	ln = ln[nt:]
	if ln == "" {
		return 0, Knone, ""
	}
	if ln == "-" {
		return nt, Kbr, ""
	}
	if ln == "_" {
		return nt, Kit, ""
	}
	if ln == "*" {
		return nt, Kbf, ""
	}
	if ln == "|" {
		return nt, Ktt, ""
	}
	for m, k := range marks {
		if strings.HasPrefix(ln, m) {
			dat := strings.TrimPrefix(ln, m)
			return nt, k, dat
		}
	}
	if len(ln)>1 && (ln[0]=='+' || ln[0]=='-') &&
		len(strings.TrimLeft(ln[1:], "0123456789"))==0 {
		return nt, Kfont, ln
	}
	return nt, Ktext, ln
}

func (t *Text) contdTitle(el *Elem) *Elem {
	for !t.eof {
		_, k, ln := lookLine(t.get())
		if k != Ktext {
			t.unget()
			break
		}
		el.Data += " " + strings.TrimSpace(ln)
	}
	return el
}

func (x *xCmd) Parse() (chan<- string, <-chan *Text) {
	lnc := make(chan string)
	tc := make(chan *Text, 1)
	go func() {
		t := &Text{
			scan: &scan{lnc: lnc, fname: x.uname},
			pprintf: app.FlagEprintf(&x.debugPars),
			sprintf: app.FlagEprintf(&x.debugSplit),
			iprintf: app.FlagEprintf(&x.debugIndent),
			refsdir: x.refsdir,
		}
		t.parse()
		tc <- t
		close(tc)
	}()
	return lnc, tc
}

func (t *Text) parse() {
	if t == nil {
		return
	}
	for p := t.parsePar(); p != nil; p = t.parsePar() {
		t.splitMarks(p)
		t.pprintf("PAR %s\n", p)
		t.Elems = append(t.Elems, p)
		if p.Kind==Ktitle || p.Kind==Khdr1 || p.Kind==Khdr2 ||
			p.Kind==Khdr3 {
			t.skipBlanks()
		}
	}
	t.fixRefs()
	t.indentPars()
	t.splitLists()
}

func sh(dflt string) string {
	if dflt != "" {
		return dflt
	}
	// NB: Don't use SHELL, to leave unix alone.
	if c := app.GetEnv("shell"); c != "" {
		return c
	}
	return "rc"
}

// executes shell command and replaces data with command output
func (e *Elem) sh() {
	cmd := exec.Command(sh(e.Tag))
	if e := app.OSEnv(); e != nil {
		cmd.Env = e
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		e.Warn("pipe to rc: %s", err)
		return
	}
	in := e.Data
	go func() {
		fmt.Fprint(stdin, in)
		stdin.Close()
	}()
	out, err := cmd.CombinedOutput()
	if err!=nil && len(out)==0 {
		e.Warn("command: %s", err)
		return
	}
	outs := string(out)
	e.Data = outs
}

func (t *Text) parsePar() (el *Elem) {
	if t.eof {
		t.pprintf("EOF\n")
		return nil
	}
	nt, k, ln := lookLine(t.get())
	fname := t.fname
	lno := t.nb
	defer func() {
		if el != nil {
			el.fname = fname
			el.lno = lno
		}
	}()
	// t.pprintf("\t%v %v %s\n", nt, k, dbg.Text(ln))
	switch k {
	case Kpar:
		t.skipBlanks()
		return &Elem{Kind: k}
	case Kbr:
		return &Elem{Kind: k}
	case Ktitle:
		el := &Elem{Kind: k, Data: strings.TrimSpace(ln)}
		return t.contdTitle(el)
	case Khdr1, Khdr2, Khdr3:
		el := &Elem{Kind: k, Data: strings.TrimSpace(ln)}
		if strings.ToLower(ln) != "abstract" {
			t.addRef(el, k)
		}
		return el
	case Knone:
		return nil
	case Kit:
		if t.itset {
			k = Kitend
		}
		t.itset = !t.itset
		return &Elem{Kind: k, indent: nt}
	case Kbf:
		if t.bfset {
			k = Kbfend
		}
		t.bfset = !t.bfset
		return &Elem{Kind: k, indent: nt}
	case Ktt:
		if t.ttset {
			k = Kttend
		}
		t.ttset = !t.ttset
		return &Elem{Kind: k, indent: nt}
	case Kverb, Ksh, Kfig, Ktbl, Keqn, Kpic, Kcode:
		// could consume ln here to select labels, captions from data.
		el := &Elem{Kind: k, Tag: strings.TrimSpace(ln), indent: nt}
		el = t.contdRaw(el)
		switch k {
		case Ktbl:
			el.parseTbl()
			fallthrough
		case Kfig, Keqn, Kcode, Kpic:
			rk := k
			if k == Kfig || k == Kpic {
				rk = Kfig
			}
			t.addRef(el, rk)
		case Ksh:
			if strings.HasPrefix(ln, QlMark) {
				el.Tag = "ql"
			} else if strings.HasPrefix(ln, RcMark) {
				el.Tag = "rc"
			}
			el.sh()
		}
		
		return el
	}
	el = &Elem{Kind: k, Data: ln, indent: nt}
	if k == Kfont {
		el.Data = strings.TrimSpace(el.Data)
		return el
	}
	// must consume Ktext lines while the indent level remains the same
	for !t.eof {
		nt, k, ln = lookLine(t.get())
		if nt!=el.indent || k!=Ktext {
			t.unget()
			break
		}
		el.Data += " " + ln
	}
	return el
}

// called for verb, fig, tbl, eqn, code to consume all lines until the end
// of the corresponding element and strip caption lines
func (t *Text) contdRaw(el *Elem) *Elem {
	end := strings.Repeat("\t", el.indent) + "]"
	first := true
	nt := 0
	incap := false
	cap := ""
	for !t.eof {
		ln := t.get()
		if ln == "" {
			continue
		}
		if ln == end {
			break
		}
		lnt := ntabs(ln)
		if first {
			nt = lnt
			first = false
		}
		if el.Kind!=Kverb && el.Kind!=Ksh && strings.TrimSpace(ln) != "" && lnt <= el.indent {
			incap = true
		}
		ln = rmtabs(ln, nt)
		if incap {
			cap += ln + "\n"
		} else {
			el.Data += ln + "\n"
		}
	}
	if cap != "" {
		el.Caption = &Elem{Kind: Ktext, indent: el.indent, Data: cap}
	}
	return el
}

func keys(s string) []string {
	words := strings.Fields(s)
	for i, w := range words {
		w = strings.ToLower(w)
		words[i] = strings.TrimFunc(w, unicode.IsPunct)
	}
	return words
}

func (ek *eKeys) setKeys()  {
	e := ek.el
	ks := keys(e.Tag)
	if e.Caption != nil {
		ks = append(ks, keys(e.Caption.Data)...)
	}
	if e.Kind == Khdr1 || e.Kind == Khdr2 || e.Kind == Khdr3 {
		ks = append(ks, keys(e.Data)...)
	}
	for _, w := range ks {
		ek.keys[w] = true
	}
	app.Dprintf("label %s %s -> %v\n", ek.el.Kind, ek.el.Nb, ks)
}

func (t *Text) addRef(el *Elem, k Kind) {
	refs := t.refs
	if refs == nil {
		refs = map[Kind][]*eKeys{}
	}
	ek := &eKeys{el: el, keys: map[string]bool{}}
	refs[k] = append(refs[k], ek)
	t.refs = refs
	prev := "??"
	nb := 0
	switch k {
	case Khdr1:
		t.nhdr1++
		t.nhdr2 = 0
		t.nhdr3 = 0
		nb = t.nhdr1
		prev=""
	case Khdr2:
		t.nhdr2++
		t.nhdr3 = 0
		prev = fmt.Sprintf("%d.", t.nhdr1)
		if h1 := refs[Khdr1]; len(h1) > 0 {
			prev = h1[len(h1)-1].el.Nb + "."
		}
		nb = t.nhdr2
	case Khdr3:
		t.nhdr3++
		prev = fmt.Sprintf("%d.%d.", t.nhdr1, t.nhdr2)
		nb = t.nhdr3
	default:
		prev = ""
		nb = len(refs[k])
	}
	el.Nb = fmt.Sprintf("%s%d", prev, nb)
	ek.setKeys()
}

func ntabs(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != '\t' {
			return i
		}
	}
	return len(s)
}

func rmtabs(s string, n int) string {
	for i := 0; i < len(s); i++ {
		if s[i] != '\t' {
			return s[i:]
		}
		if n--; n < 0 {
			return s[i:]
		}
	}
	return ""
}

// parses raw tbl data and fills e.Tbl
func (e *Elem) parseTbl() {
	lines := strings.SplitN(e.Data, "\n", -1)
	if len(lines)>0 && lines[len(lines)-1]=="" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) < 2 {
		e.Warn("table with not enough data")
		return
	}
	e.Tbl = [][]string{}
	n := 0
	for _, ln := range lines {
		toks := strings.SplitN(ln, "\t", -1)
		for i := 0; i < len(toks); i++ {
			toks[i] = strings.TrimSpace(toks[i])
		}
		e.Tbl = append(e.Tbl, toks)
		if n == 0 {
			n = len(toks)
		} else if len(toks) != n {
			e.Warn("wrong number of columns in table")
			e.Tbl = nil
			return
		}
	}
}

func appText(els []*Elem, k Kind, indent int, s string) []*Elem {
	el := &Elem{Kind: k, indent: indent, Data: s}
	if len(els) > 0 {
		last := els[len(els)-1]
		if last.Kind==k && last.indent==indent {
			last.Data += s
			return els
		}
	}
	return append(els, el)
}

func splitCite(els []*Elem, k Kind, i int, key, tag, s string) ([]*Elem, string, bool) {
	if !strings.HasPrefix(tag, "[" + key + ":") {
		return els, s, false
	}
	s = s[len(tag):]
	tag = tag[:len(tag)-1]
	tag = tag[1+len(key)+1:]
	tag = strings.TrimSpace(tag)
	ne := &Elem{Kind: k, indent: i, Data: tag}
	els = append(els, ne)
	return els, s, true
}

var cites = map[string]Kind{
	"sect": Ksref,
	"fig": Kfref,
	"code": Kcref,
	"tbl": Ktref,
	"eqn": Keref,
	"url": Kurl,
	"bib": Kbib,
	"cite": Kcite,
}

// Split the text in the elem and add children with
// inlined marks and raw text elems.
func (t *Text) splitMarks(p *Elem) {
	switch p.Kind {
	case Ktext, Kenum, Kitem, Khdr1, Ktitle, Khdr2, Khdr3:
		if !strings.ContainsAny(p.Data, "*_|[") {
			return
		}
	default:
		if p.Caption != nil {
			t.splitMarks(p.Caption)
		}
		return
	}
	s := p.Data
	k := p.Kind
	indent := p.indent
	var els []*Elem
Loop:	for len(s) > 0 {
		i := strings.IndexAny(s, "*_|[")
		if t.ttset {
			i = strings.Index(s, "|")
		}
		if i < 0 {
			els = appText(els, k, indent, s)
			break
		}
		if i<len(s)-1 && strings.ContainsRune("*_|", rune(s[i])) && s[i]==s[i+1] {
			// scaped mark
			els = appText(els, k, indent, s[:i+1])
			s = s[i+2:]
			continue
		}
		if i > 0 {
			els = appText(els, k, indent, s[:i])
		}
		if s[i] == '_' {
			tk := Kit
			if t.itset {
				tk = Kitend
			}
			t.itset = !t.itset
			els = append(els, &Elem{Kind: tk, indent: indent, Inline: true})
			s = s[i+1:]
			continue
		}
		if s[i] == '*' {
			tk := Kbf
			if t.bfset {
				tk = Kbfend
			}
			t.bfset = !t.bfset
			els = append(els, &Elem{Kind: tk, indent: indent, Inline: true})
			s = s[i+1:]
			continue
		}
		if s[i] == '|' {
			tk := Ktt
			if t.ttset {
				tk = Kttend
			}
			t.ttset = !t.ttset
			els = append(els, &Elem{Kind: tk, indent: indent, Inline: true})
			s = s[i+1:]
			continue
		}
		s = s[i:]
		tag := s
		ei := strings.IndexRune(tag, ']')
		if ei > 0 {
			tag = tag[:ei+1]
		}
		for k, v := range cites {
			var ok bool
			if els, s, ok = splitCite(els, v, indent, k, tag, s); ok {
				if v == Kbib {
					t.refer(els[len(els)-1])
				}
				continue Loop
			}
		}
		els = appText(els, k, indent, "[")
		s = s[1:]
	}
	if len(els) == 0 {
		return
	}
	if els[0].Kind == p.Kind {
		p.Data = els[0].Data
		els = els[1:]
	} else {
		p.Data = ""
	}
	if p.Child != nil {
		p.Warn("split inline bug? old had children")
	}
	p.Textchild = els
}

func (t *Text) refer(el *Elem) {
	if t.bib == nil && t.biberr == nil {
		c := app.AppCtx()
		old := c.Debug
		c.Debug = false
		t.bib, t.biberr = refs.Load(t.refsdir)
		c.Debug = old
		if t.biberr != nil {
			el.Warn("bib: %s: %s\n", refs.Dir, t.biberr)
		}
	}
	nbs := []string{}
	for _, b := range strings.Split(el.Data, ",") {
		b = strings.TrimSpace(b)
		if len(b) == 0 {
			continue
		}
		brefs := t.bib.Cites(strings.Fields(b)...)
		bs := []string{b}
		if len(brefs) == 0 {
			el.Warn("bib '%s' not found", b)
		} else {
			bref := brefs[0]
			bs = bref.Reference()
			if len(brefs) > 1 {
				el.Warn("%d refs for '%s'; using '%s'", len(brefs), b, bs[0])
			}
		}
		nb := t.addRefer(bs)
		nbs = append(nbs, strconv.Itoa(nb))
	}
	el.Data = strings.Join(nbs, ",")
}

func (t *Text) addRefer(ref []string) int {
	rs := strings.Join(ref, "\n")
	for i, r := range t.bibrefs {
		if r == rs {
			return i+1
		}
	}
	t.bibrefs = append(t.bibrefs, rs)
	return len(t.bibrefs)
}

// return from els those pars at the start with the given indent level.
// left is what's left from els.
func sameIndent(els []*Elem, indent int) (res, left []*Elem) {
	res = []*Elem{}
	left = els
	for len(left)>0 && (left[0].indent==indent || left[0].Kind==Kpar || left[0].Kind==Kbr) {
		res = append(res, left[0])
		left = left[1:]
	}
	return res, left
}

// take a flat list of els with indentation level and group them in a tree
// according to their indentation.
func indentedPars(top *Elem, els []*Elem) []*Elem {
	for len(els)>0 && els[0].indent>=top.indent {
		pars, nels := sameIndent(els, top.indent)
		top.Child = append(top.Child, pars...)
		els = nels
		if len(els)>0 && els[0].indent>top.indent {
			ri := &Elem{Kind: Kindent, indent: els[0].indent}
			top.Child = append(top.Child, ri)
			els = indentedPars(ri, els)
		}
	}
	return els
}

func (t *Text) indentPars() {
	top := &Elem{}
	t.Elems = indentedPars(top, t.Elems)
	if len(t.Elems) > 0 {
		app.Fatal("paragraphs left at lvl %d", t.Elems[0].indent)
	}
	t.Elems = top.Child
	t.iprintf("\nindented pars:\n")
	for _, e := range t.Elems {
		t.iprintf("%s\n", e)
	}
	t.iprintf("\n")
}

func (top *Elem) checkDescList() {
	if top.Kind!=Kitemize || len(top.Child)<2 {
		return
	}
	nchild := []*Elem{}
	var last *Elem
	initem := false
	fontk := Knone
	for i := 0; i < len(top.Child); i++ {
		c := top.Child[i]
		if c.Kind==Kit || c.Kind==Kbf || c.Kind==Ktt {
			fontk = c.Kind
			continue
		}
		if c.Kind==Kitend || c.Kind==Kbfend || c.Kind==Kttend {
			continue
		}
		if initem {
			if c.Kind!=Kindent || last==nil {
				return
			}
			last.Child = c.Child
			initem = false
			fontk = Knone
		} else {
			if c.Kind != Kitem {
				return
			}
			nchild = append(nchild, c)
			last = c
			last.NameKind = fontk
			initem = true
		}
	}
	if initem {
		return
	}
	for _, c := range nchild {
		c.Kind = Kname
	}
	top.Kind = Kdescription
	top.Child = nchild
}

// detect kind of list.
// description lists are specific itemizes detected later.
func listKind(els []*Elem) Kind {
	for _, e := range els {
		if e.Kind == Kitem {
			return Kitemize
		}
		if e.Kind == Kenum {
			return Kenumeration
		}
		if e.Kind == Kname {
			return Kdescription
		}
		if e.Kind!=Kfont && e.Kind!=Kit && e.Kind!=Kbf && e.Kind!=Ktt {
			break
		}
	}
	return Kindent
}

// look at Kindent elements and split them into one or more lists
// depending on the kind of items they contain.
// fixes also the type of list used.
// returns the set of elems to use instead of top
func (top *Elem) splitList() []*Elem {
	nc := []*Elem{}
	for _, c := range top.Child {
		nc = append(nc, c.splitList()...)
	}
	top.Child = nc
	if top.Kind!=Kindent || len(top.Child)==0 {
		return []*Elem{top}
	}
	els := top.Child
	top.Kind = listKind(els)
	top.Child = []*Elem{}
	res := []*Elem{}
	for _, c := range els {
		nk := top.Kind
		switch top.Kind {
		case Kindent:
			if c.Kind == Kenum {
				nk = Kenumeration
			}
			if c.Kind == Kitem {
				nk = Kitemize
			}
		case Kitemize:
			if c.Kind == Kenum {
				nk = Kenumeration
			}
		case Kenumeration:
			if c.Kind == Kitem {
				nk = Kitemize
			}
		}
		if top.Kind == nk {
			top.Child = append(top.Child, c)
			continue
		}
		ntop := &Elem{}
		*ntop = *top
		ntop.Kind = nk
		ntop.Child = []*Elem{c}
		top.checkDescList()
		res = append(res, top)
		top = ntop
	}
	top.checkDescList()
	res = append(res, top)
	return res
}

func (t *Text) splitLists() {
	top := &Elem{Child: t.Elems}
	top.splitList()
	t.Elems = top.Child
	t.sprintf("\nafter split lists:\n")
	for _, e := range top.Child {
		t.sprintf("%s\n", e)
	}
}

func (t *Text) fixRefs() {
	if t.refs == nil {
		return
	}
	t.refs[Ktitle] = append(t.refs[Ktitle], t.refs[Khdr1]...)
	t.refs[Ktitle] = append(t.refs[Ktitle], t.refs[Khdr2]...)
	t.refs[Ktitle] = append(t.refs[Ktitle], t.refs[Khdr3]...)
	for _, e := range t.Elems {
		e.fixRefs(t.refs)
	}
}

func (e *Elem) fixRefs(refs map[Kind][]*eKeys) {
	for _, ce := range e.Child {
		ce.fixRefs(refs)
	}
	for _, ce := range e.Textchild {
		ce.fixRefs(refs)
	}
	if e.Caption != nil {
		e.Caption.fixRefs(refs)
	}
	switch e.Kind {
	case Ksref:
		e.setRef(refs[Ktitle])
	case Kfref:
		e.setRef(refs[Kfig])
	case Ktref:
		e.setRef(refs[Ktbl])
	case Keref:
		e.setRef(refs[Keqn])
	case Kcref:
		e.setRef(refs[Kcode])
	}
	
}

func (ek *eKeys) matches(ks []string) bool {
	for _, k := range ks {
		if ek == nil || ek.keys == nil || !ek.keys[k] {
			return false
		}
	}
	return true
}

func (e *Elem) setRef(refs []*eKeys) {
	ks := keys(e.Data)
	var match *eKeys
	for _, r := range refs {
		if r.matches(ks) {
			if match != nil {
				e.Warn("multiple matches for ref %v; using %s", ks, e.Data)
				return
			}
			match = r
			app.Dprintf("ref %s -> %s\n", e.Data, r.el.Nb)
			e.Data = r.el.Nb
		}
	}
	if match == nil {
		e.Warn("no match for ref '%s'", e.Data)
	}
}
