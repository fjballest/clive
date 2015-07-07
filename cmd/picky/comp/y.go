//line p.y:2

//
// Px grammar. /Users/paurea/src/picky/p.y
//

package comp

import __yyfmt__ "fmt"

//line p.y:6
import (
	"fmt"
	"os"
)

type Env  {
	id   uint
	tab  map[string]*Sym
	prev *Env
	prog *Sym
	rec  *Type
}

type List  {
	kind int
	item []interface{}
}

type Builtin  {
	name string
	id   uint32
	kind int
	args string
	r    rune
	fn   func(b *Builtin, args *List) *Sym
}

type Stmt  {
	op     int
	sfname string
	lineno int

	//--one of:
	list *List // '{'

	lval *Sym // =
	rval *Sym

	cond    *Sym // IF
	thenarm *Stmt
	elsearm *Stmt

	fcall *Sym // FCALL

	expr *Sym // RETURN, DO, WHILE, FOR, CASE
	stmt *Stmt
	incr *Stmt // last statement in fors (i++|i--)
}

type Type  {
	op    int
	sym   *Sym
	first int
	last  int

	//--one of:
	lits *List // Tenum

	ref *Type // Tptr

	super *Type // Trange

	idx  *Type // Tarry, Tstr
	elem *Type

	fields *List // Trec

	parms *List // Tproc, Tfunc
	rtype *Type

	//--
	// backend
	id uint
	sz uint
}

// pc/src table
type Pcent  {
	next *Pcent
	st   *Stmt
	nd   *Sym
	pc   uint
}

// generated code
type Code  {
	addr  uint32
	pcs   *Pcent
	pcstl *Pcent
	p     []uint32
	np    uint
	ap    uint
}

type Prog  {
	psym   *Sym
	parms  *List
	rtype  *Type // ret type or nil if none
	consts *List
	types  *List
	vars   *List
	procs  *List
	stmt   *Stmt
	b      *Builtin
	nrets  int

	// backend
	code   Code
	parmsz uint
	varsz  uint
}

type Val  {
	//--one of:
	ival int

	rval float64

	sval string

	vals *List
	//--
}

type Sym  {
	id     uint
	name   string
	stype  int
	op     int
	fname  string
	lineno int
	ttype  *Type

	//--one of:
	tok int

	Val

	used int
	set  int

	left  *Sym
	right *Sym // binary, unary

	fsym  *Sym // Sfcall
	fargs *List

	rec   *Sym // "."
	field *Sym

	swfield *Sym // switch field
	swval   *Sym // variant
	//--

	prog *Prog
	// backend
	addr uint
	off  uint // fields
}

type Stats  {
	nenvs  uint // # of envs used
	menvs  uint // # of envs allocated
	nsyms  uint // # of syms allocated
	nexpr  uint // # of syms for expressions
	nlists uint // # of lists allocated
	mlist  uint // # of items in longest list
	nstmts uint // # of stmts allocated
	nprogs uint // # of progs allocated
	ntypes uint // # of types allocated
	nstrs  uint // # of strings allocated
}

//line p.y:182
type YySymType  {
	yys   int
	bval  int
	sval  string
	ival  int
	rval  float64
	sym   *Sym
	list  *List
	stmt  *Stmt
	ttype *Type
	env   *Env
}

const OR = 57346
const AND = 57347
const EQ = 57348
const NE = 57349
const LE = 57350
const GE = 57351
const BADOP = 57352
const POW = 57353
const DOTDOT = 57354
const ARRAY = 57355
const CASE = 57356
const CONSTS = 57357
const DEFAULT = 57358
const DO = 57359
const ELSE = 57360
const FOR = 57361
const FUNCTION = 57362
const IF = 57363
const SWITCH = 57364
const NIL = 57365
const NOT = 57366
const OF = 57367
const PROCEDURE = 57368
const PROGRAM = 57369
const RECORD = 57370
const REF = 57371
const RETURN = 57372
const TYPES = 57373
const VARS = 57374
const WHILE = 57375
const LEN = 57376
const INT = 57377
const CHAR = 57378
const STR = 57379
const REAL = 57380
const TRUE = 57381
const FALSE = 57382
const ID = 57383
const TYPEID = 57384
const FCALL = 57385

var YyToknames = []string{
	" ,",
	"OR",
	"AND",
	"EQ",
	"NE",
	"LE",
	"GE",
	" <",
	" >",
	"BADOP",
	" +",
	" -",
	" *",
	" /",
	" %",
	"POW",
	"DOTDOT",
	"ARRAY",
	"CASE",
	"CONSTS",
	"DEFAULT",
	"DO",
	"ELSE",
	"FOR",
	"FUNCTION",
	"IF",
	"SWITCH",
	"NIL",
	"NOT",
	"OF",
	"PROCEDURE",
	"PROGRAM",
	"RECORD",
	"REF",
	"RETURN",
	"TYPES",
	"VARS",
	"WHILE",
	"LEN",
	"INT",
	"CHAR",
	"STR",
	"REAL",
	"TRUE",
	"FALSE",
	"ID",
	"TYPEID",
	"FCALL",
}
var YyStatenames = []string{}

const YyEofCode = 1
const YyErrCode = 2
const YyMaxDepth = 200

//line p.y:1192
func puterror(fn string, ln int, name string, sfmt string, arg ...interface{}) {
	var s1, s2 string

	if name != "" {
		s1 = fmt.Sprintf("%s:%d: at '%s'", fn, ln, name)
	} else {
		s1 = fmt.Sprintf("%s:%d", fn, ln)
	}
	s2 = fmt.Sprintf(sfmt, arg...)
	if debug['S'] != 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s\n", s1, s2)
	if Nerrors > 10 {
		fmt.Fprintf(os.Stderr, "too many errors\n")
		os.Exit(1)
	}
}

func diag(sfmt string, arg ...interface{}) {
	Nerrors++
	puterror(Scanner.fname, Scanner.lineno, "", sfmt, arg...)
}

func (sc *Scan) Errorf(ffmt string, arg ...interface{}) {
	Nerrors++
	if ffmt=="syntax error" && len(Scanner.sval)>0 {

		puterror(sc.fname, sc.lineno, string(sc.sval[:]), "syntax error", arg...)
	} else {
		puterror(sc.fname, sc.lineno, "", ffmt, arg...)
	}
}

func (pl PickyLex) Error(sfmt string) {
	Scanner.Errorf(sfmt)
}

//TODO Stmt and Sym should probably have a fname, lineno, etc interface

func (s *Sym) Error(sfmt string, arg ...interface{}) {
	Nerrors++
	puterror(s.fname, s.lineno, s.name, sfmt, arg...)
}

func (s *Stmt) Error(sfmt string, arg ...interface{}) {
	Nerrors++
	puterror(s.sfname, s.lineno, "", sfmt, arg...)
}

func SetYYDebug() {
	YyDebug = debug['Y']
}

//line yacctab:1
var YyExca = []int{
	-1, 0,
	23, 2,
	28, 2,
	34, 2,
	39, 2,
	40, 2,
	-2, 0,
	-1, 1,
	1, -1,
	-2, 0,
	-1, 13,
	60, 34,
	-2, 0,
	-1, 14,
	60, 34,
	-2, 0,
	-1, 26,
	60, 33,
	-2, 0,
	-1, 38,
	1, 9,
	23, 9,
	28, 9,
	34, 9,
	39, 9,
	40, 9,
	-2, 0,
	-1, 42,
	1, 10,
	23, 10,
	28, 10,
	34, 10,
	39, 10,
	40, 10,
	-2, 0,
	-1, 47,
	1, 11,
	23, 11,
	28, 11,
	34, 11,
	39, 11,
	40, 11,
	-2, 0,
	-1, 108,
	56, 44,
	-2, 0,
	-1, 109,
	56, 44,
	-2, 0,
	-1, 159,
	56, 43,
	-2, 0,
	-1, 344,
	22, 138,
	24, 138,
	61, 138,
	-2, 0,
	-1, 353,
	22, 137,
	24, 137,
	61, 137,
	-2, 0,
}

