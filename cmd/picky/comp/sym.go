package comp

import (
	"clive/cmd/picky/paminstr"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"unicode/utf8"
)

// symbol types and subtypes
const (
	Snone   = iota
	Skey    // keyword
	Sstr    // a string buffer
	Sconst  // constant or literal
	Stype   // type def
	Svar    // obj def
	Sunary  // unary expression
	Sbinary // binary expression
	Sproc   // procedure
	Sfunc   // function
	Sfcall  // procedure or function call
)
const (
	// List kinds
	Lstmt = iota
	Lsym
)
const (
	// Operations besides any of < > =  + - * / % [ . and ^
	Onone = iota + 255
	Ole
	Oge
	Odotdot
	Oand
	Oor // 5 + 255
	Oeq
	One
	Opow
	Oint
	Onil // 10 + 255
	Ochar
	Oreal
	Ostr
	Otrue
	Ofalse // 15 + 255
	Onot
	Olit
	Ocast
	Oparm
	Orefparm // 20 + 255
	Olvar
	Ouminus
	Oaggr

	None = rune(-2)
)

var (
	env                                 *Env
	strs                                = make(map[string]*Sym, Nbighash) // strings and names
	keys                                = make(map[string]*Sym, Nhash)    // keywords and top-level
	nnames                              int
	badnode, pstdin, pstdout, pstdgraph *Sym
	Nerrors                             int
	tgen                                uint
	stats                               Stats
	stname                              = map[int]string{
		Snone:   "unknown",
		Skey:    "keyword",
		Sstr:    "name",
		Sconst:  "constant",
		Stype:   "type",
		Svar:    "variable",
		Sproc:   "procedure",
		Sfunc:   "function",
		Sunary:  "unary",
		Sbinary: "binary",
	}
	opnames = map[rune]string{
		'+':     "+",
		'-':     "-",
		'*':     "*",
		'/':     "/",
		'%':     "%",
		'<':     "<",
		'>':     ">",
		'=':     "=",
		',':     ",",
		'[':     "[]",
		'.':     ".",
		'^':     "^",
		Ole:     "<=",
		Oge:     ">=",
		Odotdot: "..",
		Oand:    "and",
		Oor:     "or",
		Oeq:     "==",
		One:     "!=",
		Opow:    "**",
		Oint:    "ival",
		Onil:    "nil",
		Ochar:   "cval",
		Oreal:   "rval",
		Ostr:    "sval",
		Otrue:   "true",
		Ofalse:  "false",
		Onot:    "not",
		Olit:    "lit",
		Ocast:   "cast",
		Ouminus: "-",
		Oaggr:   "aggr",
	}
)

const (
	SymTabSz = 20
)

func opname(op rune) string {
	name, ok := opnames[op]
	if !ok {
		s := fmt.Sprintf("opname called for op %#x", op)
		panic(s)
	}
	return name
}

func oksym(s *Sym) *Sym {
	if s == nil {
		return badnode
	}
	return s
}

func oktype(t *Type) *Type {
	if t == nil {
		return tundef
	}
	return t
}

func addstmt(s *List, n *Stmt) {
	s.item = append(s.item, n)
	if len(s.item) > int(stats.mlist) {
		stats.mlist = uint(len(s.item))
	}
}

func addsym(s *List, n *Sym) {
	s.item = append(s.item, n)
	if len(s.item) > int(stats.mlist) {
		stats.mlist = uint(len(s.item))
	}
}

func delitem(s *List, n interface{}) {
	for i, v := range s.item {
		if v == n {
			copy(s.item[i:], s.item[i+1:])
			s.item = s.item[:len(s.item)]
			return
		}
	}
	panic("delitem: no item")
}

func appsyms(l1 *List, l2 *List) {
	if l1 == nil || l2 == nil {
		return
	}
	for _, v := range l2.item {
		addsym(l1, v.(*Sym))
	}
}

func addsym0(l *List, n *Sym) {
	addsym(l, nil)
	copy(l.item[1:], l.item[0:])
	l.item[0] = n
}

func alloclist() *List {
	stats.nlists++
	return new(List)
}

func newlist(lkind int) *List {
	l := alloclist()
	l.kind = lkind
	return l
}

var (
	egen     uint
	freeenvs *Env
	zeroenv  Env
)

func Pushenv() (e *Env) {
	stats.nenvs++
	if freeenvs == nil {
		stats.menvs++
		e = new(Env)
	} else {
		e = freeenvs
		freeenvs = e.prev
		*e = zeroenv
	}
	e.tab = make(map[string]*Sym, SymTabSz)
	egen++
	e.id = egen
	e.prev = env
	if env != nil {
		e.prog = env.prog
	}
	env = e
	return e
}

func popenv() {
	var e *Env
	if env == nil {
		panic("bad env")
	}
	e = env
	env = e.prev
	e.prev = nil
	if env == nil {
		panic("pop of top level")
	}
	e.prev = freeenvs
	freeenvs = e
}

func allocsym() *Sym {
	stats.nsyms++
	return new(Sym)
}

var (
	sgen uint
)

// Create a Sym.
func newsym(n string, kind int) (s *Sym) {
	s = allocsym()
	sgen++
	s.id = sgen
	s.name = n
	s.stype = kind
	s.fname = Scanner.fname
	s.lineno = Scanner.lineno
	s.ttype = tundef
	return s
}

// Symbol for a name or a fake
// string symbol to store the string
func strlookup(n string) *Sym {
	s, ok := strs[n]
	if !ok {
		stats.nstrs++
		s = newsym(n, Sstr)
	}
	return s
}

// Identifier lookup
// If given Snone, it returns any symbol type
// otherwise, it returns exactly the requested type or nil
func lookup(n string, kind int) *Sym {
	for e := env; e != nil; e = e.prev {
		s, ok := e.tab[n]
		if ok {
			if kind != Snone && kind != s.stype {
				return nil
			}
			return s
		}
	}
	return nil
}

// Return a keyword, an existing object, or a unique copy of n
func keylookup(n string) *Sym {
	s, ok := keys[n]
	if ok {
		return s
	}
	s = lookup(n, Snone)
	if s != nil {
		return s
	}
	return strlookup(n)
}

// Define an identifier with the given kind
// define a keyword if kind is Skey
func defsym(n string, kind int) (s *Sym) {
	s = newsym(n, kind)
	if kind == Skey {
		keys[n] = s
	} else {
		env.tab[n] = s
	}
	return s
}

