//line parse.y:8
package ql

import __yyfmt__ "fmt"

//line parse.y:8
import (
	"clive/app"
	"strings"
)

//line parse.y:17
type yySymType struct {
	yys  int
	cval rune
	sval string
	nd   *Nd
	bval bool
	rdr  Redirs
}

const NAME = 57346
const NL = 57347
const INBLK = 57348
const RAWINBLK = 57349
const SINGLEINBLK = 57350
const TEEBLK = 57351
const PIPEBLK = 57352
const LEN = 57353
const INPIPE = 57354
const FOR = 57355
const APP = 57356
const WHILE = 57357
const FUNC = 57358
const INTERRUPT = 57359
const ERROR = 57360
const OR = 57361
const AND = 57362
const GFPIPE = 57363

var yyToknames = []string{
	"NAME",
	"NL",
	"INBLK",
	"RAWINBLK",
	"SINGLEINBLK",
	"TEEBLK",
	"PIPEBLK",
	"LEN",
	"INPIPE",
	"FOR",
	"APP",
	"WHILE",
	"FUNC",
	"INTERRUPT",
	"ERROR",
	"OR",
	"AND",
	"GFPIPE",
	"'^'",
}
var yyStatenames = []string{}

const yyEofCode = 1
const yyErrCode = 2
const yyMaxDepth = 200

//line parse.y:605
func yprintf(l interface{}, fmts string, args ...interface{}) {
	app.Dprintf(fmts, args...)
}

//line yacctab:1
var yyExca = []int{
	-1, 0,
	1, 2,
	5, 16,
	19, 16,
	20, 16,
	32, 16,
	-2, 0,
	-1, 1,
	1, -1,
	-2, 0,
	-1, 2,
	1, 1,
	5, 16,
	19, 16,
	20, 16,
	32, 16,
	-2, 0,
	-1, 134,
	4, 25,
	-2, 55,
}

const yyNprod = 89
const yyPrivate = 57344

var yyTokenNames []string
var yyStates []string

const yyLast = 280

