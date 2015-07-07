package comp

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	godebug "runtime/debug"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
	"unsafe"
)

var (
	debug     map[rune]int = make(map[rune]int)
	globalsok bool
)

const (
	Aincr = 128

	// limits
	Maxsval = 1024
	// limit on arry index sizes
	Maxidx = 0x40000000

	Nbighash = 997
	Nhash    = 101
	Eof      = -1

	LexerId
)

type Scan  {
	fname      string
	lineno     int
	sval       []rune
	sarr       [Maxsval]rune
	bin        *bufio.Reader
	issavedtok bool
}

func (sc *Scan) nextr() rune {
	c, _, e := sc.bin.ReadRune()
	if e == io.EOF {
		return Eof
	}
	if e != nil {
		panic(e)
	}
	if c == '\n' {
		sc.lineno++
	}
	return c
}

func (sc *Scan) nextrune() rune {
	c := sc.nextr()
	if len(sc.sval) < cap(sc.sval) {
		sc.sval = append(sc.sval, c)
	}
	return c
}

func (sc *Scan) putback(c rune) {
	if len(sc.sval) > 0 {
		sc.sval = sc.sval[0 : len(sc.sval)-1]
	}
	if c == '\n' {
		sc.lineno--
	}
	sc.bin.UnreadRune()
}

type PickyLex int

func (sc *Scan) toknum(c rune, isreal bool, Yylval *YySymType) (int, bool) {
	var (
		err error
		ii  int64
	)
	if isreal {
		c = sc.nextrune()
		if c == '.' {
			//special case, found two dots for range 23..4
			nodots := strings.TrimSuffix(string(sc.sval), "..")
			ii, err = strconv.ParseInt(nodots, 0, 64)
			if err != nil {
				diag("bad int token %s", string(sc.sval))
			}
			Yylval.ival = int(ii)
			return INT, true
		}
		for c = sc.nextrune(); unicode.IsDigit(c); c = sc.nextrune() {
		}
	}
	if c=='E' || c=='e' {
		isreal = true
		c = sc.nextrune()
		if c=='+' || c=='-' {
			c = sc.nextrune()
		}
		for ; unicode.IsDigit(c); c = sc.nextrune() {
		}
	}
	sc.putback(c)
	if isreal {
		Yylval.rval, err = strconv.ParseFloat(string(sc.sval), 64)
		if err != nil {
			diag("bad float token %s", string(sc.sval))
		}
		return REAL, false
	}
	ii, err = strconv.ParseInt(string(sc.sval), 0, 64)
	if err != nil {
		diag("bad int token %s", string(sc.sval))
	}
	Yylval.ival = int(ii)
	return INT, false
}

func (sc *Scan) skipcom() (c rune) {
	for {
		c = sc.nextr()
		if c == '*' {
			c = sc.nextr()
			if c == '/' {
				break
			}
			sc.putback(c)
		}
		if c == Eof {
			break
		}
	}
	return c
}

func (sc *Scan) tokstr() (c rune) {
	sc.sval = sc.sval[0:0]
	for {
		c = sc.nextr()
		if c=='"' || c==Eof {
			break
		}
		if len(sc.sval) < cap(sc.sval) {
			if utf8.RuneLen(c) > 1 {
				diag("rune '%c' does not fit in a char", c)
			} else if c == '\n' {
				diag("newline in string")
			} else {
				sc.sval = append(sc.sval, c)
			}
		} else {
			panic("BUG: sval too small")
		}
	}
	return c
}

var (
	Scanner *Scan
)

