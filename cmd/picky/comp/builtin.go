package comp

import (
	"clive/cmd/picky/paminstr"
	"fmt"
	"math"
)

var (
	/*
	 * u -> button
	 * b -> bool
	 * i -> int
	 * r -> real
	 * f -> file
	 * h -> strength
	 * l -> opacity
	 * p -> pointer to any
	 * s -> array[int] of char
	 * w -> sound
	 * c -> character
	 * o -> ordinal
	 * v -> lvalue
	 * n -> node
	 * Upcase means by ref.
	 */
	barg = map[rune]int{
		'b': Tbool,
		'B': Tbool,
		'c': Tchar,
		'C': Tchar,
		'f': Tfile,
		'F': Tfile,
		'p': Tptr,
		'P': Tptr,
		'r': Treal,
		'R': Treal,
		'h': Tstrength,
		'l': Topacity,
		'i': Tint,
		'u': Tbutton,
	}
	bpred, bsucc     *Sym
	bargref, bargval *Sym
)

func xrealfn(b *Builtin, al *List, fn func(float64) float64) *Sym {
	var (
		n    *Sym
		rval float64
	)

	if evaluated(al.getsym(0)) {
		rval = al.getsym(0).rval
		if (b.name == "log" || b.name == "log10") && rval <= 0 {
			diag("%s requires argument > 0", b.name)
			return nil
		}
		if b.name == "sqrt" && rval < 0 {
			diag("%s requires argument >= 0", b.name)
			return nil
		}
		if (b.name == "asin" || b.name == "acos") && (rval < -1+paminstr.Eps || rval > 1-paminstr.Eps) {
			diag("%s requires argument in [-1.0,1.0]", b.name)
			return nil
		}
		if b.name == "tan" && math.Cos(rval) < paminstr.Eps {
			diag("value out of domain of %s", b.name)
			return nil
		}
		if b.name == "atan" && (rval < -math.Pi/2+paminstr.Eps || rval > math.Pi/2-paminstr.Eps) {
			diag("%s requires argument in [-Pi/2, Pi/2]", b.name)
			return nil
		}

		n = newreal(fn(rval), al.getsym(0).ttype)
		return n
	}
	return nil
}

func xreal2fn(b *Builtin, al *List, fn func(float64, float64) float64) *Sym {
	if evaluated(al.getsym(0)) && evaluated(al.getsym(1)) {
		r := fn(al.getsym(0).rval, al.getsym(1).rval)
		return newreal(r, al.getsym(0).ttype)
	}
	return nil
}

func xsin(b *Builtin, al *List) *Sym {
	return xrealfn(b, al, math.Sin)
}

func xcos(b *Builtin, al *List) *Sym {
	return xrealfn(b, al, math.Cos)
}

func xtan(b *Builtin, al *List) *Sym {
	return xrealfn(b, al, math.Tan)
}

func xasin(b *Builtin, al *List) *Sym {
	return xrealfn(b, al, math.Asin)
}

func xacos(b *Builtin, al *List) *Sym {
	return xrealfn(b, al, math.Acos)
}

func xatan(b *Builtin, al *List) *Sym {
	return xrealfn(b, al, math.Atan)
}

func xexp(b *Builtin, al *List) *Sym {
	return xrealfn(b, al, math.Exp)
}

func xlog(b *Builtin, al *List) *Sym {
	return xrealfn(b, al, math.Log)
}

func xlog10(b *Builtin, al *List) *Sym {
	return xrealfn(b, al, math.Log10)
}

func xsqrt(b *Builtin, al *List) *Sym {

	return xrealfn(b, al, math.Sqrt)
}

func xpow(b *Builtin, al *List) *Sym {

	return xreal2fn(b, al, math.Pow)
}

func xatruntime(b *Builtin, al *List) *Sym {
	return nil
}

func xpredsucc(b *Builtin, al *List, inc int) *Sym {
	arg := al.getsym(0)
	if evaluated(arg) {
		n := newint(arg.ival+inc, Oint, arg.ttype)
		if n.ival < tfirst(n.ttype) || n.ival > tlast(n.ttype) {
			diag("value out of range in call to pred/succ")
		}
		return n
	}
	return nil
}

func xpred(b *Builtin, al *List) *Sym {
	return xpredsucc(b, al, -1)
}

func xsucc(b *Builtin, al *List) *Sym {
	return xpredsucc(b, al, +1)
}

