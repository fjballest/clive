package comp

import (
	"bufio"
	"clive/cmd/picky/paminstr"
	"fmt"
	"math"
	"os"
	"unicode/utf8"
)

//
// p.out header, followed by:
// - symbol table
// - pc/sp table
// - pc/line table
// - constant and values
// - code
//

type Hdr  {
	entry  *Sym
	ntypes uint32
	nobjs  uint32
	npcs   uint32
	nprocs uint32
	ntext  uint32
}

type Ldef  {
	addr uint32
	uses *Luse
}

type Luse  {
	next *Luse
	code *Code
	np   uint32
}

var (
	hdr     Hdr
	Oname   string
	out     *bufio.Writer
	addr    uint32
	paddr   uint32
	xaddr   uint32
	xprocid uint32
	labels  []Ldef
	nlabels int
	xcode   *Code
	opcodes = map[rune]uint{
		'+':     paminstr.ICadd,
		'-':     paminstr.ICsub,
		'*':     paminstr.ICmul,
		'/':     paminstr.ICdiv,
		'%':     paminstr.ICmod,
		'<':     paminstr.IClt,
		'>':     paminstr.ICgt,
		'=':     paminstr.ICsto,
		'[':     paminstr.ICidx,
		'^':     paminstr.ICindir,
		Ole:     paminstr.ICle,
		Oge:     paminstr.ICge,
		Oand:    paminstr.ICand,
		Oor:     paminstr.ICor,
		Oeq:     paminstr.ICeq,
		One:     paminstr.ICne,
		Opow:    paminstr.ICpow,
		Oint:    paminstr.ICdata,
		Onil:    paminstr.ICdata,
		Ochar:   paminstr.ICdata,
		Oreal:   paminstr.ICdata,
		Ostr:    paminstr.ICdata,
		Otrue:   paminstr.ICdata,
		Ofalse:  paminstr.ICdata,
		Onot:    paminstr.ICnot,
		Olit:    paminstr.ICdata,
		Ocast:   paminstr.ICcast,
		Ouminus: paminstr.ICminus,
		Oaggr:   paminstr.ICdata,
	}
)

func mklbl(lp *int) int {
	l := new(Ldef)
	l.addr = xaddr
	if nlabels%Aincr == 0 {
		labels = append(labels, make([]Ldef, Aincr)...)
	}
	labels[nlabels] = *l
	*lp = nlabels
	nlabels++
	return *lp
}

var (
	els  []Luse
	nels int
)

func uselbl(ln int) uint32 {
	l := &labels[ln]
	if nels == 0 {
		els = make([]Luse, Aincr)
		nels = Aincr
	}
	nels--
	u := &els[nels]
	u.code = xcode
	u.np = uint32(xcode.np)
	u.next = l.uses
	l.uses = u
	return l.addr
}

func setlbl(ln int, addr uint32) {
	l := &labels[ln]
	for u := l.uses; u != nil; u = u.next {
		u.code.p[u.np] = addr
	}
}

var (
	pcels  []Pcent
	pcnels int
)

func allocpc() *Pcent {
	if pcnels == 0 {
		pcnels = Aincr
		pcels = make([]Pcent, Aincr)
	}
	e := &pcels[pcnels-1]
	pcnels--
	return e
}

var (
	sfname string
	llno   uint32
)

func addpc(s *Stmt) {
	e := allocpc()
	e.st = s
	e.pc = uint(xaddr)
	if xcode.pcs == nil {
		xcode.pcs = e
	} else {
		xcode.pcstl.next = e
	}
	xcode.pcstl = e
	sn, lfn := s.FuncPos()
	if (sfname=="" || lfn!=int(llno)) && sn==sfname {
		hdr.npcs++
	}
	sfname = sn
	llno = uint32(lfn)
}

func addnode(nd *Sym) {
	e := allocpc()
	e.nd = nd
	e.pc = uint(xaddr)
	if xcode.pcs == nil {
		xcode.pcs = e
	} else {
		xcode.pcstl.next = e
	}
	xcode.pcstl = e
}

func oprint(f string, arg ...interface{}) {
	if _, err := fmt.Fprintf(out, f, arg...); err != nil {
		s := fmt.Sprintf("%s: %v", Oname, err)
		panic(s)
	}
}

func genentry(s *Sym) {
	hdr.entry = s
}

