//line parse.y:8
package main

import __yyfmt__ "fmt"

//line parse.y:8
import (
	"clive/dbg"
	"os"
	"strings"
)

var (
	debugYacc bool
	yprintf   = dbg.FlagPrintf(os.Stderr, &debugYacc)
	lvl       int
)

//line parse.y:24
type yySymType struct {
	yys  int
	sval string
	nd   *Nd
	rdr  Redirs
}

const NAME = 57346
const NL = 57347
const INBLK = 57348
const HEREBLK = 57349
const FORBLK = 57350
const PIPEBLK = 57351
const LEN = 57352
const IF = 57353
const ELSE = 57354
const ELSIF = 57355
const FOR = 57356
const APP = 57357
const WHILE = 57358
const FUNC = 57359
const INTERRUPT = 57360
const ERROR = 57361

var yyToknames = []string{
	"NAME",
	"NL",
	"INBLK",
	"HEREBLK",
	"FORBLK",
	"PIPEBLK",
	"LEN",
	"IF",
	"ELSE",
	"ELSIF",
	"FOR",
	"APP",
	"WHILE",
	"FUNC",
	"INTERRUPT",
	"ERROR",
	"'^'",
}
var yyStatenames = []string{}

const yyEofCode = 1
const yyErrCode = 2
const yyMaxDepth = 200

//line parse.y:412

//line yacctab:1
var yyExca = []int{
	-1, 1,
	1, -1,
	-2, 0,
	-1, 2,
	1, 1,
	-2, 0,
	-1, 82,
	22, 12,
	-2, 0,
}

const yyNprod = 69
const yyPrivate = 57344

var yyTokenNames []string
var yyStates []string

const yyLast = 206

