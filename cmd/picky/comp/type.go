package comp

import (
	"clive/cmd/picky/paminstr"
	"fmt"
	"os"
)

const (
	// Type kinds
	Tundef = iota
	Tint
	Tbool
	Tchar
	Treal
	Tenum // 5
	Trange
	Tarry
	Trec
	Tptr
	Tfile // 10
	Tproc
	Tfunc
	Tprog
	Tfwd
	Tstr // 15; fake: array[int] of char; but universal
	Tstrength
	Topacity
	Tcolor
	Tbutton
	Tsound
	Tlast
)

var (
	sndnames = []string{
		paminstr.Woosh:      "Woosh",
		paminstr.Beep:       "Beep",
		paminstr.Sheep:      "Sheep",
		paminstr.Phaser:     "Phaser",
		paminstr.Rocket:     "Rocket",
		paminstr.CNote:      "CNote",
		paminstr.CsharpNote: "CsharpNote",
		paminstr.DNote:      "DNote",
		paminstr.DsharpNote: "DsharpNote",
		paminstr.ENote:      "ENote",
		paminstr.FNote:      "FNote",
		paminstr.FsharpNote: "FsharpNote",
		paminstr.GNote:      "GNote",
		paminstr.GsharpNote: "GsharpNote",
		paminstr.ANote:      "ANote",
		paminstr.AsharpNote: "AsharpNote",
		paminstr.BNote:      "BNote",
		paminstr.Bomb:       "Bomb",
		paminstr.Fail:       "Fail",
		paminstr.Tada:       "Tada",
	}
	colnames = []string{
		paminstr.Black:  "Black",
		paminstr.Red:    "Red",
		paminstr.Green:  "Green",
		paminstr.Blue:   "Blue",
		paminstr.Yellow: "Yellow",
		paminstr.Orange: "Orange",
		paminstr.White:  "White",
	}
	// constant, universal types
	tcchar, tcbool, tcreal, tcint, tcnil *Type
	// predefined types
	tchar, tbool, treal, tundef, tint, tfile     *Type
	tstrength, topacity, tcolor, tbutton, tsound *Type
	topname                                      = []string{
		Tundef:    "undef",
		Tint:      "int",
		Tbool:     "bool",
		Tchar:     "char",
		Treal:     "float",
		Tenum:     "enum",
		Trange:    "range",
		Tarry:     "array",
		Trec:      "record",
		Tptr:      "ptr",
		Tfile:     "file",
		Tproc:     "procedure",
		Tfunc:     "function",
		Tprog:     "program",
		Tfwd:      "forward",
		Tstr:      "str",
		Tstrength: "strength",
		Topacity:  "opacity",
		Tcolor:    "color",
		Tbutton:   "button",
		Tsound:    "sound",
	}
	topsz = []uint{
		Tundef:    0,
		Tint:      4, //uint32
		Tbool:     4, //uint32
		Tchar:     4, //uint32
		Treal:     4, //uint32
		Tenum:     4, //uint32
		Tfile:     4, //uint32
		Tptr:      8, //uint64
		Tproc:     8, //uint64
		Tfunc:     8, //uint64
		Tstrength: 4, //uint32
		Topacity:  4, //uint32
		Tbutton:   4, //uint32
	}
)

func newtype(op int) *Type {
	stats.ntypes++
	t := new(Type)
	t.op = op
	if t.op < Tlast {
		t.sz = topsz[t.op] //BUG in original
	} else {
		panic("bad type size")
	}
	t.id = ^uint(0)
	switch op {
	case Tbool:
		t.last = 1
	case Tbutton:
		t.super = tint
		t.last = 255
	case Tchar:
		t.last = 255
	case Tint:
		t.first = paminstr.Minint
		t.last = paminstr.Maxint
	case Tstrength:
		t.super = tint
		t.first = 0
		t.last = 255
	case Topacity:
		t.super = treal
		t.first = 0
		t.last = 1
	}
	return t
}

func tderef(t *Type) *Type {
	for t!=nil && t.op==Trange {
		t = t.super
	}
	if t == nil {
		return tundef
	}
	return t
}

func (t *Type) Tis(op int) bool {
	t = tderef(t)
	if t.op==Tstrength || t.op==Tbutton || t.op==Topacity {
		t = t.super
	}
	return t.op == op
}