func defssym(s *Sym, kind int) (n *Sym) {
	n = newsym(s.name, kind)
	if kind == Skey {
		keys[s.name] = n
	} else {
		env.tab[s.name] = n
	}
	n.fname = s.fname
	n.lineno = s.lineno
	return n
}

func (s *Sym) checkdup() int {
	for e := env; e != nil; e = e.prev {
		ns, ok := e.tab[s.name]
		if ok && ns.stype != Sstr {
			diag("'%s' already defined as a %s", s.name, stname[s.stype])
			return -1
		}
	}
	return 0
}

func evaluated(s *Sym) bool {
	return s.stype == Sconst
}

func islval(n *Sym) bool {
	switch n.stype {
	case Svar:
		return true
	case Sunary:
		if n.op == '^' {
			return islval(n.left)
		}
	case Sbinary:
		if n.op == '[' || n.op == '.' {
			return islval(n.left)
		}
	}
	return false
}

func cpval(d *Sym, s *Sym) {
	d.op = s.op
	d.ttype = oktype(s.ttype)
	// used to be a union
	// we don't know which to copy.
	d.tok = s.tok
	d.Val = s.Val
	d.left = s.left
	d.right = s.right
	d.fsym = s.fsym
	d.fargs = s.fargs
	d.rec = s.rec
	d.field = s.field
	d.swfield = s.swfield
	d.swval = s.swval
	d.prog = s.prog
}

func _newexpr(k int, op int, s1 *Sym, s2 *Sym) *Sym {
	var nd *Sym

	stats.nexpr++
	nd = allocsym()
	nd.stype = k
	nd.op = op
	nd.left = s1
	nd.right = s2
	if s1 != nil && s1.stype != Snone {
		nd.fname = s1.fname
		nd.lineno = s1.lineno
	} else {
		nd.fname = Scanner.fname
		nd.lineno = Scanner.lineno
	}
	switch k {
	case Snone:
		goto Fail
	case Sunary:
		if s1 == nil || tchkunary(nd) < 0 {
			goto Fail
		}
		evalexpr(nd)
	case Sbinary:
		if s1 == nil || s2 == nil || tchkbinary(nd) < 0 {
			goto Fail
		}
		evalexpr(nd)
	case Sconst:
		switch nd.op {
		case Oint:
			nd.ttype = tcint
		case Oreal:
			if math.IsNaN(nd.rval) {
				panic("invalid  float expression")
			}
			nd.ttype = tcreal
		case Ochar:
			nd.ttype = tcchar
		case Otrue:
			nd.ttype = tcbool
			nd.ival = 1
		case Ofalse:
			nd.ttype = tcbool
			nd.ival = 0
		case Onil:
			nd.ttype = tcnil
		case Ostr, Olit, Oaggr:
			// type must be given by caller

		default:
			errs := fmt.Sprintf("newexpr: Sconst: op %d", nd.op)
			panic(errs)
		}
	default:
		errs := fmt.Sprintf("newexpr: stype %d", k)
		panic(errs)
	}
	return nd
Fail:
	return badnode

}

func debugexpr(s *Sym) {
	if _, ok := debug['E']; ok {
		if s.ttype != nil || s.ttype != tundef {
			fmt.Fprintf(os.Stderr, " -> %v %v\n", s, s.ttype)
		} else {
			fmt.Fprintf(os.Stderr, " -> %v\n", s)
		}
	}
}

func newexpr(k int, op int, s1 *Sym, s2 *Sym) *Sym {
	if _, ok := debug['E']; ok {
		fmt.Fprintf(os.Stderr, "newexpr %s %s %v %v", stname[k], opname(rune(op)), s1, s2)
	}
	s := _newexpr(k, op, s1, s2)
	debugexpr(s)
	return s
}

func checkcond(s *Stmt, c *Sym) {
	if Nerrors > 0 || c == nil {
		return
	}
	if !c.ttype.Tis(Tbool) {
		s.Error("condition must be bool")
	}
}

//
// String handling is ugly, to say the least.
// For each string length, we generate a builtin array[] of char
// type with the required length.
// This may happen in any environment, but types are defined
// in the top-level.
//
func newstrtype(len int) *Type {
	var (
		t    *Type
		s    *Sym
		e    *Env
		name string
	)

	name = fmt.Sprintf("$tstr%d", len)
	s = lookup(name, Stype)
	if s != nil {
		return s.ttype
	}
	s = defsym(name, Stype)
	t = newtype(Tstr)
	t.idx = tcint
	t.elem = tchar
	t.sz = uint(len) * tchar.sz
	t.first = 0
	t.last = len - 1
	t.op = Tstr
	s.ttype = t
	t.sym = s
	s.ttype.id = tgen
	tgen++
	for e = env; e.prev != nil; e = e.prev {
	}
	addsym(e.prog.prog.types, s)
	return t
}

var strgen int

func newstr(sval string) *Sym {
	var e *Env
	nd := _newexpr(Sconst, Ostr, nil, nil)
	nd.sval = sval
	nd.name = fmt.Sprintf("$s%d", strgen)
	strgen++
	nd.ttype = newstrtype(utf8.RuneCountInString(sval))
	for e = env; e.prev != nil; e = e.prev {
	}
	addsym(e.prog.prog.consts, nd)
	return nd
}

func newint(ival int, op int, t *Type) *Sym {
	if _, ok := debug['E']; ok {
		fmt.Fprintf(os.Stderr, "new%s %d %v", opname(rune(op)), ival, t)
	}
	s := _newexpr(Sconst, op, nil, nil)
	s.ival = ival
	if t != nil {
		s.ttype = t
	}
	debugexpr(s)
	return s
}

func newreal(rval float64, t *Type) *Sym {
	if _, ok := debug['E']; ok {
		fmt.Fprintf(os.Stderr, "newreal %2.2g %v", rval, t)
	}
	s := _newexpr(Sconst, Oreal, nil, nil)
	s.rval = rval
	if t != nil {
		s.ttype = t
	}
	debugexpr(s)
	return s
}

func newvarnode(s *Sym) *Sym {
	if s == nil {
		return badnode
	}
	if s.stype != Svar && s.stype != Sconst {
		diag("no variable or constant with name '%s'", s.name)
		return badnode
	}
	return s
}

func (l *List) getsym(i int) *Sym {
	s := l.item[i].(*Sym)
	return s
}

