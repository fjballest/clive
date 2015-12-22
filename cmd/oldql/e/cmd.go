package e

import (
	"fmt"
	"runtime"
	"unicode/utf8"
)

// Addesses as used in Commands
type Addr struct {
	Kind        rune // # l / ? . $ + - , ;
	Rexp        []rune
	N           int
	Left, Right *Addr
}

// Edit command
type Cmd struct {
	Ch    rune   // command char (op code)
	Adr   *Addr  // address for the command
	Child []*Cmd // inner commands
	Txt   []rune // text for the command
	tab   *cTab
	Flag  rune
	N     int
	Rexp  []rune
	Xaddr *Addr
	dot   eSel // dot for this command
}

// command  flags
type CFlag int

const (
	// which kind of argument does a command take
	CFnone CFlag = 0
	CFtxt  CFlag = 1 << iota
	CFrexp
	CFaddr
	CFnum
	CFflag

	// which kind of tok terminates the command?
	CFline
	CFword

	// which address by default?
	CFdot
	CFall
	CFnl

	CFnowin  // does not need a current panel
	CFinsist // user can insist to force the command

	CFsetdot // command sets dot

	CFtokmask  = CFline | CFword
	CFaddrmask = CFdot | CFall | CFnl
)

// misc commands
const (
	cdCmd = utf8.MaxRune - 1
)

// function used to implement a sam command
type Exec func(*Sam, *Cmd)

type cTab struct {
	flag   CFlag //
	dflcmd rune  // default child command
	xfn    Exec
}

type lex struct {
	c     chan rune
	saved rune
	atnl  bool
}

/* can also use DefCmd('\n', nlCmd, CFdot, 0) to add commands */
/* commands commented out are disabled for e, for a reason */
var ctab map[rune]*cTab = map[rune]*cTab{
	'\n': {xfn: cmdnl, flag: CFnl | CFsetdot},
	'B':  {xfn: cmdB, flag: CFline | CFnowin},
	'D':  {xfn: cmdD, flag: CFline | CFnowin | CFinsist},
	'X':  {xfn: cmdXY, flag: CFrexp | CFnowin, dflcmd: 'f'},
	'Y':  {xfn: cmdXY, flag: CFrexp | CFnowin, dflcmd: 'f'},
	'a':  {xfn: cmda, flag: CFdot | CFtxt | CFsetdot},
	'b':  {xfn: cmdb, flag: CFline | CFnowin},
	'c':  {xfn: cmdc, flag: CFdot | CFtxt | CFsetdot},
	'd':  {xfn: cmdd, flag: CFdot | CFsetdot},
	'e':  {xfn: cmde, flag: CFword | CFsetdot},
	'f':  {xfn: cmdf, flag: CFword},
	'g':  {xfn: cmdgv, flag: CFrexp | CFdot, dflcmd: 'p'},
	'i':  {xfn: cmdi, flag: CFtxt | CFdot | CFsetdot},
	'k':  {xfn: cmdk, flag: CFdot},
	'm':  {xfn: cmdmt, flag: CFdot | CFaddr | CFsetdot},
	'n':  {xfn: cmdn, flag: CFnowin},
	'p':  {xfn: cmdp, flag: CFdot},
	'P':  {xfn: cmdp, flag: CFdot},
	'q':  {xfn: cmdq, flag: CFnowin | CFinsist},
	'r':  {xfn: cmdr, flag: CFdot | CFword | CFsetdot},
	's':  {xfn: cmds, flag: CFdot | CFrexp | CFflag | CFnum | CFsetdot},
	't':  {xfn: cmdmt, flag: CFdot | CFaddr | CFsetdot},
	'u':  {xfn: cmdu, flag: CFnum | CFsetdot},
	'v':  {xfn: cmdgv, flag: CFdot | CFrexp, dflcmd: 'p'},
	'w':  {xfn: cmdw, flag: CFall | CFtxt | CFword},
	'x':  {xfn: cmdxy, flag: CFdot | CFrexp | CFsetdot, dflcmd: 'p'},
	'y':  {xfn: cmdxy, flag: CFdot | CFrexp | CFsetdot, dflcmd: 'p'},

	/* experiment: make word consume the rest of the line if it's
	 * just blanks, and set commands with a single word to CFword
	 * so we can write multiple commands in a single line, like in
	 * ", x { .= .=# } "
	 * As of now only = is set CFword instead of CFline
	 */

	'=':   {xfn: cmdEq, flag: CFdot | CFword},
	'<':   {xfn: cmdsh, flag: CFdot | CFline | CFsetdot},
	'>':   {xfn: cmdsh, flag: CFdot | CFline},
	'|':   {xfn: cmdsh, flag: CFdot | CFline | CFsetdot},
	'!':   {xfn: cmdsh, flag: CFline},
	'{':   {xfn: cmdBlock, flag: CFdot | CFnowin},
	cdCmd: {xfn: cmdcd, flag: CFline},
}

