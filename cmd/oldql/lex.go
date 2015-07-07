package main

//go:generate Go tool yacc parse.y

import (
	"bytes"
	"clive/dbg"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"unicode"
)

type inText interface {
	ReadRune() (r rune, size int, err error)
	UnreadRune() error
}

type lex  {
	in            []inText
	saddr         []Addr
	eofmet, wasnl bool
	saved         int
	val           []rune
	Addr
}

var (
	debugLex bool
	lprintf  = dbg.FlagPrintf(os.Stderr, &debugLex)
	nerrors  int
	addr     Addr

	builtins = map[string]int{
		"if":    IF,
		"for":   FOR,
		"else":  ELSE,
		"elsif": ELSIF,
		"while": WHILE,
		"func":  FUNC,
	}
)

func init() {
	addr = Addr{"input", 1}
	flag.BoolVar(&debugLex, "L", false, "debug lex")
}

func newLex(name string, in inText) *lex {
	return &lex{
		in:   []inText{in},
		Addr: Addr{name, 1},
	}
}

func (l *lex) source(what string) {
	dat, err := ioutil.ReadFile(what)
	if err != nil {
		dbg.Warn("open: %s: %s", what, err)
		return
	}
	l.in = append([]inText{bytes.NewBuffer(dat)}, l.in...)
	l.saddr = append([]Addr{l.Addr}, l.saddr...)
	l.Addr = Addr{what, 1}
}

func (l *lex) Error(s string) {
	nerrors++
	dbg.Warn("%s: near %s: %s", addr, l.getval(), s)
}

func (l *lex) get() rune {

	r, _, err := l.in[0].ReadRune()

	// if we are reading ignoring C-c and get a C-c
	// then we must panic so the caller of the parser
	// may recover and re-start the parser with new
	// data from stdin starting now.
	// see interactive() in ql.go
	if !IntrExits && Interrupted {
		Interrupted = false
		panic(ErrIntr)
		return 0
	}

	if err != nil {
		r = 0
		if !l.eofmet && !Interactive {
			r = '\n'
		}
		l.eofmet = err == io.EOF
		if r==0 && len(l.in)>1 {
			l.eofmet = false
			l.in = l.in[1:]
			l.Addr = l.saddr[0]
			l.saddr = l.saddr[1:]
			return l.get()
		}
		return r
	}
	l.val = append(l.val, r)
	return r
}

func (l *lex) unget() {
	if l.eofmet {
		if !Interactive {
			l.eofmet = false
		}
		return
	}
	if err := l.in[0].UnreadRune(); err != nil {
		dbg.Fatal("lex: bug: unreadrune: %s", err)
	}
	l.val = l.val[0 : len(l.val)-1]
}

func (l *lex) getval() string {
	if len(l.val) == 0 {
		return ""
	}
	return string(l.val)
}

func (l *lex) Lex(lval *yySymType) int {
	t := l.saved
	l.saved = 0
	if t == 0 {
		t = l.lex(lval)
	}
	lprintf("tok %s\n", tokstr(t, lval))
	return t
}

func (l *lex) skipComment() rune {
	for {
		c := l.get()
		if c==0 || c=='\n' {
			return c
		}
	}
}

func (l *lex) lex(lval *yySymType) int {
	l.val = l.val[:0]
	var c rune
	for {
		c = l.get()
		if c == '#' {
			c = l.skipComment()
			l.wasnl = true
		}
		if c == 0 {
			l.wasnl = true
			return 0
		}
		if c == '\n' {
			l.val = l.val[:0]
			addr = l.Addr
			l.Line++
			l.wasnl = true
			return NL
		}
		if !unicode.IsSpace(c) {
			if l.wasnl && c=='>' {
				l.val = l.val[:0]
				continue
			}
			break
		}
		l.val = l.val[:0]
	}
	l.wasnl = false
	addr = l.Addr
	switch c {
	case '\'', '`':
		return l.scanQuote(c, lval)
	case '{', '}', '&', ';', '[', ']', '~', '|', '^', '=':
		return int(c)
	case '$':
		switch c = l.get(); c {
		case '#':
			return LEN
		}
		l.unget()
		return '$'
	case '<':
		switch c = l.get(); c {
		case '{':
			return INBLK
		case '<', '|':
			if x := l.get(); x == '{' {
				if c == '|' {
					return PIPEBLK
				}
				return HEREBLK
			}
			// no way out; can't be << unless { follows.
			l.unget()
			return ERROR
		}
		l.unget()
		return '<'
	case '>':
		switch c = l.get(); c {
		case '>':
			return APP
		case '{':
			return FORBLK
		}
		l.unget()
		return '>'
	}
	return l.scanName(lval)
	return 0
}

func (l *lex) scanQuote(q rune, lval *yySymType) int {
	l.val = l.val[:0]
	for {
		c := l.get()
		if c == q {
			l.val = l.val[:len(l.val)-1]
			lval.sval = l.getval()
			return NAME
		}
		if c == 0 {
			l.Error("unclosed quoted string")
			return 0
		}
		if c == '\n' {
			l.Line++
		}
	}

}

func isPunct(c rune) bool {
	if unicode.IsSpace(c) {
		return true
	}
	return strings.ContainsRune("'`<>{}&;=[]~|$#^", c)
}

func (l *lex) scanName(lval *yySymType) int {
	for {
		c := l.get()
		if isPunct(c) {
			l.unget()
			lval.sval = l.getval()
			if tok, ok := builtins[lval.sval]; ok {
				return tok
			}
			return NAME
		}
	}
}

func tokstr(tok int, lval *yySymType) string {
	switch tok {
	case 0:
		return "EOF"
	case NAME:
		return fmt.Sprintf("name<%s>", lval.sval)
	case NL:
		return "nl"
	case '<', '>', '{', '}', '&', ';', '=', '[', ']', '~', '$', '|', '#', '^':
		return fmt.Sprintf("%c", rune(tok))
	case FORBLK:
		return ">{"
	case LEN:
		return "$#"
	case INBLK:
		return "<{"
	case HEREBLK:
		return "<<{"
	case PIPEBLK:
		return "<|{"
	case APP:
		return ">>"
	case IF:
		return "if"
	case ELSE:
		return "else"
	case ELSIF:
		return "elsif"
	case FOR:
		return "for"
	case WHILE:
		return "while"
	case FUNC:
		return "func"
	case ERROR:
		return "err"
	default:
		return fmt.Sprintf("BADTOK<%d>", tok)
	}
}