func (l *List) getstmt(i int) *Stmt {
	s := l.item[i].(*Stmt)
	return s
}

func newfcall(f *Sym, args *List, op int) *Sym {
	var (
		n    *Sym
		t    *Type
		prog *Prog
	)
	if f == nil || args == nil {
		return badnode
	}
	if _, ok := debug['E']; ok {
		fmt.Fprintf(os.Stderr, "newfcall %s()", f.name)
	}
	if (f.stype != Sproc && f.stype != Sfunc) || f.prog == nil {
		diag("'%s': is not a subprogram", f.name)
		goto Fail
	}

	if !f.ttype.Tis(op) {
		diag("'%s' is not a %s", f.name, topname[op])
		goto Fail
	}
	t = nil
	n = nil
	prog = f.prog
	if prog.b != nil {
		if bargcheck(prog.b, args, prog.b.args) < 0 {
			goto Fail
		}
		if prog.rtype != tundef {
			t = prog.rtype
			if t == tundef {
				t = args.getsym(0).ttype
			}
		}
		n = prog.b.fn(prog.b, args)
		if n == nil {
			t = brtype(prog.b, args)
		}
	} else if tchkcall(f, args) < 0 {
		goto Fail
	} else {
		t = prog.rtype
	}

	if n == nil {
		n = allocsym()
		n.stype = Sfcall
		n.ttype = t
		if n.ttype == nil {
			n.ttype = tundef
		}
		n.fsym = f
		n.fargs = args
	}
	n.fname = f.fname
	n.lineno = f.lineno
	debugexpr(n)
	return n
Fail:
	debugexpr(badnode)
	return badnode
}

func findfield(t *Type, n string, why string) *Sym {
	if t.op != Trec {
		diag("using '%s' requires a record", why)
		return nil
	}
	if t.fields == nil {
		return nil
	}
	for i := 0; i < len(t.fields.item); i++ {
		s := t.fields.getsym(i)
		if s.name == n {
			return s
		}
	}
	diag("field '%s' does not exist", n)
	return nil
}

func setswfield(l *List, sw *Sym) {
	if l == nil || sw == nil || env.rec == nil {
		return
	}
	if sw == nil {
		return
	}
	if l.kind != Lsym {
		panic("l.kind != Lsym")
	}
	for i := 0; i < len(l.item); i++ {
		l.getsym(i).swfield = sw
	}
}

func setswval(l *List, sw *Sym) {
	if l == nil {
		return
	}
	if l.kind != Lsym {
		panic("l.kind != Lsym")
	}
	for i := 0; i < len(l.item); i++ {
		l.getsym(i).swval = sw
	}
}

func fieldaccess(lval *Sym, fld string) *Sym {
	var n, fs *Sym

	if lval == nil {
		return badnode
	}
	fs = findfield(lval.ttype, fld, ".")
	if fs == nil {
		return badnode
	}
	n = allocsym()
	n.stype = Sbinary
	n.op = '.'
	n.rec = lval
	n.field = fs
	n.ttype = fs.ttype
	n.left = lval
	n.right = fs
	n.fname = lval.fname
	n.lineno = lval.lineno
	return n
}

func newstmt(op int) *Stmt {
	stats.nstmts++
	s := new(Stmt)
	s.op = op
	s.sfname = Scanner.fname
	s.lineno = Scanner.lineno
	return s
}

func cpsrc(to *Stmt, from *Stmt) {
	to.sfname = from.sfname
	to.lineno = from.lineno
}

func newbody(l *List) *Stmt {
	s := newstmt('{')
	s.list = l
	if len(l.item) > 0 {
		cpsrc(s, l.getstmt(0))
	}
	return s
}

func allocprog() *Prog {
	stats.nprogs++
	return new(Prog)
}

var (
	once, pgen int
)

//
// NB: Eol is an abstraction. It represents \n, but it may
// be \r in windows. pilib reports Eol in the first char of
// the end-of-line sequence, and consumes any char left in
// that sequence during readeol()
//
func newprog(n *Sym) *Sym {
	if n == nil {
		panic("empty program declaration")
	}
	if n.stype != Sstr {
		if n.stype != Sproc {
			diag("'%s' is already defined as a %s",
				n.name, stname[n.stype])
		} else if n.prog != nil && n.prog.stmt != nil {
			diag("%s '%s' already defined", stname[n.stype], n.name)
			// else XXX: check that header matches and return it
		}
	}
	// continue despite errors; for safety
	s := defssym(n, Sproc)
	s.ttype = newtype(Tproc)
	s.prog = allocprog()
	s.prog.psym = s
	s.prog.consts = newlist(Lsym)
	s.prog.procs = newlist(Lsym)
	s.prog.types = newlist(Lsym)
	s.prog.vars = newlist(Lsym)
	s.prog.rtype = tundef

	if once == 0 {
		once++
		declstdtypes(s.prog.types)

		pstdin = defsym("stdin", Svar)
		pstdin.ttype = tfile
		addsym(s.prog.vars, pstdin)

		pstdout = defsym("stdout", Svar)
		pstdout.ttype = tfile
		addsym(s.prog.vars, pstdout)

		pstdgraph = defsym("stdgraph", Svar)
		pstdgraph.ttype = tfile
		addsym(s.prog.vars, pstdgraph)

		ns := defsym("Maxint", Sconst)
		ns.ttype = tcint
		ns.op = Oint
		ns.ival = tcint.last
		addsym(s.prog.consts, ns)

		ns = defsym("Minint", Sconst)
		ns.ttype = tcint
		ns.op = Oint
		ns.ival = tcint.first
		addsym(s.prog.consts, ns)

		ns = defsym("Maxchar", Sconst)
		ns.ttype = tcchar
		ns.op = Ochar
		ns.ival = tcchar.last
		addsym(s.prog.consts, ns)

		ns = defsym("Minchar", Sconst)
		ns.ttype = tcchar
		ns.op = Ochar
		ns.ival = tcchar.first
		addsym(s.prog.consts, ns)

		ns = defsym("Minstrength", Sconst)
		ns.ttype = tstrength
		ns.op = Ochar
		ns.ival = tstrength.first
		addsym(s.prog.consts, ns)

		ns = defsym("Maxstrength", Sconst)
		ns.ttype = tstrength
		ns.op = Ochar
		ns.ival = tstrength.last
		addsym(s.prog.consts, ns)

		ns = defsym("Transp", Sconst)
		ns.ttype = topacity
		ns.op = Oreal
		ns.rval = 0.0
		addsym(s.prog.consts, ns)

		ns = defsym("Tlucid", Sconst)
		ns.ttype = topacity
		ns.op = Oreal
		ns.rval = 0.5
		addsym(s.prog.consts, ns)

		ns = defsym("Opaque", Sconst)
		ns.ttype = topacity
		ns.op = Oreal
		ns.rval = 1.0
		addsym(s.prog.consts, ns)

		ns = defsym("NoBut", Sconst)
		ns.ttype = tbutton
		ns.op = Oint
		ns.rval = 0
		addsym(s.prog.consts, ns)

		chrs := map[string]byte{
			"Eol":       '\n',
			"Eof":       paminstr.Eof,
			"Tab":       paminstr.Tab,
			"Esc":       paminstr.Esc,
			"Nul":       paminstr.Nul,
			"Up":        paminstr.Up,
			"Down":      paminstr.Down,
			"Left":      paminstr.Left,
			"Right":     paminstr.Right,
			"Shift":     paminstr.Shift,
			"Ctrl":      paminstr.Ctrl,
			"Return":    paminstr.Return,
			"Del":       paminstr.Del,
			"MetaRight": paminstr.MetaRight,
			"MetaLeft":  paminstr.MetaLeft,
		}

		chrs["Eol"] = paminstr.EOL[0]
		for name, v := range chrs {
			ns = defsym(name, Sconst)
			ns.op = Ochar
			ns.ival = int(v)
			ns.ttype = tcchar
			addsym(s.prog.consts, ns)
		}
	} else {
		s.id = uint(pgen)
		pgen++
	}
	return s
}