func un(s string) {
	dprintf("out %s\n", s)
}

func trz(s string) string {
	dprintf("in %s\n", s)
	return s
}

// Define a new command (see cTab in the source for examples)
func DefCmd(c rune, fun Exec, flag CFlag, dflcmd rune) {
	ctab[c] = &cTab{flag, dflcmd, fun}
}

func (a *Addr) String() string {
	if a == nil || a.Kind == 0 {
		return "<>"
	}
	r := ""
	if a.Rexp != nil {
		r = fmt.Sprintf(" '%s'", string(a.Rexp))
	}
	n := ""
	if a.N != 0 || a.Kind == 'l' || a.Kind == '#' {
		n = fmt.Sprintf(" %d", a.N)
	}
	if a.Left != nil || a.Right != nil {
		return fmt.Sprintf("<%s <%c%s%s> %s>",
			a.Left, a.Kind, n, r, a.Right)
	}
	return fmt.Sprintf("<%c%s%s>", a.Kind, n, r)
}

var cstab = 0 // hack

func (c *Cmd) String() string {
	if c == nil {
		return "nil"
	}
	t := ""
	for i := 0; i < cstab; i++ {
		t += "  "
	}
	ch := fmt.Sprintf("%c", c.Ch)
	if c.Ch == '\n' {
		ch = `\n`
	}
	s := fmt.Sprintf("%s%s %s", t, c.Adr, ch)
	if c.Txt != nil {
		s += fmt.Sprintf(" t='%s'", string(c.Txt))
	}
	if c.Rexp != nil {
		s += fmt.Sprintf(" /%s/", string(c.Rexp))
	}
	if c.Xaddr != nil {
		s += fmt.Sprintf(" x=%s", c.Xaddr)
	}
	if c.Flag != 0 {
		s += fmt.Sprintf(" f=%c", c.Flag)
	}
	s += "\n"
	if c.Child == nil {
		return s
	}
	for _, cc := range c.Child {
		cstab++
		s += fmt.Sprintf("%s", cc)
		cstab--
	}
	return s
}

func (l *lex) getc() rune {
	x := l.saved
	if x != 0 {
		l.saved = 0
	} else {
		x = <-l.c
	}
	// dprintf("->'%c'\n", x)
	l.atnl = x == '\n'
	return x
}

func (l *lex) dropln() {
	for c := l.peek(); !l.atnl && c != 0; c = l.getc() {
	}
}

func (l *lex) ungetc(r rune) {
	l.saved = r
}

func (l *lex) peek() rune {
	if l.saved != 0 {
		return l.saved
	}
	l.saved = <-l.c
	return l.saved
}

func (l *lex) skipBlanks() {
	for c := l.peek(); c == ' ' || c == '\t'; c = l.peek() {
		l.getc()
	}
}

func (l *lex) nl() {
	l.skipBlanks()
	if l.peek() == '\n' {
		l.getc()
	}
}