var yyAct = []int{

	26, 27, 21, 19, 64, 111, 35, 4, 56, 9,
	11, 10, 38, 81, 134, 61, 52, 135, 49, 44,
	51, 133, 62, 63, 36, 29, 59, 67, 66, 150,
	60, 47, 141, 139, 128, 6, 68, 17, 46, 30,
	31, 32, 23, 33, 29, 16, 24, 28, 25, 8,
	77, 37, 83, 84, 79, 130, 22, 112, 18, 113,
	100, 15, 82, 91, 87, 147, 28, 146, 62, 63,
	99, 125, 124, 95, 90, 92, 123, 122, 121, 96,
	93, 98, 78, 106, 102, 103, 104, 105, 109, 49,
	69, 30, 31, 32, 23, 33, 29, 16, 24, 70,
	25, 117, 42, 40, 14, 127, 39, 148, 22, 145,
	62, 63, 132, 15, 120, 116, 131, 71, 28, 115,
	48, 50, 114, 137, 138, 136, 108, 101, 89, 86,
	140, 80, 72, 53, 142, 143, 41, 17, 76, 30,
	31, 32, 23, 33, 29, 16, 24, 149, 25, 3,
	62, 63, 34, 75, 74, 73, 22, 119, 18, 118,
	107, 15, 126, 65, 13, 49, 28, 30, 31, 32,
	23, 33, 29, 5, 24, 49, 25, 30, 31, 32,
	2, 33, 29, 49, 22, 30, 31, 32, 1, 33,
	29, 45, 55, 54, 28, 151, 58, 57, 88, 43,
	7, 110, 144, 49, 28, 30, 31, 32, 12, 33,
	29, 49, 28, 30, 31, 32, 20, 33, 29, 94,
	0, 0, 49, 129, 30, 31, 32, 0, 33, 29,
	0, 0, 28, 112, 0, 0, 0, 0, 0, 0,
	28, 97, 49, 0, 30, 31, 32, 0, 33, 29,
	49, 28, 30, 31, 32, 0, 33, 29, 0, 0,
	0, 85, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 28, 0, 0, 0, 0, 0, 0, 0, 28,
}
var yyPact = []int{

	33, -1000, 33, -1000, 19, 19, 101, 84, 132, 82,
	-1000, -12, -1000, -1000, 10, 161, 161, -10, 129, -1000,
	1, 246, -1000, -1000, 246, 85, 68, -1000, 95, 128,
	-1000, -1000, -1000, -1000, -1000, -1000, -1000, -1000, -1000, -1000,
	133, 59, 133, -1000, 127, 36, -1000, -1000, 10, -1000,
	10, 238, 125, -1000, -1000, 1, -1000, -1000, -1000, 124,
	37, 36, 68, -1000, 56, 133, 55, 218, 58, 14,
	34, 123, -1000, 85, 85, 85, 85, 82, -1000, -1000,
	-1000, -1000, 122, 68, -1000, 207, 32, -1000, -1000, -1000,
	118, 115, 111, -1000, 19, -1000, -1000, -1000, -1000, -1000,
	110, -1000, 54, 53, 52, 48, 47, 100, 7, 199,
	31, -1000, 108, -9, -1000, -13, -1000, 133, -1000, -1000,
	6, -1000, -1000, -1000, -1000, -1000, 161, -1000, -1000, -1000,
	-1000, -1000, 5, 179, -1000, 105, -1000, 43, 41, -1000,
	-1000, 103, 68, -1000, 246, 2, -1000, -1000, -1000, 171,
	-1000, -1000,
}
var yyPgo = []int{

	0, 219, 11, 10, 216, 208, 4, 104, 201, 5,
	2, 0, 1, 3, 7, 200, 9, 199, 13, 198,
	6, 8, 197, 196, 193, 192, 191, 188, 180, 149,
	173, 164, 163, 162, 160, 159, 157, 155, 154, 153,
	138,
}
var yyR1 = []int{

	0, 27, 27, 28, 28, 29, 29, 29, 14, 15,
	15, 16, 16, 2, 2, 2, 2, 32, 6, 1,
	1, 30, 31, 33, 33, 18, 18, 26, 26, 3,
	3, 3, 34, 7, 7, 13, 4, 4, 4, 35,
	4, 36, 4, 24, 24, 25, 25, 21, 21, 22,
	19, 19, 23, 23, 23, 23, 17, 17, 17, 5,
	5, 5, 5, 5, 5, 5, 37, 12, 38, 12,
	39, 12, 40, 12, 8, 8, 9, 20, 20, 10,
	10, 10, 10, 11, 11, 11, 11, 11, 11,
}
var yyR2 = []int{

	0, 1, 0, 2, 1, 2, 2, 2, 1, 3,
	1, 3, 1, 2, 1, 1, 0, 0, 2, 3,
	1, 5, 2, 1, 0, 3, 0, 1, 1, 1,
	2, 2, 0, 6, 1, 2, 1, 3, 3, 0,
	6, 0, 6, 1, 0, 2, 1, 1, 1, 2,
	1, 0, 3, 3, 6, 4, 1, 2, 0, 3,
	3, 5, 6, 6, 8, 5, 0, 4, 0, 4,
	0, 4, 0, 4, 2, 1, 4, 1, 1, 2,
	2, 1, 1, 1, 2, 3, 5, 2, 3,
}
var yyChk = []int{

	-1000, -27, -28, -29, -14, -30, 2, -15, 16, -16,
	-2, -3, -5, -31, -7, 28, 12, 4, 25, -13,
	-4, -10, 23, 9, 13, 15, -11, -12, 33, 11,
	6, 7, 8, 10, -29, -20, 5, 32, -20, 5,
	19, 4, 20, -17, 31, -26, 28, 21, -7, 4,
	-7, 30, 26, 4, -24, -25, -21, -22, -23, 25,
	29, 14, -11, -12, -6, -32, -6, -10, -3, 22,
	4, 22, 4, -37, -38, -39, -40, -16, 23, -2,
	4, -18, 26, -11, -12, 23, 4, -21, -19, 4,
	-18, 26, -18, 24, -1, -14, 24, 23, 23, -11,
	26, 4, -3, -3, -3, -3, -6, -34, 4, -10,
	-8, -9, 26, 27, 4, 4, 4, -20, -35, -36,
	4, 24, 24, 24, 24, 24, -33, 5, 27, 24,
	24, -9, 4, 30, 27, 30, -14, -6, -6, 27,
	-13, 27, -11, -12, 23, 4, 24, 24, 4, -10,
	27, 24,
}
var yyDef = []int{

	-2, -2, -2, 4, 0, 0, 0, 8, 0, 10,
	12, 58, 14, 15, 29, 0, 0, 83, 0, 34,
	44, 36, 17, 17, 0, 0, 81, 82, 0, 0,
	66, 68, 70, 72, 3, 5, 77, 78, 6, 7,
	16, 0, 16, 13, 56, 26, 27, 28, 30, 83,
	31, 0, 0, 22, 35, 43, 46, 47, 48, 51,
	26, 26, 79, 80, 0, 16, 0, 0, 0, 0,
	84, 0, 87, 0, 0, 0, 0, 9, 17, 11,
	57, 32, 0, 59, 60, 0, 0, 45, 49, 50,
	0, 0, 0, 37, 18, 20, 38, 39, 41, 88,
	0, 85, 0, 0, 0, 0, 0, 24, 0, 0,
	0, 75, 0, 0, 52, 0, 53, 16, 17, 17,
	0, 67, 69, 71, 73, 21, 0, 23, 25, 61,
	65, 74, 0, 0, -2, 0, 19, 0, 0, 86,
	33, 0, 62, 63, 0, 0, 40, 42, 76, 0,
	54, 64,
}
var yyTok1 = []int{

	1, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 33, 3, 31, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 32,
	25, 30, 29, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 26, 3, 27, 22, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 23, 28, 24,
}
var yyTok2 = []int{

	2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
	12, 13, 14, 15, 16, 17, 18, 19, 20, 21,
}
var yyTok3 = []int{
	0,
}