func tisatom(t *Type) bool {
	if t == nil {
		return true
	}
	return t.op!=Trec && t.op!=Tarry && t.op!=Tstr
}

func tisord(t *Type) bool {
	t = tderef(t)
	return t.op==Tint || t.op==Tbool || t.op==Tchar || t.op==Tenum || t.op==Tstrength || t.op==Tbutton
}

func tiscmp(t *Type) bool {
	if tisord(t) {
		return true
	}
	t = tderef(t)
	res := t.op==Treal || (t.op==Tarry && tiscmp(t.elem)) || t.op==Tstr
	return res || t.op==Trec || t.op==Tptr || t.op==Tstrength || t.op==Topacity || t.op==Tbutton
}

func tfirst(t *Type) int {
	return t.first
}

func tlast(t *Type) int {
	return t.last
}

func (t *Type) Tsz() uint {
	return t.sz
}

func tlen(t *Type) int {
	switch t.op {
	case Tbool, Tchar, Tenum, Trange, Tarry, Tstr:
		return t.last - t.first + 1
	case Trec:
		return len(t.fields.item)
	default:
		return 0
	}
	return 0
}

func (t *Type) Trange() (fst, last, ttlen int) {
	return tfirst(t), tlast(t), tlen(t)
}

func newarrytype(idx *Type, elem *Type) *Type {
	t := newtype(Tarry)
	idx = oktype(idx)
	elem = oktype(elem)
	t.idx = idx
	t.elem = elem
	t.first = idx.first
	t.last = idx.last
	sz := tlen(t.idx)
	if sz<=0 || sz>=Maxidx {
		diag("array size is too small or too large")
		idx.last = idx.first
		sz = 1
	}
	t.sz = uint(sz*int(t.elem.sz))
	if t.sz<=0 || t.sz>=Maxidx {
		diag("array size is too small or too large")
		idx.last = idx.first
		t.sz = t.elem.sz
	}
	return t
}

func newordtype(nl *List) *Type {
	if nl == nil {
		return tundef
	}
	if nl.kind != Lsym {
		panic("nl.kind != Lsym")
	}
	t := newtype(Tenum)
	t.lits = nl
	t.last = len(nl.item) - 1
	for i := 0; i < len(nl.item); i++ {
		ns := nl.item[i].(*Sym)
		ns.checkdup()
		nl.item[i] = defssym(ns, Sconst)
		s := nl.getsym(i)
		s.op = Olit
		s.ttype = t
		s.ival = i
	}
	return t
}

func mklitcast(t *Type, ival int) *Sym {
	r := newexpr(Sconst, Olit, nil, nil)
	r.ival = ival
	r.ttype = t
	return r
}

//
// Valid casts are float(int|float), int(ordinal|float), and ordinal(int).
//

func newcast(t *Type, n *Sym) *Sym {
	var (
		dummy *Type
	)

	if t==nil || n==nil {
		return badnode
	}
	if tcompat(t, n.ttype, &dummy) {
		diag("useless type cast (operand and result are type-compatible)")
		return n
	}

	if t.Tis(Treal) && n.ttype.Tis(Treal) {
		if evaluated(n) {
			return newreal(n.rval, t)
		}
	} else if t.Tis(Treal) && n.ttype.op==Tint {
		if evaluated(n) {
			return newreal(float64(n.ival), t)
		}
	} else if t.op==Tint && n.ttype.Tis(Treal) {
		if evaluated(n) {
			return newint(int(n.rval), Oint, t)
		}
	} else if t.op==Tint && tisord(n.ttype) {
		if evaluated(n) {
			return newint(n.ival, Oint, t)
		}
	} else if tisord(t) && n.ttype.op==Tint {
		if evaluated(n) {
			if t.op!=Tint && (n.ival<t.first || n.ival>t.last) {
				diag("value out of range in type cast")
				return badnode
			}
			if t.Tis(Tint) {
				return newint(n.ival, Oint, t)
			}
			if t.Tis(Tchar) {
				return newint(n.ival, Ochar, t)
			}
			if t.Tis(Tbool) {
				v := Otrue
				if n.ival == 0 {
					n.ival = Ofalse
				}
				return newint(n.ival, v, t)
			}
			return mklitcast(t, n.ival)
		}
	} else {
		diag("invalid type cast")
		return badnode
	}

	r := newexpr(Sunary, Ocast, n, nil)
	r.ttype = t
	return r
}

