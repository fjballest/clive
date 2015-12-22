//line parse.y:6
package xp

import __yyfmt__ "fmt"

//line parse.y:6
import (
	"clive/dbg"
	"math"
	"os"
	"time"
)

var (
	debugYacc bool
	yprintf   = dbg.FlagPrintf(os.Stderr, &debugYacc)
	lvl       int
	result    value
)

//line parse.y:24
type yySymType struct {
	yys  int
	ival uint64
	fval float64
	sval string
	tval time.Time
	vval interface{}
}

const INT = 57346
const NUM = 57347
const FUNC = 57348
const NAME = 57349
const TIME = 57350
const OR = 57351
const AND = 57352
const EQN = 57353
const NEQ = 57354
const LE = 57355
const GE = 57356
const UMINUS = 57357

var yyToknames = []string{
	"INT",
	"NUM",
	"FUNC",
	"NAME",
	"TIME",
	"OR",
	"AND",
	" =",
	"EQN",
	"NEQ",
	" <",
	" >",
	"LE",
	"GE",
	" +",
	" -",
	" *",
	" /",
	" %",
	" &",
	" |",
	"UMINUS",
	" !",
	" ^",
}
var yyStatenames = []string{}

const yyEofCode = 1
const yyErrCode = 2
const yyMaxDepth = 200

//line parse.y:158
var funcs = map[string]func(float64) float64{
	"abs":   math.Abs,
	"acos":  math.Acos,
	"acosh": math.Acosh,
	"asin":  math.Asin,
	"asinh": math.Asinh,
	"atan":  math.Atan,
	"atanh": math.Atanh,
	"cbrt":  math.Cbrt,
	"cos":   math.Cos,
	"cosh":  math.Cosh,
	"exp":   math.Exp,
	"exp2":  math.Exp2,
	"floor": math.Floor,
	"Γ":     math.Gamma,
	"log":   math.Log,
	"log10": math.Log10,
	"log2":  math.Log2,
	"sin":   math.Sin,
	"sinh":  math.Sinh,
	"sqrt":  math.Sqrt,
	"tan":   math.Tan,
	"tanh":  math.Tanh,
	"trunc": math.Trunc,
}

//line yacctab:1
var yyExca = []int{
	-1, 1,
	1, -1,
	-2, 0,
}

const yyNprod = 27
const yyPrivate = 57344

var yyTokenNames []string
var yyStates []string

const yyLast = 131

