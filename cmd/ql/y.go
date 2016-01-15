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
	redirs []*Redir
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

//line parse.y:356

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
	-1, 101,
	24, 45,
	-2, 16,
}

const yyNprod = 67
const yyPrivate = 57344

var yyTokenNames []string
var yyStates []string

const yyLast = 262

var yyAct = [...]int{

	35, 48, 62, 82, 4, 51, 4, 55, 39, 28,
	64, 65, 56, 6, 112, 6, 11, 16, 17, 63,
	73, 64, 65, 120, 121, 72, 8, 31, 32, 111,
	49, 108, 29, 25, 24, 59, 36, 94, 25, 24,
	12, 137, 37, 41, 42, 136, 30, 22, 40, 70,
	71, 135, 25, 24, 74, 23, 76, 75, 49, 58,
	23, 22, 41, 42, 57, 83, 85, 40, 131, 86,
	49, 81, 126, 69, 23, 90, 93, 125, 124, 114,
	84, 61, 46, 66, 67, 45, 44, 43, 49, 100,
	60, 102, 103, 104, 101, 87, 38, 52, 53, 26,
	54, 20, 14, 109, 110, 115, 21, 113, 101, 101,
	47, 123, 101, 119, 18, 7, 127, 2, 101, 10,
	11, 69, 1, 49, 128, 129, 130, 14, 9, 101,
	101, 101, 50, 49, 13, 49, 3, 139, 140, 15,
	122, 19, 95, 96, 12, 78, 99, 14, 9, 77,
	79, 80, 52, 53, 132, 54, 134, 34, 105, 33,
	89, 5, 27, 91, 92, 106, 68, 0, 0, 97,
	98, 0, 25, 24, 116, 117, 118, 52, 53, 0,
	54, 22, 41, 42, 0, 0, 0, 40, 25, 24,
	0, 0, 0, 0, 23, 0, 0, 22, 41, 42,
	0, 0, 0, 40, 25, 24, 138, 0, 0, 0,
	23, 0, 0, 22, 41, 42, 0, 0, 0, 40,
	25, 24, 133, 25, 24, 0, 23, 0, 0, 22,
	41, 42, 22, 41, 42, 40, 0, 107, 40, 25,
	24, 0, 23, 0, 0, 23, 0, 0, 22, 41,
	42, 0, 0, 0, 88, 0, 0, 0, 0, 0,
	0, 23,
}
var yyPact = [...]int{

	113, -1000, 113, -1000, 9, 9, -1000, 107, 84, 28,
	80, -1000, -1000, 23, -1000, -1000, -1000, -1000, -1000, -1000,
	-1000, -1000, -1000, 68, 67, 66, 59, 96, -1000, 162,
	9, 213, 88, 82, -1000, -1000, 58, -8, 61, 62,
	213, 9, 9, -2, -7, -1000, 9, 23, -1000, -1000,
	137, -1000, 28, 28, 28, 133, -1000, 42, 57, -1000,
	43, 9, 229, 28, -1000, -1000, 213, 213, 11, 213,
	133, 133, 28, 28, 133, -1000, -1000, -1000, -1000, -1000,
	-1000, 9, -1000, 9, 9, 9, 133, 213, 210, 3,
	-1000, 61, -1000, -1000, -1000, 9, 9, 1, -14, 9,
	55, 133, 133, 133, 133, 9, -3, 213, -19, 54,
	53, -1000, -1000, 48, 137, -1000, 9, 9, 9, 44,
	-1000, 213, 194, 213, -1000, -1000, -1000, -1000, 27, 21,
	17, -1000, 178, -1000, 213, 137, 137, -1000, -1000, -1000,
	-1000,
}
var yyPgo = [...]int{

	0, 96, 32, 9, 166, 8, 0, 165, 3, 26,
	5, 162, 71, 161, 159, 157, 145, 141, 134, 132,
	1, 122, 117, 136, 12, 7, 2,
}
var yyR1 = [...]int{

	0, 21, 21, 22, 22, 23, 23, 23, 23, 13,
	8, 8, 17, 17, 9, 18, 18, 11, 11, 3,
	3, 3, 3, 3, 3, 15, 15, 15, 26, 26,
	14, 14, 12, 12, 20, 20, 19, 19, 10, 10,
	10, 16, 16, 24, 24, 25, 25, 2, 2, 6,
	6, 5, 5, 5, 5, 5, 5, 7, 7, 4,
	4, 1, 1, 1, 1, 1, 1,
}
var yyR2 = [...]int{

	0, 1, 0, 2, 1, 2, 2, 1, 2, 7,
	2, 2, 1, 0, 2, 1, 0, 3, 1, 2,
	6, 8, 8, 2, 1, 3, 5, 6, 1, 1,
	6, 7, 3, 1, 1, 0, 2, 1, 2, 2,
	2, 1, 0, 1, 1, 1, 0, 2, 1, 1,
	1, 3, 3, 3, 3, 5, 5, 4, 3, 1,
	0, 1, 2, 2, 5, 5, 2,
}
var yyChk = [...]int{

	-1000, -21, -22, -23, -8, -13, -24, 2, -9, 15,
	6, 7, 31, -18, 14, -23, -24, -24, 7, -17,
	17, -1, 19, 32, 11, 10, 19, -11, -3, -2,
	23, 4, 5, -14, -15, -6, 13, 19, -1, -5,
	25, 20, 21, 19, 19, 19, 23, 14, -20, -6,
	-19, -10, 15, 16, 18, -25, -24, -2, -9, -20,
	8, 23, -26, 27, 29, 30, 22, 22, -4, -2,
	-25, -25, 27, 27, -25, -3, -10, -1, -16, -1,
	-1, -12, -8, 23, 23, 23, -25, -2, 25, -1,
	-5, -1, -1, -5, 26, -12, -12, -1, -1, -12,
	-25, -24, -25, -25, -25, -12, -7, 27, 28, -25,
	-25, 28, 28, -25, 24, -8, -12, -12, -12, -25,
	26, 27, -2, -26, 24, 24, 24, -20, -25, -25,
	-25, 24, -2, 28, -2, 24, 24, 24, 28, -20,
	-20,
}
var yyDef = [...]int{

	-2, -2, -2, 4, 0, 0, 7, 0, 13, 0,
	0, 43, 44, 0, 15, 3, 5, 6, 8, 10,
	12, 11, 61, 0, 0, 0, 0, 14, 18, 35,
	46, 0, 16, 35, 24, 48, 0, 61, 49, 50,
	60, 46, 46, 62, 63, 66, 46, 0, 19, 47,
	34, 37, 0, 42, 0, 16, 45, 0, 0, 23,
	0, 46, 0, 0, 28, 29, 0, 0, 0, 59,
	16, 16, 0, 0, 16, 17, 36, 38, 39, 41,
	40, 46, 33, 46, 46, 46, 16, 25, 60, 0,
	52, 0, 53, 54, 51, 46, 46, 0, 0, 46,
	0, -2, 16, 16, 16, 46, 0, 0, 0, 0,
	0, 64, 65, 0, 35, 32, 46, 46, 46, 0,
	26, 0, 0, 0, 55, 56, 9, 20, 0, 0,
	0, 30, 0, 58, 27, 35, 35, 31, 57, 21,
	22,
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
	case 12:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line parse.y:84
		{
			yyVAL.sval = yyDollar[1].sval
			if yyVAL.sval == "" {
				yyVAL.sval = "&"
			}
		}
	case 13:
		yyDollar = yyS[yypt-0 : yypt+1]
		//line parse.y:91
		{
			yyVAL.sval = ""
		}
	case 14:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:98
		{
			yyVAL.nd = yyDollar[2].nd
			yyVAL.nd.Args = append([]string{""}, yyVAL.nd.Args...)
			yyVAL.nd.addPipeRedirs(yyDollar[1].bval)
		}
	case 15:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line parse.y:107
		{
			yyVAL.bval = true
		}
	case 16:
		yyDollar = yyS[yypt-0 : yypt+1]
		//line parse.y:111
		{
			yyVAL.bval = false
		}
	case 17:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line parse.y:118
		{
			yyVAL.nd = yyDollar[1].nd.Add(yyDollar[3].nd)
			yyVAL.nd.Args = append(yyVAL.nd.Args, yyDollar[2].sval)
		}
	case 18:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line parse.y:123
		{
			yyVAL.nd = newList(Npipe, yyDollar[1].nd)
		}
	case 19:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:130
		{
			yyVAL.nd = newList(Ncmd, yyDollar[1].nd)
			yyVAL.nd.Redirs = yyDollar[2].redirs
		}
	case 20:
		yyDollar = yyS[yypt-6 : yypt+1]
		//line parse.y:135
		{
			yyVAL.nd = yyDollar[3].nd
			yyVAL.nd.Redirs = yyDollar[6].redirs
		}
	case 21:
		yyDollar = yyS[yypt-8 : yypt+1]
		//line parse.y:140
		{
			yyVAL.nd = newList(Nfor, yyDollar[2].nd, yyDollar[5].nd)
			yyVAL.nd.Redirs = yyDollar[8].redirs
		}
	case 22:
		yyDollar = yyS[yypt-8 : yypt+1]
		//line parse.y:145
		{
			yyVAL.nd = newList(Nwhile, yyDollar[2].nd, yyDollar[5].nd)
			yyVAL.nd.Redirs = yyDollar[8].redirs
		}
	case 23:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:150
		{
			yyVAL.nd = yyDollar[1].nd
			yyDollar[1].nd.Redirs = yyDollar[2].redirs
		}
	case 25:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line parse.y:159
		{
			yyVAL.nd = newNd(Nset, yyDollar[1].sval).Add(yyDollar[3].nd)
		}
	case 26:
		yyDollar = yyS[yypt-5 : yypt+1]
		//line parse.y:163
		{
			yyVAL.nd = yyDollar[4].nd
			yyVAL.nd.Args = []string{yyDollar[1].sval}
		}
	case 27:
		yyDollar = yyS[yypt-6 : yypt+1]
		//line parse.y:168
		{
			yyVAL.nd = newNd(Nset, yyDollar[1].sval).Add(yyDollar[3].nd).Add(yyDollar[6].nd)
		}
	case 30:
		yyDollar = yyS[yypt-6 : yypt+1]
		//line parse.y:179
		{
			nd := yyDollar[4].nd
			nd.typ = Nor
			yyVAL.nd = newList(Ncond, nd)
		}
	case 31:
		yyDollar = yyS[yypt-7 : yypt+1]
		//line parse.y:185
		{
			nd := yyDollar[5].nd
			nd.typ = Nor
			yyVAL.nd = yyDollar[1].nd.Add(nd)
		}
	case 32:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line parse.y:193
		{
			yyVAL.nd = yyDollar[1].nd.Add(yyDollar[3].nd)
		}
	case 33:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line parse.y:197
		{
			yyVAL.nd = newList(Nblock, yyDollar[1].nd)
		}
	case 34:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line parse.y:204
		{
			yyVAL.redirs = yyDollar[1].redirs
		}
	case 35:
		yyDollar = yyS[yypt-0 : yypt+1]
		//line parse.y:208
		{
			yyVAL.redirs = nil
		}
	case 36:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:215
		{
			yyVAL.redirs = yyDollar[1].redirs
			yyVAL.redirs = yyDollar[2].nd.addRedirTo(yyVAL.redirs)
		}
	case 37:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line parse.y:220
		{
			yyVAL.redirs = nil
			yyVAL.redirs = yyDollar[1].nd.addRedirTo(yyVAL.redirs)
		}
	case 38:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:228
		{
			yyVAL.nd = newRedir("<", yyDollar[1].sval, yyDollar[2].nd)
		}
	case 39:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:232
		{
			yyVAL.nd = newRedir(">", yyDollar[1].sval, yyDollar[2].nd)
		}
	case 40:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:235
		{
			yyVAL.nd = newRedir(">>", yyDollar[1].sval, yyDollar[2].nd)
		}
	case 42:
		yyDollar = yyS[yypt-0 : yypt+1]
		//line parse.y:243
		{
			yyVAL.nd = nil
		}
	case 47:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:259
		{
			yyVAL.nd = yyDollar[1].nd.Add(yyDollar[2].nd)
		}
	case 48:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line parse.y:263
		{
			yyVAL.nd = newList(Nnames, yyDollar[1].nd)
		}
	case 51:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line parse.y:274
		{
			yyVAL.nd = yyDollar[2].nd
		}
	case 52:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line parse.y:278
		{
			nd := newList(Nnames, yyDollar[1].nd)
			yyVAL.nd = newList(Napp, nd, yyDollar[3].nd)
		}
	case 53:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line parse.y:283
		{
			nd := newList(Nnames, yyDollar[3].nd)
			yyVAL.nd = newList(Napp, yyDollar[1].nd, nd)
		}
	case 54:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line parse.y:288
		{
			yyVAL.nd = newList(Napp, yyDollar[1].nd, yyDollar[3].nd)
		}
	case 55:
		yyDollar = yyS[yypt-5 : yypt+1]
		//line parse.y:292
		{
			yyVAL.nd = yyDollar[3].nd
			yyDollar[3].nd.Args = []string{"<"}
			if yyDollar[1].sval != "" {
				yyDollar[3].nd.Args = append(yyDollar[3].nd.Args, yyDollar[1].sval)
			}
			yyDollar[3].nd.typ = Nioblk
		}
	case 56:
		yyDollar = yyS[yypt-5 : yypt+1]
		//line parse.y:301
		{
			yyVAL.nd = yyDollar[3].nd
			if yyDollar[1].sval == "" {
				yyDollar[1].sval = "out"
			}
			yyDollar[3].nd.Args = []string{">", yyDollar[1].sval}
			yyDollar[3].nd.typ = Nioblk
		}
	case 57:
		yyDollar = yyS[yypt-4 : yypt+1]
		//line parse.y:313
		{
			yyVAL.nd = yyDollar[1].nd.Add(yyDollar[3].nd)
		}
	case 58:
		yyDollar = yyS[yypt-3 : yypt+1]
		//line parse.y:317
		{
			// the parent adds Args with the var name
			yyVAL.nd = newList(Nsetmap, yyDollar[2].nd)
		}
	case 60:
		yyDollar = yyS[yypt-0 : yypt+1]
		//line parse.y:326
		{
			yyVAL.nd = newList(Nnames)
		}
	case 61:
		yyDollar = yyS[yypt-1 : yypt+1]
		//line parse.y:332
		{
			yyVAL.nd = newNd(Nname, yyDollar[1].sval)
		}
	case 62:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:336
		{
			yyVAL.nd = newNd(Nval, yyDollar[2].sval)
		}
	case 63:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:340
		{
			yyVAL.nd = newNd(Nsingle, yyDollar[2].sval)
		}
	case 64:
		yyDollar = yyS[yypt-5 : yypt+1]
		//line parse.y:344
		{
			yyVAL.nd = newNd(Nval, yyDollar[2].sval).Add(yyDollar[4].nd)
		}
	case 65:
		yyDollar = yyS[yypt-5 : yypt+1]
		//line parse.y:348
		{
			yyVAL.nd = newNd(Nsingle, yyDollar[2].sval).Add(yyDollar[4].nd)
		}
	case 66:
		yyDollar = yyS[yypt-2 : yypt+1]
		//line parse.y:352
		{
			yyVAL.nd = newNd(Nlen, yyDollar[2].sval)
		}
	}
	goto yystack /* stack new state and value */
}
