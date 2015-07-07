package wax

import (
	"fmt"
	"io"
	"unicode"
)

type tok int

const (
	tNone tok = iota
	tId
	tDo
	tDot
	tEOF
	tEdit
	tElse
	tEnd
	tErr
	tFor
	tIf
	tIn
	tLbra
	tRbra
	tShow
	tText

	Chr = '$' // start/end of command character
)

type tfn func() (tok, tfn)

type lex  {
	txt          []rune
	p0, p1       int
	err          error
	tfn          tfn
	lastt        tok
	saved, debug bool
}

func (t tok) String() string {
	switch t {
	case tId:
		return "id"
	case tDo:
		return "do"
	case tDot:
		return "dot"
	case tEOF:
		return "eof"
	case tEdit:
		return "edit"
	case tElse:
		return "else"
	case tEnd:
		return "end"
	case tErr:
		return "err"
	case tFor:
		return "for"
	case tIf:
		return "if"
	case tIn:
		return "in"
	case tLbra:
		return "lbra"
	case tRbra:
		return "rbra"
	case tShow:
		return "show"
	case tText:
		return "text"
	}
	return "none"
}

func newLex(s string) *lex {
	l := &lex{
		txt: []rune(s),
	}
	l.tfn = l.outside
	return l
}

func (l *lex) val() string {
	return string(l.txt[l.p0:l.p1])
}

func (l *lex) next() tok {
	if l.saved {
		l.saved = false
		return l.lastt
	}
	var t tok
	l.p0 = l.p1
	t, l.tfn = l.tfn()
	l.lastt = t
	if t == tErr {
		l.p0 = l.p1
	}
	if l.debug {
		fmt.Printf("%s `%s`\n", t, l.val())
	}
	return t
}

func (l *lex) undo() {
	l.saved = true
}

func (l *lex) outside() (tok, tfn) {
	if l.p1 >= len(l.txt) {
		l.err = io.EOF
		return tEOF, l.done
	}
	for l.p1 < len(l.txt) {
		if l.txt[l.p1] == Chr {
			if l.p1 == len(l.txt)-1 {
				l.err = fmt.Errorf("no closing '%c'", Chr)
				return tErr, l.failed
			}
			if l.txt[l.p1+1] == Chr { // escape
				copy(l.txt[l.p1+1:], l.txt[l.p1+2:])
				l.txt = l.txt[:len(l.txt)-1]
				l.p1++
				continue
			}
			if l.p0 == l.p1 { // empty prefix text
				return l.goingIn() // call next state
			}
			return tText, l.goingIn
		}
		l.p1++
	}
	return tText, l.done
}

func (l *lex) goingIn() (tok, tfn) {
	l.p1++
	l.p0++
	return l.inside()
}

var kwords = map[string]tok{
	"do":   tDo,
	"edit": tEdit,
	"else": tElse,
	"end":  tEnd,
	"for":  tFor,
	"if":   tIf,
	"in":   tIn,
	"show": tShow,
}

func (l *lex) inside() (tok, tfn) {
	if l.p1 >= len(l.txt) {
		l.err = fmt.Errorf("no closing '%c'", Chr)
		return tErr, l.failed
	}
	for l.p1<len(l.txt) && unicode.IsSpace(l.txt[l.p1]) {
		l.p1++
	}
	l.p0 = l.p1
	for l.p1<len(l.txt) && l.txt[l.p1]!=Chr {
		r := l.txt[l.p1]
		if l.p1 == l.p0 {
			switch r {
			case '.':
				l.p1++
				return tDot, l.inside
			case '[':
				l.p1++
				return tLbra, l.inside
			case ']':
				l.p1++
				return tRbra, l.inside
			}
		}
		if r=='.' || r=='[' || r==']' || unicode.IsSpace(r) {
			if t, ok := kwords[l.val()]; ok {
				return t, l.inside
			}
			return tId, l.inside
		}
		l.p1++
	}
	if l.p1 == len(l.txt) {
		l.err = fmt.Errorf("no closing '%c'", Chr)
		return tErr, l.failed
	}
	if t, ok := kwords[l.val()]; ok {
		return t, l.goingOut
	}
	if l.p0 == l.p1 {
		return l.goingOut()
	}
	return tId, l.goingOut
}

func (l *lex) goingOut() (tok, tfn) {
	l.p1++
	l.p0++
	return l.outside()
}

func (l *lex) done() (tok, tfn) {
	return tEOF, l.done
}

func (l *lex) failed() (tok, tfn) {
	return tErr, l.failed
}