func emithdr() {
	oprint("#!/bin/pam\n")
	oprint("entry %d\n", hdr.entry.id)
}

func genconst(s *Sym) {
	s.addr = uint(addr)
	if s.ttype.Tis(Tstr) {
		addr += uint32(tcchar.sz)*uint32(utf8.RuneCountInString(s.sval))
	} else {
		addr += uint32(s.ttype.Tsz())
	}
	hdr.nobjs++
}

//
// BUG: There's a possible source of bugs here.
// For records and the like, we emitconst() if we don't have a name
// or an address, but it should be done depending on the symbol type,
// and not on which name or address it has.
// This must be reworked.
//
func emitconst(s *Sym) {
	var (
		sname string
	)

	if s.ttype.id >= uint(hdr.ntypes) {
		es := fmt.Sprintf("emitconst: tid for %v not defined", s.ttype)
		panic(es)
	}
	if s.name == "" {
		sname = "_"
	} else {
		sname = s.name
	}
	oprint("%s %d %#x", sname, s.ttype.id, s.addr)
	if s.ttype.Tis(Treal) {
		oprint(" %e", s.rval)
	} else if s.ttype.Tis(Trec) || s.ttype.Tis(Tarry) || s.ttype.Tis(Tstr) {
		if s.ttype.Tis(Tstr) {
			fs := fmt.Sprintf(" %s", CEscape(s.sval))
			oprint("%s", fs)
		} else {
			fn, ln := s.FuncPos()
			fs := fmt.Sprintf(" %d %s %d\n", len(s.vals.item), CEscape(fn), ln)
			oprint(fs)
			emitlist := func(ss *Sym) {
				if ss.name=="" || ss.addr==0 {
					emitconst(ss)
				} else {
					oprint("%s %d %#x\n", ss.name, ss.ttype.id, ss.addr)
				}
			}
			mapl(s.vals, emitlist)
			return
		}
	} else {
		oprint(" %d", s.ival)
	}

	fn, ln := s.FuncPos()
	fs := fmt.Sprintf(" %s %d\n", CEscape(fn), ln)
	oprint(fs)
}

func genvar(s *Sym) {
	genconst(s)
}

func genlvar(s *Sym) {
	s.addr = uint(paddr)
	paddr += uint32(s.ttype.Tsz())
	if uint32(s.addr) >= paddr {
		panic("s.addr >= paddr")
	}
}

func emitvar(s *Sym) {
	oprint("%s %d %#x -", s.name, s.ttype.id, s.addr)
	fn, ln := s.FuncPos()
	fs := fmt.Sprintf(" %s %d\n", CEscape(fn), ln)
	oprint(fs)
}

func genparm(s *Sym) {
	s.addr = uint(paddr)
	if s.op == Orefparm {
		paddr += 8
	} else {
		paddr += uint32(s.ttype.Tsz())
	}
}

type genType Type

func (t *Type) fmt() rune {
	tt := tderef(t)
	switch tt.op {
	case Tbool, Tchar, Tint, Tenum, Tfile, Tptr, Tarry, Tstr:
		r, _ := utf8.DecodeRuneInString(topname[tt.op])
		return r
	case Tbutton:
		return 'u'
	case Tstrength:
		return 'h'
	case Topacity:
		return 'l'
	case Treal:
		return 'r'
	case Trec:
		return 'R'
	case Tproc:
		return 'X'
	case Tfunc:
		return 'F'
	default:
		s := fmt.Sprintf("tfmt bug: op %d", tt.op)
		panic(s)
	}
	return '?'
}

func emitfield(s *Sym) {
	oprint("%s %d %#x\n", s.name, s.ttype.id, s.addr)
}

func emittype(s *Sym) {
	t := s.ttype
	c := t.fmt()
	if t == nil {
		t = tundef //cannot use tderef
	}
	oprint("%d %s %c", s.ttype.id, s.name, c)
	st := tderef(t)
	fst, lst, ttlen := t.Trange()
	oprint("  %d %d %d %d", fst, lst, ttlen, st.Tsz())
	switch t.op {
	case Tarry, Tstr:
		oprint(" %d\n", st.elem.id)
		break
	case Tenum:
		oprint(" 0\n")
		emitname := func(ss *Sym) {
			oprint("%s\n", ss.name)
		}
		mapl(t.lits, emitname)
	case Trec:
		oprint(" 0\n")
		mapl(st.fields, emitfield)
	case Tptr:
		if st.ref == nil {
			oprint(" 0\n")
		} else {
			oprint(" %d\n", st.ref.id)
		}
	case Trange:
		oprint(" 0\n")
		if c == 'e' {
			for i := fst; i <= lst; i++ {
				oprint("%s\n", st.lits.getsym(i).name)
			}
		}
	default:
		oprint(" 0\n")
	}
}

