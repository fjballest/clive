package paminstr

import (
	"fmt"
	"os"
)

const (
	Eps    = 0.000001 //for comparing floats
	Maxint = 0x7FFFFFFF
	Minint = -Maxint + 1

	//Eol is EOL[0]
	Eof = 0xff
	Tab = '\t'
	Esc = 0x1b
	Nul = 0

	//key scan codes, highest bit is 1
	KeyScMsk = 0x80

	Del       = 0xf9
	MetaRight = 0xf8
	MetaLeft  = 0xf7
	Return    = 0xf6
	Up        = 0xf5
	Down      = 0xf4
	Left      = 0xf3
	Right     = 0xf2
	Shift     = 0xf1
	Ctrl      = 0xf0
)

const (
	Black = iota
	Red
	Green
	Blue
	Yellow
	Orange
	White
)

const (
	Woosh = iota
	Beep
	Sheep
	Phaser
	Rocket
	CNote
	CsharpNote
	DNote
	DsharpNote
	ENote
	FNote
	FsharpNote
	GNote
	GsharpNote
	ANote
	AsharpNote
	BNote
	Bomb
	Fail
	Tada
)

const (
	// picky abstract machine: PAM:
	//
	// Instructions are: ic|it
	// They correspond to a stack machine that takes
	// operands from the stack (-sp) and leaves
	// results on the stack (+sp).
	// Some instructions carry an additional parameter
	// that takes the following instruction slot.
	// data addresses given to addr, arg, and lvar take one slot
	// in the text but are stored as u64ints in both data and stack.
	//
	// bools, chars, and ints are u32ints.
	// reals are float (same size of u32ints).
	//
	// Those flagged |r have a real variant with type ITreal.
	// Those flagged |a have a variant with type ITaddr
	//
	// Strings are always handed through references.
	//
	// The activation frame is:
	//		<- sp
	//	temp
	//	temp	<- fp
	//	savedpc		fp[-1]
	//	savedpid		fp[-2]		(procid for caller)
	//	savedfp		fp[-3,-4]
	//	savedvp		fp[-5,-6]
	//	savedap		fp[-7,-8]
	//	lvar
	//	lvar	<- vp
	//	arg
	//	arg
	//	arg	<- ap
	//	XXX
	//	...
	//		<- savedvp
	//	...
	//		<- savedfp
	//
	//

	// instruction code (ic)
	ICnop   = iota // nop
	ICle           // le|r -sp -sp +sp
	ICge           // ge|r -sp -sp +sp
	ICpow          // pow|r -sp -sp +sp
	IClt           // lt|r -sp -sp +sp
	ICgt           // gt|r -sp -sp +sp
	ICmul          // mul|r -sp -sp +sp
	ICdiv          // div|r -sp -sp +spPBacos *.y
	ICmod          // mod|r -sp -sp +sp
	ICadd          // add|r -sp -sp +sp
	ICsub          // sub|r -sp -sp +sp
	ICminus        // minus|r -sp +sp
	ICnot          // not -sp +sp
	ICor           // or -sp -sp +sp
	ICand          // and -sp -sp +sp
	ICeq           // eq|r|a -sp -sp +sp
	ICne           // ne|r|a -sp -sp +sp
	ICptr          // ptr -sp +sp
	// obtain address for ptr in stack

	ICargs          // those after have an argument
	ICpush = ICargs // push|r n +sp
	// push n in the stack
)
const (
	ICindir = iota + ICpush + 1 // indir|a  n -sp +sp
	// replace address with referenced bytes
	ICjmp  // jmp addr
	ICjmpt // jmpt addr
	ICjmpf // jmpf addr
	ICidx  // idx tid  -sp -sp +sp
	// replace address[index] with elem. addr.
	ICfld // fld n -sp +sp
	// replace obj addr with field (at n) addr.
	ICdaddr // daddr n +sp
	// push address for data at n
	ICdata // data n +sp
	// push n bytes of data following instruction
	ICeqm // eqm n -sp -sp +sp
	// compare data pointed to by addresses
	ICnem // nem n -sp -sp +sp
	// compare data pointed to by addresses
	ICcall // call pid
	ICret  // ret pid
	ICarg  // arg n +sp
	// push address for arg object at n
	IClvar // lvar n +sp
	// push address for lvar object at n
	ICstom // stom tid -sp -sp
	// cp tid's sz bytes from address to address
	ICsto // sto tid -sp -sp
	// cp tid's sz bytes to address from stack
	ICcast // cast|r tid -sp +sp
	// convert int (or real |r) to type tid
	/* instr. type (it) */
	ITint  = 0
	ITaddr = 0x40
	ITreal = 0x80
	ITmask = ITreal | ITaddr

	// Builtin addresses
	PAMbuiltin = 0x80000000
	// builtin numbers (must be |PAMbuiltin)
)

const (
	PBacos = iota
	PBasin
	PBatan
	PBclose
	PBcos
	PBdispose // 0x5
	PBexp
	PBfatal
	PBfeof
	PBfeol
	PBfpeek // 0xa
	PBfread
	PBfreadeol
	PBfreadln
	PBfrewind
	PBfwrite // 0xf
	PBfwriteln
	PBfwriteeol
	PBlog
	PBlog10
	PBnew // 0x14
	PBopen
	PBpow
	PBpred
	PBsin
	PBsqrt // 0x19
	PBdata
	PBfflush
	PBgclear
	PBgclose
	PBgshowcursor
	PBgellipse
	PBgfillcol
	PBgfillrgb
	PBgkeypress
	PBgline
	PBgloc
	PBgopen
	PBgpencol
	PBgpenrgb
	PBgpenwidth
	PBgplay
	PBgpolygon
	PBgreadmouse
	PBgstop
	PBgtextheight
	PBrand
	PBsleep
	PBstack
	PBsucc
	PBtan

	Nbuiltins
)