func decltype(n *Sym, t *Type) *Sym {
	var (
		s *Sym
		p *Prog
	)
	if env.prog == nil {
		panic("missing program declaration")
	}
	if n.stype != Sstr && n.stype != Stype {
		diag("'%s' is already defined as a %s", n.name, stname[n.stype])
		return nil
	} else if n.stype == Stype {
		if n.ttype.op != Tfwd {
			diag("'%s' already defined", n.name)
			return nil
		}
		if _, ok := debug['D']; ok {
			t.id = tgen
			fmt.Fprintf(os.Stderr, "decltype: fwd %v with type %#v\n", n, t)
		}
		s = n
		*s.ttype = *t
	} else {
		s = defssym(n, Stype)
		if t == nil {
			t = newtype(Tfwd)
			s.ttype = t
		} else if t.sym != nil {
			s.ttype = newtype(t.op)
			*s.ttype = *t
		} else {
			s.ttype = t
		}
	}
	s.ttype.sym = s
	if t.op != Tfwd {
		s.ttype.id = tgen
		tgen++
		p = env.prog.prog
		addsym(p.types, s)
	}
	if _, ok := debug['D']; ok {
		fmt.Fprintf(os.Stderr, "decltype: id %d: %v = %#v\n", s.ttype.id, n, s.ttype)
	}
	return s
}

func declproc(n *Sym) *Sym {
	if env.prog == nil {
		panic("missing program declaration")
	}
	s := newprog(n)
	addsym(env.prog.prog.procs, s)
	s.ttype.op = Tproc
	s.stype = Sproc
	Pushenv()
	env.prog = s
	return s
}

func declfunc(n *Sym) *Sym {
	if env.prog == nil {
		panic("missing program declaration")
	}
	s := newprog(n)
	addsym(env.prog.prog.procs, s)
	s.ttype.op = Tfunc
	s.stype = Sfunc
	Pushenv()
	env.prog = s
	return s
}

func declprogdone(n *Sym) {
	if _, ok := debug['D']; ok {
		fmt.Fprintf(os.Stderr, "declfunc: %v\n", n)
	}
	p := n.prog
	parms := p.parms
	arevars := false
	if n.stype != Sfunc {
		if p.nrets > 0 {
			n.Error("procedure with return statements")
		}
		arevars = true
	}
	if !arevars {
		for i := 0; i < len(parms.item); i++ {
			if parms.getsym(i).op == Orefparm {
				parms.getsym(i).Error(
					"ref argument not allowed in function '%s'",
					n.name)
				break
			}
		}
		nr := returnsok(p.stmt, p.rtype)
		if nr < 0 {
			n.Error("function must end with a return statement")
		} else if p.nrets > nr {
			sn := firstret(p.stmt)
			if sn != nil {
				n = sn.expr
			}
			n.Error("return misused")
		}
	}
	vars := p.vars
	for i := 0; i < len(vars.item); i++ {
		setused(p.stmt, vars.getsym(i))
	}
}

func makevar(s *Sym, t *Type) {
	if env.prog == nil {
		panic("missing program declaration")
	}
	s.ttype = oktype(t)
	addsym(env.prog.prog.vars, s)
	if env.prev != nil {
		s.op = Olvar
	}
	if _, ok := debug['D']; ok {
		fmt.Fprintf(os.Stderr, "declvar: %v\n", s)
	}
}

func declvar(n *Sym, t *Type) *Sym {
	n.checkdup()
	s := defssym(n, Svar)
	makevar(s, t)
	return s
}

func declfield(n *Sym, t *Type) *Sym {
	s := defssym(n, Svar)
	s.ttype = oktype(t)
	if _, ok := debug['D']; ok {
		fmt.Fprintf(os.Stderr, "declfield: %v = %#v\n", s, t)
	}
	return s
}

func declconst(n *Sym, expr *Sym) *Sym {
	var e *Env

	if env.prog == nil {
		panic("missing program declaration")
	}
	n.checkdup()
	expr = oksym(expr)
	if !evaluated(expr) {
		diag("value for %s is not constant", n.name)
		return badnode
	}
	if expr.stype == Sconst && expr.op == Ostr && expr.name[0] == '$' {
		// temporary string has now a name
		for e = env; e.prev != nil; e = e.prev {
		}

		delitem(e.prog.prog.consts, expr)
	}
	if expr.stype == Sconst && expr.op == Oaggr && expr.name[0] == '$' {
		// temporary aggr has now a name
		for e = env; e.prev != nil; e = e.prev {
		}
		delitem(e.prog.prog.consts, expr)
	}
	s := defssym(n, Sconst)
	cpval(s, expr)
	addsym(env.prog.prog.consts, s)
	if _, ok := debug['D']; ok {
		fmt.Fprintf(os.Stderr, "declconst: %v %v -> %v %v\n", n, expr, s, s.ttype)
	}
	return s
}