func (l *lex) parseCmd(lvl int) (cmd *Cmd, err error) {
	//defer un(trz("parseCmd"))
	c := &Cmd{}
	defer func() {
		if s := recover(); s != nil {
			if x, ok := s.(runtime.Error); ok {
				panic(x)
			}
			l.dropln()
			err = fmt.Errorf("%s", s)
		}
	}()
	defer dprintf("parse cmd: %s\n", c)
	l.skipBlanks()
	c.Adr = l.parseCompAddr()
	l.skipBlanks()
	switch c.Ch = l.getc(); c.Ch {
	case 0:
		return nil, nil
	case '{':
		for {
			l.nl()
			nc, err := l.parseCmd(lvl + 1)
			if err != nil {
				return nil, err
			}
			if nc == nil {
				break
			}
			if nc.Ch == '}' {
				c.tab = ctab[c.Ch]
				return c, nil
			}
			c.Child = append(c.Child, nc)
		}
		panic("missing '}'")
	case '}':
		if lvl == 0 {
			panic("extra '}'")
		}
		l.nl()
		return &Cmd{Ch: '}'}, nil
	case 'c':
		if l.peek() == 'd' {
			l.getc()
			c.Ch = cdCmd
		}
	}
	ct, ok := ctab[c.Ch]
	if !ok {
		panic(fmt.Sprintf("unknown command '%c'", c.Ch))
	}
	if c.Ch != '\n' {
		l.parseArgs(c, ct, lvl)
	}
	c.tab = ct
	return c, nil
}

func (l *lex) parseArgs(c *Cmd, ct *cTab, lvl int) {
	//defer un(trz("parseArgs"))
	if (ct.flag&CFaddrmask) == 0 && c.Adr != nil {
		panic(fmt.Sprintf("extra address given to '%c'", c.Ch))
	}
	if ct.flag&CFnum != 0 {
		c.N = l.parseNum()
	}
	if ct.flag&CFrexp != 0 {
		ch := l.peek()
		if (c.Ch != 'x' && c.Ch != 'X') ||
			(ch != ' ' && ch != '\t' && ch != '\n') {
			l.skipBlanks()
			ch = l.peek()
			if ch == '\n' || ch == 0 {
				panic(fmt.Sprintf("missing address for '%c' at '%c'", c.Ch, ch))
			}
			var sep rune
			c.Rexp, sep = l.parseRexp()
			if ct.flag&CFflag != 0 {
				c.Txt = l.parseText(sep, true)
				if l.peek() == sep {
					l.getc()
					if l.peek() == 'g' {
						l.getc()
						c.Flag = 'g'
					}
				}
			}
		}
	}
	if ct.flag&CFaddr != 0 {
		if c.Xaddr = l.parseAddr(); c.Xaddr == nil {
			panic(fmt.Sprintf("bad address for '%c'", c.Ch))
		}
	}
	if ct.dflcmd != 0 {
		l.skipBlanks()
		var nc *Cmd
		if l.peek() == '\n' {
			l.getc()
			nc = &Cmd{Ch: ct.dflcmd}
		} else {
			var err error
			if nc, err = l.parseCmd(lvl); err != nil {
				panic(fmt.Sprintf("in '%c': %s", c.Ch, err))
			}
		}
		nc.tab = ctab[nc.Ch]
		c.Child = append(c.Child, nc)
	} else if ct.flag&CFtxt != 0 {
		c.Txt = l.getText()
	} else if ct.flag&CFtokmask != 0 {
		c.Txt = l.getTok(ct.flag&CFword != 0)
	} else {
		l.nl()
	}
}

func (l *lex) getTok(w bool) []rune {
	//defer un(trz("getTok"))
	s := []rune{}
	var c rune
	for c = l.peek(); c == ' ' || c == '\t'; c = l.peek() {
		s = append(s, l.getc())
	}
	for c = l.peek(); c != 0 && c != '\n' &&
		(!w || (c != ' ' && c != '\t')); c = l.peek() {
		if w && c == '}' {
			break
		}
		s = append(s, l.getc())
	}

	/* experiment: make word consume the rest of the line if it's
	 * just blanks, and set commands with a single word to CFword
	 * so we can write multiple commands in a single line, like in
	 * ", x { .= .=# } "
	 */
	//if c != '\n' {
	l.nl()
	//}
	return s
}