var (
	EOL     = "\n"
	itnames = map[uint32]string{
		ITint:  "",
		ITreal: "r",
		ITaddr: "a",
	}

	icnames = map[uint32]string{
		ICadd:            "add",
		ICadd | ITreal:   "addr",
		ICand:            "and",
		ICarg:            "arg",
		ICcall:           "call",
		ICcast:           "cast",
		ICcast | ITreal:  "castr",
		ICdaddr:          "daddr",
		ICdata:           "data",
		ICdata | ITreal:  "datar",
		ICdiv:            "div",
		ICdiv | ITreal:   "divr",
		ICeq:             "eq",
		ICeqm:            "eqm",
		ICeq | ITaddr:    "eqa",
		ICeq | ITreal:    "eqr",
		ICfld:            "fld",
		ICge:             "ge",
		ICge | ITreal:    "ger",
		ICgt:             "gt",
		ICgt | ITreal:    "gtr",
		ICidx:            "idx",
		ICindir:          "ind",
		ICjmp:            "jmp",
		ICjmpf:           "jmpf",
		ICjmpt:           "jmpt",
		ICle:             "le",
		ICle | ITreal:    "ler",
		IClt:             "lt",
		IClt | ITreal:    "ltr",
		IClvar:           "lvar",
		ICminus:          "minus",
		ICminus | ITreal: "minusr",
		ICmod:            "mod",
		ICmod | ITreal:   "modr",
		ICmul:            "mul",
		ICmul | ITreal:   "mulr",
		ICne:             "ne",
		ICnem:            "nem",
		ICne | ITaddr:    "nea",
		ICne | ITreal:    "ner",
		ICnop:            "nop",
		ICnot:            "not",
		ICor:             "or",
		ICpow:            "pow",
		ICpow | ITreal:   "powr",
		ICptr:            "ptr",
		ICpush:           "push",
		ICpush | ITreal:  "pushr",
		ICret:            "ret",
		ICsto:            "sto",
		ICstom:           "stom",
		ICsub:            "sub",
		ICsub | ITreal:   "subr",
	}
	nameics = map[string]uint32{
		"add":    ICadd,
		"addr":   ICadd | ITreal,
		"and":    ICand,
		"arg":    ICarg,
		"call":   ICcall,
		"cast":   ICcast,
		"castr":  ICcast | ITreal,
		"daddr":  ICdaddr,
		"data":   ICdata,
		"datar":  ICdata | ITreal,
		"div":    ICdiv,
		"divr":   ICdiv | ITreal,
		"eq":     ICeq,
		"eqm":    ICeqm,
		"eqa":    ICeq | ITaddr,
		"eqr":    ICeq | ITreal,
		"fld":    ICfld,
		"ge":     ICge,
		"ger":    ICge | ITreal,
		"gt":     ICgt,
		"gtr":    ICgt | ITreal,
		"idx":    ICidx,
		"ind":    ICindir,
		"jmp":    ICjmp,
		"jmpf":   ICjmpf,
		"jmpt":   ICjmpt,
		"le":     ICle,
		"ler":    ICle | ITreal,
		"lt":     IClt,
		"ltr":    IClt | ITreal,
		"lvar":   IClvar,
		"minus":  ICminus,
		"minusr": ICminus | ITreal,
		"mod":    ICmod,
		"modr":   ICmod | ITreal,
		"mul":    ICmul,
		"mulr":   ICmul | ITreal,
		"ne":     ICne,
		"nem":    ICnem,
		"nea":    ICne | ITaddr,
		"ner":    ICne | ITreal,
		"nop":    ICnop,
		"not":    ICnot,
		"or":     ICor,
		"pow":    ICpow,
		"powr":   ICpow | ITreal,
		"ptr":    ICptr,
		"push":   ICpush,
		"pushr":  ICpush | ITreal,
		"ret":    ICret,
		"sto":    ICsto,
		"stom":   ICstom,
		"sub":    ICsub,
		"subr":   ICsub | ITreal,
	}
)

func Hasarg(ir uint32) bool {
	return IC(ir) >= ICargs
}

func IC(c uint32) uint32 {
	return c & 0x3f
}

func IT(c uint32) uint32 {
	return c & ITmask
}

type Instr uint32

func (irp Instr) String() string {
	ir := uint32(irp)
	ic := IC(ir)
	it := IT(ir)

	icname, okic := icnames[ic]
	if !okic {
		es := fmt.Sprintf("bad IC %d", okic)
		panic(es)
	}
	itname, okit := itnames[it]
	if !okit {
		es := fmt.Sprintf("bad IT %#x", it)
		panic(es)
	}
	return fmt.Sprintf("%s%s", icname, itname)
}

func Icode(s string) uint {
	ic, okic := nameics[s]
	if okic {
		return uint(ic)
	}
	fmt.Fprintf(os.Stderr, "no instruction '%s'\n", s)
	return uint(0xffffffff)
}