var lgen int

func genlit(n *Sym) {
	nm := fmt.Sprintf("$lit%d", lgen)
	lgen++
	cs := defsym(nm, Sconst)
	declconst(cs, n)
	n.addr = uint(addr)
	addr += uint32(n.ttype.Tsz())
	hdr.nobjs++
}

func emit32(o uint32) {
	if xcode.np%Aincr == 0 {
		xcode.ap += Aincr
		xcode.p = append(xcode.p, make([]uint32, Aincr)...)
	}
	xcode.p[xcode.np] = o
	xcode.np++
	xaddr++
}

//BUG!!!, needs fixing, also float != float32
func emitr(r float32) {
	emit32(math.Float32bits(r))
}

func emitda(addr uint64) {
	emit32(uint32(addr&0xFFFFFFFF))
	emit32(uint32(addr>>32))
}

func genjmp(op int, l int) {
	emit32(uint32(op))
	emit32(uselbl(l))
}

func genop(op int, arg uint32) {
	emit32(uint32(op))
	if paminstr.Hasarg(uint32(op)) {
		emit32(arg)
	}
}

func genlval(nd *Sym) {
	addr := uint32(nd.addr)
	switch nd.stype {
	case Svar:
		addnode(nd)
		switch nd.op {
		case Orefparm:
			genop(paminstr.ICarg, addr)
			genop(paminstr.ICindir, 8) // de-reference */
			//
			// XXX: generate checks for ranges.
			//
		case Oparm:
			genop(paminstr.ICarg, addr)
		case Olvar:
			genop(paminstr.IClvar, addr)
		default:
			genop(paminstr.ICdaddr, addr)
		}
		break
	case Sconst:
		addnode(nd)
		if nd.op!=Ostr && nd.op!=Oaggr {
			es := fmt.Sprintf("genlval: const: op %d", nd.op)
			panic(es)
		}
		genop(paminstr.ICdaddr, addr)
	case Sunary:
		addnode(nd)
		switch nd.op {
		case '^':
			genexpr(nd.left)
			genop(paminstr.ICptr, 0)
		default:
			es := fmt.Sprintf("genlval: unary: op %d", nd.op)
			panic(es)
		}
	case Sbinary:
		addnode(nd)
		switch nd.op {
		case '.':
			genlval(nd.rec)
			if nd.field.addr != 0 {
				genop(paminstr.ICfld, uint32(nd.field.addr))
			}
		case '[':
			genexpr(nd.right)
			genlval(nd.left)
			genop(paminstr.ICidx, uint32(nd.left.ttype.id))
		default:
			es := fmt.Sprintf("genlval: binary: op %d", nd.op)
			panic(es)
		}
	default:
		es := fmt.Sprintf("genlval: stype %d", nd.stype)
		panic(es)
	}
}

func genunary(nd *Sym) {
	switch nd.op {
	case '+':
		genexpr(nd.left)
	case Ouminus, Onot:
		genexpr(nd.left)
		if nd.left.ttype.Tis(Treal) {
			emit32(uint32(opcodes[rune(nd.op)]) | paminstr.ITreal)
		} else {
			emit32(uint32(opcodes[rune(nd.op)]))
		}
	case Ocast:
		genexpr(nd.left)
		if nd.left.ttype.Tis(Treal) {
			genop(int(opcodes[rune(nd.op)]|paminstr.ITreal), uint32(nd.ttype.id))
		} else {
			genop(int(opcodes[rune(nd.op)]), uint32(nd.ttype.id))
		}
	case '^':
		genexpr(nd.left)
		genop(paminstr.ICptr, 0)
		genop(paminstr.ICindir, uint32(nd.left.ttype.ref.Tsz()))
	default:
		es := fmt.Sprintf("bad unary op %d", nd.op)
		panic(es)
	}
}