//line yaccpar:1

/*	parser for yacc output	*/

var yyDebug = 0

type yyLexer interface {
	Lex(lval *yySymType) int
	Error(s string)
}

const yyFlag = -1000

func yyTokname(c int) string {
	// 4 is TOKSTART above
	if c >= 4 && c-4 < len(yyToknames) {
		if yyToknames[c-4] != "" {
			return yyToknames[c-4]
		}
	}
	return __yyfmt__.Sprintf("tok-%v", c)
}

func yyStatname(s int) string {
	if s >= 0 && s < len(yyStatenames) {
		if yyStatenames[s] != "" {
			return yyStatenames[s]
		}
	}
	return __yyfmt__.Sprintf("state-%v", s)
}

func yylex1(lex yyLexer, lval *yySymType) int {
	c := 0
	char := lex.Lex(lval)
	if char <= 0 {
		c = yyTok1[0]
		goto out
	}
	if char < len(yyTok1) {
		c = yyTok1[char]
		goto out
	}
	if char >= yyPrivate {
		if char < yyPrivate+len(yyTok2) {
			c = yyTok2[char-yyPrivate]
			goto out
		}
	}
	for i := 0; i < len(yyTok3); i += 2 {
		c = yyTok3[i+0]
		if c == char {
			c = yyTok3[i+1]
			goto out
		}
	}

out:
	if c == 0 {
		c = yyTok2[1] /* unknown char */
	}
	if yyDebug >= 3 {
		__yyfmt__.Printf("lex %s(%d)\n", yyTokname(c), uint(char))
	}
	return c
}