var agen int

func newaggr(t *Type, nl *List) *Sym {
	var (
		n         int
		et, dummy *Type
		e         *Env
	)

	if len(nl.item) == 0 {
		return badnode
	}
	switch t.op {
	case Tarry:
		n = t.last - t.first + 1
	case Trec:
		n = len(t.fields.item)
	default:
		if tisatom(t) && len(nl.item)==1 {
			return newcast(t, nl.item[0].(*Sym))
		}
		diag("can't define aggregates for this type")
		return badnode
	}
	if len(nl.item) < n {
		diag("not enough elements in aggregate")
		return badnode
	}
	if len(nl.item) > n {
		diag("too many elements in aggregate")
		return badnode
	}
	for i := 0; i < len(nl.item); i++ {
		s := nl.getsym(i)
		if t.op == Tarry {
			et = t.elem
		} else {
			et = t.fields.getsym(i).ttype
		}
		if !tcompat(et, s.ttype, &dummy) {
			diag("incompatible type in aggregate element\n"+"\t('%v' expected; got '%v')", et, s.ttype)
			return badnode
		}
		if !evaluated(s) {
			diag("aggregate element must be a constant")
			return badnode
		}
	}
	r := newexpr(Sconst, Oaggr, nil, nil)
	r.vals = nl
	r.ttype = t
	r.name = fmt.Sprintf("$a%d", agen)
	agen++
	for e = env; e.prev != nil; e = e.prev {
	}

	addsym(e.prog.prog.consts, r)
	return r
}

func rvalchk(n *Sym) int {
	evalexpr(n)
	if n.stype == Sconst {
		switch n.op {
		case Ochar, Oint, Otrue, Ofalse, Olit:
			return 0
		}
	}
	return -1
}

//
// Super may be nil, if we are building an implicit range type
// for array indexes. In this case, we define a type sym for the range.
//

var rgen int

func newrangetype(super *Type, v0 *Sym, v1 *Sym) *Type {
	var (
		st *Type
		s  *Sym
		e  *Env
	)
	if rvalchk(v0)<0 || rvalchk(v1)<0 {
		diag("range limits are not definite constants")
		return tundef
	}
	if v0.ival >= v1.ival {
		diag("empty range")
		return tundef
	}
	if super!=nil && super.op==Tenum {
		if v0.ival<tfirst(super) || v1.ival>tlast(super) {
			diag("range limits are off limits")
		}
	}
	if !tcompat(v0.ttype, v1.ttype, &st) {
		diag("types not compatible in range")
	}
	if super!=nil && !tcompat(st, super, &st) {
		diag("range value types not compatible with super type\n"+"\t%v vs. %v\n", st, super)
	}
	t := newtype(Trange)
	t.super = tderef(st)
	t.first = v0.ival
	t.last = v1.ival
	t.sz = t.super.sz

	if super == nil {
		rgen++
		name := fmt.Sprintf("$range%d", rgen)
		s = defsym(name, Stype)
		t.id = tgen
		tgen++
		s.ttype = t
		t.sym = s
		for e = env; e.prev != nil; e = e.prev {
		}

		addsym(e.prog.prog.types, s)
	}
	return t
}

func initrectype(t *Type) {
	var (
		s  *Sym
		n  string
		tp *Type
	)

	tl := t.fields
	t.sz = 0
	for i := 0; i < len(tl.item); i++ {
		if tl.getsym(i).swfield != nil {
			n = tl.getsym(i).swfield.name
			s = findfield(t, n, "switch")
			if s != tl.getsym(i).swfield {
				diag("'%s' is not a field to switch on", n)
				return
			}
			if tl.getsym(i).swfield.swfield != nil {
				tl.getsym(i).swfield.Error("can't switch on a variant field")
				return
			}
			if Nerrors > 0 {
				continue
			}
			if tl.getsym(i).swval != nil {
				panic("tl.getsym(i).swval != nil")
			}
			if !s.ttype.Tis(Tenum) {
				tl.getsym(i).swfield.Error("switch field is not an enumerated type")
				return
			}
			if !tcompat(s.ttype, tl.getsym(i).swval.ttype, &tp) {
				tl.getsym(i).swfield.Error("case value of the wrong type")
				return
			}
		}
		s = findfield(t, tl.getsym(i).name, "dup")
		if s != tl.getsym(i) {
			tl.getsym(i).Error("dup field '%s'", s.name)
		}
		tl.getsym(i).addr = t.sz
		t.sz += tl.getsym(i).ttype.sz
	}
}

