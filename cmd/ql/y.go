//line parse.y:18
package main

import __yyfmt__ "fmt"

//line parse.y:18
//line parse.y:22
type yySymType struct {
	yys    int
	sval   string
	nd     *Nd
	bval   bool
	redirs map[string]*Nd
}

const FOR = 57346
const WHILE = 57347
const FUNC = 57348
const NL = 57349
const OR = 57350
const AND = 57351
const LEN = 57352
const SINGLE = 57353
const ERROR = 57354
const COND = 57355
const PIPE = 57356
const IREDIR = 57357
const OREDIR = 57358
const BG = 57359
const APP = 57360
const NAME = 57361
const INBLK = 57362
const OUTBLK = 57363

var yyToknames = [...]string{
	"$end",
	"error",
	"$unk",
	"FOR",
	"WHILE",
	"FUNC",
	"NL",
	"OR",
	"AND",
	"LEN",
	"SINGLE",
	"ERROR",
	"COND",
	"PIPE",
	"IREDIR",
	"OREDIR",
	"BG",
	"APP",
	"NAME",
	"INBLK",
	"OUTBLK",
	"'^'",
	"'{'",
	"'}'",
	"'('",
	"')'",
	"'['",
	"']'",
	"'='",
	"'←'",
	"';'",
	"'$'",
}
var yyStatenames = [...]string{}

const yyEofCode = 1
const yyErrCode = 2
const yyInitialStackSize = 16

//line parse.y:347

//line yacctab:1
var yyExca = [...]int{
	-1, 0,
	1, 2,
	4, 16,
	5, 16,
	10, 16,
	11, 16,
	13, 16,
	19, 16,
	20, 16,
	21, 16,
	23, 16,
	25, 16,
	32, 16,
	-2, 0,
	-1, 1,
	1, -1,
	-2, 0,
	-1, 2,
	1, 1,
	4, 16,
	5, 16,
	10, 16,
	11, 16,
	13, 16,
	19, 16,
	20, 16,
	21, 16,
	23, 16,
	25, 16,
	32, 16,
	-2, 0,
	-1, 100,
	24, 43,
	-2, 16,
}

const yyNprod = 65
const yyPrivate = 57344

var yyTokenNames []string
var yyStates []string

const yyLast = 257