func Lex(Yylval *YySymType) int {
	var (
		c      rune
		isreal bool
		s      *Sym
		tn     int
	)

	sc := Scanner
	if sc.issavedtok {
		sc.issavedtok = false
		sc.sval = sc.sarr[0:2]
		sc.sval[0] = '.'
		sc.sval[1] = '.' // BUG: if savedtok != ".."
		return DOTDOT
	}
Again:
	sc.sval = sc.sarr[0:0]
	for c = sc.nextr(); unicode.IsSpace(c); {
		c = sc.nextr()
	}
	if c == Eof {
		return int(c)
	}
	sc.putback(c)
	sc.nextrune() // save in sval
	if c>=utf8.RuneSelf || unicode.IsLetter(c) {
		for {
			if cap(sc.sval) <= len(sc.sval) {
				panic("max word size exceeded")
			}
			c = sc.nextrune()
			valrune := unicode.IsDigit(c)
			valrune = valrune || unicode.IsLetter(c) || c=='_' || c>utf8.RuneSelf
			if !valrune {
				break
			}
		}
		sc.putback(c)
		return ID
	}
	if unicode.IsDigit(c) {
		for c = sc.nextrune(); unicode.IsDigit(c); c = sc.nextrune() {
		}
		isreal = c == '.'
		tn, sc.issavedtok = sc.toknum(c, isreal, Yylval)
		return tn
	}
	switch c {
	case '[', ']', ',', '(', ')', '{', '}', '+', '^', ':', ';', '%':
		return int(c)
	case '=':
		c = sc.nextrune()
		if c == '=' {
			return EQ
		}
		sc.putback(c)
		return '='
	case '*':
		c = sc.nextrune()
		if c == '*' {
			return POW
		}
		sc.putback(c)
		return '*'
	case '"':
		c = sc.tokstr()
		if c == Eof {
			diag("missing closing '\"'")
			return Eof
		}
		s = strlookup(string(sc.sval))
		Yylval.sval = s.name
		return STR
	case '\'':
		c = sc.nextrune()
		if c == Eof {
			diag("missing closing \"'\"")
			return Eof
		}
		if sc.nextrune() != '\'' {
			diag("missing closing \"'\"")
		}
		if utf8.RuneLen(c) > 1 {
			diag("rune '%c' does not fit in a char", c)
		}
		Yylval.ival = int(c)
		return CHAR
	case '.':
		c = sc.nextrune()
		if c == '.' {
			return DOTDOT
		}
		sc.putback(c)
		if unicode.IsDigit(c) {
			isreal = true
			tn, sc.issavedtok = sc.toknum(c, isreal, Yylval)
			return tn
		}
		return '.'
	case '>':
		c = sc.nextrune()
		if c == '=' {
			return GE
		}
		sc.putback(c)
		return '>'
	case '!':
		c = sc.nextrune()
		if c != '=' {
			diag("missing '=' after '!'")
			sc.putback(c)
			return BADOP
		}
		return NE
	case '<':
		c = sc.nextrune()
		if c == '=' {
			return LE
		}
		sc.putback(c)
		return '<'
	case '/':
		c = sc.nextrune()
		if c == '*' {
			c = sc.skipcom()
			if c == Eof {
				diag("missing end of comment")
				return Eof
			}
			goto Again
		}
		sc.putback(c)
		return '/'
	case '-':
		c = sc.nextrune()
		if c == '>' {
			return '^'
		}
		sc.putback(c)
		return '-'
	case Eof:
		return Eof
	default:
		diag("bad character '%c'0x%d in input", c, c)
		goto Again
	}
	return -1
}