//
// Handle universal type compatibility. Return:
// 0: it's this type, but not compatible;
// 1: it's compatible;
// -1: not this type at all.
//
func tuniv(t0 *Type, t1 *Type, u *Type, op int, tp **Type) int {
	if t0 == u {
		if t1.Tis(op) {
			*tp = t1
			return 1
		}
		return 0
	}
	if t1 == u {
		if t0.Tis(op) {
			*tp = t0
			return 1
		}
		return 0
	}
	return -1
}

//
// Types are compatible if they are the same.
// Universal bool, int, char, real, nil, and str are compatible
// with any isomorphic type (in which case *tp is set to the
// non-universal type, if any).
//
func tcompat(t0 *Type, t1 *Type, tp **Type) bool {
	var (
		u int
		t *Type
	)
	if t0==tundef || t1==tundef {
		*tp = tundef
		return true
	}
	t0 = tderef(t0)
	t1 = tderef(t1)
	if t0 == t1 {
		*tp = t0
		return true
	}
	*tp = tundef
	u = tuniv(t0, t1, tcint, Tint, tp)
	if u >= 0 {
		return u != 0
	}
	u = tuniv(t0, t1, tcchar, Tchar, tp)
	if u >= 0 {
		return u != 0
	}
	u = tuniv(t0, t1, tcbool, Tbool, tp)
	if u >= 0 {
		return u != 0
	}
	u = tuniv(t0, t1, tcreal, Treal, tp)
	if u >= 0 {
		return u != 0
	}
	u = tuniv(t0, t1, tcnil, Tptr, tp)
	if u >= 0 {
		return u != 0
	}
	if t0.Tis(Tstr) || t1.Tis(Tstr) {
		if t0.Tis(Tstr) {
			t = t1
		} else {
			t = t0
		}
		if t.Tis(Tarry) && t.idx.Tis(Tint) && t.elem.Tis(Tchar) && tlen(t0)==tlen(t1) {
			*tp = t
			return true
		}
	}
	return false
}

func tchkunary(n *Sym) int {
	if n.stype == Snone {
		return -1
	}
	if n.stype != Sunary {
		ss := fmt.Sprintf("tchkunary: stype %d", n.stype)
		panic(ss)
	}
	if n.ttype != nil {
		return 0
	}
	if n.left.stype==Snone || n.left.ttype==nil {
		return -1
	}

	lt := n.left.ttype
	switch n.op {
	case '+':

	case Ouminus:
		if lt.Tis(Tint) || lt.Tis(Treal) {
			n.ttype = lt
		} else {
			diag("'%c' requires a numeric argument", n.op)
			goto Fail
		}
		n.ttype = lt
	case Onot:
		if lt.Tis(Tbool) {
			n.ttype = lt
		} else {
			diag("'not' requires a %s argument", topname[Tbool])
			goto Fail
		}
		n.ttype = lt
	case Ocast:
		// arg types checked and n's type set by newcast
		return 0
	case '^':
		if !lt.Tis(Tptr) {
			diag("'^' requires a pointer as an argument")
			goto Fail
		}
		n.ttype = tderef(lt).ref
	default:
		ss := fmt.Sprintf("bad unary op '%s'", opname(rune(n.op)))
		panic(ss)
	}
	if n.ttype == nil {
		panic("n.ttype == nil")
	}
	return 0
Fail:
	n.stype = Snone
	n.ttype = tundef
	return -1
}

