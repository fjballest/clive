package main

import (
	"bytes"
	"clive/cmd"
	"clive/dbg"
	"clive/zx"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
)

interface inText {
	ReadRune() (r rune, size int, err error)
	Name() string
}

struct bufRdr {
	bytes.Buffer
	name string
}

// address in file
struct Addr {
	rdr  inText
	Line int
}

struct lex {
	dbg.Flag
	in            []inText
	saddr         []Addr
	eofmet, wasnl bool
	notfirst bool
	saved         rune
	val           []rune
	Addr
	interactive, interrupted bool
	prompt                   string
	nerrors                  int
}

var (
	ErrIntr  = errors.New("interrupted")
	keywords = map[string]int{
		"for":   FOR,
		"while": WHILE,
		"func":  FUNC,
		"cond": COND,
		"or": OR,
	}
)

func (a Addr) String() string {
	return fmt.Sprintf("%s:%d", a.rdr.Name(), a.Line)
}

func (b *bufRdr) Name() string {
	if b == nil {
		return "in"
	}
	return b.name
}

func newLex(in inText) *lex {
	return &lex{
		in:   []inText{in},
		Addr: Addr{in, 1},
		prompt: "> ",
		wasnl: true,
	}
}

func (l *lex) AtEof() bool {
	return l.eofmet
}

func (l *lex) source(what string) error {
	dat, err := zx.GetAll(cmd.NS(), cmd.AbsPath(what))
	if err != nil {
		cmd.Warn("open: %s: %s", what, err)
		return err
	}
	rdr := &bufRdr{name: what}
	rdr.Write(dat)
	l.in = append([]inText{rdr}, l.in...)
	l.saddr = append([]Addr{l.Addr}, l.saddr...)
	l.Addr = Addr{rdr, 1}
	return nil
}

func (l *lex) Error(s string) {
	l.nerrors++
	cmd.Warn("%s: %s", l.Addr, s)
}

func (l *lex) Errs(fmts string, args ...face{}) {
	l.Error(fmt.Sprintf(fmts, args...))
}