var yyAct = [...]int{

	35, 48, 62, 81, 4, 51, 4, 55, 8, 39,
	111, 38, 56, 6, 28, 6, 11, 16, 17, 64,
	65, 21, 110, 25, 24, 119, 120, 31, 32, 107,
	49, 29, 22, 25, 24, 59, 36, 73, 72, 136,
	12, 58, 37, 41, 42, 23, 30, 93, 40, 70,
	71, 135, 134, 130, 74, 23, 76, 63, 49, 64,
	65, 125, 75, 57, 77, 78, 79, 124, 123, 85,
	49, 80, 69, 113, 84, 88, 89, 92, 90, 91,
	83, 61, 46, 66, 96, 97, 67, 49, 99, 60,
	101, 102, 103, 100, 86, 45, 52, 53, 44, 54,
	20, 43, 108, 109, 114, 26, 112, 100, 100, 14,
	122, 100, 118, 52, 53, 126, 54, 100, 47, 69,
	14, 9, 49, 127, 128, 129, 18, 2, 100, 100,
	100, 1, 49, 50, 49, 7, 138, 139, 121, 10,
	11, 3, 94, 95, 15, 13, 98, 14, 9, 19,
	25, 24, 131, 34, 133, 52, 53, 104, 54, 22,
	41, 42, 33, 5, 12, 40, 27, 25, 24, 105,
	68, 0, 23, 115, 116, 117, 22, 41, 42, 0,
	0, 0, 40, 25, 24, 137, 0, 0, 0, 23,
	0, 0, 22, 41, 42, 0, 0, 0, 40, 25,
	24, 132, 25, 24, 0, 23, 0, 0, 22, 41,
	42, 22, 41, 42, 40, 82, 106, 40, 25, 24,
	0, 23, 0, 0, 23, 0, 0, 22, 41, 42,
	0, 0, 0, 40, 25, 24, 0, 0, 0, 0,
	23, 0, 0, 22, 41, 42, 0, 0, 0, 87,
	0, 0, 0, 0, 0, 0, 23,
}
var yyPact = [...]int{

	133, -1000, 133, -1000, 9, 9, -1000, 119, 83, 13,
	86, -1000, -1000, 23, -1000, -1000, -1000, -1000, -1000, -1000,
	-1000, -1000, -1000, 82, 79, 76, 59, 104, -1000, 140,
	9, 208, 95, 81, -1000, -1000, 58, 30, 61, 64,
	208, 9, 9, 11, 10, -1000, 9, 23, -1000, -1000,
	98, -1000, 13, 13, 13, 106, -1000, 192, 57, -1000,
	51, 9, 224, 13, -1000, -1000, 208, 208, 21, 208,
	106, 106, 13, 13, 106, -1000, -1000, -1000, -1000, -1000,
	9, -1000, 9, 9, 9, 106, 208, 189, 1, -1000,
	61, -1000, -1000, -1000, 9, 9, -6, -18, 9, 49,
	106, 106, 106, 106, 9, -1, 208, -10, 44, 43,
	-1000, -1000, 37, 98, -1000, 9, 9, 9, 29, -1000,
	208, 173, 208, -1000, -1000, -1000, -1000, 28, 27, 15,
	-1000, 157, -1000, 208, 98, 98, -1000, -1000, -1000, -1000,
}
var yyPgo = [...]int{

	0, 11, 31, 14, 170, 9, 0, 169, 3, 8,
	5, 166, 71, 163, 162, 153, 149, 145, 133, 1,
	131, 127, 141, 12, 7, 2,
}
var yyR1 = [...]int{

	0, 20, 20, 21, 21, 22, 22, 22, 22, 13,
	8, 8, 16, 16, 9, 17, 17, 11, 11, 3,
	3, 3, 3, 3, 3, 15, 15, 15, 25, 25,
	14, 14, 12, 12, 19, 19, 18, 18, 10, 10,
	10, 23, 23, 24, 24, 2, 2, 6, 6, 5,
	5, 5, 5, 5, 5, 7, 7, 4, 4, 1,
	1, 1, 1, 1, 1,
}
var yyR2 = [...]int{

	0, 1, 0, 2, 1, 2, 2, 1, 2, 7,
	2, 2, 1, 0, 2, 1, 0, 3, 1, 2,
	6, 8, 8, 2, 1, 3, 5, 6, 1, 1,
	6, 7, 3, 1, 1, 0, 2, 1, 2, 2,
	2, 1, 1, 1, 0, 2, 1, 1, 1, 3,
	3, 3, 3, 5, 5, 4, 3, 1, 0, 1,
	2, 2, 5, 5, 2,
}
var yyChk = [...]int{

	-1000, -20, -21, -22, -8, -13, -23, 2, -9, 15,
	6, 7, 31, -17, 14, -22, -23, -23, 7, -16,
	17, -1, 19, 32, 11, 10, 19, -11, -3, -2,
	23, 4, 5, -14, -15, -6, 13, 19, -1, -5,
	25, 20, 21, 19, 19, 19, 23, 14, -19, -6,
	-18, -10, 15, 16, 18, -24, -23, -2, -9, -19,
	8, 23, -25, 27, 29, 30, 22, 22, -4, -2,
	-24, -24, 27, 27, -24, -3, -10, -1, -1, -1,
	-12, -8, 23, 23, 23, -24, -2, 25, -1, -5,
	-1, -1, -5, 26, -12, -12, -1, -1, -12, -24,
	-23, -24, -24, -24, -12, -7, 27, 28, -24, -24,
	28, 28, -24, 24, -8, -12, -12, -12, -24, 26,
	27, -2, -25, 24, 24, 24, -19, -24, -24, -24,
	24, -2, 28, -2, 24, 24, 24, 28, -19, -19,
}
var yyDef = [...]int{

	-2, -2, -2, 4, 0, 0, 7, 0, 13, 0,
	0, 41, 42, 0, 15, 3, 5, 6, 8, 10,
	12, 11, 59, 0, 0, 0, 0, 14, 18, 35,
	44, 0, 16, 35, 24, 46, 0, 59, 47, 48,
	58, 44, 44, 60, 61, 64, 44, 0, 19, 45,
	34, 37, 0, 0, 0, 16, 43, 0, 0, 23,
	0, 44, 0, 0, 28, 29, 0, 0, 0, 57,
	16, 16, 0, 0, 16, 17, 36, 38, 39, 40,
	44, 33, 44, 44, 44, 16, 25, 58, 0, 50,
	0, 51, 52, 49, 44, 44, 0, 0, 44, 0,
	-2, 16, 16, 16, 44, 0, 0, 0, 0, 0,
	62, 63, 0, 35, 32, 44, 44, 44, 0, 26,
	0, 0, 0, 53, 54, 9, 20, 0, 0, 0,
	30, 0, 56, 27, 35, 35, 31, 55, 21, 22,
}
var yyTok1 = [...]int{

	1, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 32, 3, 3, 3,
	25, 26, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 31,
	3, 29, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 27, 3, 28, 22, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 23, 3, 24,
}
var yyTok2 = [...]int{

	2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
	12, 13, 14, 15, 16, 17, 18, 19, 20, 21,
}
var yyTok3 = [...]int{
	8592, 30, 0,
}