func tchkbinary(n *Sym) int {
	var dummy *Type

	if n.stype == Snone {
		return -1
	}
	if n.stype != Sbinary {
		ss := fmt.Sprintf("tchkbinary: stype %d", n.stype)
		panic(ss)
	}
	if n.ttype != nil {
		return 0
	}

	lt := n.left.ttype
	rt := n.right.ttype
	if n.left.stype==Snone || n.right.stype==Snone {
		goto Fail
	}

	switch n.op {
	case '+', '-', '*', '/', Opow:
		/* check only the left arg. tcompat will ensure rt is ok. */
		if !lt.Tis(Tint) && !lt.Tis(Treal) {
			diag("'%s' requires numeric arguments", opname(rune(n.op)))
			goto Fail
		}
		goto Lchk
	case '%':
		if !lt.Tis(Tint) || !rt.Tis(Tint) {
			diag("'%s' requires int arguments", opname(rune(n.op)))
			goto Fail
		}
		goto Lchk
	case Oand, Oor:
		if !lt.Tis(Tbool) || !rt.Tis(Tbool) {
			diag("'%s' requires bool arguments", opname(rune(rune(n.op))))
			goto Fail
		}
		goto Lchk
	case '<', '>', Ole, Oge:
		/* check only the left arg. tcompat will ensure rt is ok. */
		if !tisord(lt) && !lt.Tis(Treal) {
			diag("'%s' requires numeric or ordinal arguments", opname(rune(n.op)))
			goto Fail
		}
		goto Bchk
	case Oeq, One:
		if !tiscmp(lt) || !tiscmp(rt) {
			diag("'%s' not defined for this type", opname(rune(n.op)))
			goto Fail
		}
		goto Bchk
	case '[':
		if !lt.Tis(Tarry) && !lt.Tis(Tstr) {
			diag("using '[]' requires an %s", topname[Tarry])
			goto Fail
		}
		if !tcompat(tderef(lt).idx, rt, &n.ttype) {
			diag("index type is not compatible")
			goto Fail
		}
		n.ttype = lt.elem
	case '.':
		panic("tchkbinary: findfield must check this")
	case Odotdot:
		if !tisord(lt) || !tisord(rt) {
			diag("'%s' requires an ordinal type", opname(rune(n.op)))
			goto Fail
		}
		goto Lchk
	case ',':
		if n.left.op!=',' && n.left.op!=Odotdot && !tisord(lt) {
			diag("case requires ordinal values")
			goto Fail
		}
		if n.right.op!=',' && n.right.op!=Odotdot && !tisord(rt) {
			diag("case requires ordinal values")
			goto Fail
		}
		goto Lchk
	default:
		ss := fmt.Sprintf("tchkbinary: bad op %d", n.op)
		panic(ss)
	}
	return 0
Fail:
	n.stype = Snone
	n.op = 0
	n.ttype = tundef
	return -1
Lchk:
	n.ttype = lt
	if !tcompat(lt, rt, &n.ttype) {
		diag("incompatible argument types (%v and %v) for op '%s'", lt, rt, opname(rune(n.op)))
		goto Fail
	}
	return 0
Bchk:
	if !tcompat(lt, rt, &dummy) {
		diag("incompatible argument types for op '%s'",
			opname(rune(n.op)))
		goto Fail
	}
	n.ttype = tbool
	return 0
}

func tchkcall(fs *Sym, args *List) int {
	var dummy *Type

	if fs.stype==Snone || args==nil {
		return -1
	}
	if fs.stype!=Sproc && fs.stype!=Sfunc {
		diag("'%s' is not a %s or %s.", fs.name, topname[Tproc], topname[Tfunc])
		return -1
	}
	if fs.ttype.op == Tundef {
		return -1
	}

	if fs.ttype.op!=Tproc && fs.ttype.op!=Tfunc {
		diag("'%s' is not a %s or %s.", fs.name, topname[Tproc], topname[Tfunc])
		return -1
	}
	parms := fs.prog.parms
	if parms == nil {
		diag("'%s' not fully defined", fs.name)
		return -1
	}
	if len(parms.item) < len(args.item) {
		diag("too many arguments for '%s'", fs.name)
		return -1
	}
	if len(parms.item) > len(args.item) {
		diag("not enough arguments for '%s'", fs.name)
		return -1
	}
	for i := 0; i < len(parms.item); i++ {
		if !tcompat(parms.getsym(i).ttype, args.getsym(i).ttype, &dummy) {
			diag("argument '%s' for '%s': incompatible types\n"+"\texpected %v\n\tfound %v",
				parms.getsym(i).name, fs.name,
				parms.getsym(i).ttype, args.getsym(i).ttype)
			return -1
		}
		if parms.getsym(i).op==Orefparm && !islval(args.getsym(i)) {
			diag("argument '%s' for '%s': ref requires an l-value\n",
				parms.getsym(i).name, fs.name)
			return -1
		}
	}
	return 0
}