func (l *lex) get() rune {
	if l.saved != 0 {
		r := l.saved
		l.saved = 0
		l.val = append(l.val, r)
		return r
	}
	r, _, err := l.in[0].ReadRune()

	// if we are reading ignoring C-c and get a C-c
	// then we must panic so the caller of the parser
	// may recover and re-start the parser with new
	// data from stdin starting now.
	if err == ErrIntr {
		l.interrupted = true
	}
	if l.interactive && l.interrupted {
		l.interrupted = false
		cmd.Printf("\n")
		panic(ErrIntr)
		return 0
	}

	if err != nil {
		r = 0
		if !l.eofmet && !l.interactive {
			r = '\n'
		}
		l.eofmet = err == io.EOF
		if r == 0 && len(l.in) > 1 {
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
	if l.eofmet && false {
		if !l.interactive {
			l.eofmet = false
		}
		return
	}
	l.saved = l.val[len(l.val)-1]
	l.val = l.val[0 : len(l.val)-1]
}

func (l *lex) getval() string {
	if len(l.val) == 0 {
		return ""
	}
	return string(l.val)
}

func (l *lex) Lex(lval *yySymType) int {
	t := l.lex(lval)
	l.Dprintf("%s: tok %s\n", l.Addr, tokstr(t, lval))
	return t
}

func (l *lex) skipComment() rune {
	for {
		c := l.get()
		if c == 0 || c == '\n' {
			return c
		}
	}
}

// Using channels for the input make it really tricky to issue a prompt
// at the right time.
func (l *lex) lex(lval *yySymType) int {
	l.val = l.val[:0]
	lval.sval = ""
	var c rune
	prompted := false
	for {
		if l.interactive && l.wasnl && l.prompt != "" && !prompted {
			cmd.Printf("%s", l.prompt)
			prompted = true
		}
		c = l.get()
		if c == '#' {
			c = l.skipComment()
			l.wasnl = true
			prompted = false
		}
		if c == 0 {
			l.wasnl = true
			return 0
		}
		if c == '\n' {
			l.val = l.val[:0]
			l.Line++
			l.wasnl = true
			return NL
		}
		if !unicode.IsSpace(c) {
			if l.wasnl && c == '>' {
				l.val = l.val[:0]
				continue
			}
			break
		}
		l.val = l.val[:0]
	}
	if l.wasnl {
		l.notfirst = false
	}
	l.wasnl = false
	switch c {
	case '\'', '`':
		return l.scanQuote(c, lval, "quote")
	case '←':
		return '='
	case '|':
		switch c = l.get(); c {
		case '|':
			return OR
		case '[':
			l.scanQuote(']', lval, "[")
		default:
			l.unget()
		}
		return PIPE
	case '&':
		switch c = l.get(); {
		case c == '&':
			return AND
		case isPunct(c):
			l.unget()
		default:
			l.val = l.val[:1]
			l.val[0] = c
			l.scanName(lval)
		}
		return BG
	case '{', '}', ';', '[', ']', '^', '=', '(', ')':
		if c == ';' || c == '{' {
			l.notfirst = false
		}
		return int(c)
	case '$':
		switch c = l.get(); c {
		case '#':
			return LEN
		case '^':
			return SINGLE
		default:
			l.unget()
		}
		return '$'
	case '<':
		switch c = l.get(); c {
		case '[':
			l.scanQuote(']', lval, "[")
			if c := l.get(); c == '{' {
				return INBLK
			} else {
				l.unget()
			}
		case '{':
			return INBLK
		default:
			l.unget()
		}
		return IREDIR
	case '>':
		switch c = l.get(); c {
		case '>':
			if c2 := l.get(); c2 == '[' {
				l.scanQuote(']', lval, "[")
			} else {
				l.unget()
			}
			return APP
		case '[':
			l.scanQuote(']', lval, "[")
			if c := l.get(); c == '{' {
				return OUTBLK
			} else {
				l.unget()
			}
		default:
			l.unget()
		}
		return OREDIR
	}
	return l.scanName(lval)
}

func (l *lex) scanQuote(q rune, lval *yySymType, what string) int {
	ln := l.Line
	l.val = l.val[:0]
	for {
		c := l.get()
		if c == q {
			l.val = l.val[:len(l.val)-1]
			lval.sval = l.getval()
			return NAME
		}
		if c == 0 {
			l.Error(fmt.Sprintf("unclosed %s open at %s:%d",
				what, l.rdr.Name(), ln))
			return 0
		}
		if c == '\n' {
			l.Line++
		}
	}

}

func isPunct(c rune) bool {
	// runes =$ found within a word do not break the word.
	// they are tokens on their own only if they are found outside a word
	return unicode.IsSpace(c) || strings.ContainsRune("'←`<>{}&;[]|#^()", c)
}

func (l *lex) scanName(lval *yySymType,) int {
	for {
		c := l.get()
		if isPunct(c) || !l.notfirst && c == '=' {
			l.unget()
			lval.sval = l.getval()
			l.notfirst = true
			if tok, ok := keywords[lval.sval]; ok {
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
	case IREDIR:
		return fmt.Sprintf("<(%s)", lval.sval)
	case OREDIR:
		return fmt.Sprintf(">(%s)", lval.sval)
	case PIPE:
		return fmt.Sprintf("|(%s)", lval.sval)
	case BG:
		return fmt.Sprintf("&(%s)", lval.sval)
	case '{', '}', ';', '=', '[', ']', '$', '^':
		return fmt.Sprintf("%c", rune(tok))
	case FOR:
		return "for"
	case WHILE:
		return "while"
	case FUNC:
		return "func"
	case NL:
		return "nl"
	case NAME:
		return fmt.Sprintf("name(%s)", lval.sval)
	case OR:
		return "||"
	case AND:
		return "&&"
	case LEN:
		return "$#"
	case SINGLE:
		return "$^"
	case APP:
		return ">>"
	case INBLK:
		return "<{}"
	case OUTBLK:
		return ">{}"
	case ERROR:
		return "err"
	default:
		return fmt.Sprintf("BADTOK<%d>", tok)
	}
}