func newparm(n *Sym, t *Type, byref int) *Sym {
	d := defssym(n, Svar)
	d.ttype = t
	if byref != 0 {
		d.op = Orefparm
	} else {
		d.op = Oparm
	}
	return d
}

func caseexpr(vnd *Sym, e *Sym) *Sym {
	if e.stype == Sbinary {
		switch e.op {
		case ',':
			e.left = caseexpr(vnd, e.left)
			e.right = caseexpr(vnd, e.right)
			e.op = Oor
			e.ttype = e.left.ttype
			return e
		case Odotdot:
			e.op = Oand
			e.left = newexpr(Sbinary, Oge, vnd, e.left)
			e.right = newexpr(Sbinary, Ole, vnd, e.right)
			e.ttype = e.left.ttype
			return e
		}
	}
	return newexpr(Sbinary, Oeq, vnd, e)
}

func newassign(lval *Sym, rval *Sym) *Stmt {
	var dummy *Type

	s := newstmt('=')
	lval = oksym(lval)
	rval = oksym(rval)
	s.lval = lval
	s.rval = rval
	if !tcompat(lval.ttype, rval.ttype, &dummy) {
		diag("incompatible argument types (%v and %v) for assignment", lval.ttype, rval.ttype)
	} else if lval != badnode && !islval(lval) {
		diag("left part of assignment must be an l-value")
	}
	return s
}

func dupvals(v1 *Sym, v2 *Sym) bool {
	if v1 == nil || v2 == nil {
		return false
	}
	switch v2.stype {
	case Sbinary:
		switch v2.op {
		case ',', Odotdot: // aprox: check left and right boundaries
			return dupvals(v1, v2.left) || dupvals(v1, v2.right)
		}
		return false
	case Sconst:
	default:
		return false
	}
	if v2.stype != Sconst {
		panic("v2.stype != Sconst")
	}
	switch v1.stype {
	case Sbinary:
		switch v1.op {
		case ',':
			return dupvals(v1.left, v2) || dupvals(v1.right, v2)
		case Odotdot:
			return v1.left.ival >= v2.ival &&
				v1.right.ival <= v2.ival
		}
		return false
	case Sconst:
		return v1.ival == v2.ival
	}
	return false
}

func dupcases(cases *List) {
	for i := 1; i < len(cases.item); i++ {
		for j := 0; j < i; j++ {
			if dupvals(cases.getstmt(i).expr, cases.getstmt(j).expr) {
				cases.getstmt(j).expr.Error("value used in previous case")
				return
			}
		}
	}
}

var vgen int

func newswitch(expr *Sym, cases *List) *Stmt {
	var xp *Stmt
	if Nerrors > 0 {
		return newstmt(';')
	}
	sx := newbody(newlist(Lstmt))
	sx.sfname = expr.fname
	sx.lineno = expr.lineno
	nm := fmt.Sprintf("$v%d", vgen)
	vgen++
	vs := defsym(nm, Svar)
	makevar(vs, cases.getstmt(0).expr.ttype)
	vnd := newvarnode(vs)
	addstmt(sx.list, newassign(vnd, expr))
	addstmt(sx.list, nil)
	nnil := 0
	dupcases(cases)
	xp = nil
	for i := 0; i < len(cases.item); i++ {
		cx := cases.getstmt(i)
		if cx.op != CASE {
			ss := fmt.Sprintf("p.y bug: switch stmt is %d", cx.op)
			panic(ss)
		}
		if cx.expr == nil {
			nnil++
			if i < len(cases.item)-1 {
				diag("default must be the last case")
			}
			if xp != nil {
				xp.elsearm = cx.stmt
			} else {
				sx.list.item[1] = cx.stmt
			}
			break
		} else {
			te := caseexpr(vnd, cx.expr)
			tx := cx.stmt
			cx.op = IF
			cx.cond = te
			cx.thenarm = tx
			cx.elsearm = nil
			if xp != nil {
				xp.elsearm = cx
			} else {
				sx.list.item[1] = cx
			}
			xp = cx
		}
	}
	if nnil > 1 {
		diag("only a default per switch")
	}
	return sx
}

func newfor(lval *Sym, from *Sym, to *Sym, body *Stmt) *Stmt {
	var ns *Stmt

	s := newbody(newlist(Lstmt))
	addstmt(s.list, newassign(lval, from))
	ws := newstmt(FOR)
	ws.expr = to
	ws.stmt = body
	if Nerrors == 0 {
		setslval(ws, lval)
	}
	addstmt(s.list, ws)
	arg := newlist(Lsym)
	addsym(arg, lval)
	if to.op == '>' || to.op == Oge {
		ns = newassign(lval, newfcall(bpred, arg, Tfunc))
	} else {
		ns = newassign(lval, newfcall(bsucc, arg, Tfunc))
	}
	ws.incr = ns
	return s
}

func mkint(n *Sym, v int) {
	n.stype = Sconst
	n.op = Oint
	n.ival = v
}

func mkreal(n *Sym, r float64) {
	n.stype = Sconst
	n.op = Oreal
	n.rval = r
}

func mkbool(n *Sym, b bool) {
	n.stype = Sconst
	if b {
		n.op = Otrue
		n.ival = 1
	} else {
		n.op = Ofalse
		n.ival = 0
	}
}

func ueval(n *Sym) {

	if n.stype != Sunary {
		panic("n.stype != Sunary")
	}
	if !evaluated(n.left) {
		return
	}
	ln := n.left
	switch n.op {
	case '+':
		cpval(n, ln)
	case Ouminus:
		if ln.ttype.Tis(Tint) {
			mkint(n, -ln.ival)
		} else {
			mkreal(n, -ln.rval)
		}
	case Onot:
		mkbool(n, ln.ival == 0)
	case Ocast:
		panic("ueval: Ocast should do this")
	case '^':
		panic("ueval: ^ arg can't be evaluated")
	default:
		ss := fmt.Sprintf("bad unary op %d", n.op)
		panic(ss)
	}
}

func cancmp(ln *Sym, rn *Sym) {
	b := (tisord(ln.ttype) && tisord(rn.ttype)) || (ln.ttype.Tis(Treal) && rn.ttype.Tis(Treal))
	if !b {
		panic("cancmp")
	}
}