func mktype(name string, t int) *Type {
	s := defsym(name, Stype)
	s.ttype = newtype(t)
	s.ttype.sym = s
	return s.ttype
}

func Typeinit() {
	tundef = newtype(Tundef)
	badnode.ttype = tundef
	tcbool = newtype(Tbool)
	tcchar = newtype(Tchar)
	tcint = newtype(Tint)
	tcreal = newtype(Treal)
	tbool = mktype(topname[Tbool], Tbool)
	tcbool.sym = tbool.sym
	tchar = mktype(topname[Tchar], Tchar)
	tcchar.sym = tchar.sym
	tint = mktype(topname[Tint], Tint)
	tcint.sym = tint.sym
	treal = mktype(topname[Treal], Treal)
	tcreal.sym = treal.sym
	tcnil = mktype("$nil", Tptr)
	tfile = mktype(topname[Tfile], Tfile)
	tstrength = mktype(topname[Tstrength], Tstrength)
	tbutton = mktype(topname[Tbutton], Tbutton)
	topacity = mktype(topname[Topacity], Topacity)
	tcolor = mkenumtype(topname[Tcolor], colnames)
	tsound = mkenumtype(topname[Tsound], sndnames)

}

func mkenumtype(tname string, litnames []string) (t *Type) {
	l := newlist(Lsym)
	for i := range litnames {
		c := newsym(litnames[i], Sconst)
		addsym(l, c)
	}
	t = newordtype(l)
	t.sym = defsym(tname, Stype)
	t.sym.ttype = t
	return t
}

func declstdtypes(tl *List) {
	nms := []string{"bool", "char", "int", "float", "$nil", "file", "strength", "opacity", "color", "button", "sound"}
	tps := []*Type{tcbool, tcchar, tcint, tcreal, tcnil, tfile, tstrength, topacity, tcolor, tbutton, tsound}

	for i := 0; i < len(nms); i++ {
		s := lookup(nms[i], Stype)
		addsym(tl, s)
		s.ttype.id = tgen
		tps[i].id = s.ttype.id
		tgen++
	}
}

func enumname(t *Type, v int) string {
	t = tderef(t)
	if v<0 || v>=len(t.lits.item) {
		return "BAD"
	}
	return t.lits.getsym(v).name
}

func (t *Type) fmtargs() string {
	s := fmt.Sprintf("(")
	pl := t.parms
	for i := 0; i < len(pl.item); i++ {
		if i > 0 {
			s += fmt.Sprintf(", ")
		}
		if pl.getsym(i).op == Orefparm {
			s += fmt.Sprintf("ref ")
		}
		s += fmt.Sprintf("%v", pl.getsym(i).ttype)
	}
	return s + fmt.Sprintf(")")
}

//
// Detect variables used and not set. Must be called for
// each local variable with the body stmt.
//
// sweep x depth-first:
// - if(x is set) skip children & following siblings
// - if(x is used) issue diag and return
//
// return -1: diag made; 1: set by statement; 0: nop
//

const (
	Used = true
	Nop  = false
	Set  = true
)

func isset(lval *Sym, v *Sym) bool {
	if lval == v {
		return Set
	}
	switch lval.stype {
	case Sunary:
		return isset(lval.left, v)
	case Sbinary:
		if lval.op=='.' || lval.op=='[' {
			return isset(lval.left, v)
		}
		return isset(lval.left, v) || isset(lval.right, v)
	}
	return Nop
}

func isused(expr *Sym, v *Sym) bool {
	if expr == v {
		expr.Error("variable '%s' used before set", v.name)
		return Used
	}
	switch expr.stype {
	case Sunary:
		return isused(expr.left, v)
	case Sbinary:
		return isused(expr.left, v) || isused(expr.right, v)
	case Sfcall:
		return setusedfcall(expr, v)
	}
	return Nop
}