func checkfargs(al *List) {
	if len(al.item) < 2 {
		return
	}
	s := al.getsym(1)
	t := tderef(s.ttype)
	switch t.op {
	case Tarry:
		if t.elem.Tis(Tchar) {
			return
		}
		fallthrough
	case Tptr, Tfile, Trec:
		diag("%ss cannot be used in read or write", topname[t.op])
		break
	}
}

func xstdio(b *Builtin, al *List) *Sym {

	switch b.args[0] {
	case '<':
		addsym0(al, pstdin)
		break
	case '>':
		addsym0(al, pstdout)
		break
	default:
		return nil
	}
	n := allocsym()
	n.stype = Sfcall
	n.ttype = brtype(b, al)
	n.fargs = al
	switch b.id {
	case paminstr.PBfeof:
		n.fsym = lookup("feof", Snone)
		break
	case paminstr.PBfeol:
		n.fsym = lookup("feol", Snone)
		break
	case paminstr.PBfpeek:
		n.fsym = lookup("fpeek", Snone)
		break
	case paminstr.PBfread:
		checkfargs(al)
		n.fsym = lookup("fread", Snone)
		break
	case paminstr.PBfreadln:
		checkfargs(al)
		n.fsym = lookup("freadln", Snone)
		break
	case paminstr.PBfreadeol:
		n.fsym = lookup("freadeol", Snone)
		break
	case paminstr.PBfwrite:
		checkfargs(al)
		n.fsym = lookup("fwrite", Snone)
		break
	case paminstr.PBfwriteln:
		checkfargs(al)
		n.fsym = lookup("fwriteln", Snone)
		break
	case paminstr.PBfwriteeol:
		n.fsym = lookup("fwriteeol", Snone)
		break
	case paminstr.PBfflush:
		n.fsym = lookup("fflush", Snone)
		break
	default:
		s := fmt.Sprintf("file builtin bug: id %#ux", b.id)
		panic(s)
	}
	return n
}

func diagtypes(s string, t1 *Type, top int) {
	diag("incompatible argument in call '%s'\n\t"+"got %v; need %s", s, t1, topname[top])
}

func mkbtype(b *Builtin) *Type {
	var t *Type

	if b.kind == Sproc {
		t = newtype(Tproc)
	} else {
		t = newtype(Tfunc)
	}
	if b.r != 0 {
		if b.r == 'o' {
			t.rtype = tundef /* ordinal actually */
		} else {
			t.rtype = newtype(barg[b.r])
		}
	}

	//
	// result and argument types are not built for type checks
	// they have exceptions and they are checked by bargcheck().
	// Parameters are built only to know if they are ref or not,
	// for setused().
	//
	t.parms = newlist(Lsym)
	for _, r := range b.args {
		if r >= 'A' && r <= 'Z' {
			addsym(t.parms, bargref)
		} else {
			addsym(t.parms, bargval)
		}
	}
	return t
}

//
// Generic argument check for builtins. see barg[]
// There are extra rguments, > and < for stdin and stdout
// s is string, o is ordinal
func bargcheck(b *Builtin, al *List, as string) int {
	var (
		i   int
		asv rune
	)

	if len(as) > 0 && (as[0] == '<' || as[0] == '>') {
		if len(as) > 1 {
			as = as[1:]
		} else {
			as = ""
		}
	}

	for i = 0; i < len(as); i++ {
		asv = rune(as[i])
		if i >= len(al.item) {
			break
		}
		n := al.getsym(i)
		n = oksym(n)
		if n.ttype == tundef {
			continue
		}
		switch asv {
		case 's', 'S':
			t := tderef(n.ttype)
			if (t.op != Tstr && t.op != Tarry) || !t.idx.Tis(Tint) || !t.elem.Tis(Tchar) {
				diag("argument to '%s' is not a string", b.name)
				return -1
			}
		case 'o', 'O':
			if !tisord(n.ttype) {
				diag("argument to '%s' is not of ordinal ttype", b.name)
				return -1
			}
		case 'w':
			if !tisord(n.ttype) || n.ttype != tsound {
				diag("argument to '%s' is not a sound", b.name)
				return -1
			}
		case 'z':
			if !tisord(n.ttype) || n.ttype != tcolor {
				diag("argument to '%s' is not a color", b.name)
				return -1
			}
		case 'l':
			if n.ttype != topacity && n.ttype != tcreal {
				diag("argument to '%s' is not an opacity", b.name)
				return -1
			}
		case 'h', 'H':
			if n.ttype != tstrength && n.ttype != tcint {
				diag("argument to '%s' is not an strength", b.name)
				return -1
			}
		case 'u', 'U':
			if n.ttype != tbutton && n.ttype != tcint {
				diag("argument to '%s' is not an button", b.name)
				return -1
			}
		case 'v', 'V', 'n', 'N':
			/* ignore */
		case 'b', 'r', 'R', 'f', 'F', 'p', 'P', 'c', 'C', 'i', 'I':
			op := barg[asv]
			if barg[asv] != 0 && !n.ttype.Tis(op) {
				diagtypes(b.name, n.ttype, op)
				return -1
			}
			break
		default:
			s := fmt.Sprintf("xargcheck: bad type code '%c'", asv)
			panic(s)
		}
		if asv == 'v' || (asv >= 'A' && asv <= 'Z') {
			if !islval(n) {
				diag("argument to '%s' not an l-lvalue", b.name)
				return -1
			}
		}
	}
	if len(as) > len(al.item) {
		diag("not enough arguments in call to '%s' ", b.name)
		return -1
	}
	if i < len(al.item) {
		diag("too many arguments in call to '%s'", b.name)
		return -1
	}

	return 0
}