func (l *lex) getText() []rune {
	//defer un(trz("getText"))
	s := []rune{}
	l.skipBlanks()
	if l.peek() != '\n' {
		sep := l.getc()
		txt := l.parseText(sep, false)
		if l.peek() == sep {
			l.getc()
		}
		l.nl()
		return txt
	}
	l.nl()
	atnl := true
	for c := l.getc(); c != 0; c = l.getc() {
		if atnl && c == '.' {
			if l.peek() == '\n' {
				l.getc()
				break
			}
		}
		atnl = c == '\n'
		s = append(s, c)
	}
	return s
}

func (l *lex) parseText(sep rune, keepbs bool) []rune {
	//defer un(trz("parseText"))
	s := []rune{}
	for c := l.peek(); c != 0 && c != sep && c != '\n'; c = l.peek() {
		l.getc()
		if c == '\\' {
			c = l.peek()
			switch {
			case c == 0:
				panic("bad text argument")
			case c == 'n':
				c = '\n'
			case c == 't':
				c = '\t'
			case c == sep:
				s = append(s, c)
				return s
			default:
				if c != '\\' || keepbs {
					s = append(s, '\\')
				}
			}
			l.getc()

		}
		s = append(s, c)
	}
	return s
}

func (l *lex) parseCompAddr() *Addr {
	//defer un(trz("parseCompAddr"))
	a := &Addr{}
	defer dprintf("compaddr = %s\n", a)
	a.Left = l.parseAddr()
	l.skipBlanks()
	c := l.peek()
	if c != ',' && c != ';' {
		return a.Left
	}
	a.Kind = l.getc()
	if a.Right = l.parseCompAddr(); a.Right == nil {
		return a
	}
	if (a.Right.Kind == ',' || a.Right.Kind == ';') && a.Right.Left == nil {
		panic("bad address")
	}
	return a
}

func (l *lex) parseNum() int {
	//defer un(trz("parseNum"))
	l.skipBlanks()
	c := l.peek()
	s := 1
	n := 0
	if c == '-' {
		s = -1
		l.getc()
		c = l.peek()
	}
	for c >= '0' && c <= '9' {
		n = n*10 + (int(c) - '0')
		l.getc()
		c = l.peek()
	}
	return n * s
}

func (l *lex) parseRexp() ([]rune, rune) {
	//defer un(trz("parseRexp"))
	re := []rune{}
	sep := l.getc()
	for {
		c := l.getc()
		if c == '\n' {
			l.ungetc(c)
			break
		}
		if c == sep || c == 0 {
			break
		}
		re = append(re, c)
	}
	return re, sep
}

func (l *lex) parseAddr() *Addr {
	//defer un(trz("parseAddr"))
	a := &Addr{}
	defer dprintf("addr = %s\n", a)
	l.skipBlanks()
	switch c := l.peek(); {
	case c == '#':
		a.Kind = l.getc()
		a.N = l.parseNum()
	case c >= '0' && c <= '9':
		a.N = l.parseNum()
		a.Kind = 'l'
	case c == '/' || c == '?' || c == '"':
		a.Kind = l.peek()
		a.Rexp, _ = l.parseRexp()
	case c == '.' || c == '$' || c == '+' || c == '-' || c == '\'':
		a.Kind = l.getc()
	default:
		return nil
	}
	if a.Left = l.parseAddr(); a.Left == nil {
		return a
	}
	switch a.Left.Kind {
	case '.', '$', '\'':
		if a.Kind != '"' {
			panic("bad address")
		}
	case '"':
		panic("bad address")
	case 'l', '#':
		if a.Kind != '"' && a.Kind != '+' && a.Kind != '-' {
			na := &Addr{Kind: '+', Left: a.Left}
			a.Left = na
		}
	case '/', '?':
		if a.Kind != '"' && a.Kind != '+' && a.Kind != '-' {
			na := &Addr{Kind: '+', Left: a.Left}
			a.Left = na
		}
	case '+', '-':
	default:
		panic("parseaddr")
	}
	return a
}