func beval(n *Sym) {
	if n.stype != Sbinary {
		panic("n.stype != Sbinary")
	}
	if !evaluated(n.left) || !evaluated(n.right) {
		return
	}
	ln := n.left
	rn := n.right
	switch n.op {
	case '+':
		if ln.ttype.Tis(Tint) {
			mkint(n, ln.ival+rn.ival)
		} else {
			mkreal(n, ln.rval+rn.rval)
		}
	case '-':
		if ln.ttype.Tis(Tint) {
			mkint(n, ln.ival-rn.ival)
		} else {
			mkreal(n, ln.rval-rn.rval)
		}
	case '*':
		if ln.ttype.Tis(Tint) {
			mkint(n, ln.ival*rn.ival)
		} else {
			mkreal(n, ln.rval*rn.rval)
		}
	case '/':
		if ln.ttype.Tis(Tint) {
			if rn.ival == 0 {
				diag("divide by 0 in constant expression")
				mkint(n, 1)
			} else {
				mkint(n, ln.ival/rn.ival)
			}
		} else {
			if float64(rn.rval) == 0.0 {
				diag("divide by 0.0 in constant expression")
				mkreal(n, 1.0)
			} else {
				mkreal(n, ln.rval/rn.rval)
			}
		}
	case '%':
		mkint(n, ln.ival%rn.ival)
	case Opow:
		if ln.ttype.Tis(Tint) {
			if ln.ival == 2 {
				n.ival = 1 << uint(rn.ival)
			} else {
				n.ival = int(math.Pow(float64(ln.ival), float64(rn.ival)))
			}
			mkint(n, n.ival)
		} else {
			mkreal(n, math.Pow(ln.rval, rn.rval))
		}
	case Oand:
		mkbool(n, ln.ival != 0 && rn.ival != 0)
	case Oor:
		mkbool(n, ln.ival != 0 || rn.ival != 0)
	case '<':
		cancmp(ln, rn)
		if ln.ttype.Tis(Treal) {
			mkbool(n, ln.rval < rn.rval)
		} else {
			mkbool(n, ln.ival < rn.ival)
		}
	case '>':
		cancmp(ln, rn)
		if ln.ttype.Tis(Treal) {
			mkbool(n, ln.rval > rn.rval)
		} else {
			mkbool(n, ln.ival > rn.ival)
		}
	case Ole:
		cancmp(ln, rn)
		if ln.ttype.Tis(Treal) {
			mkbool(n, ln.rval <= rn.rval)
		} else {
			mkbool(n, ln.ival <= rn.ival)
		}
	case Oge:
		cancmp(ln, rn)
		if ln.ttype.Tis(Treal) {
			mkbool(n, ln.rval >= rn.rval)
		} else {
			mkbool(n, ln.ival >= rn.ival)
		}
	case Oeq:
		if ln.ttype.Tis(Trec) || ln.ttype.Tis(Tarry) || ln.ttype.Tis(Tstr) {
			break
		}
		if ln.ttype.Tis(Treal) {
			mkbool(n, ln.rval == rn.rval)
		} else {
			mkbool(n, ln.ival == rn.ival)
		}
	case One:
		if ln.ttype.Tis(Trec) || ln.ttype.Tis(Tarry) || ln.ttype.Tis(Tstr) {
			break
		}
		if ln.ttype.Tis(Treal) {
			mkbool(n, ln.rval != rn.rval)
		} else {
			mkbool(n, ln.ival != rn.ival)
		}
	case Odotdot, ',':
		fallthrough
	case '.': // can happen if we access an aggretate constant
		fallthrough
	case '[': // can happen if we access an aggretate constant

		//
		// BUG: for aggregate constant field/member access we
		// could evaluate the accessor and return the member.
		// We are deferring until run-time instead.
		//
	default:
		ss := fmt.Sprintf("tchkbinary: bad op %d", n.op)
		panic(ss)
	}
}

//
// Try to evaluate the expression. If we can, the node
// mutates to become a value, but the type is kept untouched.
//
func evalexpr(n *Sym) {
	switch n.stype {
	case Snone:

	case Sconst, Svar:

	case Sunary:
		ueval(n)
	case Sbinary:
		beval(n)
	case Sfcall:
		panic("evalexpr: newfcall should do this")
	default:
		ss := fmt.Sprintf("evalexpr: stype %d", n.stype)
		panic(ss)
	}
}

var skeynm = map[string]int{
	"False":     FALSE,
	"True":      TRUE,
	"and":       AND,
	"array":     ARRAY,
	"case":      CASE,
	"consts":    CONSTS,
	"default":   DEFAULT,
	"do":        DO,
	"else":      ELSE,
	"for":       FOR,
	"function":  FUNCTION,
	"if":        IF,
	"len":       LEN,
	"nil":       NIL,
	"not":       NOT,
	"of":        OF,
	"or":        OR,
	"procedure": PROCEDURE,
	"program":   PROGRAM,
	"record":    RECORD,
	"ref":       REF,
	"return":    RETURN,
	"switch":    SWITCH,
	"types":     TYPES,
	"vars":      VARS,
	"while":     WHILE,
}

func Syminit() {
	for name, tok := range skeynm {
		s := defsym(name, Skey)
		s.tok = tok
	}

	badnode = newsym("$undefined", Snone)
}