func genbinary(nd *Sym) {
	switch nd.op {
	case '+', '-', '*', '/', '%', Opow, '<', '>', Ole, Oge, Oand, Oor:
		genexpr(nd.right)
		genexpr(nd.left)
		if nd.left.ttype.Tis(Treal) {
			genop(int(opcodes[rune(nd.op)]|paminstr.ITreal), 0)
		} else {
			genop(int(opcodes[rune(nd.op)]), 0)
		}
	case Oeq, One:
		if tisatom(nd.left.ttype) {
			genexpr(nd.right)
			genexpr(nd.left)
			if tisord(nd.left.ttype) {
				genop(int(opcodes[rune(nd.op)]), 0)
			} else if nd.left.ttype.Tis(Tptr) {
				genop(int(opcodes[rune(nd.op)]|paminstr.ITaddr), 0)
			} else {
				genop(int(opcodes[rune(nd.op)]|paminstr.ITreal), 0)
			}
		} else {
			genlval(nd.left)
			genlval(nd.right)
			if nd.op == Oeq {
				genop(paminstr.ICeqm, uint32(nd.left.ttype.Tsz()))
			} else {
				genop(paminstr.ICnem, uint32(nd.left.ttype.Tsz()))
			}
		}
	case '.', '[':
		genlval(nd)
		genop(paminstr.ICindir, uint32(nd.ttype.Tsz()))
	default:
		es := fmt.Sprintf("tchkbinary: bad op %d", nd.op)
		panic(es)
	}
}

func gencall(n *Sym) {
	f := n.fsym
	args := n.fargs
	parms := f.prog.parms
	for i := len(args.item) - 1; i >= 0; i-- {
		arg := args.getsym(i)
		if i >= len(parms.item) {
			/* should never arrive here if tchkcall is ok */
			arg.Error("too many arguments")
			panic("too many arguments")
			return
		}
		parm := parms.getsym(i)
		if parm.op == Orefparm {
			genlval(arg)
		} else {
			genexpr(arg)
			//
			// XXX: generate checks for ranges.
			//
		}
	}
	switch f.id {
	case paminstr.PAMbuiltin | paminstr.PBfread, paminstr.PAMbuiltin | paminstr.PBfreadln, paminstr.PAMbuiltin | paminstr.PBfwrite:
		genop(paminstr.ICpush, uint32(args.getsym(1).ttype.id))
	case paminstr.PAMbuiltin | paminstr.PBfwriteln, paminstr.PAMbuiltin | paminstr.PBgopen, paminstr.PAMbuiltin | paminstr.PBgkeypress:
		genop(paminstr.ICpush, uint32(args.getsym(1).ttype.id))
	case paminstr.PAMbuiltin | paminstr.PBopen:
		genop(paminstr.ICpush, uint32(args.getsym(2).ttype.id))
		genop(paminstr.ICpush, uint32(args.getsym(1).ttype.id))
	case paminstr.PAMbuiltin | paminstr.PBfatal:
		genop(paminstr.ICpush, uint32(args.getsym(0).ttype.id))
	case paminstr.PAMbuiltin | paminstr.PBnew:
		genop(paminstr.ICpush, uint32(args.getsym(0).ttype.id))
	}
	addnode(n)
	genop(paminstr.ICcall, uint32(f.id))
}

func genexpr(nd *Sym) {
	switch nd.stype {
	case Sfcall:
		gencall(nd)
	case Sconst:
		addnode(nd)
		switch nd.op {
		case Ochar, Oint, Otrue, Ofalse, Olit:
			genop(paminstr.ICpush, uint32(nd.ival))
		case Onil:
			genop(paminstr.ICdata, 8)
			emitda(uint64(0))
		case Oreal:
			emit32(paminstr.ICpush | paminstr.ITreal)
			emitr(float32(nd.rval))
		case Ostr, Oaggr:
			genop(paminstr.ICdaddr, uint32(nd.addr))
			genop(paminstr.ICindir, uint32(nd.ttype.Tsz()))
		default:
			es := fmt.Sprintf("genexpr: Sconst: op %d", nd.op)
			panic(es)
		}
		break
	case Svar:
		genlval(nd)
		genop(paminstr.ICindir, uint32(nd.ttype.Tsz()))
	case Sunary:
		genunary(nd)
	case Sbinary:
		genbinary(nd)
	default:
		es := fmt.Sprintf("genexpr: stype %d", nd.stype)
		panic(es)
	}
}