const YyNprod = 184
const YyPrivate = 57344

var YyTokenNames []string
var YyStates []string

const YyLast = 700

var YyAct = []int{

	200, 111, 271, 329, 270, 66, 112, 360, 69, 309,
	64, 315, 160, 159, 72, 199, 198, 158, 172, 171,
	173, 174, 179, 178, 176, 177, 181, 167, 168, 169,
	170, 175, 180, 274, 48, 133, 132, 358, 303, 136,
	134, 54, 156, 49, 135, 260, 113, 114, 136, 134,
	249, 247, 219, 135, 292, 248, 78, 273, 272, 110,
	136, 134, 254, 119, 124, 135, 91, 352, 345, 300,
	130, 78, 322, 361, 126, 115, 116, 118, 117, 120,
	121, 122, 125, 316, 285, 317, 143, 123, 88, 253,
	139, 140, 228, 148, 172, 171, 173, 174, 179, 178,
	176, 177, 181, 167, 168, 169, 170, 175, 180, 187,
	218, 220, 362, 268, 286, 182, 183, 323, 256, 49,
	214, 213, 327, 187, 185, 105, 186, 165, 127, 128,
	184, 144, 104, 193, 142, 195, 141, 138, 103, 201,
	106, 137, 204, 109, 211, 208, 210, 108, 312, 205,
	223, 194, 221, 342, 92, 62, 61, 216, 30, 101,
	58, 102, 342, 331, 320, 223, 30, 221, 229, 230,
	231, 232, 233, 234, 235, 236, 237, 238, 239, 240,
	241, 242, 243, 172, 171, 173, 174, 179, 178, 176,
	177, 181, 167, 168, 169, 170, 175, 180, 212, 305,
	222, 244, 365, 246, 297, 28, 29, 304, 31, 37,
	262, 341, 257, 28, 29, 222, 24, 77, 258, 296,
	284, 223, 282, 221, 281, 227, 224, 266, 267, 52,
	51, 303, 23, 22, 275, 277, 278, 252, 21, 276,
	81, 340, 84, 339, 88, 80, 338, 332, 287, 290,
	273, 272, 289, 85, 77, 291, 83, 293, 192, 36,
	191, 295, 147, 161, 79, 87, 146, 86, 294, 77,
	299, 222, 163, 301, 163, 49, 129, 81, 145, 84,
	131, 88, 80, 63, 162, 164, 162, 164, 30, 337,
	85, 59, 81, 83, 84, 53, 88, 80, 163, 35,
	19, 79, 87, 351, 86, 85, 273, 272, 83, 46,
	162, 164, 49, 65, 311, 310, 79, 87, 330, 86,
	325, 326, 307, 306, 280, 279, 328, 49, 225, 226,
	335, 206, 87, 196, 197, 28, 29, 324, 78, 190,
	189, 321, 344, 308, 330, 355, 354, 357, 78, 283,
	130, 78, 353, 334, 356, 288, 44, 45, 265, 130,
	78, 153, 152, 330, 34, 364, 33, 363, 18, 301,
	366, 172, 171, 173, 174, 179, 178, 176, 177, 181,
	167, 168, 169, 170, 175, 180, 172, 171, 173, 174,
	179, 178, 176, 177, 181, 167, 168, 169, 170, 175,
	180, 172, 171, 173, 174, 179, 178, 176, 177, 181,
	167, 168, 169, 170, 175, 180, 90, 89, 17, 202,
	336, 298, 318, 203, 8, 316, 26, 317, 361, 16,
	169, 170, 175, 180, 180, 15, 264, 261, 151, 41,
	9, 10, 343, 254, 43, 251, 155, 154, 157, 39,
	47, 5, 259, 172, 171, 173, 174, 179, 178, 176,
	177, 181, 167, 168, 169, 170, 175, 180, 172, 171,
	173, 174, 179, 178, 176, 177, 181, 167, 168, 169,
	170, 175, 180, 263, 4, 150, 40, 60, 57, 56,
	25, 107, 113, 114, 349, 350, 347, 348, 3, 7,
	55, 12, 11, 42, 255, 32, 20, 38, 6, 119,
	124, 167, 168, 169, 170, 175, 180, 2, 1, 245,
	126, 115, 116, 118, 117, 120, 121, 122, 215, 346,
	188, 333, 14, 123, 172, 171, 173, 174, 179, 178,
	176, 177, 181, 167, 168, 169, 170, 175, 180, 172,
	171, 173, 174, 179, 178, 176, 177, 181, 167, 168,
	169, 170, 175, 180, 172, 171, 173, 174, 179, 178,
	176, 177, 181, 167, 168, 169, 170, 175, 180, 27,
	13, 313, 173, 174, 179, 178, 176, 177, 181, 167,
	168, 169, 170, 175, 180, 359, 250, 302, 314, 149,
	217, 99, 100, 98, 97, 95, 50, 96, 94, 93,
	76, 207, 172, 171, 173, 174, 179, 178, 176, 177,
	181, 167, 168, 169, 170, 175, 180, 50, 172, 171,
	173, 174, 179, 178, 176, 177, 181, 167, 168, 169,
	170, 175, 180, 269, 172, 171, 173, 174, 179, 178,
	176, 177, 181, 167, 168, 169, 170, 175, 180, 166,
	172, 171, 173, 174, 179, 178, 176, 177, 181, 167,
	168, 169, 170, 175, 180, 209, 319, 172, 171, 173,
	174, 179, 178, 176, 177, 181, 167, 168, 169, 170,
	175, 180, 82, 73, 75, 71, 68, 74, 70, 67,
}
var YyPact = []int{

	449, -1000, 401, 369, 319, 248, 401, -1000, 185, 180,
	179, -1000, -1000, 164, 156, 317, 315, 247, 207, -1000,
	-1000, 437, 307, 286, -1000, -17, 286, -1000, 177, 176,
	243, -1000, -17, -1000, -1000, -1000, -1000, -1000, 437, -1000,
	106, 239, 307, -1000, 102, 101, 231, 286, -1000, 252,
	-1000, 367, 104, -1000, -1000, 92, 88, -1000, 32, -1000,
	-1000, 104, 104, -1000, 215, -1000, -1000, -1000, -1000, -1000,
	-1000, -1000, -1000, -1000, -1000, -1000, -1000, 228, -18, 86,
	82, -17, -17, 81, 79, 32, -1000, -1000, 76, 226,
	214, 210, 32, -1000, -1000, -1000, -1000, -1000, -1000, -1000,
	-1000, 436, 312, -1000, -1000, -16, -1000, -1000, 261, 261,
	607, -1000, 3, 32, 32, -1000, -1000, -1000, -1000, -1000,
	-1000, -1000, 75, 32, 32, 68, 290, 208, 206, -1000,
	-1000, -1000, 32, 97, 32, 284, -1000, 32, 32, 378,
	397, 32, 282, 559, 32, -1000, -1000, -1000, 655, 142,
	-1000, -1000, -1000, -1000, 66, 65, 478, 50, 55, 219,
	-1000, -1000, 173, 279, 172, 36, -1000, 32, 32, 32,
	32, 32, 32, 32, 32, 32, 32, 32, 32, 32,
	32, 32, -1000, -1000, 32, 463, -1000, 32, -1000, -1000,
	-7, -1000, -1000, 544, -1000, 178, -1000, -1000, 33, 439,
	639, 448, 63, 59, 396, -9, -1000, -1000, 381, 32,
	434, 309, -1000, 261, 261, 54, 623, -1000, 257, -28,
	-1000, 237, 235, 235, 275, 171, 169, 299, 167, 414,
	414, 415, 415, 575, 575, 497, 497, 415, 497, 497,
	497, 497, -1000, 497, 28, -1000, 58, 32, 306, -1000,
	-1000, 32, -1000, 197, 32, -6, 32, -1000, -1000, -17,
	32, -1000, 639, -1000, -1000, -1000, 163, 148, 388, 32,
	8, -1000, 154, 146, -1000, -1000, -1000, -1000, -1000, -1000,
	-1000, 273, 293, -1000, 265, -1000, -1000, 89, -1000, 529,
	-1000, 639, 403, 366, -1000, 672, -1000, 111, 291, 13,
	-1000, -1000, -1000, 62, 287, 271, -1000, -1000, -1000, -1000,
	-1000, -1000, -1000, -1000, 61, -1000, 32, 110, 195, 304,
	265, -1000, 387, 240, 194, 191, 189, -1000, -1000, 158,
	422, 267, -1000, 12, 485, -1000, 253, 11, -1000, -1000,
	-1000, 267, 32, 32, 267, -17, 32, -1000, -1000, -1000,
	-1000, -1000, -23, 267, -1000, -1000, -1000, 639, 406, 51,
	-1000, 32, -1000, -1000, 149, 257, 201,
}
var YyPgo = []int{

	0, 5, 8, 699, 698, 697, 14, 696, 695, 694,
	693, 11, 692, 610, 66, 609, 608, 607, 605, 9,
	604, 603, 602, 601, 600, 15, 599, 4, 13, 17,
	10, 16, 598, 597, 595, 7, 2, 580, 532, 0,
	1, 6, 531, 3, 12, 530, 529, 518, 517, 508,
	499, 507, 503, 426, 502, 501, 449, 444, 579, 490,
	500, 491, 489, 448, 447, 446, 445,
}
var YyR1 = []int{

	0, 47, 48, 48, 48, 48, 48, 49, 49, 50,
	50, 50, 50, 50, 51, 51, 52, 52, 53, 53,
	56, 56, 57, 57, 57, 14, 14, 58, 58, 58,
	58, 54, 54, 59, 59, 60, 37, 61, 61, 55,
	55, 62, 38, 29, 29, 28, 28, 28, 28, 28,
	28, 28, 44, 44, 44, 44, 44, 44, 15, 15,
	15, 15, 15, 15, 15, 17, 17, 26, 26, 26,
	26, 26, 26, 16, 18, 23, 23, 63, 22, 24,
	24, 27, 27, 27, 36, 36, 36, 33, 34, 34,
	35, 64, 20, 65, 21, 19, 19, 2, 2, 30,
	30, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 13, 9, 8, 10, 5, 42, 46, 46,
	46, 46, 6, 6, 6, 12, 3, 66, 3, 7,
	31, 31, 25, 25, 4, 32, 32, 11, 11, 43,
	43, 43, 39, 39, 39, 39, 39, 39, 39, 39,
	39, 39, 39, 39, 39, 39, 39, 39, 40, 40,
	40, 40, 40, 40, 40, 40, 40, 40, 40, 40,
	40, 40, 40, 45, 45, 45, 45, 45, 41, 41,
	41, 41, 41, 41,
}
var YyR2 = []int{

	0, 2, 0, 3, 3, 3, 2, 2, 1, 3,
	3, 3, 1, 1, 2, 1, 2, 1, 2, 1,
	4, 2, 4, 4, 2, 1, 1, 4, 4, 4,
	2, 2, 3, 1, 0, 0, 4, 3, 0, 2,
	3, 0, 8, 1, 0, 3, 3, 1, 2, 3,
	3, 1, 3, 3, 4, 4, 3, 4, 1, 1,
	1, 1, 1, 1, 1, 2, 2, 3, 1, 1,
	2, 3, 3, 3, 4, 6, 8, 0, 3, 3,
	2, 2, 2, 1, 4, 4, 4, 7, 2, 1,
	4, 0, 5, 0, 7, 1, 1, 3, 2, 2,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 2, 1, 3, 7, 5, 9, 3, 1, 1,
	1, 1, 2, 4, 4, 4, 4, 0, 6, 5,
	1, 0, 3, 1, 7, 2, 1, 4, 3, 3,
	1, 3, 1, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 1, 2,
	2, 1, 1, 1, 1, 1, 1, 1, 4, 3,
	2, 4, 2, 1, 1, 4, 3, 2, 1, 4,
	3, 3, 2, 1,
}
var YyChk = []int{

	-1000, -47, -48, 49, 35, 2, -49, -50, 23, 39,
	40, -54, -55, -37, -38, 34, 28, 49, 49, 52,
	-50, 53, 53, 53, 52, -59, -53, -58, 49, 50,
	2, 52, -59, 49, 49, 52, 52, 2, -51, -56,
	49, 2, -52, -57, 49, 50, 2, -53, -2, 60,
	-58, 53, 53, 52, -2, -60, -62, -56, 54, 52,
	-57, 54, 54, 52, -30, 61, -1, -3, -7, -2,
	-4, -8, -6, -10, -5, -9, -13, 2, -41, 49,
	30, 25, -12, 41, 27, 38, 52, 50, 29, 50,
	49, -14, 50, -15, -16, -18, -17, -20, -21, -23,
	-22, 55, 57, 34, 28, 21, 36, -61, 55, 55,
	-39, -40, -41, 14, 15, 43, 44, 46, 45, 31,
	47, 48, 49, 55, 32, 50, 42, -14, -14, 61,
	-1, 52, 54, 53, 58, 62, 57, 55, 55, -2,
	-2, 55, 55, -39, 55, 52, 52, 52, -39, -26,
	49, 2, 50, 49, -64, -65, 58, -63, -29, -28,
	-44, 2, 49, 37, 50, -29, 52, 14, 15, 16,
	17, 6, 5, 7, 8, 18, 11, 12, 10, 9,
	19, 13, -40, -40, 55, -39, -40, 55, -45, 50,
	49, 52, 52, -39, 54, -39, 49, 50, -31, -25,
	-39, -39, 41, 26, -39, -41, 49, 52, -39, 20,
	4, 2, 56, 55, 55, 50, -39, -24, 60, 2,
	56, 4, 52, 2, 53, 49, 50, 53, 56, -39,
	-39, -39, -39, -39, -39, -39, -39, -39, -39, -39,
	-39, -39, -39, -39, -31, 56, -25, 58, 62, 57,
	52, -66, 59, 56, 4, 56, 55, -2, -6, 56,
	54, 56, -39, 49, 2, 49, -28, -28, 59, 20,
	-27, -36, 50, 49, 61, -44, 2, -44, -44, 50,
	49, 53, 53, 50, 53, 56, 56, -39, 49, -39,
	52, -39, 60, -39, -2, -39, 56, 56, 33, -39,
	61, -36, -33, 30, 53, 53, 50, 49, 50, -19,
	50, 49, 59, 52, -32, -11, 22, 24, 56, 4,
	53, 50, 59, 55, 50, 49, 50, 61, -11, -43,
	-40, 53, 52, -42, 49, -19, 33, 49, 52, 52,
	52, 53, 4, 20, -30, 56, -46, 11, 12, 9,
	10, 50, 56, -30, -43, -40, -2, -39, 60, -34,
	-35, 22, 61, -35, -43, 53, -27,
}
var YyDef = []int{

	-2, -2, 0, 0, 0, 0, 1, 8, 0, 0,
	0, 12, 13, -2, -2, 0, 0, 0, 0, 6,
	7, 0, 0, 0, 31, 0, -2, 19, 0, 0,
	0, 39, 0, 35, 41, 3, 4, 5, -2, 15,
	0, 0, -2, 17, 0, 0, 0, -2, 32, 0,
	18, 0, 0, 30, 40, 38, 0, 14, 0, 21,
	16, 0, 0, 24, 0, 98, 100, 101, 102, 103,
	104, 105, 106, 107, 108, 109, 110, 0, 0, 178,
	0, 0, 0, 0, 0, 0, 112, 183, 0, 0,
	0, 0, 25, 26, 58, 59, 60, 61, 62, 63,
	64, 0, 0, 91, 93, 0, 77, 36, -2, -2,
	0, 142, 158, 0, 0, 161, 162, 163, 164, 165,
	166, 167, 178, 0, 0, 183, 0, 0, 0, 97,
	99, 111, 0, 0, 0, 0, 182, 131, 0, 0,
	122, 0, 0, 0, 0, 27, 29, 28, 0, 0,
	68, 69, 65, 66, 0, 0, 0, 0, 0, -2,
	47, 51, 0, 0, 0, 0, 20, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 159, 160, 131, 0, 170, 0, 172, 173,
	174, 22, 23, 0, 127, 0, 180, 181, 0, 130,
	133, 0, 0, 0, 0, 0, 178, 113, 0, 0,
	0, 70, 73, 0, 0, 183, 0, 78, 0, 0,
	37, 0, 0, 48, 0, 0, 0, 0, 0, 143,
	144, 145, 146, 147, 148, 149, 150, 151, 152, 153,
	154, 155, 156, 157, 0, 169, 0, 0, 0, 177,
	126, 0, 179, 0, 0, 0, 0, 123, 124, 0,
	0, 125, 74, 67, 72, 71, 0, 0, 0, 0,
	0, 83, 0, 0, 80, 45, 50, 46, 49, 52,
	53, 0, 0, 56, 0, 168, 171, 0, 176, 0,
	129, 132, 0, 0, 115, 0, 92, 0, 0, 0,
	79, 81, 82, 0, 0, 0, 54, 55, 57, 42,
	95, 96, 175, 128, 0, 136, 0, 0, 0, 0,
	0, 75, 0, 0, 0, 0, 0, 134, 135, 0,
	140, 0, 114, 0, 0, 94, 0, 0, 84, 85,
	86, 0, 0, 0, -2, 0, 0, 118, 119, 120,
	121, 76, 0, -2, 139, 141, 116, 117, 0, 0,
	89, 0, 87, 88, 0, 0, 90,
}
var YyTok1 = []int{

	1, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 18, 3, 3,
	55, 56, 16, 14, 4, 15, 62, 17, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 53, 52,
	11, 54, 12, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 58, 3, 59, 57, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 60, 3, 61,
}
var YyTok2 = []int{

	2, 3, 5, 6, 7, 8, 9, 10, 13, 19,
	20, 21, 22, 23, 24, 25, 26, 27, 28, 29,
	30, 31, 32, 33, 34, 35, 36, 37, 38, 39,
	40, 41, 42, 43, 44, 45, 46, 47, 48, 49,
	50, 51,
}
var YyTok3 = []int{
	0,
}