func (n *Sym) fstring(ishash bool) string {
	s := ""
	if n == nil {
		return fmt.Sprintf("<nil>")
	}
	switch n.stype {
	case Snone:
		return fmt.Sprintf("undef")
	case Skey, Sstr:
		s = fmt.Sprintf("%s", CEscape(n.name))
		return s
	case Sconst:
		if n.name != "" {
			s = fmt.Sprintf("%s=", n.name)
		}
		switch n.op {
		case Ochar:
			switch n.ival {
			case int(paminstr.EOL[0]):
				return s + fmt.Sprintf("Eol")
			case '\'':
				return s + fmt.Sprintf("'")
			case paminstr.Tab:
				return s + fmt.Sprintf("Tab")
			case paminstr.Nul:
				return s + fmt.Sprintf("Nul")
			case paminstr.Eof:
				return s + fmt.Sprintf("Eof")
			case paminstr.Esc:
				return s + fmt.Sprintf("Esc")
			case paminstr.Up:
				return s + fmt.Sprintf("Up")
			case paminstr.Down:
				return s + fmt.Sprintf("Down")
			case paminstr.Left:
				return s + fmt.Sprintf("Left")
			case paminstr.Right:
				return s + fmt.Sprintf("Right")
			case paminstr.Shift:
				return s + fmt.Sprintf("Shift")
			case paminstr.Ctrl:
				return s + fmt.Sprintf("Ctrl")
			case paminstr.Return:
				return s + fmt.Sprintf("Return")
			case paminstr.MetaRight:
				return s + fmt.Sprintf("MetaRight")
			case paminstr.MetaLeft:
				return s + fmt.Sprintf("MetaLeft")
			case paminstr.Del:
				return s + fmt.Sprintf("Del")
			default:
				return s + fmt.Sprintf("'%c'", byte(n.ival))
			}
		case Oint:
			return s + fmt.Sprintf("%d", n.ival)
		case Olit:
			if n.ttype.Tis(Tenum) {
				return s + fmt.Sprintf("%s", enumname(n.ttype, n.ival))
			}
			return s + fmt.Sprintf("%d", n.ival)
		case Oreal:
			return s + fmt.Sprintf("%f", n.rval) //BUG probably better %g
		case Ostr:
			return s + fmt.Sprintf("\"%s\"", n.sval)
		case Oaggr:
			s += fmt.Sprintf("%v(", n.ttype)
			if n.vals != nil {
				for i := 0; i < len(n.vals.item); i++ {
					if i > 0 {
						s += fmt.Sprintf(", ")
					}
					s += fmt.Sprintf("%v", n.vals.getsym(i))
				}
			}
			return s + fmt.Sprintf(")")
		case Otrue, Ofalse, Onil:
			return s + fmt.Sprintf("%s", opname(rune(n.op)))
		default:
			fmt.Fprintf(os.Stderr, "Sym fmt called with op %d\n", n.op)
		}
		return s + fmt.Sprintf("NCBUG(%d)", n.op)
	case Svar:
		switch n.op {
		case Olvar:
			s += fmt.Sprintf("%%")
		case Oparm:
			s += fmt.Sprintf("$")
		case Orefparm:
			s += fmt.Sprintf("&")
		}
		fallthrough
	case Stype:
		if n.name != "" {
			s += fmt.Sprintf("%s", n.name)
		} else {
			s += fmt.Sprintf("<noname>")
		}
		if n.ttype != nil {
			s += fmt.Sprintf(": %v", n.ttype)
		}
		return s
	case Sunary:
		if ishash {
			return s + fmt.Sprintf("%s", opname(rune(n.op)))
		}
		return s + fmt.Sprintf("%s(%v)", opname(rune(n.op)), n.left)
	case Sbinary:
		if ishash {
			return s + fmt.Sprintf("%s", opname(rune(n.op)))
		}
		return s + fmt.Sprintf("%s(%v, %v)", opname(rune(n.op)), n.left, n.right)
	case Sproc, Sfunc:
		return s + fmt.Sprintf("%s %s()", stname[n.stype], n.name)
	case Sfcall:
		if ishash {
			return s + fmt.Sprintf("%s()", n.fsym.name)
		}
		s += fmt.Sprintf("%s(", n.fsym.name)
		for i := 0; i < len(n.fargs.item); i++ {
			if i > 0 {
				s += fmt.Sprintf(", ")
			}
			s += fmt.Sprintf("%v", n.fargs.getsym(i))
		}
		return s + fmt.Sprintf(")")
	default:
		fmt.Fprintf(os.Stderr, "Sym fmt called with stype %d\n", n.stype)
		return s + fmt.Sprintf("NBUG(%d)", n.stype)
	}
	return s
}

func (n *Sym) String() string {
	return n.fstring(false)
}

func (n *Sym) GoString() string {
	return n.fstring(true)
}

func tabs(lvl int) string {
	s := ""
	for lvl > 0 {
		lvl--
		s = s + "\t"
	}
	return s
}

var xlvl int

func (x *Stmt) block(lvl int) (s string) {
	if x.list != nil {
		for i := 0; i < len(x.list.item); i++ {
			s += fmt.Sprintf("%v\n", x.list.getstmt(i))
		}
	}
	s += tabs(lvl - 1)
	s += fmt.Sprintf("}")
	return s
}

func (x *Stmt) fstring(ishash bool) string {
	s := ""
	if false && x != nil {
		s = fmt.Sprintf("%s:%d", CEscape(x.sfname), x.lineno)
		s = CEscape(s)
	}
	s += tabs(xlvl)
	if x == nil {
		return s + fmt.Sprintf("<nilstmt>")
	}
	xlvl++

	switch x.op {
	case DO:
		s += fmt.Sprintf("dowhile(%v){\n", x.expr)
		if ishash {
			break
		}
		x = x.stmt
		s += x.block(xlvl)
	case WHILE:
		s += fmt.Sprintf("while(%v){\n", x.expr)
		x = x.stmt
		s += x.block(xlvl)
	case FOR:
		/* Does not print x.incr; */
		s += fmt.Sprintf("for(%v){\n", x.expr)
		x = x.stmt
		s += x.block(xlvl)
	case CASE:
		s += fmt.Sprintf("case %v {\n", x.expr)
		x = x.stmt
		s += x.block(xlvl)
	case '{', ELSE:
		s += fmt.Sprintf("{\n")
		s += x.block(xlvl)
	case FCALL:
		if x.op != FCALL {
			panic("x.op != FCALL")
		}
		s += fmt.Sprintf("%v", x.fcall)
	case '=':
		s += fmt.Sprintf("%v = %v", x.lval, x.rval)
	case IF:
		s += fmt.Sprintf("if(%v)\n", x.cond)
		if x.thenarm.op == '{' {
			xlvl--
		}
		s += fmt.Sprintf("%v\n", x.thenarm)
		if x.thenarm.op == '{' {
			xlvl++
		}
		if x.elsearm != nil {
			s += tabs(xlvl - 1)
			s += fmt.Sprintf("else\n")
			xlvl--
			s += fmt.Sprintf("%v\n", x.elsearm)
			xlvl++
		}
	case ';':
		s += fmt.Sprintf("nop\n")
	case 0:
		s += fmt.Sprintf("undef\n")
	case RETURN:
		s += fmt.Sprintf("return %v\n", x.expr)
	default:
		fmt.Fprintf(os.Stderr, "Stmt fmt called with op %d\n", x.op)
		s += fmt.Sprintf("XBUG(%d)", x.op)
	}
	xlvl--
	return s
}

func (x *Stmt) String() string {
	return x.fstring(false)
}

func (x *Stmt) GoString() string {
	return x.fstring(true)
}