func gencode(x *Stmt) {
	var lbl, elbl, saved int
	addpc(x)
	switch x.op {
	case ';':

	case DO:
		mklbl(&lbl)
		setlbl(lbl, xaddr)
		gencode(x.stmt)
		genexpr(x.expr)
		genjmp(paminstr.ICjmpt, lbl)
	case WHILE, FOR:
		mklbl(&lbl)
		setlbl(lbl, xaddr)
		genexpr(x.expr)
		genjmp(paminstr.ICjmpf, mklbl(&elbl))
		gencode(x.stmt)
		if x.incr != nil { // FOR */
			if x.expr.op==Oge || x.expr.op==Ole {
				saved = x.expr.op
				x.expr.op = Oeq
				genexpr(x.expr)
				x.expr.op = saved
				genjmp(paminstr.ICjmpt, elbl)
			}
			gencode(x.incr)
		}
		genjmp(paminstr.ICjmp, lbl)
		setlbl(elbl, xaddr)
	case '{', ELSE:
		mapxl(x.list, gencode)
	case FCALL:
		gencall(x.fcall)
	case '=':
		if islval(x.rval) {
			genlval(x.rval)
			genlval(x.lval)
			genop(paminstr.ICstom, uint32(x.lval.ttype.id))
		} else {
			genexpr(x.rval)
			genlval(x.lval)
			genop(paminstr.ICsto, uint32(x.lval.ttype.id))
		}
	case IF:
		genexpr(x.cond)
		genjmp(paminstr.ICjmpf, mklbl(&lbl))
		gencode(x.thenarm)
		if x.elsearm != nil {
			genjmp(paminstr.ICjmp, mklbl(&elbl))
		}
		setlbl(lbl, xaddr)
		if x.elsearm != nil {
			gencode(x.elsearm)
			setlbl(elbl, xaddr)
		}
	case RETURN:
		if x.expr != nil {
			genexpr(x.expr)
		}
		genop(paminstr.ICret, xprocid)
	case SWITCH, CASE:
		panic("gencode: FOR/SWpaminstr.ITCH/CASE must be handled by frontend")
	default:
		es := fmt.Sprintf("gencode: op %d", x.op)
		panic(es)
	}
}

type genStmt Stmt

func emitcode(c *Code) {

	var arg uint32
	addr := c.addr
	e := c.pcs
	for i := uint32(0); i < uint32(c.np); i++ {
		for ; e!=nil && uint32(e.pc)<=addr+i && e.st!=nil; e = e.next {
			oprint("# %#v\n", (*genStmt)(e.st))
		}
		oprint("%05x\t%v", addr+i, paminstr.Instr(c.p[i]))
		ir := c.p[i]
		ic := paminstr.IC(ir)
		if paminstr.Hasarg(c.p[i]) {
			i++
			arg = c.p[i]
			if paminstr.IT(ir)==paminstr.ITreal && ic!=paminstr.ICcast {
				oprint("\t%e", math.Float32frombits(arg))
			} else {
				oprint("\t%#010x", arg)
			}
		} else {
			oprint("\t")
		}
		first := 0
		for ; e!=nil && uint32(e.pc)<=addr+i && e.nd!=nil; e = e.next {
			if e.nd != nil {
				if first == 0 {
					oprint("\t#")
				}
				first++
				oprint(" %#v;", e.nd)
			}
		}
		oprint("\n")
		if ic == paminstr.ICdata {
			for ; arg > 0; arg -= 4 {
				i++
				oprint("%05x\t%#x\n", addr+i, c.p[i])
			}
		}
	}
}

func genproc(s *Sym) {
	p := s.prog
	hdr.nprocs++
	s.addr = uint(xaddr)
	xcode = &p.code
	xprocid = uint32(s.id)
	xcode.addr = xaddr
	paddr = 0
	maprl(p.parms, genparm)
	p.parmsz = uint(paddr)
	paddr = 0
	mapl(p.vars, genlvar)
	p.varsz = uint(paddr)
	xl := p.stmt.list
	if xl.getstmt(len(xl.item)-1).op != RETURN {
		addstmt(xl, newstmt(RETURN))
	}
	gencode(p.stmt)
}