//line yaccpar:1

/*	parser for yacc output	*/

var YyDebug = 0

type YyLexer interface {
	Lex(lval *YySymType) int
	Error(s string)
}

const YyFlag = -1000

func YyTokname(c int) string {
	// 4 is TOKSTART above
	if c>=4 && c-4<len(YyToknames) {
		if YyToknames[c-4] != "" {
			return YyToknames[c-4]
		}
	}
	return __yyfmt__.Sprintf("tok-%v", c)
}

func YyStatname(s int) string {
	if s>=0 && s<len(YyStatenames) {
		if YyStatenames[s] != "" {
			return YyStatenames[s]
		}
	}
	return __yyfmt__.Sprintf("state-%v", s)
}

func Yylex1(lex YyLexer, lval *YySymType) int {
	c := 0
	char := lex.Lex(lval)
	if char <= 0 {
		c = YyTok1[0]
		goto out
	}
	if char < len(YyTok1) {
		c = YyTok1[char]
		goto out
	}
	if char >= YyPrivate {
		if char < YyPrivate+len(YyTok2) {
			c = YyTok2[char-YyPrivate]
			goto out
		}
	}
	for i := 0; i < len(YyTok3); i += 2 {
		c = YyTok3[i+0]
		if c == char {
			c = YyTok3[i+1]
			goto out
		}
	}

out:
	if c == 0 {
		c = YyTok2[1] /* unknown char */
	}
	if YyDebug >= 3 {
		__yyfmt__.Printf("lex %s(%d)\n", YyTokname(c), uint(char))
	}
	return c
}

