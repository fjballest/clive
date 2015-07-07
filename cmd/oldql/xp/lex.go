package xp

import (
	"clive/cmd/opt"
	"clive/dbg"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type tok int

const (
	tNone    tok = 0
	tNum     tok = NUM
	tInt     tok = INT
	tFunc    tok = FUNC
	tLpar    tok = '('
	tRpar    tok = ')'
	tAdd     tok = '+'
	tSub     tok = '-'
	tDiv     tok = '/'
	tMul     tok = '*'
	tMod     tok = '%'
	tBnot    tok = '^'
	tBand    tok = '&'
	tBor     tok = '|'
	tName    tok = NAME
	tLE      tok = LE
	tGE      tok = GE
	tLess    tok = '<'
	tGreater tok = '>'
	tEq      tok = '='
	tEqn     tok = EQN
	tNeq     tok = NEQ
	tNot     tok = '!'
	tAnd     tok = AND
	tOr      tok = OR
	tTime    tok = TIME
)

type lex  {
	in     []rune
	p0, p1 int

	wasfunc bool
}

var (
	debugLex bool
	lprintf  = dbg.FlagPrintf(os.Stderr, &debugLex)
)

func newLex(input string) *lex {
	return &lex{
		in: []rune(input),
	}
}

func (l *lex) Error(s string) {
	panic(fmt.Errorf("near %s: %s", l.val(), s))
}

func (l *lex) get() rune {
	if l.p1 >= len(l.in) {
		return 0
	}
	r := l.in[l.p1]
	l.p1++
	return r
}

func (l *lex) unget() {
	if l.p1 > 0 {
		l.p1--
	}
}

func (l *lex) val() string {
	s := string(l.in[l.p0:l.p1])
	l.p0 = l.p1
	return s
}

func (l *lex) drop() {
	l.p0 = l.p1
}

func (l *lex) Lex(lval *yySymType) int {
	eqtoks := map[rune]tok{
		'<': tLE,
		'>': tGE,
		'=': tEqn,
		'!': tNeq,
	}
	var c rune
	wasfunc := l.wasfunc
	l.wasfunc = false
	for {
		l.drop()
		c = l.get()
		t := tok(c)
		switch {
		case c == 0:
			lprintf("tok %v\n", t)
			return int(t)
		case unicode.IsSpace(c):
			continue
		case c == '"':
			l.brackets('"')
			str := l.val()
			if str[0] == '"' {
				str = str[1:]
			}
			if str[len(str)-1] == '"' {
				str = str[:len(str)-1]
			}
			lval.sval = str
			lprintf("tok %v\n", lval.tval)
			return int(tName)
		case c == '[':
			l.brackets(']')
			str := l.val()
			if str[0] == '[' {
				str = str[1:]
			}
			if str[len(str)-1] == ']' {
				str = str[:len(str)-1]
			}
			tm, err := opt.ParseTime(str)
			if err != nil {
				l.Error(err.Error())
			}
			lval.tval = tm
			lprintf("tok %v\n", lval.tval)
			return int(tTime)
		case c == '&':
			nc := l.get()
			if nc == c {
				return int(tAnd)
			}
			return int(t)
		case c == '|':
			nc := l.get()
			if nc == c {
				return int(tOr)
			}
			return int(t)
		case strings.ContainsRune("+-*/%()^<>=!", c):
			if et := eqtoks[c]; et != 0 {
				nc := l.get()
				if nc == '=' {
					lprintf("tok %c\n", et)
					return int(et)
				}
				l.unget()
			}
			lprintf("tok %c\n", t)
			return int(t)
		case c == '-':
			nc := l.get()
			l.unget()
			if !unicode.IsDigit(nc) {
				lprintf("tok %c\n", t)
				return int(t)
			}
			l.unget()
		case unicode.IsDigit(c):
			l.unget()
		case !unicode.IsSpace(c):
			l.alpha()
			lval.sval = l.val()
			if lval.sval == "now" {
				lval.tval = time.Now()
				lprintf("tok %v\n", lval.tval)
				return int(tTime)
			}
			lprintf("tok %v\n", lval.sval)
			if wasfunc {
				return int(tName)
			}
			l.wasfunc = true
			return int(tFunc)
		default:
			l.Error("unknown token")
		}
		// TODO: look ahead for things that look like
		// a date and return a time: 01/02/03 or 04:02:01
		// this requires being able to move the lex back more than one rune.
		t, lval.ival, lval.fval = l.number()
		if t == tNum {
			lprintf("tok %v\n", lval.fval)
		} else {
			lprintf("tok %v\n", lval.ival)
		}
		return int(t)
	}
}

func (l *lex) alpha() {
	for {
		c := l.get()
		if c == 0 {
			break
		}
		if unicode.IsSpace(c) {
			l.unget()
			break
		}
	}
}

func (l *lex) digits() {
	for {
		c := l.get()
		if c == 0 {
			break
		}
		if !unicode.IsDigit(c) {
			l.unget()
			break
		}
	}
}

func (l *lex) brackets(end rune) {
	for {
		c := l.get()
		if c==0 || c==end {
			break
		}
	}
}

func (l *lex) hexdigits() {
	for {
		c := l.get()
		if c == 0 {
			break
		}
		if !unicode.IsDigit(c) && !strings.ContainsRune("ABCDEF", c) {
			l.unget()
			break
		}
	}
}

// 0digits	-> uint
// 0xdigits -> uint
// digits [kKmMgG] -> uint [in kb, mb, gb]
// ±[digits][.]digits[eE]±digits -> float
func (l *lex) number() (tok, uint64, float64) {
	if c := l.get(); c == '0' {
		if c = l.get(); c=='x' || c=='X' {
			l.hexdigits()
		} else {
			l.digits()
		}
		n, err := strconv.ParseUint(l.val(), 0, 64)
		if err != nil {
			l.Error(err.Error())
		}
		return tInt, n, 0.0
	}
	if c := l.get(); c!=0 && c!='+' && c!='-' {
		l.unget()
	}
	l.digits()
	c := l.get()
	if x := unicode.ToLower(c); x=='k' || x=='m' || x=='g' {
		l.unget()
		n, err := strconv.ParseUint(l.val(), 0, 64)
		l.get()
		if err != nil {
			l.Error(err.Error())
		}
		switch c {
		case 'k':
			n *= 1024
		case 'm':
			n *= 1024*1024
		case 'g':
			n *= 1024*1024*1024
		}
		return tInt, n, 0.0
	}
	if c != '.' {
		if c != 0 {
			l.unget()
		}
		n, err := strconv.ParseUint(l.val(), 0, 64)
		if err != nil {
			l.Error(err.Error())
		}
		return tInt, n, 0.0
	}
	l.digits()
	if c := l.get(); c!='e' && c!='E' {
		if c != 0 {
			l.unget()
		}
	} else {
		if c := l.get(); c!='+' && c!='-' {
			if c != 0 {
				l.unget()
			}
		}
		l.digits()
	}
	n, err := strconv.ParseFloat(l.val(), 64)
	if err != nil {
		l.Error(err.Error())
	}
	return tNum, 0, n
}
