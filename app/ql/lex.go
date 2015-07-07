package ql

import (
	"fmt"
	"clive/app"
	"clive/app/nsutil"
	"bytes"
	"io"
	"clive/dbg"
	"unicode"
	"strings"
)

type inText interface {
	ReadRune() (r rune, size int, err error)
}

// address in file
type Addr  {
	File string
	Line int
}

type lex  {
	in            []inText
	saddr         []Addr
	eofmet, wasnl bool
	saved         rune
	val           []rune
	Addr
	interactive, interrupted bool
	prompt string
	nerrors int
	debug bool
}

var (
	builtins = map[string]int{
		"for":   FOR,
		"while": WHILE,
		"func":  FUNC,
	}
)

func (a Addr) String() string {
	return fmt.Sprintf("%s:%d", a.File, a.Line)
}

func newLex(name string, in inText) *lex {
	return &lex{
		in:   []inText{in},
		Addr: Addr{name, 1},
	}
}

func (l *lex) AtEof() bool {
	return l.eofmet
}

func (l *lex) dprintf(fmts string, args ...interface{}) {
	if l.debug {
		app.Eprintf(fmts, args...)
	}
}

func (l *lex) source(what string) {
	dat, err := nsutil.GetAll(what)
	if err != nil {
		app.Warn("open: %s: %s", what, err)
		return
	}
	l.in = append([]inText{bytes.NewBuffer(dat)}, l.in...)
	l.saddr = append([]Addr{l.Addr}, l.saddr...)
	l.Addr = Addr{what, 1}
}

func (l *lex) Error(s string) {
	l.nerrors++
	app.Warn("%s: %s", l.Addr, s)
}

func (l *lex) Errs(fmts string, args ...interface{}) {
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
	if err == dbg.ErrIntr {
		l.interrupted = true
	}
	if l.interactive && l.interrupted {
		l.interrupted = false
		app.Printf("\n")
		panic(dbg.ErrIntr)
		return 0
	}

	if err != nil {
		r = 0
		if !l.eofmet && !l.interactive {
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
	l.dprintf("%s: tok %s\n", l.Addr, tokstr(t, lval))
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

// Using channels for the input make it really tricky to issue a prompt
// at the right time.
func (l *lex) lex(lval *yySymType) int {
	l.val = l.val[:0]
	var c rune
	prompted := false
	for {
		if l.interactive && l.wasnl && l.prompt != "" && !prompted{
			app.Printf("%s", l.prompt)
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
			if l.wasnl && c=='%' {
				l.val = l.val[:0]
				continue
			}
			break
		}
		l.val = l.val[:0]
	}
	l.wasnl = false
	switch c {
	case '\'', '`':
		return l.scanQuote(c, lval)
	case '←':
		return '='
	case '|':
		switch c = l.get(); c {
		case '|':
			return OR
		case '>':
			return GFPIPE
		}
		l.unget()
		return '|'
	case '&':
		if c = l.get(); c == '&' {
			return AND
		}
		l.unget()
		return '&'
	case '{', '}', ';', '[', ']', '^', '=':
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
			switch x := l.get(); x {
			case '{':
				if c == '|' {
					return PIPEBLK
				}
				return RAWINBLK
			case '<':
				if c == '<' && l.get() == '{' {
					return SINGLEINBLK
				}
			}
			// no way out and can't be any other thing as valid syntax.
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
			return TEEBLK
		}
		l.unget()
		return '>'
	case '-':
		if c = l.get(); c == '|' {
			return INPIPE;
		}
		l.unget()
		// and fall to scanName
	}
	return l.scanName(lval)
}

func (l *lex) scanQuote(q rune, lval *yySymType) int {
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
			l.Error(fmt.Sprintf("unclosed quote open at %s:%d", l.File, ln))
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
	return  unicode.IsSpace(c) || strings.ContainsRune("'←`<>{}&;[]|#^", c)
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
	case TEEBLK:
		return ">{"
	case LEN:
		return "$#"
	case INBLK:
		return "<{"
	case RAWINBLK:
		return "<<{"
	case SINGLEINBLK:
		return "<<<{"
	case PIPEBLK:
		return "<|{"
	case APP:
		return ">>"
	case FOR:
		return "for"
	case WHILE:
		return "while"
	case FUNC:
		return "func"
	case ERROR:
		return "err"
	case OR:
		return "||"
	case AND:
		return "&&"
	case GFPIPE:
		return "|>"
	case INPIPE:
		return "-|"
	default:
		return fmt.Sprintf("BADTOK<%d>", tok)
	}
}