func emitproc(s *Sym) {
	p := s.prog
	if p==nil || p.parms==nil || p.vars==nil {
		return
	}
	oprint("%d %s %#05x", s.id, s.name, s.addr)
	oprint(" %d %d %d", len(p.parms.item), len(p.vars.item), p.rtype.Tsz())
	oprint(" %d %d", p.parmsz, p.varsz)
	fn, ln := s.FuncPos()
	fs := fmt.Sprintf(" %s %d\n", CEscape(fn), ln)
	oprint(fs)
	mapl(p.parms, emitvar)
	mapl(p.vars, emitvar)
}

func emittext(s *Sym) {
	p := s.prog
	if p==nil || p.parms==nil || p.vars==nil {
		return
	}
	xcode = &p.code
	oprint("# %s()\n", s.name)
	emitcode(xcode)
}

func emitpcs(s *Sym) {

	sfname := ""
	llno := 0
	for pc := s.prog.code.pcs; pc != nil; pc = pc.next {
		st := pc.st
		if st == nil {
			continue
		}
		sn, ln := st.FuncPos()
		if sfname!="" && sn==sfname && ln==llno {
			continue
		}
		sfname = sn
		llno = ln
		fs := fmt.Sprintf("%05x\t%s\t%d\n", pc.pc, CEscape(sfname), llno)
		oprint(fs)
	}
}

func Gen(bout *bufio.Writer, nm string) {
	Oname = nm
	out = bout
	s := lookup("main", Sproc)
	if s==nil || s.prog==nil || s.stype!=Sproc {
		panic("missing declaration of procedure 'main'")
	}
	if s.prog.parms!=nil && len(s.prog.parms.item)>0 {
		panic("procedure 'main' may not have parameters")
	}
	genentry(s)
	if env.prog == nil {
		panic("missing program")
	}
	p := env.prog.prog
	hdr.ntypes = uint32(tgen)
	if p.consts != nil {
		mapl(p.consts, genconst)
	}
	if p.vars != nil {
		mapl(p.vars, genvar)
	}
	if p.procs != nil {
		mapl(p.procs, genproc)
	}
	hdr.ntext = xaddr
	emithdr()
	oprint("types %d\n", hdr.ntypes)

	mapl(p.types, emittype)
	oprint("vars %d\n", hdr.nobjs)
	if p.consts != nil {
		mapl(p.consts, emitconst)
	}
	if p.vars != nil {
		mapl(p.vars, emitvar)
	}
	oprint("procs %d\n", hdr.nprocs)
	mapl(p.procs, emitproc)
	oprint("text %d\n", hdr.ntext)
	mapl(p.procs, emittext)
	oprint("pcs %d\n", hdr.npcs+1)
	mapl(p.procs, emitpcs)
	bout.Flush()
}

func (x *genStmt) GoString() string {
	var s string
	if false && x!=nil {
		sn, ln := (*Stmt)(x).FuncPos()
		s += fmt.Sprintf("%s:%d", CEscape(sn), ln)
	}
	if x == nil {
		return fmt.Sprintf("<nilstmt>")
	}
	switch x.op {
	case DO:
		s = fmt.Sprintf("dowhile(%v)", x.expr)
	case FOR:
		s = fmt.Sprintf("for(%v)", x.expr)
	case WHILE:
		s = fmt.Sprintf("while(%v)", x.expr)
	case CASE:
		s = fmt.Sprintf("case %v", x.expr)
	case '{':
		s = fmt.Sprintf("{...}")
	case FCALL:
		if x.op != FCALL {
			panic("x.op == FCALL")
		}
		s = fmt.Sprintf("%v", x.fcall)
	case '=':
		s = fmt.Sprintf("%v = %v", x.lval, x.rval)
	case IF:
		s = fmt.Sprintf("if(%v)", x.cond)
	case ELSE:
		s = fmt.Sprintf("else")
	case ';':
		s = fmt.Sprintf("nop")
	case 0:
		s = fmt.Sprintf("undef")
	case RETURN:
		s = fmt.Sprintf("return %v", x.expr)
	default:
		fmt.Fprintf(os.Stderr, "Stmt fmt called with op %d\n", x.op)
		s = fmt.Sprintf("RBUG(%d)", x.op)
	}
	return s
}