func yyParse(yylex yyLexer) int {
	var yyn int
	var yylval yySymType
	var yyVAL yySymType
	yyS := make([]yySymType, yyMaxDepth)

	Nerrs := 0   /* number of errors */
	Errflag := 0 /* error recovery flag */
	yystate := 0
	yychar := -1
	yyp := -1
	goto yystack

ret0:
	return 0

ret1:
	return 1

yystack:
	/* put a state and value onto the stack */
	if yyDebug >= 4 {
		__yyfmt__.Printf("char %v in %v\n", yyTokname(yychar), yyStatname(yystate))
	}

	yyp++
	if yyp >= len(yyS) {
		nyys := make([]yySymType, len(yyS)*2)
		copy(nyys, yyS)
		yyS = nyys
	}
	yyS[yyp] = yyVAL
	yyS[yyp].yys = yystate

yynewstate:
	yyn = yyPact[yystate]
	if yyn <= yyFlag {
		goto yydefault /* simple state */
	}
	if yychar < 0 {
		yychar = yylex1(yylex, &yylval)
	}
	yyn += yychar
	if yyn < 0 || yyn >= yyLast {
		goto yydefault
	}
	yyn = yyAct[yyn]
	if yyChk[yyn] == yychar { /* valid shift */
		yychar = -1
		yyVAL = yylval
		yystate = yyn
		if Errflag > 0 {
			Errflag--
		}
		goto yystack
	}

yydefault:
	/* default state action */
	yyn = yyDef[yystate]
	if yyn == -2 {
		if yychar < 0 {
			yychar = yylex1(yylex, &yylval)
		}

		/* look through exception table */
		xi := 0
		for {
			if yyExca[xi+0] == -1 && yyExca[xi+1] == yystate {
				break
			}
			xi += 2
		}
		for xi += 2; ; xi += 2 {
			yyn = yyExca[xi+0]
			if yyn < 0 || yyn == yychar {
				break
			}
		}
		yyn = yyExca[xi+1]
		if yyn < 0 {
			goto ret0
		}
	}
	if yyn == 0 {
		/* error ... attempt to resume parsing */
		switch Errflag {
		case 0: /* brand new error */
			yylex.Error("syntax error")
			Nerrs++
			if yyDebug >= 1 {
				__yyfmt__.Printf("%s", yyStatname(yystate))
				__yyfmt__.Printf(" saw %s\n", yyTokname(yychar))
			}
			fallthrough

		case 1, 2: /* incompletely recovered error ... try again */
			Errflag = 3

			/* find a state where "error" is a legal shift action */
			for yyp >= 0 {
				yyn = yyPact[yyS[yyp].yys] + yyErrCode
				if yyn >= 0 && yyn < yyLast {
					yystate = yyAct[yyn] /* simulate a shift of "error" */
					if yyChk[yystate] == yyErrCode {
						goto yystack
					}
				}

				/* the current p has no shift on "error", pop stack */
				if yyDebug >= 2 {
					__yyfmt__.Printf("error recovery pops state %d\n", yyS[yyp].yys)
				}
				yyp--
			}
			/* there is no state on the stack with an error shift ... abort */
			goto ret1

		case 3: /* no shift yet; clobber input char */
			if yyDebug >= 2 {
				__yyfmt__.Printf("error recovery discards %s\n", yyTokname(yychar))
			}
			if yychar == yyEofCode {
				goto ret1
			}
			yychar = -1
			goto yynewstate /* try again in the same state */
		}
	}

	/* reduction by production yyn */
	if yyDebug >= 2 {
		__yyfmt__.Printf("reduce %v in:\n\t%v\n", yyn, yyStatname(yystate))
	}

	yynt := yyn
	yypt := yyp
	_ = yypt // guard against "declared and not used"

	yyp -= yyR2[yyn]
	yyVAL = yyS[yyp+1]

	/* consult goto table to find next state */
	yyn = yyR1[yyn]
	yyg := yyPgo[yyn]
	yyj := yyg + yyS[yyp].yys + 1

	if yyj >= yyLast {
		yystate = yyAct[yyg]
	} else {
		yystate = yyAct[yyj]
		if yyChk[yystate] != -yyn {
			yystate = yyAct[yyg]
		}
	}
	// dummy call; replaced with literal code
	switch yynt {

	case 5:
		//line parse.y:48
		{
			x := yylex.(*xCmd)
			if yyS[yypt-1].nd != nil && yyS[yypt-1].nd.Kind != Nnop && !x.interrupted {
				if x.nerrors > 0 {
					yprintf("ERR %s\n", yyS[yypt-1].nd.sprint())
				} else {
					yprintf("%s\n", yyS[yypt-1].nd.sprint())
					x.Run(yyS[yypt-1].nd)
				}
			}
		}
	case 7:
		//line parse.y:61
		{
			x := yylex.(*xCmd)
			x.nerrors = 0
			// Discard all errors only at top-level
			// so errors within commands discard full commands.
		}
	case 8:
		//line parse.y:72
		{
			yyVAL.nd = yyS[yypt-0].nd
			// If it's or(and(x)) then return just x
			if yyVAL.nd != nil && len(yyVAL.nd.Child) == 1 {
				c := yyVAL.nd.Child[0]
				if c != nil && len(c.Child) == 1 {
					yyVAL.nd = c.Child[0]
				}
			}
			// XXX: TODO: must check out that we don't have weird
			// &s in the pipes used in the cond.
		}
	case 9:
		//line parse.y:88
		{
			yyVAL.nd = yyS[yypt-2].nd.Add(yyS[yypt-0].nd)
		}
	case 10:
		//line parse.y:92
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newList(Ncond, yyS[yypt-0].nd)
		}
	case 11:
		//line parse.y:100
		{
			yyVAL.nd = yyS[yypt-2].nd.Add(yyS[yypt-0].nd)
		}
	case 12:
		//line parse.y:104
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newList(Ncond, yyS[yypt-0].nd)
		}
	case 13:
		//line parse.y:112
		{
			yyVAL.nd = yyS[yypt-1].nd
			if yyS[yypt-0].sval != "" {
				yyVAL.nd.Args = append(yyVAL.nd.Args, yyS[yypt-0].sval)
			}
		}
	case 14:
		//line parse.y:119
		{
			yyVAL.nd = yyS[yypt-0].nd
		}
	case 15:
		//line parse.y:123
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newNd(Nnop)
		}
	case 16:
		//line parse.y:128
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newNd(Nnop)
		}
	case 17:
		//line parse.y:137
		{
			x := yylex.(*xCmd)
			x.lvl++
			x.plvl++
			x.promptLvl(1)
		}
	case 18:
		//line parse.y:144
		{
			x := yylex.(*xCmd)
			x.lvl--
			x.plvl--
			if x.lvl == 0 {
				x.promptLvl(0)
			}
			yyVAL.nd = yyS[yypt-0].nd
		}
	case 19:
		//line parse.y:157
		{
			yyVAL.nd = yyS[yypt-2].nd.Add(yyS[yypt-0].nd)
		}
	case 20:
		//line parse.y:161
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newList(Nblk, yyS[yypt-0].nd)
		}
	case 21:
		//line parse.y:169
		{
			x := yylex.(*xCmd)
			if x.nerrors == 0 {
				f := x.newNd(Nfunc, yyS[yypt-3].sval).Add(yyS[yypt-1].nd)
				yprintf("%s\n", f.sprint())
				x.funcs[yyS[yypt-3].sval] = f
			}
		}
	case 22:
		//line parse.y:181
		{
			x := yylex.(*xCmd)
			if x.nerrors == 0 {
				yprintf("< %s\n", yyS[yypt-0].sval)
				x.source(yyS[yypt-0].sval)
			}
		}
	case 25:
		//line parse.y:197
		{
			yyVAL.sval = yyS[yypt-1].sval
		}
	case 26:
		//line parse.y:201
		{
			yyVAL.sval = "1"
		}
	case 27:
		//line parse.y:208
		{
			yyVAL.bval = false
		}
	case 28:
		//line parse.y:212
		{
			yyVAL.bval = true
		}
	case 29:
		//line parse.y:219
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.pipeRewrite(yyS[yypt-0].nd)
		}
	case 30:
		//line parse.y:224
		{
			yyVAL.nd = yyS[yypt-0].nd
			x := yylex.(*xCmd)
			if len(yyS[yypt-0].nd.Child) > 0 {
				c := yyS[yypt-0].nd.Child[0]
				c.Redirs = append(c.Redirs, x.newRedir("0", "/dev/null", false)...)
				x.noDups(c.Redirs)
			}
		}
	case 31:
		//line parse.y:234
		{
			yyVAL.nd = yyS[yypt-0].nd
		}
	case 32:
		//line parse.y:241
		{
			x := yylex.(*xCmd)
			x.promptLvl(1)
		}
	case 33:
		//line parse.y:246
		{
			x := yylex.(*xCmd)
			x.promptLvl(0)
			last := yyS[yypt-5].nd.Last()
			if strings.Contains(yyS[yypt-3].sval, "0") {
				x.Errs("bad redirect for pipe")
			}
			if yyS[yypt-4].bval {
				if yyS[yypt-5].nd.IsGet {
					x.Errs("'||' valid only in the first component of a pipe.")
				}
				yyS[yypt-5].nd.IsGet = true
			}
			last.Redirs = append(last.Redirs, x.newRedir(yyS[yypt-3].sval, "|", false)...)
			x.noDups(last.Redirs)
			yyS[yypt-0].nd.Redirs = append(yyS[yypt-0].nd.Redirs, x.newRedir("0", "|", false)...)
			x.noDups(yyS[yypt-0].nd.Redirs)
			yyVAL.nd = yyS[yypt-5].nd.Add(yyS[yypt-0].nd)
		}
	case 34:
		//line parse.y:266
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newList(Npipe, yyS[yypt-0].nd)
		}
	case 35:
		//line parse.y:274
		{
			x := yylex.(*xCmd)
			yyVAL.nd = yyS[yypt-1].nd
			x.noDups(yyS[yypt-0].rdr)
			yyVAL.nd.Redirs = yyS[yypt-0].rdr
		}
	case 36:
		//line parse.y:284
		{
			yyS[yypt-0].nd.Kind = Nexec
			yyVAL.nd = yyS[yypt-0].nd
		}
	case 37:
		//line parse.y:289
		{
			yyVAL.nd = yyS[yypt-1].nd
		}
	case 38:
		//line parse.y:293
		{
			yyS[yypt-1].nd.Kind = Nteeblk
			yyVAL.nd = yyS[yypt-1].nd
		}
	case 39:
		//line parse.y:298
		{
			x := yylex.(*xCmd)
			x.lvl++
			x.plvl++
		}
	case 40:
		//line parse.y:304
		{
			x := yylex.(*xCmd)
			x.lvl--
			x.plvl--
			yyVAL.nd = x.newList(Nfor, yyS[yypt-4].nd, yyS[yypt-1].nd)
			if yyS[yypt-4].nd.Kind == Nnames && len(yyS[yypt-4].nd.Child) == 1 {
				yyVAL.nd.IsGet = true
			}
		}
	case 41:
		//line parse.y:314
		{
			x := yylex.(*xCmd)
			x.lvl++
			x.plvl++
		}
	case 42:
		//line parse.y:320
		{
			x := yylex.(*xCmd)
			x.lvl--
			x.plvl--
			yyVAL.nd = x.newList(Nwhile, yyS[yypt-4].nd, yyS[yypt-1].nd)
		}
	case 43:
		//line parse.y:330
		{
			yyVAL.rdr = yyS[yypt-0].rdr
		}
	case 44:
		//line parse.y:334
		{
			yyVAL.rdr = nil
		}
	case 45:
		//line parse.y:341
		{
			yyVAL.rdr = append(yyS[yypt-1].rdr, yyS[yypt-0].rdr...)
		}
	case 46:
		yyVAL.rdr = yyS[yypt-0].rdr
	case 47:
		yyVAL.rdr = yyS[yypt-0].rdr
	case 48:
		yyVAL.rdr = yyS[yypt-0].rdr
	case 49:
		//line parse.y:355
		{
			x := yylex.(*xCmd)
			yyVAL.rdr = x.newRedir("0", yyS[yypt-0].sval, false)
		}
	case 50:
		yyVAL.sval = yyS[yypt-0].sval
	case 51:
		//line parse.y:364
		{
			yyVAL.sval = "/dev/null"
		}
	case 52:
		//line parse.y:370
		{
			x := yylex.(*xCmd)
			if strings.Contains(yyS[yypt-1].sval, "0") {
				x.Errs("bad redirect for '>'")
			}
			yyVAL.rdr = x.newRedir(yyS[yypt-1].sval, yyS[yypt-0].sval, false)
		}
	case 53:
		//line parse.y:378
		{
			x := yylex.(*xCmd)
			if strings.Contains(yyS[yypt-1].sval, "0") {
				x.Errs("bad redirect for '>'")
			}
			yyVAL.rdr = x.newRedir(yyS[yypt-1].sval, yyS[yypt-0].sval, true)
		}
	case 54:
		//line parse.y:386
		{
			x := yylex.(*xCmd)
			yyVAL.rdr = x.newDup(yyS[yypt-3].sval, yyS[yypt-1].sval)
		}
	case 55:
		//line parse.y:391
		{
			x := yylex.(*xCmd)
			toks := strings.SplitN(yyS[yypt-1].sval, "=", -1)
			if len(toks) != 2 {
				x.Errs("bad [] redirection")
				toks = append(toks, "???")
			}
			yyVAL.rdr = x.newDup(toks[0], toks[1])
		}
	case 56:
		//line parse.y:404
		{
			yyVAL.sval = "&"
		}
	case 57:
		//line parse.y:408
		{
			yyVAL.sval = yyS[yypt-0].sval
		}
	case 58:
		//line parse.y:412
		{
			yyVAL.sval = ""
		}
	case 59:
		//line parse.y:419
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newNd(Nset, yyS[yypt-2].sval).Add(yyS[yypt-0].nd)
		}
	case 60:
		//line parse.y:424
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newNd(Nset, yyS[yypt-2].sval).Add(yyS[yypt-0].nd)
		}
	case 61:
		//line parse.y:429
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newNd(Nset, yyS[yypt-4].sval).Add(yyS[yypt-1].nd.Child...)
		}
	case 62:
		//line parse.y:434
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newNd(Nset, yyS[yypt-5].sval, yyS[yypt-3].sval).Add(yyS[yypt-0].nd)
		}
	case 63:
		//line parse.y:439
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newNd(Nset, yyS[yypt-5].sval, yyS[yypt-3].sval).Add(yyS[yypt-0].nd)
		}
	case 64:
		//line parse.y:444
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newNd(Nset, yyS[yypt-7].sval, yyS[yypt-5].sval).Add(yyS[yypt-1].nd.Child...)
		}
	case 65:
		//line parse.y:449
		{
			yyS[yypt-1].nd.Args = append(yyS[yypt-1].nd.Args, yyS[yypt-4].sval)
			yyVAL.nd = yyS[yypt-1].nd
		}
	case 66:
		//line parse.y:457
		{
			x := yylex.(*xCmd)
			x.lvl++
		}
	case 67:
		//line parse.y:462
		{
			x := yylex.(*xCmd)
			x.lvl--
			yyVAL.nd = x.newList(Ninblk, yyS[yypt-1].nd)
			// The last child of the pipe must have its output redirected to the pipe.
			if last := yyS[yypt-1].nd.Last(); last != nil {
				last.Redirs = append(last.Redirs, x.newRedir("1", "|", false)...)
				x.noDups(last.Redirs)
			}
		}
	case 68:
		//line parse.y:473
		{
			x := yylex.(*xCmd)
			x.lvl++
		}
	case 69:
		//line parse.y:478
		{
			x := yylex.(*xCmd)
			x.lvl--
			yyVAL.nd = x.newList(Nrawinblk, yyS[yypt-1].nd)
			// The last child of the pipe must have its output redirected to the pipe.
			if last := yyS[yypt-1].nd.Last(); last != nil {
				last.Redirs = append(last.Redirs, x.newRedir("1", "|", false)...)
				x.noDups(last.Redirs)
			}
		}
	case 70:
		//line parse.y:489
		{
			x := yylex.(*xCmd)
			x.lvl++
		}
	case 71:
		//line parse.y:494
		{
			x := yylex.(*xCmd)
			x.lvl--
			yyVAL.nd = x.newList(Nsingleinblk, yyS[yypt-1].nd)
			// The last child of the pipe must have its output redirected to the pipe.
			if last := yyS[yypt-1].nd.Last(); last != nil {
				last.Redirs = append(last.Redirs, x.newRedir("1", "|", false)...)
				x.noDups(last.Redirs)
			}
		}
	case 72:
		//line parse.y:505
		{
			x := yylex.(*xCmd)
			x.lvl++
		}
	case 73:
		//line parse.y:510
		{
			x := yylex.(*xCmd)
			x.lvl--
			yyVAL.nd = x.newList(Npipeblk, yyS[yypt-1].nd)
			// The last child of the pipe must have its output redirected to the pipe.
			if last := yyS[yypt-1].nd.Last(); last != nil {
				last.Redirs = append(last.Redirs, x.newRedir("1", "|", false)...)
				x.noDups(last.Redirs)
			}
		}
	case 74:
		//line parse.y:524
		{
			yyVAL.nd = yyS[yypt-1].nd.Add(yyS[yypt-0].nd)
		}
	case 75:
		//line parse.y:528
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newList(Nset, yyS[yypt-0].nd)
		}
	case 76:
		//line parse.y:535
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newNd(Nset, yyS[yypt-2].sval, yyS[yypt-0].sval)
		}
	case 77:
		//line parse.y:543
		{
			yyVAL.cval = '\n'
		}
	case 78:
		//line parse.y:547
		{
			yyVAL.cval = ';'
		}
	case 79:
		//line parse.y:554
		{
			yyVAL.nd = yyS[yypt-1].nd.Add(yyS[yypt-0].nd)
		}
	case 80:
		//line parse.y:558
		{
			yyVAL.nd = yyS[yypt-1].nd.Add(yyS[yypt-0].nd)
		}
	case 81:
		//line parse.y:562
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newList(Nnames, yyS[yypt-0].nd)
		}
	case 82:
		//line parse.y:567
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newList(Nnames, yyS[yypt-0].nd)
		}
	case 83:
		//line parse.y:575
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newNd(Nname, yyS[yypt-0].sval)
		}
	case 84:
		//line parse.y:580
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newNd(Nval, yyS[yypt-0].sval)
		}
	case 85:
		//line parse.y:585
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newNd(Njoin, yyS[yypt-0].sval)
		}
	case 86:
		//line parse.y:590
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newNd(Nval, yyS[yypt-3].sval, yyS[yypt-1].sval)
		}
	case 87:
		//line parse.y:595
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newNd(Nlen, yyS[yypt-0].sval)
		}
	case 88:
		//line parse.y:600
		{
			x := yylex.(*xCmd)
			yyVAL.nd = x.newList(Napp, yyS[yypt-2].nd, yyS[yypt-0].nd)
		}
	}
	goto yystack /* stack new state and value */
}