var yyAct = []int{

	24, 25, 18, 52, 13, 100, 3, 7, 2, 32,
	43, 15, 34, 120, 136, 35, 51, 68, 9, 49,
	50, 129, 28, 114, 54, 57, 121, 36, 40, 90,
	107, 58, 39, 131, 60, 34, 16, 64, 65, 66,
	72, 73, 10, 27, 14, 15, 29, 30, 20, 31,
	28, 26, 67, 76, 22, 127, 23, 11, 49, 50,
	88, 19, 82, 12, 85, 78, 80, 114, 51, 94,
	16, 27, 48, 34, 28, 97, 51, 98, 29, 30,
	46, 31, 28, 93, 47, 102, 69, 34, 106, 32,
	108, 109, 92, 110, 137, 27, 34, 89, 79, 49,
	50, 133, 34, 27, 51, 118, 29, 30, 87, 31,
	28, 123, 117, 34, 101, 126, 125, 128, 124, 122,
	115, 130, 116, 51, 83, 29, 30, 20, 31, 28,
	26, 27, 81, 22, 135, 23, 49, 50, 84, 51,
	19, 29, 30, 71, 31, 28, 51, 59, 29, 30,
	27, 31, 28, 61, 134, 51, 132, 29, 30, 101,
	31, 28, 51, 86, 29, 30, 27, 31, 28, 62,
	113, 119, 74, 27, 55, 56, 111, 105, 104, 103,
	96, 91, 27, 77, 75, 70, 63, 38, 37, 27,
	95, 112, 53, 42, 41, 45, 44, 33, 6, 1,
	99, 5, 8, 21, 17, 4,
}
var yyPact = []int{

	40, -1000, 40, -1000, -1000, -1000, -1000, -14, 6, -1000,
	-1000, 184, 183, -1000, 4, -1000, -1000, 57, 158, -1000,
	-1000, 162, 158, 119, 127, -1000, 119, 149, 182, 119,
	119, 119, -1000, 6, 62, 181, -1000, 122, -1000, 151,
	180, -1000, 57, -1000, -1000, -1000, 179, 74, 62, 127,
	-1000, -1000, 110, 40, 102, 117, 119, 142, 87, 64,
	76, 5, 177, -1000, 70, 61, 47, -1000, -1000, 176,
	-1000, -1000, 127, -1000, 135, 60, -1000, -1000, 175, 174,
	173, -1000, 40, -1000, -1000, 9, -1000, -1000, -1000, -1000,
	172, -1000, -1000, -1000, -1000, 165, 42, 98, 100, 90,
	-1000, 167, -15, -1000, -2, -1000, 97, -1000, 96, 94,
	93, 30, 119, -1000, -1000, -1000, -1000, -1000, -1000, -4,
	12, 152, -1000, 79, -1000, -1000, -1000, -1000, -1000, 150,
	127, 158, -11, -1000, -1000, 72, -1000, -1000,
}
var yyPgo = []int{

	0, 8, 205, 7, 204, 203, 202, 3, 201, 200,
	5, 2, 0, 1, 4, 199, 6, 198, 197, 17,
	10, 196, 195, 194, 193, 18, 192, 191, 190,
}
var yyR1 = []int{

	0, 15, 1, 1, 16, 16, 16, 2, 2, 2,
	2, 26, 7, 8, 17, 27, 27, 19, 19, 28,
	3, 3, 14, 4, 4, 4, 4, 4, 4, 4,
	5, 5, 23, 23, 24, 24, 20, 20, 21, 22,
	22, 22, 18, 18, 18, 6, 6, 6, 6, 6,
	6, 13, 13, 13, 9, 9, 10, 25, 25, 11,
	11, 11, 11, 12, 12, 12, 12, 12, 12,
}
var yyR2 = []int{

	0, 1, 2, 1, 1, 1, 1, 3, 2, 1,
	1, 0, 2, 5, 2, 1, 0, 3, 0, 0,
	6, 1, 2, 1, 3, 3, 1, 5, 5, 5,
	5, 6, 1, 0, 2, 1, 1, 1, 2, 3,
	3, 6, 1, 2, 0, 3, 3, 5, 6, 8,
	5, 3, 3, 3, 2, 1, 4, 1, 1, 2,
	2, 1, 1, 1, 2, 3, 5, 2, 3,
}
var yyChk = []int{

	-1000, -15, -1, -16, -2, -8, -17, -3, -6, -25,
	2, 17, 23, -14, 4, 5, 30, -4, -11, 21,
	8, -5, 14, 16, -12, -13, 11, 31, 10, 6,
	7, 9, -16, -18, 26, 29, -25, 4, 4, 28,
	24, -23, -24, -20, -21, -22, 23, 27, 15, -12,
	-13, 4, -7, -26, -7, 12, 13, -11, -3, 20,
	-3, 4, 20, 4, -3, -3, -3, -25, -19, 24,
	4, 21, -12, -13, 21, 4, -20, 4, -19, 24,
	-19, 22, -1, 22, 21, -3, 21, 21, -12, 21,
	24, 4, 22, 22, 22, -28, 4, -7, -11, -9,
	-10, 24, 25, 4, 4, 4, -7, 21, -7, -7,
	-7, 4, -27, 5, 25, 22, 22, 22, -10, 4,
	28, 28, 22, -7, 22, 22, 22, 25, -14, 25,
	-12, 21, 4, 22, 4, -11, 25, 22,
}
var yyDef = []int{

	0, -2, -2, 3, 4, 5, 6, 44, 0, 9,
	10, 0, 0, 21, 63, 57, 58, 33, 23, 11,
	11, 26, 0, 0, 61, 62, 0, 0, 0, 0,
	0, 0, 2, 0, 18, 42, 8, 0, 14, 0,
	0, 22, 32, 35, 36, 37, 0, 18, 18, 59,
	60, 63, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 64, 0, 67, 0, 0, 0, 7, 19, 0,
	43, 11, 45, 46, 0, 0, 34, 38, 0, 0,
	0, 24, -2, 25, 11, 0, 11, 11, 68, 11,
	0, 65, 51, 52, 53, 16, 0, 0, 0, 0,
	55, 0, 0, 39, 0, 40, 0, 11, 0, 0,
	0, 0, 0, 15, 17, 13, 47, 50, 54, 0,
	0, 0, 27, 0, 28, 29, 30, 66, 20, 0,
	48, 0, 0, 31, 56, 0, 41, 49,
}
var yyTok1 = []int{

	1, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 31, 3, 29, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 30,
	23, 28, 27, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 24, 3, 25, 20, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 21, 26, 22,
}
var yyTok2 = []int{

	2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
	12, 13, 14, 15, 16, 17, 18, 19,
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
	if c>=4 && c-4<len(yyToknames) {
		if yyToknames[c-4] != "" {
			return yyToknames[c-4]
		}
	}
	return __yyfmt__.Sprintf("tok-%v", c)
}