func setusedfcall(fcall *Sym, v *Sym) bool {
	if fcall.fsym == nil {
		return Nop
	}
	parms := fcall.fsym.prog.parms
	args := fcall.fargs

	for i := 0; i < len(parms.item); i++ {
		if parms.getsym(i).op == Oparm {
			if isused(args.getsym(i), v) {
				return Used
			}
		}
	}
	for i := 0; i < len(parms.item); i++ {
		if parms.getsym(i).op == Orefparm {
			if isset(args.getsym(i), v) {
				return Set
			}
		}
	}
	return Nop
}

func setslval(x *Stmt, v *Sym) bool {

	if x==nil || v==nil {
		return false
	}
	switch x.op {
	case 0, ';', FCALL, RETURN:

	case '{', ELSE:
		if x.list != nil {
			for i := 0; i < len(x.list.item); i++ {
				if setslval(x.list.getstmt(i), v) {
					return true
				}
			}
		}
	case IF:
		return setslval(x.thenarm, v) || setslval(x.elsearm, v)
	case '=':
		if isset(x.lval, v) {
			x.Error("can't assign to control variable '%s'",
				v.name)
			return true
		}
	case DO:
		return setslval(x.stmt, v)
	case WHILE, FOR:
		/* We assume loops are entered once,
		 * otherwise, initialization for arrays is not detected.
		 */
		return setslval(x.stmt, v)
	default:
		ss := fmt.Sprintf("setslval op %d", x.op)
		panic(ss)
	}
	return false
}

func setused(x *Stmt, v *Sym) bool {
	var c1, c2 bool

	if x==nil || v==nil {
		return Nop
	}
	c := Nop
	switch x.op {
	case 0, ';':

	case '{', ELSE:
		if x.list != nil {
			for i := 0; c==Nop && i<len(x.list.item); i++ {
				c = setused(x.list.getstmt(i), v)
			}
		}
	case IF:
		c = isused(x.cond, v)
		c1 = Nop
		c2 = Nop
		if c == Nop {
			c1 = setused(x.thenarm, v)
		}
		if c == Nop {
			c2 = setused(x.elsearm, v)
		}
		if c1==Used || c2==Used {
			c = Used
		} else if c1==Set && c2==Set {
			c = Set
		}
	case '=':
		c = isused(x.rval, v)
		if c==Nop && isset(x.lval, v) {
			c = Set
		}
	case FCALL:
		c = setusedfcall(x.fcall, v)
	case RETURN:
		c = isused(x.expr, v)
	case DO:
		c = setused(x.stmt, v)
		if c == Nop {
			c = isused(x.expr, v)
		}
	case WHILE, FOR:
		/* We assume loops are entered once,
		 * otherwise, initialization for arrays is not detected.
		 */
		c = isused(x.expr, v)
		if c == Nop {
			c = setused(x.stmt, v)
		}
	default:
		ss := fmt.Sprintf("setused op %d", x.op)
		panic(ss)
	}
	return c
}

func returnsok(x *Stmt, rt *Type) int {
	var dummy *Type

	if x==nil || rt==nil || rt==tundef {
		return -1
	}
	switch x.op {
	case '{', ELSE:
		if x.list==nil || len(x.list.item)==0 {
			return -1
		}
		return returnsok(x.list.getstmt(len(x.list.item)-1), rt)
	case IF:
		if x.elsearm == nil {
			return -1
		}
		n1 := returnsok(x.thenarm, rt)
		if n1 < 0 {
			return -1
		}
		n2 := returnsok(x.elsearm, rt)
		if n2 < 0 {
			return -1
		}
		return n1 + n2
	case RETURN:
		if !tcompat(rt, x.expr.ttype, &dummy) {
			diag("return value of incompatible type\n"+"\texpected %v\n\tfound %v", rt, x.expr.ttype)
			return -1
		}
		return 1
	}
	return -1
}