func fmttabs(lvl int) string {
	s := ""
	for lvl > 0 {
		lvl--
		s += fmt.Sprint("\t")
	}
	return s
}

var plvl int

const (
	SCPARAM = iota
	SCCONST
	SCTYPES
	SCVARS
)

func fmtparm(s *Sym, l *List, sect int) string {
	str := ""
	switch sect {
	case SCPARAM:
		if s.op == Orefparm {
			str += fmt.Sprintf("ref %v\n", s)
		} else {
			str += fmt.Sprintf("val %v\n", s)
		}
	case SCCONST:
		str += fmt.Sprintf("%v\n", s)
	case SCTYPES:
		str += fmt.Sprintf("%s = %#v\n", s.name, s.ttype)
	case SCVARS:
		str += fmt.Sprintf("%v\n", s)
	default:
		panic("bad prparm")
	}
	return str
}

func fmtw(l *List, sect int) string {
	s := ""
	if l != nil && len(l.item) > 0 {
		s += fmttabs(plvl)
		s += fmt.Sprint("parms:\n")
		for i := 0; i < len(l.item); i++ {
			sym := l.getsym(i)
			s += fmttabs(plvl + 1)
			s += fmtparm(sym, l, sect)
		}
		s += fmt.Sprint("\n")
	}
	return s
}

func fmtprog(s *Sym) string {
	str := ""
	str += fmttabs(plvl)
	p := s.prog
	if p == nil {
		str += fmt.Sprint("<nullprog>\n")
		return str
	}
	if p.rtype == tundef {
		str += fmt.Sprintf("prog: %v\n", s)
	} else {
		str += fmt.Sprintf("prog: %v: %v\n", s, p.rtype)
	}
	plvl++
	str += fmtw(p.parms, SCPARAM)
	str += fmtw(p.consts, SCCONST)
	str += fmtw(p.types, SCTYPES)
	str += fmtw(p.vars, SCVARS)
	if p.procs != nil {
		for i := 0; i < len(p.procs.item); i++ {
			sym := p.procs.getsym(i)
			str += fmtprog(sym)
		}
	}
	fmttabs(plvl)
	str += fmt.Sprintf("stmt:\n")
	xlvl = plvl
	str += fmt.Sprintf("%v\n\n", p.stmt)
	xlvl = 0
	plvl--
	return str
}

func dumpprog(w io.Writer, s *Sym) {
	fmt.Fprintf(w, "%s", fmtprog(s))
}

func dumpenv(w io.Writer, e *Env, recur int) {
	xs := uint(0)
	if env == nil {
		fmt.Fprintf(w, "nilenv\n")
		return
	}
	if e != nil {
		xs = e.id
	}
	fmt.Fprintf(w, "env %uld:\n", xs)
	for _, s := range env.tab {
		xs = 0
		if s != nil {
			xs = s.id
		}
		fmt.Fprintf(w, "\t%uld: %v\n", xs, s)
	}
	if env.prog != nil {
		dumpprog(w, env.prog)
	}
	fmt.Fprintf(w, "\n")

	if recur != 0 && e.prev != nil {
		fmt.Fprintf(w, "prev ")
		dumpenv(w, e.prev, recur)
	}

}

//KLUDGE to be compatible with C pam
func CEscape(s string) string {
	s = quoteWith(s, '\'')
	return s
}

//Taken from strconv, rewritten to be compatible with fmtquote
const lowerhex = "0123456789abcdef"

func quoteWith(s string, quote byte) string {
	var runeTmp [utf8.UTFMax]byte
	buf := make([]byte, 0, 3*len(s)/2) // Try to avoid more allocations.
	buf = append(buf, quote)
	for width := 0; len(s) > 0; s = s[width:] {
		r := rune(s[0])
		width = 1
		if r >= utf8.RuneSelf {
			r, width = utf8.DecodeRuneInString(s)
		}
		if width == 1 && r == utf8.RuneError {
			buf = append(buf, `\x`...)
			buf = append(buf, lowerhex[s[0]>>4])
			buf = append(buf, lowerhex[s[0]&0xF])
			continue
		}
		if r == rune(quote) {
			buf = append(buf, byte(r))
			buf = append(buf, byte(r))
			continue
		}
		if r == '\\' { // always backslashed
			buf = append(buf, '\\')
			buf = append(buf, byte(r))
			continue
		}
		if strconv.IsPrint(r) {
			n := utf8.EncodeRune(runeTmp[:], r)
			buf = append(buf, runeTmp[:n]...)
			continue
		}
		switch r {
		case '\a':
			buf = append(buf, `\a`...)
		case '\b':
			buf = append(buf, `\b`...)
		case '\f':
			buf = append(buf, `\f`...)
		case '\n':
			buf = append(buf, `\n`...)
		case '\r':
			buf = append(buf, `\r`...)
		case '\t':
			buf = append(buf, `\t`...)
		case '\v':
			buf = append(buf, `\v`...)
		default:
			switch {
			case r < ' ':
				buf = append(buf, `\x`...)
				buf = append(buf, lowerhex[s[0]>>4])
				buf = append(buf, lowerhex[s[0]&0xF])
			case r > utf8.MaxRune:
				r = 0xFFFD
				fallthrough
			case r < 0x10000:
				buf = append(buf, `\u`...)
				for s := 12; s >= 0; s -= 4 {
					buf = append(buf, lowerhex[r>>uint(s)&0xF])
				}
			default:
				buf = append(buf, `\U`...)
				for s := 28; s >= 0; s -= 4 {
					buf = append(buf, lowerhex[r>>uint(s)&0xF])
				}
			}
		}
	}
	buf = append(buf, quote)
	return string(buf)

}

func mapl(cl *List, fn func(*Sym)) {
	if cl != nil {
		for i := 0; i < len(cl.item); i++ {
			fn(cl.getsym(i))
		}
	}
}

func mapxl(cl *List, fn func(*Stmt)) {
	if cl != nil {
		for i := 0; i < len(cl.item); i++ {
			fn(cl.getstmt(i))
		}
	}
}

func maprl(cl *List, fn func(*Sym)) {
	if cl != nil {
		for i := len(cl.item) - 1; i >= 0; i-- {
			fn(cl.getsym(i))
		}
	}

}

func (s *Stmt) FuncPos() (string, int) {
	return s.sfname, s.lineno
}

func (s *Sym) FuncPos() (string, int) {
	return s.fname, s.lineno
}