func brtype(b *Builtin, al *List) *Type {
	switch b.r {
	case 'i':
		return tcint
	case 'r':
		return tcreal
	case 'b':
		return tcbool
	case '=':
		if len(al.item) > 0 {
			return al.getsym(0).ttype
		}
	}
	return tundef
}

var (
	builtins = map[string]Builtin{
		"acos":        {"acos", paminstr.PBacos, Sfunc, "r", 'r', xacos},
		"asin":        {"asin", paminstr.PBasin, Sfunc, "r", 'r', xasin},
		"atan":        {"atan", paminstr.PBatan, Sfunc, "r", 'r', xatan},
		"close":       {"close", paminstr.PBclose, Sproc, "f", 0, xatruntime},
		"cos":         {"cos", paminstr.PBcos, Sfunc, "r", 'r', xcos},
		"data":        {"data", paminstr.PBdata, Sproc, "", 0, xatruntime},
		"dispose":     {"dispose", paminstr.PBdispose, Sproc, "P", 0, xatruntime},
		"eof":         {"eof", paminstr.PBfeof, Sfunc, "<", 'b', xstdio},
		"eol":         {"eol", paminstr.PBfeol, Sfunc, "<", 'b', xstdio},
		"exp":         {"exp", paminstr.PBexp, Sfunc, "r", 'r', xexp},
		"fatal":       {"fatal", paminstr.PBfatal, Sproc, "s", 0, xatruntime},
		"feof":        {"feof", paminstr.PBfeof, Sfunc, "f", 'b', xatruntime},
		"feol":        {"feol", paminstr.PBfeol, Sfunc, "f", 'b', xatruntime},
		"fflush":      {"fflush", paminstr.PBfflush, Sproc, "f", 0, xatruntime},
		"flush":       {"flush", paminstr.PBfflush, Sproc, ">", 0, xstdio},
		"fpeek":       {"fpeek", paminstr.PBfpeek, Sproc, "fC", 0, xatruntime},
		"fread":       {"fread", paminstr.PBfread, Sproc, "fV", 0, xatruntime},
		"freadeol":    {"freadeol", paminstr.PBfreadeol, Sproc, "f", 0, xatruntime},
		"freadln":     {"freadln", paminstr.PBfreadln, Sproc, "fV", 0, xatruntime},
		"frewind":     {"frewind", paminstr.PBfrewind, Sproc, "f", 0, xatruntime},
		"fwrite":      {"fwrite", paminstr.PBfwrite, Sproc, "fn", 0, xatruntime},
		"fwriteeol":   {"fwriteeol", paminstr.PBfwriteeol, Sproc, "f", 0, xatruntime},
		"fwriteln":    {"fwriteln", paminstr.PBfwriteln, Sproc, "fn", 0, xatruntime},
		"gclear":      {"gclear", paminstr.PBgclear, Sproc, "f", 0, xatruntime},
		"gclose":      {"gclose", paminstr.PBgclose, Sproc, "f", 0, xatruntime},
		"gshowcursor": {"gshowcursor", paminstr.PBgshowcursor, Sproc, "fb", 0, xatruntime},
		"gellipse":    {"gellipse", paminstr.PBgellipse, Sproc, "fiiiir", 0, xatruntime},
		"gfillcol":    {"gfillcol", paminstr.PBgfillcol, Sproc, "fzl", 0, xatruntime},
		"gfillrgb":    {"gfillrgb", paminstr.PBgfillrgb, Sproc, "fhhhl", 0, xatruntime},
		"gline":       {"gline", paminstr.PBgline, Sproc, "fiiii", 0, xatruntime},
		"gloc":        {"gloc", paminstr.PBgloc, Sproc, "fiir", 0, xatruntime},
		"gopen":       {"gopen", paminstr.PBgopen, Sproc, "Fs", 0, xatruntime},
		"gpencol":     {"gpencol", paminstr.PBgpencol, Sproc, "fzl", 0, xatruntime},
		"gpenrgb":     {"gpenrgb", paminstr.PBgpenrgb, Sproc, "fhhhl", 0, xatruntime},
		"gpenwidth":   {"gpenwidth", paminstr.PBgpenwidth, Sproc, "fi", 0, xatruntime},
		"gplay":       {"gplay", paminstr.PBgplay, Sproc, "fw", 0, xatruntime},
		"gpolygon":    {"gpolygon", paminstr.PBgpolygon, Sproc, "fiiiir", 0, xatruntime},
		"gkeypress":   {"gkeypress", paminstr.PBgkeypress, Sproc, "fV", 0, xatruntime},
		"greadmouse":  {"greadmouse", paminstr.PBgreadmouse, Sproc, "fIIU", 0, xatruntime},
		"gstop":       {"gstop", paminstr.PBgstop, Sproc, "f", 0, xatruntime},
		"gtextheight": {"gtextheight", paminstr.PBgtextheight, Sfunc, "f", 'i', xatruntime},
		"log":         {"log", paminstr.PBlog, Sfunc, "r", 'r', xlog},
		"log10":       {"log10", paminstr.PBlog10, Sfunc, "r", 'r', xlog10},
		"new":         {"new", paminstr.PBnew, Sproc, "P", 0, xatruntime},
		"open":        {"open", paminstr.PBopen, Sproc, "Fss", 0, xatruntime},
		"peek":        {"peek", paminstr.PBfpeek, Sproc, "<C", 0, xstdio},
		"pow":         {"pow", paminstr.PBpow, Sfunc, "rr", 'r', xpow},
		"pred":        {"pred", paminstr.PBpred, Sfunc, "o", '=', xpred},
		"rand":        {"rand", paminstr.PBrand, Sproc, "iI", 0, xatruntime},
		"read":        {"read", paminstr.PBfread, Sproc, "<V", 0, xstdio},
		"readeol":     {"readeol", paminstr.PBfreadeol, Sproc, "<", 0, xstdio},
		"readln":      {"readln", paminstr.PBfreadln, Sproc, "<V", 0, xstdio},
		"sin":         {"sin", paminstr.PBsin, Sfunc, "r", 'r', xsin},
		"sleep":       {"sleep", paminstr.PBsleep, Sproc, "i", 0, xatruntime},
		"sqrt":        {"sqrt", paminstr.PBsqrt, Sfunc, "r", 'r', xsqrt},
		"stack":       {"stack", paminstr.PBstack, Sproc, "", 0, xatruntime},
		"succ":        {"succ", paminstr.PBsucc, Sfunc, "o", '=', xsucc},
		"tan":         {"tan", paminstr.PBtan, Sfunc, "r", 'r', xtan},
		"write":       {"write", paminstr.PBfwrite, Sproc, ">n", 0, xstdio},
		"writeeol":    {"writeeol", paminstr.PBfwriteeol, Sproc, ">", 0, xstdio},
		"writeln":     {"writeln", paminstr.PBfwriteln, Sproc, ">n", 0, xstdio},
	}
)

func Builtininit() {
	bargval = defsym("$bav", Svar)
	bargval.op = Oparm
	bargref = defsym("$bar", Svar)
	bargref.op = Orefparm
	bargval.ttype = tundef
	bargref.ttype = tundef

	for name, builtin := range builtins {
		s := defsym(name, builtin.kind)
		s.prog = allocprog()
		s.prog.psym = s
		newb := new(Builtin)
		*newb = builtin
		s.prog.b = newb
		s.ttype = mkbtype(newb)
		s.prog.parms = s.ttype.parms
		s.id = paminstr.PAMbuiltin | uint(builtin.id)
	}
	bpred = lookup("pred", Sfunc)
	bsucc = lookup("succ", Sfunc)
	if bpred == nil || bsucc == nil {
		panic("bpred == nil || bsucc == nil")
	}
}