func yyStatname(s int) string {
	if s>=0 && s<len(yyStatenames) {
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
	if yyn<0 || yyn>=yyLast {
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
			if yyExca[xi+0]==-1 && yyExca[xi+1]==yystate {
				break
			}
			xi += 2
		}
		for xi += 2; ; xi += 2 {
			yyn = yyExca[xi+0]
			if yyn<0 || yyn==yychar {
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
				if yyn>=0 && yyn<yyLast {
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
		yyVAL.nd = yyS[yypt-0].nd
	case 2:
		//line parse.y:43
		{
			if lvl == 0 {
				yyVAL.nd = nil
			} else {
				yyVAL.nd = yyS[yypt-1].nd.Add(yyS[yypt-0].nd)
			}
		}
	case 3:
		//line parse.y:51
		{
			if lvl == 0 {
				yyVAL.nd = nil
			} else {
				yyVAL.nd = NewList(Ncmds, yyS[yypt-0].nd)
			}
		}
	case 4:
		//line parse.y:62
		{
			if lvl==0 && yyS[yypt-0].nd!=nil && !Interrupted {
				if nerrors > 0 {
					yprintf("ERR %s\n", yyS[yypt-0].nd.sprint())
				} else {
					yprintf("%s\n", yyS[yypt-0].nd.sprint())
					yyS[yypt-0].nd.Exec()
				}
				nerrors = 0
			}
			yyVAL.nd = yyS[yypt-0].nd
		}
	case 5:
		//line parse.y:75
		{
			yyVAL.nd = yyS[yypt-0].nd
		}
	case 6:
		//line parse.y:79
		{
			yyVAL.nd = yyS[yypt-0].nd
		}
	case 7:
		//line parse.y:86
		{
			yyVAL.nd = yyS[yypt-2].nd
			if yyS[yypt-1].sval != "" {
				yyVAL.nd.Args = append(yyVAL.nd.Args, yyS[yypt-1].sval)
			}
		}
	case 8:
		//line parse.y:93
		{
			yyVAL.nd = yyS[yypt-1].nd
		}
	case 9:
		//line parse.y:97
		{
			yyVAL.nd = NewNd(Nnop)
		}
	case 10:
		//line parse.y:101
		{
			yyVAL.nd = nil
		}
	case 11:
		//line parse.y:108
		{
			lvl++
			Prompter.SetPrompt(prompt2)
		}
	case 12:
		//line parse.y:113
		{
			lvl--
			if lvl == 0 {
				Prompter.SetPrompt(prompt)
			}
			yyVAL.nd = yyS[yypt-0].nd
		}
	case 13:
		//line parse.y:124
		{
			f := NewNd(Nfunc, yyS[yypt-3].sval).Add(yyS[yypt-1].nd)
			yprintf("%s\n", f.sprint())
			funcs[yyS[yypt-3].sval] = f
			yyVAL.nd = nil
		}
	case 14:
		//line parse.y:134
		{
			yprintf("< %s\n", yyS[yypt-0].sval)
			lexer.source(yyS[yypt-0].sval)
			yyVAL.nd = nil
		}
	case 17:
		//line parse.y:148
		{
			yyVAL.sval = yyS[yypt-1].sval
		}
	case 18:
		//line parse.y:152
		{
			yyVAL.sval = "1"
		}
	case 19:
		//line parse.y:159
		{
			Prompter.SetPrompt(prompt2)
		}
	case 20:
		//line parse.y:163
		{
			Prompter.SetPrompt(prompt)
			last := yyS[yypt-5].nd.Last()
			if strings.Contains(yyS[yypt-3].sval, "0") {
				dbg.Warn("bad redirect for pipe")
				nerrors++
			}
			last.Redirs = append(last.Redirs, NewRedir(yyS[yypt-3].sval, "|", false)...)
			last.Redirs.NoDups()
			yyS[yypt-0].nd.Redirs = append(yyS[yypt-0].nd.Redirs, NewRedir("0", "|", false)...)
			yyS[yypt-0].nd.Redirs.NoDups()
			yyVAL.nd = yyS[yypt-5].nd.Add(yyS[yypt-0].nd)
		}
	case 21:
		//line parse.y:177
		{
			yyVAL.nd = NewList(Npipe, yyS[yypt-0].nd)
		}
	case 22:
		//line parse.y:184
		{
			yyVAL.nd = yyS[yypt-1].nd
			yyS[yypt-0].rdr.NoDups()
			yyVAL.nd.Redirs = yyS[yypt-0].rdr
		}
	case 23:
		//line parse.y:193
		{
			yyS[yypt-0].nd.Kind = Nexec
			yyVAL.nd = yyS[yypt-0].nd
		}
	case 24:
		//line parse.y:198
		{
			yyVAL.nd = yyS[yypt-1].nd
		}
	case 25:
		//line parse.y:202
		{
			yyS[yypt-1].nd.Kind = Nforblk
			yyVAL.nd = yyS[yypt-1].nd
		}
	case 26:
		yyVAL.nd = yyS[yypt-0].nd
	case 27:
		//line parse.y:208
		{
			yyVAL.nd = yyS[yypt-4].nd.Add(nil, yyS[yypt-1].nd)
		}
	case 28:
		//line parse.y:212
		{
			yyVAL.nd = NewList(Nfor, yyS[yypt-3].nd, yyS[yypt-1].nd)
		}
	case 29:
		//line parse.y:216
		{
			yyVAL.nd = NewList(Nwhile, yyS[yypt-3].nd, yyS[yypt-1].nd)
		}
	case 30:
		//line parse.y:223
		{
			yyVAL.nd = NewList(Nif, yyS[yypt-3].nd, yyS[yypt-1].nd)
		}
	case 31:
		//line parse.y:227
		{
			yyVAL.nd = yyS[yypt-5].nd.Add(yyS[yypt-3].nd, yyS[yypt-1].nd)
		}
	case 32:
		//line parse.y:234
		{
			yyVAL.rdr = yyS[yypt-0].rdr
		}
	case 33:
		//line parse.y:238
		{
			yyVAL.rdr = nil
		}
	case 34:
		//line parse.y:245
		{
			yyVAL.rdr = append(yyS[yypt-1].rdr, yyS[yypt-0].rdr...)
		}
	case 35:
		yyVAL.rdr = yyS[yypt-0].rdr
	case 36:
		yyVAL.rdr = yyS[yypt-0].rdr
	case 37:
		yyVAL.rdr = yyS[yypt-0].rdr
	case 38:
		//line parse.y:259
		{
			yyVAL.rdr = NewRedir("0", yyS[yypt-0].sval, false)
		}
	case 39:
		//line parse.y:266
		{
			if strings.Contains(yyS[yypt-1].sval, "0") {
				dbg.Warn("bad redirect for '>'")
				nerrors++
			}
			yyVAL.rdr = NewRedir(yyS[yypt-1].sval, yyS[yypt-0].sval, false)
		}
	case 40:
		//line parse.y:274
		{
			if strings.Contains(yyS[yypt-1].sval, "0") {
				dbg.Warn("bad redirect for '>'")
				nerrors++
			}
			yyVAL.rdr = NewRedir(yyS[yypt-1].sval, yyS[yypt-0].sval, true)
		}
	case 41:
		//line parse.y:282
		{
			yyVAL.rdr = NewDup(yyS[yypt-3].sval, yyS[yypt-1].sval)
		}
	case 42:
		//line parse.y:289
		{
			yyVAL.sval = "&"
		}
	case 43:
		//line parse.y:293
		{
			yyVAL.sval = yyS[yypt-0].sval
		}
	case 44:
		//line parse.y:297
		{
			yyVAL.sval = ""
		}
	case 45:
		//line parse.y:304
		{
			yyVAL.nd = NewNd(Nset, yyS[yypt-2].sval).Add(yyS[yypt-0].nd)
		}
	case 46:
		//line parse.y:308
		{
			yyVAL.nd = NewNd(Nset, yyS[yypt-2].sval).Add(yyS[yypt-0].nd)
		}
	case 47:
		//line parse.y:312
		{
			yyVAL.nd = NewNd(Nset, yyS[yypt-4].sval).Add(yyS[yypt-1].nd.Child...)
		}
	case 48:
		//line parse.y:316
		{
			yyVAL.nd = NewNd(Nset, yyS[yypt-5].sval, yyS[yypt-3].sval).Add(yyS[yypt-0].nd)
		}
	case 49:
		//line parse.y:320
		{
			yyVAL.nd = NewNd(Nset, yyS[yypt-7].sval, yyS[yypt-5].sval).Add(yyS[yypt-1].nd.Child...)
		}
	case 50:
		//line parse.y:324
		{
			yyS[yypt-1].nd.Args = append(yyS[yypt-1].nd.Args, yyS[yypt-4].sval)
			yyVAL.nd = yyS[yypt-1].nd
		}
	case 51:
		//line parse.y:332
		{
			yyVAL.nd = NewList(Ninblk, yyS[yypt-1].nd)
		}
	case 52:
		//line parse.y:336
		{
			yyVAL.nd = NewList(Nhereblk, yyS[yypt-1].nd)
		}
	case 53:
		//line parse.y:340
		{
			yyVAL.nd = NewList(Npipeblk, yyS[yypt-1].nd)
		}
	case 54:
		//line parse.y:347
		{
			yyVAL.nd = yyS[yypt-1].nd.Add(yyS[yypt-0].nd)
		}
	case 55:
		//line parse.y:351
		{
			yyVAL.nd = NewList(Nset, yyS[yypt-0].nd)
		}
	case 56:
		//line parse.y:357
		{
			yyVAL.nd = NewNd(Nset, yyS[yypt-2].sval, yyS[yypt-0].sval)
		}
	case 59:
		//line parse.y:369
		{
			yyVAL.nd = yyS[yypt-1].nd.Add(yyS[yypt-0].nd)
		}
	case 60:
		//line parse.y:373
		{
			yyVAL.nd = yyS[yypt-1].nd.Add(yyS[yypt-0].nd)
		}
	case 61:
		//line parse.y:377
		{
			yyVAL.nd = NewList(Nnames, yyS[yypt-0].nd)
		}
	case 62:
		//line parse.y:381
		{
			yyVAL.nd = NewList(Nnames, yyS[yypt-0].nd)
		}
	case 63:
		//line parse.y:388
		{
			yyVAL.nd = NewNd(Nname, yyS[yypt-0].sval)
		}
	case 64:
		//line parse.y:392
		{
			yyVAL.nd = NewNd(Nval, yyS[yypt-0].sval)
		}
	case 65:
		//line parse.y:396
		{
			yyVAL.nd = NewNd(Njoin, yyS[yypt-0].sval)
		}
	case 66:
		//line parse.y:400
		{
			yyVAL.nd = NewNd(Nval, yyS[yypt-3].sval, yyS[yypt-1].sval)
		}
	case 67:
		//line parse.y:404
		{
			yyVAL.nd = NewNd(Nlen, yyS[yypt-0].sval)
		}
	case 68:
		//line parse.y:408
		{
			yyVAL.nd = NewList(Napp, yyS[yypt-2].nd, yyS[yypt-0].nd)
		}
	}
	goto yystack /* stack new state and value */
}