var toknames = map[rune]string{
	OR:        "OR",
	AND:       "AND",
	EQ:        "EQ",
	NE:        "NE",
	LE:        "LE",
	GE:        "GE",
	'<':       "LT",
	'>':       "GT",
	'+':       "PLUS",
	'-':       "MINUS",
	'*':       "STAR",
	POW:       "POW",
	'/':       "SLASH",
	'%':       "MOD",
	'[':       "LBRAC",
	'(':       "LPAREN",
	'{':       "LCURL",
	']':       "RBRAC",
	')':       "RPAREN",
	'}':       "RCURL",
	'=':       "ASSIGN",
	',':       "COMMA",
	':':       "COLON",
	';':       "SEMICOLON",
	'.':       "DOT",
	'^':       "ARROW",
	ARRAY:     "ARRAY",
	CASE:      "CASE",
	CONSTS:    "CONSTS",
	DEFAULT:   "DEFAULT",
	DO:        "DO",
	DOTDOT:    "DOTDOT",
	ELSE:      "ELSE",
	FALSE:     "FALSE",
	FOR:       "FOR",
	FUNCTION:  "FUNCTION",
	IF:        "IF",
	SWITCH:    "SWITCH",
	NIL:       "NIL",
	NOT:       "NOT",
	OF:        "OF",
	PROCEDURE: "PROCEDURE",
	PROGRAM:   "PROGRAM",
	RECORD:    "RECORD",
	REF:       "REF",
	RETURN:    "RETURN",
	TRUE:      "TRUE",
	TYPES:     "TYPES",
	VARS:      "VARS",
	WHILE:     "WHILE",
	Eof:       "EOF",
	BADOP:     "BADOP",
	LEN:       "LEN",
}

func fmtyval(c rune, Yylval *YySymType) string {
	//BUG, convert to m literal of formats
	switch c {
	case CHAR:
		return fmt.Sprintf("CHAR '%c'", Yylval.ival)
	case ID:
		return fmt.Sprintf("ID '%s'", Yylval.sym.name)
	case INT:
		return fmt.Sprintf("INT %d", Yylval.ival)
	case REAL:
		return fmt.Sprintf("REAL %2g", Yylval.rval)
	case STR:
		return fmt.Sprintf("STR \"%s\"", Yylval.sval)
	case TYPEID:
		return fmt.Sprintf("TYPEID '%s'", Yylval.sym.name)
	default:
		s, ok := toknames[c]
		if !ok {
			s := fmt.Sprintf("bad tok %d", c)
			panic(s)
		}
		return fmt.Sprintf("%s", s)
	}
}

func (PickyLex) Lex(Yylval *YySymType) int {
	var c rune

	sc := Scanner
	t := Lex(Yylval)
	if t == ID {
		s := keylookup(string(sc.sval))
		Yylval.sym = s
		s.fname = sc.fname
		s.lineno = sc.lineno
		switch s.stype {
		case Skey:
			t = s.tok
		case Stype:
			t = TYPEID
		}
	}
	c = rune(t)
	if debug['L'] != 0 { //BUG, convert to m literal of formats
		fmt.Fprintf(os.Stderr, "%s\n", fmtyval(c, Yylval))
	}
	return int(c)

}

type Derr string

func (d Derr) Error() string {
	return string(d)
}

type Dflag  {
	name rune
}

func (d *Dflag) Set(s string) error {
	_, ok := debug[d.name]
	if ok {
		debug[d.name]++
	} else {
		debug[d.name] = 1
	}
	//fmt.Printf("setting %c: %d", d.name,  debug[d.name])
	return nil
}

func (d *Dflag) String() string {
	return fmt.Sprintf("[%d]", debug[d.name])
}

func (d *Dflag) IsBoolFlag() bool {
	return true
}

var (
	Sflag = false
	V     = false
	cflag Dflag
	lflag Dflag
	eflag Dflag
	pflag Dflag
	yflag Dflag

	gflag bool
	vflag bool
	dflag bool
	nflag Dflag
)

func init() {
	flag.Var(&cflag, "D", "debug declarations")
	cflag.name = 'D'
	flag.Var(&lflag, "L", "debug lex")
	lflag.name = 'L'
	flag.Var(&eflag, "E", "debug expressions")
	eflag.name = 'E'
	flag.Var(&pflag, "P", "debug program")
	pflag.name = 'P'
	flag.Var(&yflag, "Y", "Yydebug")
	yflag.name = 'Y'

	flag.BoolVar(&Sflag, "S", false, "sflag")
	flag.BoolVar(&globalsok, "g", false, "globals ok")
	flag.BoolVar(&V, "v", false, "return version")
	flag.BoolVar(&dflag, "d", false, "dump stack")

	flag.StringVar(&Oname, "o", "out.pam", "output file name")
}