func YyParse(Yylex YyLexer) int {
	var Yyn int
	var Yylval YySymType
	var YyVAL YySymType
	YyS := make([]YySymType, YyMaxDepth)

	Nerrs := 0   /* number of errors */
	Errflag := 0 /* error recovery flag */
	Yystate := 0
	Yychar := -1
	Yyp := -1
	goto Yystack

ret0:
	return 0

ret1:
	return 1

Yystack:
	/* put a state and value onto the stack */
	if YyDebug >= 4 {
		__yyfmt__.Printf("char %v in %v\n", YyTokname(Yychar), YyStatname(Yystate))
	}

	Yyp++
	if Yyp >= len(YyS) {
		nyys := make([]YySymType, len(YyS)*2)
		copy(nyys, YyS)
		YyS = nyys
	}
	YyS[Yyp] = YyVAL
	YyS[Yyp].yys = Yystate

Yynewstate:
	Yyn = YyPact[Yystate]
	if Yyn <= YyFlag {
		goto Yydefault /* simple state */
	}
	if Yychar < 0 {
		Yychar = Yylex1(Yylex, &Yylval)
	}
	Yyn += Yychar
	if Yyn<0 || Yyn>=YyLast {
		goto Yydefault
	}
	Yyn = YyAct[Yyn]
	if YyChk[Yyn] == Yychar { /* valid shift */
		Yychar = -1
		YyVAL = Yylval
		Yystate = Yyn
		if Errflag > 0 {
			Errflag--
		}
		goto Yystack
	}

Yydefault:
	/* default state action */
	Yyn = YyDef[Yystate]
	if Yyn == -2 {
		if Yychar < 0 {
			Yychar = Yylex1(Yylex, &Yylval)
		}

		/* look through exception table */
		xi := 0
		for {
			if YyExca[xi+0]==-1 && YyExca[xi+1]==Yystate {
				break
			}
			xi += 2
		}
		for xi += 2; ; xi += 2 {
			Yyn = YyExca[xi+0]
			if Yyn<0 || Yyn==Yychar {
				break
			}
		}
		Yyn = YyExca[xi+1]
		if Yyn < 0 {
			goto ret0
		}
	}
	if Yyn == 0 {
		/* error ... attempt to resume parsing */
		switch Errflag {
		case 0: /* brand new error */
			Yylex.Error("syntax error")
			Nerrs++
			if YyDebug >= 1 {
				__yyfmt__.Printf("%s", YyStatname(Yystate))
				__yyfmt__.Printf(" saw %s\n", YyTokname(Yychar))
			}
			fallthrough

		case 1, 2: /* incompletely recovered error ... try again */
			Errflag = 3

			/* find a state where "error" is a legal shift action */
			for Yyp >= 0 {
				Yyn = YyPact[YyS[Yyp].yys] + YyErrCode
				if Yyn>=0 && Yyn<YyLast {
					Yystate = YyAct[Yyn] /* simulate a shift of "error" */
					if YyChk[Yystate] == YyErrCode {
						goto Yystack
					}
				}

				/* the current p has no shift on "error", pop stack */
				if YyDebug >= 2 {
					__yyfmt__.Printf("error recovery pops state %d\n", YyS[Yyp].yys)
				}
				Yyp--
			}
			/* there is no state on the stack with an error shift ... abort */
			goto ret1

		case 3: /* no shift yet; clobber input char */
			if YyDebug >= 2 {
				__yyfmt__.Printf("error recovery discards %s\n", YyTokname(Yychar))
			}
			if Yychar == YyEofCode {
				goto ret1
			}
			Yychar = -1
			goto Yynewstate /* try again in the same state */
		}
	}

	/* reduction by production Yyn */
	if YyDebug >= 2 {
		__yyfmt__.Printf("reduce %v in:\n\t%v\n", Yyn, YyStatname(Yystate))
	}

	Yynt := Yyn
	Yypt := Yyp
	_ = Yypt // guard against "declared and not used"

	Yyp -= YyR2[Yyn]
	YyVAL = YyS[Yyp+1]

	/* consult goto table to find next state */
	Yyn = YyR1[Yyn]
	Yyg := YyPgo[Yyn]
	Yyj := Yyg + YyS[Yyp].yys + 1

	if Yyj >= YyLast {
		Yystate = YyAct[Yyg]
	} else {
		Yystate = YyAct[Yyj]
		if YyChk[Yystate] != -Yyn {
			Yystate = YyAct[Yyg]
		}
	}
	// dummy call; replaced with literal code
	switch Yynt {

	case 1:
		//line p.y:230
		{
			if env.prev != nil {
				panic("env stack not empty; missing popenv()")
			}
			if debug['P'] != 0 {
				dumpprog(os.Stderr, env.prog)
			}
		}
	case 3:
		//line p.y:239
		{
			env.prog = newprog(YyS[Yypt-1].sym)
			s := oksym(YyS[Yypt-2].sym)
			errmsg := fmt.Sprintf("'program' expected, found %s", s.name)
			panic(errmsg)
		}
	case 4:
		//line p.y:247
		{
			env.prog = newprog(YyS[Yypt-1].sym)
		}
	case 5:
		//line p.y:252
		{
			env.prog = newprog(YyS[Yypt-1].sym)
			Yylex.Error("';' missing after program name")
		}
	case 6:
		//line p.y:257
		{
			panic("not a picky program")
		}
	case 9:
		//line p.y:268
		{
			if env.prog == nil {
				panic("missing program declaration")
			}
		}
	case 10:
		//line p.y:273
		{
			if env.prog == nil {
				panic("missing program declaration")
			}
		}
	case 11:
		//line p.y:278
		{
			if env.prog == nil {
				panic("missing program declaration")
			}
			if !globalsok {
				diag("global variables are not allowed")
			}
		}
	case 20:
		//line p.y:304
		{
			declconst(YyS[Yypt-3].sym, YyS[Yypt-1].sym)
		}
	case 21:
		//line p.y:309
		{
			diag("constant declaration expected")
			Errflag = 0
		}
	case 22:
		//line p.y:316
		{
			decltype(YyS[Yypt-3].sym, YyS[Yypt-1].ttype)
		}
	case 23:
		//line p.y:321
		{
			decltype(YyS[Yypt-3].sym, YyS[Yypt-1].ttype)
		}
	case 24:
		//line p.y:326
		{
			diag("type declaration expected")
			Errflag = 0
		}
	case 25:
		//line p.y:334
		{
			YyVAL.ttype = YyS[Yypt-0].sym.ttype
		}
	case 26:
		YyVAL.ttype = YyS[Yypt-0].ttype
	case 27:
		//line p.y:342
		{
			declvar(YyS[Yypt-3].sym, YyS[Yypt-1].sym.ttype)
		}
	case 28:
		//line p.y:347
		{
			diag("'%s' is a type name", YyS[Yypt-3].sym.name)
		}
	case 29:
		//line p.y:352
		{
			diag("type name expected; found '%s'", YyS[Yypt-1].sym.name)
		}
	case 30:
		//line p.y:357
		{
			diag("var declaration expected")
			Errflag = 0
		}
	case 31:
		//line p.y:364
		{
			declprogdone(env.prog)
			popenv()
		}
	case 32:
		//line p.y:370
		{
			if env.prog == nil {
				panic("missing program declaration")
			}
			env.prog.prog.stmt = YyS[Yypt-0].stmt
			declprogdone(env.prog)
			popenv()
		}
	case 35:
		//line p.y:384
		{
			declproc(YyS[Yypt-0].sym)
		}
	case 36:
		//line p.y:388
		{
			YyVAL.sym = env.prog
		}
	case 37:
		//line p.y:394
		{
			if env.prog == nil {
				panic("missing program declaration")
			}
			env.prog.prog.parms = YyS[Yypt-1].list
		}
	case 38:
		//line p.y:399
		{
			diag("missing '()'")
		}
	case 39:
		//line p.y:405
		{
			declprogdone(env.prog)
			popenv()
		}
	case 40:
		//line p.y:411
		{
			if env.prog == nil {
				panic("missing program declaration")
			}
			env.prog.prog.stmt = YyS[Yypt-0].stmt
			declprogdone(env.prog)
			popenv()
		}
	case 41:
		//line p.y:420
		{
			declfunc(YyS[Yypt-0].sym)
		}
	case 42:
		//line p.y:424
		{
			if env.prog == nil {
				panic("missing program declaration")
			}
			YyVAL.sym = env.prog
			YyVAL.sym.prog.parms = YyS[Yypt-3].list
			YyVAL.sym.prog.rtype = YyS[Yypt-0].ttype
		}
	case 43:
		YyVAL.list = YyS[Yypt-0].list
	case 44:
		//line p.y:435
		{
			YyVAL.list = newlist(Lsym)
		}
	case 45:
		//line p.y:441
		{
			YyVAL.list = YyS[Yypt-2].list
			if YyS[Yypt-2].list != nil {
				addsym(YyS[Yypt-2].list, YyS[Yypt-0].sym)
			}
			Errflag = 0
		}
	case 46:
		//line p.y:448
		{
			diag("',' expected; not ';'")
			YyVAL.list = YyS[Yypt-2].list
			if YyS[Yypt-2].list != nil {
				addsym(YyS[Yypt-2].list, YyS[Yypt-0].sym)
			}
		}
	case 47:
		//line p.y:455
		{
			YyVAL.list = newlist(Lsym)
			addsym(YyVAL.list, YyS[Yypt-0].sym)
		}
	case 48:
		//line p.y:461
		{
			YyVAL.list = newlist(Lsym)
		}
	case 49:
		//line p.y:466
		{
			YyVAL.list = newlist(Lsym)
			Errflag = 0
		}
	case 50:
		//line p.y:472
		{
			YyVAL.list = newlist(Lsym)
		}
	case 51:
		//line p.y:477
		{
			YyVAL.list = nil
		}
	case 52:
		//line p.y:483
		{
			YyVAL.sym = newparm(YyS[Yypt-2].sym, YyS[Yypt-0].sym.ttype, 0)
		}
	case 53:
		//line p.y:488
		{
			diag("'%s' is not a type name", YyS[Yypt-0].sym.name)
			YyVAL.sym = newparm(YyS[Yypt-2].sym, tundef, 0)
		}
	case 54:
		//line p.y:494
		{
			YyVAL.sym = newparm(YyS[Yypt-2].sym, YyS[Yypt-0].sym.ttype, 1)
		}
	case 55:
		//line p.y:499
		{
			diag("'%s' is not a type name", YyS[Yypt-0].sym.name)
			YyVAL.sym = newparm(YyS[Yypt-2].sym, tundef, 0)
		}
	case 56:
		//line p.y:505
		{
			diag("type name '%s' is an invalid parameter name", YyS[Yypt-2].sym.name)
			YyVAL.sym = newparm(YyS[Yypt-2].sym, YyS[Yypt-0].sym.ttype, 0)
		}
	case 57:
		//line p.y:511
		{
			diag("type name '%s' is an invalid parameter name", YyS[Yypt-2].sym.name)
			YyVAL.sym = newparm(YyS[Yypt-2].sym, YyS[Yypt-0].sym.ttype, 1)
		}
	case 58:
		YyVAL.ttype = YyS[Yypt-0].ttype
	case 59:
		YyVAL.ttype = YyS[Yypt-0].ttype
	case 60:
		YyVAL.ttype = YyS[Yypt-0].ttype
	case 61:
		YyVAL.ttype = YyS[Yypt-0].ttype
	case 62:
		YyVAL.ttype = YyS[Yypt-0].ttype
	case 63:
		YyVAL.ttype = YyS[Yypt-0].ttype
	case 64:
		YyVAL.ttype = YyS[Yypt-0].ttype
	case 65:
		//line p.y:533
		{
			YyVAL.ttype = newtype(Tptr)
			YyVAL.ttype.ref = YyS[Yypt-0].sym.ttype
		}
	case 66:
		//line p.y:539
		{
			ft := decltype(YyS[Yypt-0].sym, nil)
			YyVAL.ttype = newtype(Tptr)
			YyVAL.ttype.ref = ft.ttype
		}
	case 67:
		//line p.y:547
		{
			YyVAL.list = YyS[Yypt-2].list
			if YyVAL.list != nil {
				addsym(YyVAL.list, YyS[Yypt-0].sym)
			}
			Errflag = 0
		}
	case 68:
		//line p.y:554
		{
			YyVAL.list = newlist(Lsym)
			if YyVAL.list != nil {
				addsym(YyVAL.list, YyS[Yypt-0].sym)
			}
		}
	case 69:
		//line p.y:560
		{
			diag("identifier expected")
			YyVAL.list = nil
		}
	case 70:
		//line p.y:566
		{
			YyVAL.list = nil
		}
	case 71:
		//line p.y:571
		{
			YyVAL.list = YyS[Yypt-2].list
			Errflag = 0
		}
	case 72:
		//line p.y:577
		{
			YyVAL.list = nil
		}
	case 73:
		//line p.y:584
		{
			YyVAL.ttype = newordtype(YyS[Yypt-1].list)
		}
	case 74:
		//line p.y:590
		{
			YyVAL.ttype = newrangetype(YyS[Yypt-3].sym.ttype, YyS[Yypt-2].sym, YyS[Yypt-0].sym)
		}
	case 75:
		//line p.y:596
		{
			YyVAL.ttype = newarrytype(YyS[Yypt-3].sym.ttype, YyS[Yypt-0].sym.ttype)
		}
	case 76:
		//line p.y:601
		{
			if env.prog == nil {
				panic("missing program declaration")
			}
			YyVAL.ttype = newarrytype(newrangetype(nil, YyS[Yypt-5].sym, YyS[Yypt-3].sym), YyS[Yypt-0].sym.ttype)
		}
	case 77:
		//line p.y:608
		{
			t := newtype(Trec)
			Pushenv()
			env.rec = t
		}
	case 78:
		//line p.y:614
		{
			YyVAL.ttype = YyS[Yypt-0].ttype
		}
	case 79:
		//line p.y:620
		{
			YyVAL.ttype = env.rec
			YyVAL.ttype.fields = YyS[Yypt-1].list
			popenv()
			initrectype(YyVAL.ttype)
		}
	case 80:
		//line p.y:628
		{
			YyVAL.ttype = env.rec
			YyVAL.ttype.fields = newlist(Lsym)
			popenv()
			initrectype(YyVAL.ttype)
		}
	case 81:
		//line p.y:638
		{
			YyVAL.list = YyS[Yypt-1].list
			if YyVAL.list != nil {
				addsym(YyVAL.list, YyS[Yypt-0].sym)
			}
		}
	case 82:
		//line p.y:644
		{
			YyVAL.list = YyS[Yypt-1].list
			if YyVAL.list != nil {
				appsyms(YyVAL.list, YyS[Yypt-0].list)
			}
		}
	case 83:
		//line p.y:650
		{
			YyVAL.list = newlist(Lsym)
			if YyVAL.list != nil {
				addsym(YyVAL.list, YyS[Yypt-0].sym)
			}
		}
	case 84:
		//line p.y:657
		{
			diag("'%s' is a type name", YyS[Yypt-3].sym.name)
		}
	case 85:
		//line p.y:662
		{
			diag("type name expected; found '%s'", YyS[Yypt-1].sym.name)
		}
	case 86:
		//line p.y:667
		{
			YyVAL.sym = declfield(YyS[Yypt-3].sym, YyS[Yypt-1].sym.ttype)
		}
	case 87:
		//line p.y:673
		{
			setswfield(YyS[Yypt-1].list, YyS[Yypt-4].sym)
			YyVAL.list = YyS[Yypt-1].list
		}
	case 88:
		//line p.y:680
		{
			YyVAL.list = YyS[Yypt-1].list
			if YyVAL.list != nil {
				appsyms(YyVAL.list, YyS[Yypt-0].list)
			}
		}
	case 89:
		//line p.y:686
		{
			YyVAL.list = YyS[Yypt-0].list
		}
	case 90:
		//line p.y:692
		{
			setswval(YyS[Yypt-0].list, YyS[Yypt-2].sym)
			YyVAL.list = YyS[Yypt-0].list
		}
	case 91:
		//line p.y:699
		{
			Pushenv()
		}
	case 92:
		//line p.y:703
		{
			YyVAL.ttype = newtype(Tproc)
			YyVAL.ttype.parms = YyS[Yypt-1].list
			popenv()
		}
	case 93:
		//line p.y:711
		{
			Pushenv()
		}
	case 94:
		//line p.y:715
		{
			YyVAL.ttype = newtype(Tfunc)
			YyVAL.ttype.parms = YyS[Yypt-3].list
			YyVAL.ttype.rtype = YyS[Yypt-0].ttype
			popenv()
		}
	case 95:
		//line p.y:724
		{
			YyVAL.ttype = YyS[Yypt-0].sym.ttype
		}
	case 96:
		//line p.y:729
		{
			diag("type name expected; found '%s'", YyS[Yypt-0].sym.name)
			YyVAL.ttype = tundef
		}
	case 97:
		//line p.y:735
		{
			YyVAL.stmt = newbody(YyS[Yypt-1].list)
		}
	case 98:
		//line p.y:740
		{
			diag("empty block")
			YyVAL.stmt = newbody(newlist(Lstmt))
		}
	case 99:
		//line p.y:747
		{
			YyVAL.list = YyS[Yypt-1].list
			addstmt(YyVAL.list, YyS[Yypt-0].stmt)
		}
	case 100:
		//line p.y:753
		{
			YyVAL.list = newlist(Lstmt)
			addstmt(YyVAL.list, YyS[Yypt-0].stmt)
		}
	case 101:
		YyVAL.stmt = YyS[Yypt-0].stmt
	case 102:
		YyVAL.stmt = YyS[Yypt-0].stmt
	case 103:
		YyVAL.stmt = YyS[Yypt-0].stmt
	case 104:
		YyVAL.stmt = YyS[Yypt-0].stmt
	case 105:
		YyVAL.stmt = YyS[Yypt-0].stmt
	case 106:
		YyVAL.stmt = YyS[Yypt-0].stmt
	case 107:
		YyVAL.stmt = YyS[Yypt-0].stmt
	case 108:
		YyVAL.stmt = YyS[Yypt-0].stmt
	case 109:
		YyVAL.stmt = YyS[Yypt-0].stmt
	case 110:
		YyVAL.stmt = YyS[Yypt-0].stmt
	case 111:
		//line p.y:780
		{
			YyVAL.stmt = newstmt(0)
			diag("statement expected")
		}
	case 112:
		//line p.y:788
		{
			YyVAL.stmt = newstmt(';')
		}
	case 113:
		//line p.y:794
		{
			YyVAL.stmt = newstmt(RETURN)
			YyVAL.stmt.expr = YyS[Yypt-1].sym
			if env.prog == nil {
				panic("missing program declaration")
			}
			env.prog.prog.nrets++
		}
	case 114:
		//line p.y:803
		{
			YyVAL.stmt = newstmt(DO)
			YyVAL.stmt.expr = YyS[Yypt-2].sym
			YyVAL.stmt.stmt = YyS[Yypt-5].stmt
			cpsrc(YyVAL.stmt, YyS[Yypt-5].stmt)
			checkcond(YyVAL.stmt, YyVAL.stmt.expr)
		}
	case 115:
		//line p.y:813
		{
			YyVAL.stmt = newstmt(WHILE)
			YyVAL.stmt.expr = YyS[Yypt-2].sym
			YyVAL.stmt.stmt = YyS[Yypt-0].stmt
			YyVAL.stmt.sfname = YyS[Yypt-2].sym.fname
			YyVAL.stmt.lineno = YyS[Yypt-2].sym.lineno
			checkcond(YyVAL.stmt, YyVAL.stmt.expr)
		}
	case 116:
		//line p.y:824
		{
			YyVAL.stmt = newfor(YyS[Yypt-6].sym, YyS[Yypt-4].sym, YyS[Yypt-2].sym, YyS[Yypt-0].stmt)
			YyVAL.stmt.sfname = YyS[Yypt-6].sym.fname
			YyVAL.stmt.lineno = YyS[Yypt-6].sym.lineno
		}
	case 117:
		//line p.y:832
		{
			YyVAL.sym = newexpr(Sbinary, YyS[Yypt-1].ival, newvarnode(YyS[Yypt-2].sym), YyS[Yypt-0].sym)
		}
	case 118:
		//line p.y:838
		{
			YyVAL.ival = '<'
		}
	case 119:
		//line p.y:843
		{
			YyVAL.ival = '>'
		}
	case 120:
		//line p.y:848
		{
			YyVAL.ival = Ole
		}
	case 121:
		//line p.y:853
		{
			YyVAL.ival = Oge
		}
	case 122:
		//line p.y:859
		{
			YyVAL.stmt = YyS[Yypt-1].stmt
			YyVAL.stmt.thenarm = YyS[Yypt-0].stmt
		}
	case 123:
		//line p.y:865
		{
			YyVAL.stmt = YyS[Yypt-3].stmt
			YyVAL.stmt.thenarm = YyS[Yypt-2].stmt
			YyVAL.stmt.elsearm = YyS[Yypt-0].stmt
			if YyS[Yypt-0].stmt.op == '{' {
				YyS[Yypt-0].stmt.op = ELSE
			}
		}
	case 124:
		//line p.y:873
		{
			YyVAL.stmt = YyS[Yypt-3].stmt
			YyVAL.stmt.thenarm = YyS[Yypt-2].stmt
			YyVAL.stmt.elsearm = YyS[Yypt-0].stmt
		}
	case 125:
		//line p.y:881
		{
			YyVAL.stmt = newstmt(IF)
			YyVAL.stmt.cond = YyS[Yypt-1].sym
			checkcond(YyVAL.stmt, YyVAL.stmt.cond)
		}
	case 126:
		//line p.y:889
		{
			YyVAL.stmt = newassign(YyS[Yypt-3].sym, YyS[Yypt-1].sym)
		}
	case 127:
		//line p.y:894
		{
			diag("unexpected ':'")
		}
	case 128:
		//line p.y:898
		{
			YyVAL.stmt = newstmt(';')
		}
	case 129:
		//line p.y:904
		{
			YyVAL.stmt = newstmt(FCALL)
			YyVAL.stmt.fcall = newfcall(YyS[Yypt-4].sym, YyS[Yypt-2].list, Tproc)
		}
	case 130:
		YyVAL.list = YyS[Yypt-0].list
	case 131:
		//line p.y:913
		{
			YyVAL.list = newlist(Lsym)
		}
	case 132:
		//line p.y:919
		{
			YyVAL.list = YyS[Yypt-2].list
			if YyVAL.list != nil {
				addsym(YyVAL.list, YyS[Yypt-0].sym)
			}
		}
	case 133:
		//line p.y:925
		{
			YyVAL.list = newlist(Lsym)
			addsym(YyVAL.list, YyS[Yypt-0].sym)
		}
	case 134:
		//line p.y:932
		{
			YyVAL.stmt = newswitch(YyS[Yypt-4].sym, YyS[Yypt-1].list)
		}
	case 135:
		//line p.y:937
		{
			YyVAL.list = YyS[Yypt-1].list
			addstmt(YyVAL.list, YyS[Yypt-0].stmt)
		}
	case 136:
		//line p.y:943
		{
			YyVAL.list = newlist(Lstmt)
			addstmt(YyVAL.list, YyS[Yypt-0].stmt)
		}
	case 137:
		//line p.y:950
		{
			YyVAL.stmt = newstmt(CASE)
			YyVAL.stmt.expr = YyS[Yypt-2].sym
			YyVAL.stmt.stmt = newbody(YyS[Yypt-0].list)
			cpsrc(YyVAL.stmt, YyVAL.stmt.stmt)
		}
	case 138:
		//line p.y:958
		{
			YyVAL.stmt = newstmt(CASE)
			YyVAL.stmt.stmt = newbody(YyS[Yypt-0].list)
		}
	case 139:
		//line p.y:965
		{
			YyVAL.sym = newexpr(Sbinary, ',', YyS[Yypt-2].sym, YyS[Yypt-0].sym)
		}
	case 140:
		//line p.y:970
		{
			if !evaluated(YyS[Yypt-0].sym) {
				diag("case expression must be a constant")
			}
			YyVAL.sym = YyS[Yypt-0].sym
		}
	case 141:
		//line p.y:976
		{
			YyVAL.sym = newexpr(Sbinary, Odotdot, YyS[Yypt-2].sym, YyS[Yypt-0].sym)
			if !evaluated(YyS[Yypt-2].sym) {
				diag("case expression must be a constant")
			}
			if !evaluated(YyS[Yypt-0].sym) {
				diag("case expression must be a constant")
			}
		}
	case 142:
		YyVAL.sym = YyS[Yypt-0].sym
	case 143:
		//line p.y:985
		{
			YyVAL.sym = newexpr(Sbinary, '+', YyS[Yypt-2].sym, YyS[Yypt-0].sym)
		}
	case 144:
		//line p.y:990
		{
			YyVAL.sym = newexpr(Sbinary, '-', YyS[Yypt-2].sym, YyS[Yypt-0].sym)
		}
	case 145:
		//line p.y:995
		{
			YyVAL.sym = newexpr(Sbinary, '*', YyS[Yypt-2].sym, YyS[Yypt-0].sym)
		}
	case 146:
		//line p.y:1000
		{
			YyVAL.sym = newexpr(Sbinary, '/', YyS[Yypt-2].sym, YyS[Yypt-0].sym)
		}
	case 147:
		//line p.y:1005
		{
			YyVAL.sym = newexpr(Sbinary, Oand, YyS[Yypt-2].sym, YyS[Yypt-0].sym)
		}
	case 148:
		//line p.y:1010
		{
			YyVAL.sym = newexpr(Sbinary, Oor, YyS[Yypt-2].sym, YyS[Yypt-0].sym)
		}
	case 149:
		//line p.y:1015
		{
			YyVAL.sym = newexpr(Sbinary, Oeq, YyS[Yypt-2].sym, YyS[Yypt-0].sym)
		}
	case 150:
		//line p.y:1020
		{
			YyVAL.sym = newexpr(Sbinary, One, YyS[Yypt-2].sym, YyS[Yypt-0].sym)
		}
	case 151:
		//line p.y:1025
		{
			YyVAL.sym = newexpr(Sbinary, '%', YyS[Yypt-2].sym, YyS[Yypt-0].sym)
		}
	case 152:
		//line p.y:1030
		{
			YyVAL.sym = newexpr(Sbinary, '<', YyS[Yypt-2].sym, YyS[Yypt-0].sym)
		}
	case 153:
		//line p.y:1035
		{
			YyVAL.sym = newexpr(Sbinary, '>', YyS[Yypt-2].sym, YyS[Yypt-0].sym)
		}
	case 154:
		//line p.y:1040
		{
			YyVAL.sym = newexpr(Sbinary, Oge, YyS[Yypt-2].sym, YyS[Yypt-0].sym)
		}
	case 155:
		//line p.y:1045
		{
			YyVAL.sym = newexpr(Sbinary, Ole, YyS[Yypt-2].sym, YyS[Yypt-0].sym)
		}
	case 156:
		//line p.y:1050
		{
			YyVAL.sym = newexpr(Sbinary, Opow, YyS[Yypt-2].sym, YyS[Yypt-0].sym)
		}
	case 157:
		//line p.y:1055
		{
			YyVAL.sym = nil
		}
	case 158:
		YyVAL.sym = YyS[Yypt-0].sym
	case 159:
		//line p.y:1063
		{
			YyVAL.sym = YyS[Yypt-0].sym
		}
	case 160:
		//line p.y:1068
		{
			YyVAL.sym = newexpr(Sunary, Ouminus, YyS[Yypt-0].sym, nil)
		}
	case 161:
		//line p.y:1073
		{
			YyVAL.sym = newint(YyS[Yypt-0].ival, Oint, nil)
		}
	case 162:
		//line p.y:1078
		{
			YyVAL.sym = newint(YyS[Yypt-0].ival, Ochar, nil)
		}
	case 163:
		//line p.y:1083
		{
			YyVAL.sym = newreal(YyS[Yypt-0].rval, nil)
		}
	case 164:
		//line p.y:1088
		{
			YyVAL.sym = newstr(YyS[Yypt-0].sval)
		}
	case 165:
		//line p.y:1093
		{
			YyVAL.sym = newexpr(Sconst, Onil, nil, nil)
		}
	case 166:
		//line p.y:1098
		{
			YyVAL.sym = newint(1, Otrue, nil)
		}
	case 167:
		//line p.y:1103
		{
			YyVAL.sym = newint(0, Ofalse, nil)
		}
	case 168:
		//line p.y:1108
		{
			YyVAL.sym = newfcall(YyS[Yypt-3].sym, YyS[Yypt-1].list, Tfunc)
		}
	case 169:
		//line p.y:1113
		{
			YyVAL.sym = YyS[Yypt-1].sym
		}
	case 170:
		//line p.y:1118
		{
			YyVAL.sym = newexpr(Sunary, Onot, YyS[Yypt-0].sym, nil)
		}
	case 171:
		//line p.y:1123
		{
			YyVAL.sym = newaggr(YyS[Yypt-3].sym.ttype, YyS[Yypt-1].list)
		}
	case 172:
		//line p.y:1128
		{
			YyS[Yypt-0].sym.ttype = tderef(YyS[Yypt-0].sym.ttype)
			if YyS[Yypt-0].sym.ttype == tundef {
				diag("argument '%s' of len is undefined", YyS[Yypt-0].sym.name)
			}
			if tisatom(YyS[Yypt-0].sym.ttype) {
				YyVAL.sym = newint(1, Oint, nil)
			} else {
				YyVAL.sym = newint(tlen(YyS[Yypt-0].sym.ttype), Oint, nil)
			}
		}
	case 173:
		YyVAL.sym = YyS[Yypt-0].sym
	case 174:
		YyVAL.sym = YyS[Yypt-0].sym
	case 175:
		//line p.y:1144
		{
			YyVAL.sym = newexpr(Sbinary, '[', YyS[Yypt-3].sym, YyS[Yypt-1].sym)
		}
	case 176:
		//line p.y:1149
		{
			YyVAL.sym = fieldaccess(YyS[Yypt-2].sym, YyS[Yypt-0].sym.name)
		}
	case 177:
		//line p.y:1154
		{
			YyVAL.sym = newexpr(Sunary, '^', YyS[Yypt-1].sym, nil)
		}
	case 178:
		//line p.y:1161
		{
			YyVAL.sym = newvarnode(YyS[Yypt-0].sym)
		}
	case 179:
		//line p.y:1166
		{
			YyVAL.sym = newexpr(Sbinary, '[', YyS[Yypt-3].sym, YyS[Yypt-1].sym)
		}
	case 180:
		//line p.y:1171
		{
			YyVAL.sym = fieldaccess(YyS[Yypt-2].sym, YyS[Yypt-0].sym.name)
		}
	case 181:
		//line p.y:1176
		{
			diag("'%s' is a type name", YyS[Yypt-0].sym.name)
			YyVAL.sym = YyS[Yypt-2].sym
		}
	case 182:
		//line p.y:1182
		{
			YyVAL.sym = newexpr(Sunary, '^', YyS[Yypt-1].sym, nil)
		}
	case 183:
		//line p.y:1187
		{
			diag("'%s' is a type name", YyS[Yypt-0].sym.name)
			YyVAL.sym = newexpr(Snone, 0, nil, nil)
		}
	}
	goto Yystack /* stack new state and value */
}