var yyErrorMessages = [...]struct {
	state int
	token int
	msg   string
}{}

//line yaccpar:1

/*	parser for yacc output	*/

var (
	yyDebug        = 0
	yyErrorVerbose = false
)

type yyLexer interface {
	Lex(lval *yySymType) int
	Error(s string)
}

type yyParser interface {
	Parse(yyLexer) int
	Lookahead() int
}

type yyParserImpl struct {
	lval  yySymType
	stack [yyInitialStackSize]yySymType
	char  int
}

func (p *yyParserImpl) Lookahead() int {
	return p.char
}

func yyNewParser() yyParser {
	return &yyParserImpl{}
}

const yyFlag = -1000

func yyTokname(c int) string {
	if c >= 1 && c-1 < len(yyToknames) {
		if yyToknames[c-1] != "" {
			return yyToknames[c-1]
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

func yyErrorMessage(state, lookAhead int) string {
	const TOKSTART = 4

	if !yyErrorVerbose {
		return "syntax error"
	}

	for _, e := range yyErrorMessages {
		if e.state == state && e.token == lookAhead {
			return "syntax error: " + e.msg
		}
	}

	res := "syntax error: unexpected " + yyTokname(lookAhead)

	// To match Bison, suggest at most four expected tokens.
	expected := make([]int, 0, 4)

	// Look for shiftable tokens.
	base := yyPact[state]
	for tok := TOKSTART; tok-1 < len(yyToknames); tok++ {
		if n := base + tok; n >= 0 && n < yyLast && yyChk[yyAct[n]] == tok {
			if len(expected) == cap(expected) {
				return res
			}
			expected = append(expected, tok)
		}
	}

	if yyDef[state] == -2 {
		i := 0
		for yyExca[i] != -1 || yyExca[i+1] != state {
			i += 2
		}

		// Look for tokens that we accept or reduce.
		for i += 2; yyExca[i] >= 0; i += 2 {
			tok := yyExca[i]
			if tok < TOKSTART || yyExca[i+1] == 0 {
				continue
			}
			if len(expected) == cap(expected) {
				return res
			}
			expected = append(expected, tok)
		}

		// If the default action is to accept or reduce, give up.
		if yyExca[i+1] != 0 {
			return res
		}
	}

	for i, tok := range expected {
		if i == 0 {
			res += ", expecting "
		} else {
			res += " or "
		}
		res += yyTokname(tok)
	}
	return res
}

func yylex1(lex yyLexer, lval *yySymType) (char, token int) {
	token = 0
	char = lex.Lex(lval)
	if char <= 0 {
		token = yyTok1[0]
		goto out
	}
	if char < len(yyTok1) {
		token = yyTok1[char]
		goto out
	}
	if char >= yyPrivate {
		if char < yyPrivate+len(yyTok2) {
			token = yyTok2[char-yyPrivate]
			goto out
		}
	}
	for i := 0; i < len(yyTok3); i += 2 {
		token = yyTok3[i+0]
		if token == char {
			token = yyTok3[i+1]
			goto out
		}
	}

out:
	if token == 0 {
		token = yyTok2[1] /* unknown char */
	}
	if yyDebug >= 3 {
		__yyfmt__.Printf("lex %s(%d)\n", yyTokname(token), uint(char))
	}
	return char, token
}

func yyParse(yylex yyLexer) int {
	return yyNewParser().Parse(yylex)
}

func (yyrcvr *yyParserImpl) Parse(yylex yyLexer) int {
	var yyn int
	var yyVAL yySymType
	var yyDollar []yySymType
	_ = yyDollar // silence set and not used
	yyS := yyrcvr.stack[:]

	Nerrs := 0   /* number of errors */
	Errflag := 0 /* error recovery flag */
	yystate := 0
	yyrcvr.char = -1
	yytoken := -1 // yyrcvr.char translated into internal numbering
	defer func() {
		// Make sure we report no lookahead when not parsing.
		yystate = -1
		yyrcvr.char = -1
		yytoken = -1
	}()
	yyp := -1
	goto yystack

ret0:
	return 0

ret1:
	return 1

yystack:
	/* put a state and value onto the stack */
	if yyDebug >= 4 {
		__yyfmt__.Printf("char %v in %v\n", yyTokname(yytoken), yyStatname(yystate))
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
	if yyrcvr.char < 0 {
		yyrcvr.char, yytoken = yylex1(yylex, &yyrcvr.lval)
	}
	yyn += yytoken
	if yyn < 0 || yyn >= yyLast {
		goto yydefault
	}
	yyn = yyAct[yyn]
	if yyChk[yyn] == yytoken { /* valid shift */
		yyrcvr.char = -1
		yytoken = -1
		yyVAL = yyrcvr.lval
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
		if yyrcvr.char < 0 {
			yyrcvr.char, yytoken = yylex1(yylex, &yyrcvr.lval)
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
			if yyn < 0 || yyn == yytoken {
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
			yylex.Error(yyErrorMessage(yystate, yytoken))
			Nerrs++
			if yyDebug >= 1 {
				__yyfmt__.Printf("%s", yyStatname(yystate))
				__yyfmt__.Printf(" saw %s\n", yyTokname(yytoken))
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
				__yyfmt__.Printf("error recovery discards %s\n", yyTokname(yytoken))
			}
			if yytoken == yyEofCode {
				goto ret1
			}
			yyrcvr.char = -1
			yytoken = -1
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
	// yyp is now the index of $0. Perform the default action. Iff the
	// reduced production is ε, $1 is possibly out of range.
	if yyp+1 >= len(yyS) {
		nyys := make([]yySymType, len(yyS)*2)
		copy(nyys, yyS)
		yyS = nyys
	}
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
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:45
		{
			yyDollar[1].nd.run()
		}
	case 6:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:49
		{
			yyDollar[1].nd.run()
		}
	case 8:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:54
		{
			// scripts won't continue upon errors
			yylex.(*lex).nerrors++
			if !yylex.(*lex).interactive {
				panic(parseErr)
			}
		}
	case 9:
		yyDollar = yyS[yypt-7 : yypt+1]
		//line parse.y:65
		{
			yyVAL.nd = newNd(Nfunc, yyDollar[2].sval).Add(yyDollar[5].nd)
		}
	case 10:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:72
		{
			yyVAL.nd = yyDollar[1].nd
			yyVAL.nd.Args[0] = yyDollar[2].sval
		}
	case 11:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:77
		{
			yyVAL.nd = newList(Nsrc, yyDollar[2].nd)
		}
	case 13:
		yyDollar = yyS[yypt-0 : yypt+1]
		//line parse.y:85
		{
			yyVAL.sval = ""
		}
	case 14:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:92
		{
			yyVAL.nd = yyDollar[2].nd
			yyVAL.nd.Args = append([]string{""}, yyVAL.nd.Args...)
			yyVAL.nd.addPipeRedirs(yyDollar[1].bval)
		}
	case 15:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line parse.y:101
		{
			yyVAL.bval = true
		}
	case 16:
		yyDollar = yyS[yypt-0 : yypt+1]
		//line parse.y:105
		{
			yyVAL.bval = false
		}
	case 17:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line parse.y:112
		{
			yyVAL.nd = yyDollar[1].nd.Add(yyDollar[3].nd)
			yyVAL.nd.Args = append(yyVAL.nd.Args, yyDollar[2].sval)
		}
	case 18:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line parse.y:117
		{
			yyVAL.nd = newList(Npipe, yyDollar[1].nd)
		}
	case 19:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:124
		{
			yyVAL.nd = newList(Ncmd, yyDollar[1].nd)
			yyVAL.nd.Redirs = yyDollar[2].redirs
		}
	case 20:
		yyDollar = yyS[yypt-6 : yypt+1]
		//line parse.y:129
		{
			yyVAL.nd = yyDollar[3].nd
			yyVAL.nd.Redirs = yyDollar[6].redirs
		}
	case 21:
		yyDollar = yyS[yypt-8 : yypt+1]
		//line parse.y:134
		{
			yyDollar[5].nd.Redirs = map[string]*Nd{}
			yyVAL.nd = newList(Nfor, yyDollar[2].nd, yyDollar[5].nd)
			yyVAL.nd.Redirs = yyDollar[8].redirs
		}
	case 22:
		yyDollar = yyS[yypt-8 : yypt+1]
		//line parse.y:140
		{
			yyDollar[5].nd.Redirs = map[string]*Nd{}
			yyVAL.nd = newList(Nwhile, yyDollar[2].nd, yyDollar[5].nd)
			yyVAL.nd.Redirs = yyDollar[8].redirs
		}
	case 23:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:146
		{
			yyVAL.nd = yyDollar[1].nd
			yyDollar[1].nd.Redirs = yyDollar[2].redirs
		}
	case 25:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line parse.y:155
		{
			yyVAL.nd = newNd(Nset, yyDollar[1].sval).Add(yyDollar[3].nd)
		}
	case 26:
		yyDollar = yyS[yypt-5 : yypt+1]
		//line parse.y:159
		{
			yyVAL.nd = yyDollar[4].nd
			yyVAL.nd.Args = []string{yyDollar[1].sval}
		}
	case 27:
		yyDollar = yyS[yypt-6 : yypt+1]
		//line parse.y:164
		{
			yyVAL.nd = newNd(Nset, yyDollar[1].sval).Add(yyDollar[3].nd).Add(yyDollar[6].nd)
		}
	case 30:
		yyDollar = yyS[yypt-6 : yypt+1]
		//line parse.y:175
		{
			nd := yyDollar[4].nd
			nd.typ = Nor
			yyVAL.nd = newList(Ncond, nd)
		}
	case 31:
		yyDollar = yyS[yypt-7 : yypt+1]
		//line parse.y:181
		{
			nd := yyDollar[5].nd
			nd.typ = Nor
			yyVAL.nd = yyDollar[1].nd.Add(nd)
		}
	case 32:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line parse.y:189
		{
			yyVAL.nd = yyDollar[1].nd.Add(yyDollar[3].nd)
		}
	case 33:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line parse.y:193
		{
			yyVAL.nd = newList(Nblock, yyDollar[1].nd)
		}
	case 34:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line parse.y:200
		{
			yyVAL.redirs = yyDollar[1].redirs
		}
	case 35:
		yyDollar = yyS[yypt-0 : yypt+1]
		//line parse.y:204
		{
			yyVAL.redirs = map[string]*Nd{}
		}
	case 36:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:211
		{
			yyVAL.redirs = yyDollar[1].redirs
			yyDollar[2].nd.addRedirTo(yyDollar[1].redirs)
		}
	case 37:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line parse.y:216
		{
			yyVAL.redirs = map[string]*Nd{}
			yyDollar[1].nd.addRedirTo(yyVAL.redirs)
		}
	case 38:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:224
		{
			yyVAL.nd = newRedir("<", yyDollar[1].sval, yyDollar[2].nd)
		}
	case 39:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:228
		{
			yyVAL.nd = newRedir(">", yyDollar[1].sval, yyDollar[2].nd)
		}
	case 40:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:231
		{
			yyVAL.nd = newRedir(">>", yyDollar[1].sval, yyDollar[2].nd)
		}
	case 45:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:248
		{
			yyVAL.nd = yyDollar[1].nd.Add(yyDollar[2].nd)
		}
	case 46:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line parse.y:252
		{
			yyVAL.nd = newList(Nnames, yyDollar[1].nd)
		}
	case 49:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line parse.y:263
		{
			yyVAL.nd = yyDollar[2].nd
		}
	case 50:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line parse.y:267
		{
			nd := newList(Nnames, yyDollar[1].nd)
			yyVAL.nd = newList(Napp, nd, yyDollar[3].nd)
		}
	case 51:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line parse.y:272
		{
			nd := newList(Nnames, yyDollar[3].nd)
			yyVAL.nd = newList(Napp, yyDollar[1].nd, nd)
		}
	case 52:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line parse.y:277
		{
			yyVAL.nd = newList(Napp, yyDollar[1].nd, yyDollar[3].nd)
		}
	case 53:
		yyDollar = yyS[yypt-5 : yypt+1]
		//line parse.y:281
		{
			yyVAL.nd = yyDollar[3].nd
			yyDollar[3].nd.Args = []string{"<"}
			if yyDollar[1].sval != "" {
				yyDollar[3].nd.Args = append(yyDollar[3].nd.Args, yyDollar[1].sval)
			}
			yyDollar[3].nd.Redirs = map[string]*Nd{}
			yyDollar[3].nd.typ = Nioblk
		}
	case 54:
		yyDollar = yyS[yypt-5 : yypt+1]
		//line parse.y:291
		{
			yyVAL.nd = yyDollar[3].nd
			if yyDollar[1].sval == "" {
				yyDollar[1].sval = "out"
			}
			yyDollar[3].nd.Args = []string{">", yyDollar[1].sval}
			yyDollar[3].nd.typ = Nioblk
			yyDollar[3].nd.Redirs = map[string]*Nd{}
		}
	case 55:
		yyDollar = yyS[yypt-4 : yypt+1]
		//line parse.y:304
		{
			yyVAL.nd = yyDollar[1].nd.Add(yyDollar[3].nd)
		}
	case 56:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line parse.y:308
		{
			// the parent adds Args with the var name
			yyVAL.nd = newList(Nsetmap, yyDollar[2].nd)
		}
	case 58:
		yyDollar = yyS[yypt-0 : yypt+1]
		//line parse.y:317
		{
			yyVAL.nd = newList(Nnames)
		}
	case 59:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line parse.y:323
		{
			yyVAL.nd = newNd(Nname, yyDollar[1].sval)
		}
	case 60:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:327
		{
			yyVAL.nd = newNd(Nval, yyDollar[2].sval)
		}
	case 61:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:331
		{
			yyVAL.nd = newNd(Nsingle, yyDollar[2].sval)
		}
	case 62:
		yyDollar = yyS[yypt-5 : yypt+1]
		//line parse.y:335
		{
			yyVAL.nd = newNd(Nval, yyDollar[2].sval).Add(yyDollar[4].nd)
		}
	case 63:
		yyDollar = yyS[yypt-5 : yypt+1]
		//line parse.y:339
		{
			yyVAL.nd = newNd(Nsingle, yyDollar[2].sval).Add(yyDollar[4].nd)
		}
	case 64:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:343
		{
			yyVAL.nd = newNd(Nlen, yyDollar[2].sval)
		}
	}
	goto yystack /* stack new state and value */
}