func Processfile(name string) (e error) {
	defer func() {
		if r := recover(); r != nil {
			errs := fmt.Sprint(r)
			if strings.HasPrefix(errs, "runtime error:") {
				errs = strings.Replace(errs, "runtime error:", "compiler error:", 1)
			}
			e = errors.New(errs)
			if dflag {
				fmt.Fprintf(os.Stderr, "%s", godebug.Stack())
			}
		}
	}()
	e = nil
	fd, err := os.Open(name)
	Scanner = NewScanner(bufio.NewReader(fd), name, 1)
	defer fd.Close()
	s := fmt.Sprintf("%s %v", name, err)
	if err != nil {
		panic(s)
	}
	YyParse(PickyLex(LexerId))
	return nil
}

func Mkout(n string) (io.Closer, *bufio.Writer) {
	os.Remove(n)
	f, err := os.Create(n)
	if err != nil {
		es := fmt.Sprintf("%s: %v", err)
		panic(es)
	}
	out := bufio.NewWriter(f)
	return f, out
}

//BUG, TODO: The sizes printed here are indicative,
//they do not take in account the size of the
//reference types, maps and so on
func Dumpstats() {
	var (
		e    Env
		s    Sym
		t    Type
		st   Stmt
		l    List
		intf interface{}
	)
	fmt.Fprintf(os.Stderr, "%s stats:\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "envsz:\t%d bytes\n", unsafe.Sizeof(e))
	fmt.Fprintf(os.Stderr, "symsz:\t%d bytes\n", unsafe.Sizeof(s))
	fmt.Fprintf(os.Stderr, "typesz:\t%d bytes\n", unsafe.Sizeof(t))
	fmt.Fprintf(os.Stderr, "stmtsz:\t%d bytes\n", unsafe.Sizeof(st))
	fmt.Fprintf(os.Stderr, "nenvs:\t%d\n", stats.nenvs)
	fmt.Fprintf(os.Stderr, "menvs:\t%d\t%d bytes\n",
		stats.menvs, uintptr(stats.menvs)*unsafe.Sizeof(e))
	fmt.Fprintf(os.Stderr, "nsyms:\t%d\t%d bytes\n",
		stats.nsyms, uintptr(stats.nsyms)*unsafe.Sizeof(s))
	fmt.Fprintf(os.Stderr, "nexpr:\t%d\n", stats.nexpr)
	fmt.Fprintf(os.Stderr, "nlists:\t%d\t%d bytes\n", stats.nlists,
		uintptr(stats.nlists)*(unsafe.Sizeof(l)+uintptr(stats.mlist)*uintptr(unsafe.Sizeof(intf))))
	fmt.Fprintf(os.Stderr, "mlist:\t%d\n", stats.mlist)
	fmt.Fprintf(os.Stderr, "nstmts:\t%d\n", stats.nstmts)
	fmt.Fprintf(os.Stderr, "nprogs:\t%d\n", stats.nprogs)
	fmt.Fprintf(os.Stderr, "ntypes:\t%d\n", stats.ntypes)
	fmt.Fprintf(os.Stderr, "nstrs:\t%d\n", stats.nstrs)
	fmt.Fprintf(os.Stderr, "\n")
}

func Goodbye(e interface{}) {
	errs := fmt.Sprint(e)
	if strings.HasPrefix(errs, "runtime error:") {
		errs = strings.Replace(errs, "runtime error:", "internal error:", 1)
		fmt.Fprintf(os.Stderr, "%s: last at %s:%d\n", os.Args[0], Scanner.fname, Scanner.lineno)
	}
	fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], errs)
	if dflag {
		fmt.Fprintf(os.Stderr, "%s", godebug.Stack())
	}
	os.Exit(1)
}

func NewScanner(b *bufio.Reader, name string, lineno int) (sc *Scan) {
	sc = new(Scan)
	sc.sval = sc.sarr[0:0]
	sc.bin = b
	sc.fname = name
	sc.lineno = 1
	return sc
}