var yyAct = []int{

	2, 1, 0, 0, 28, 29, 30, 0, 0, 0,
	0, 31, 32, 33, 34, 35, 36, 37, 38, 39,
	40, 41, 42, 43, 44, 45, 46, 47, 48, 25,
	24, 21, 22, 23, 17, 18, 19, 20, 12, 13,
	14, 15, 16, 26, 27, 0, 0, 0, 0, 49,
	25, 24, 21, 22, 23, 17, 18, 19, 20, 12,
	13, 14, 15, 16, 26, 27, 24, 21, 22, 23,
	17, 18, 19, 20, 12, 13, 14, 15, 16, 26,
	27, 21, 22, 23, 17, 18, 19, 20, 12, 13,
	14, 15, 16, 26, 27, 7, 6, 5, 8, 9,
	12, 13, 14, 15, 16, 26, 27, 0, 0, 0,
	3, 0, 14, 15, 16, 26, 27, 10, 11, 4,
	17, 18, 19, 20, 12, 13, 14, 15, 16, 26,
	27,
}
var yyPact = []int{

	91, -1000, 41, 91, 91, 91, -1000, -1000, -1000, -1000,
	91, 91, 91, 91, 91, 91, 91, 91, 91, 91,
	91, 91, 91, 91, 91, 91, 91, 91, -1000, 20,
	-1000, -1000, -1000, 92, 92, -1000, -1000, -1000, 82, 82,
	82, 82, 106, 106, 106, 70, 56, -1000, -1000, -1000,
}
var yyPgo = []int{

	0, 0, 1,
}
var yyR1 = []int{

	0, 2, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1,
	1, 1, 1, 1, 1, 1, 1,
}
var yyR2 = []int{

	0, 1, 3, 3, 3, 2, 3, 3, 3, 2,
	1, 1, 1, 1, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 2, 2,
}
var yyChk = []int{

	-1000, -2, -1, 19, 28, 6, 5, 4, 7, 8,
	26, 27, 18, 19, 20, 21, 22, 14, 15, 16,
	17, 11, 12, 13, 10, 9, 23, 24, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, -1,
	-1, -1, -1, -1, -1, -1, -1, -1, -1, 29,
}
var yyDef = []int{

	0, -2, 1, 0, 0, 0, 10, 11, 12, 13,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 5, 0,
	9, 25, 26, 2, 3, 4, 6, 7, 14, 15,
	16, 17, 18, 19, 20, 21, 22, 23, 24, 8,
}
var yyTok1 = []int{

	1, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 26, 3, 3, 3, 22, 23, 3,
	28, 29, 20, 18, 3, 19, 3, 21, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	14, 11, 15, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 27, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 24,
}
var yyTok2 = []int{

	2, 3, 4, 5, 6, 7, 8, 9, 10, 12,
	13, 16, 17, 25,
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

	case 1:
		//line parse.y:46
		{
			result = yyS[yypt-0].vval
		}
	case 2:
		//line parse.y:52
		{
			yyVAL.vval = add(yyS[yypt-2].vval, yyS[yypt-0].vval)
		}
	case 3:
		//line parse.y:56
		{
			yyVAL.vval = sub(yyS[yypt-2].vval, yyS[yypt-0].vval)
		}
	case 4:
		//line parse.y:60
		{
			yyVAL.vval = mul(yyS[yypt-2].vval, yyS[yypt-0].vval)
		}
	case 5:
		//line parse.y:64
		{
			yyVAL.vval = minus(yyS[yypt-0].vval)
		}
	case 6:
		//line parse.y:68
		{
			yyVAL.vval = div(yyS[yypt-2].vval, yyS[yypt-0].vval)
		}
	case 7:
		//line parse.y:72
		{
			yyVAL.vval = mod(yyS[yypt-2].vval, yyS[yypt-0].vval)
		}
	case 8:
		//line parse.y:76
		{
			yyVAL.vval = yyS[yypt-1].vval
		}
	case 9:
		//line parse.y:80
		{
			if f, ok := funcs[yyS[yypt-1].sval]; ok {
				n := Nval(yyS[yypt-0].vval)
				yyVAL.vval = f(n)
			} else if v, err := attr(yyS[yypt-1].sval, yyS[yypt-0].vval); err == nil {
				yyVAL.vval = v
			} else {
				panic("unknown function")
			}
		}
	case 10:
		//line parse.y:91
		{
			yyVAL.vval = value(yyS[yypt-0].fval)
		}
	case 11:
		//line parse.y:95
		{
			yyVAL.vval = value(yyS[yypt-0].ival)
		}
	case 12:
		//line parse.y:99
		{
			yyVAL.vval = value(yyS[yypt-0].sval)
		}
	case 13:
		//line parse.y:103
		{
			yyVAL.vval = value(yyS[yypt-0].tval)
		}
	case 14:
		//line parse.y:107
		{
			yyVAL.vval = value(cmp(yyS[yypt-2].vval, yyS[yypt-0].vval) < 0)
		}
	case 15:
		//line parse.y:111
		{
			yyVAL.vval = value(cmp(yyS[yypt-2].vval, yyS[yypt-0].vval) > 0)
		}
	case 16:
		//line parse.y:115
		{
			yyVAL.vval = value(cmp(yyS[yypt-2].vval, yyS[yypt-0].vval) <= 0)
		}
	case 17:
		//line parse.y:119
		{
			yyVAL.vval = value(cmp(yyS[yypt-2].vval, yyS[yypt-0].vval) >= 0)
		}
	case 18:
		//line parse.y:123
		{
			yyVAL.vval = value(cmp(yyS[yypt-2].vval, yyS[yypt-0].vval) == 0)
		}
	case 19:
		//line parse.y:127
		{
			yyVAL.vval = value(cmp(yyS[yypt-2].vval, yyS[yypt-0].vval) == 0)
		}
	case 20:
		//line parse.y:131
		{
			yyVAL.vval = value(cmp(yyS[yypt-2].vval, yyS[yypt-0].vval) != 0)
		}
	case 21:
		//line parse.y:135
		{
			yyVAL.vval = value(Bval(yyS[yypt-2].vval) && Bval(yyS[yypt-0].vval))
		}
	case 22:
		//line parse.y:139
		{
			yyVAL.vval = value(Bval(yyS[yypt-2].vval) || Bval(yyS[yypt-0].vval))
		}
	case 23:
		//line parse.y:143
		{
			yyVAL.vval = value(Ival(yyS[yypt-2].vval) & Ival(yyS[yypt-0].vval))
		}
	case 24:
		//line parse.y:147
		{
			yyVAL.vval = value(Ival(yyS[yypt-2].vval) | Ival(yyS[yypt-0].vval))
		}
	case 25:
		//line parse.y:151
		{
			yyVAL.vval = value(!Bval(yyS[yypt-0].vval))
		}
	case 26:
		//line parse.y:155
		{
			yyVAL.vval = value(^Ival(yyS[yypt-0].vval))
		}
	}
	goto yystack /* stack new state and value */
}