func firstret(x *Stmt) *Stmt {
	if x == nil {
		return nil
	}
	switch x.op {
	case IF:
		xt := firstret(x.thenarm)
		if xt!=nil || x.elsearm==nil {
			return xt
		}
		xt = firstret(x.elsearm)
		return xt
	case RETURN:
		return x
	case '{', ELSE:
		if x.list==nil || len(x.list.item)==0 {
			return nil
		}
		for i := 0; i < len(x.list.item); i++ {
			xt := firstret(x.list.getstmt(i))
			if xt != nil {
				return xt
			}
		}
	default:
		if x.list==nil || len(x.list.item)==0 {
			return nil
		}
		return firstret(x.list.getstmt(0))
	}
	return nil
}

func (t *Type) fstring(ishash bool) string {
	if t == nil {
		return fmt.Sprintf("<nilt>")
	}
	if t.sym!=nil && !ishash {
		return fmt.Sprintf("%s", t.sym.name)
	}
	if t.op >= len(topname) {
		serr := fmt.Sprintf("bug: Type fmt: type op %d", t.op)
		panic(serr)
	}
	xs := fmt.Sprintf("%s:%d", topname[t.op], int(t.id))
	switch t.op {
	case Tundef, Tint, Tbool, Tchar, Treal, Tenum, Tprog, Tfwd, Tfile:
		return xs
	case Trange, Tcolor, Tstrength, Topacity, Tbutton, Tsound:
		if t.super.Tis(Tchar) {
			xs += fmt.Sprintf(" '%c'..'%c'", byte(t.first), byte(t.last))
		} else if t.super.Tis(Tenum) {
			xs += fmt.Sprintf(" %s..%s", enumname(t.super, t.first), enumname(t.super, t.last))
		} else {
			xs += fmt.Sprintf(" %d..%d", t.first, t.last)
		}
		return xs + fmt.Sprintf(" of %v", t.super)
	case Tarry:
		return xs + fmt.Sprintf(" [%v] of %v", t.idx, t.elem)
	case Tstr:
		return xs + fmt.Sprintf(" [0..%d] of %v", t.last, t.elem)
	case Trec:
		xs += fmt.Sprintf("{")
		if t.fields != nil {
			for i := 0; i < len(t.fields.item); i++ {
				if i > 0 {
					xs += fmt.Sprintf(" ")
				}
				s := t.fields.getsym(i).swfield
				sv := t.fields.getsym(i).swval
				if s!=nil && sv!=nil {
					xs += fmt.Sprintf("(%s=%s){", s.name, sv.name)
				}
				xs += fmt.Sprintf("%v;", t.fields.getsym(i))
				if s!=nil && sv!=nil {
					xs += fmt.Sprintf("}")
				}
			}
		}
		return xs + fmt.Sprintf("}")
	case Tptr:
		return xs + fmt.Sprintf(" to %v", t.ref)
	case Tproc:
		return xs + t.fmtargs()
	case Tfunc:
		return xs + t.fmtargs()
		return xs + fmt.Sprintf(":%v", t.rtype)
	default:
		fmt.Fprintf(os.Stderr, "Tfmt BUG: op=%d\n", t.op)
		return xs + fmt.Sprintf("TBUG(%d)", t.op)
	}
	return xs
}

func (t *Type) String() string {
	return t.fstring(false)
}
func (t *Type) GoString() string {
	return t.fstring(true)
}

//
// Checking undefined types is tricky, because they may have
// inner components undefined and it would be easy to cross a nil
// pointer due to a bug in the compiler.
// By now, we check the only case that can happen in theory,
// which is forward references for pointer types.
//
func hasundefs(t *Type) bool {
	if t == nil {
		return true
	}

	switch t.op {
	case Tfwd, Tundef:
		return true
	case Tptr:
		if t.ref != nil {
			if t.ref.op==Tundef || t.ref.op==Tfwd {
				return true
			}
		}
		break
	}
	return false
}

func Checkundefs() {
	if Nerrors > 0 {
		return
	}
	if env==nil || env.prog==nil || env.prog.prog==nil {
		panic("checkundefs")
	}
	p := env.prog.prog

	tl := p.types
	if tl.kind != Lsym {
		panic("tl.kind != Lsym")
	}
	for i := 0; i < len(tl.item); i++ {
		t := tl.getsym(i).ttype
		if hasundefs(t) {
			tl.getsym(i).Error("undefined type\n")
		}
	}
}
