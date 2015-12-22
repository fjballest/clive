package pred

import (
	"clive/dbg"
	"errors"
	"fmt"
	"io"
	"strconv"
)

/*
	keep Ops and toks exactly the same for the ops used as tokens.
*/

type tok int

const (
	tNone  tok = 0
	tLe    tok = '≤'
	tEq    tok = '≡'
	tNeq   tok = 'd'
	tGe    tok = '≥'
	tLt    tok = '<'
	tGt    tok = '>'
	tEqs   tok = '='
	tNeqs  tok = '≠'
	tMatch tok = '~'
	tRexp  tok = '≈'
	tNot   tok = '!'
	tOr    tok = ':'
	tAnd   tok = ','
	tName  tok = 'n'
	tLpar  tok = '('
	tRpar  tok = ')'
	tPrune tok = 'p'
	tTrue  tok = 't'
	tFalse tok = 'f'
)

type tClass int

const (
	cBin tClass = iota
	cUn
	cOp
	cName
	cPar
	cAtom
)

// Don't use strings/utf8 for the next couple of functions; this is faster.

func ispunct(r rune) bool {
	switch r {
	case '!', '&', '|', '~', '=', '(', ')', '>', '<', '\'', ',', ':', '≈', '≡', '≠', '≥', '≤':
		return true
	}
	return false
}

func isspace(t rune) bool {
	return t == ' ' || t == '\t' || t == '\n'
}

func (t tok) class() tClass {
	switch t {
	case tNot:
		return cUn
	case tAnd, tOr:
		return cBin
	case tLe, tEq, tNeq, tGe, tLt, tGt, tEqs, tNeqs, tMatch, tRexp:
		return cOp
	case tLpar, tRpar:
		return cPar
	case tPrune, tTrue, tFalse:
		return cAtom
	default:
		return cName
	}
}

func (o op) prec() int {
	switch o {
	case oOr:
		return 1
	case oAnd:
		return 2
	case oNot:
		return 3
	default:
		return 4
	}
}

func (o op) String() string {
	switch o {
	default:
		return "???"
	case oLt:
		return "<"
	case oLe:
		return "<="
	case oNeq:
		return "!=="
	case oEq:
		return "=="
	case oGe:
		return ">="
	case oGt:
		return ">"
	case oNeqs:
		return "!="
	case oEqs:
		return "="
	case oMatch:
		return "~"
	case oRexp:
		return "~~"
	case oOr:
		return "|"
	case oAnd:
		return "&"
	case oNot:
		return "!"
	case oPrune:
		return "p"
	case oTrue:
		return "t"
	case oFalse:
		return "f"
	}
}

// return a debug string for p
func (p *Pred) DebugString() string {
	if p == nil {
		return ""
	}
	switch tok(p.op).class() {
	case cUn:
		if len(p.args) == 0 {
			return p.op.String() + "?"
		}
		return p.op.String() + "(" + p.args[0].DebugString() + ")"
	case cBin:
		s := p.op.String() + "("
		for i := 0; i < len(p.args); i++ {
			s += p.args[i].DebugString() + ","
		}
		return s + ")"
	case cOp:
		return p.name + " " + p.op.String() + " " + p.value
	case cAtom:
		switch p.op {
		case oPrune:
			return "prune"
		case oTrue:
			return "true"
		case oFalse:
			return "false"
		}
		return p.op.String() + "?"
	default:
		return p.op.String() + "?"
	}

}

// return a string that can be parsed back to this predicate.
func (p *Pred) String() string {
	if p == nil {
		return ""
	}
	switch tok(p.op).class() {
	case cUn:
		if len(p.args) == 0 {
			return p.op.String() + "?"
		}
		if p.args[0].op.prec() < p.op.prec() {
			return p.op.String() + "(" + p.args[0].String() + ")"
		}
		return p.op.String() + p.args[0].String()
	case cBin:
		s := ""
		for i := 0; i < len(p.args); i++ {
			if i > 0 {
				s += " " + p.op.String() + " "
			}
			if p.args[i].op.prec() < p.op.prec() {
				s += "(" + p.args[i].String() + ")"
			} else {
				s += p.args[i].String()
			}
		}
		return s
	case cOp:
		return p.name + " " + p.op.String() + " " + p.value
	case cAtom:
		switch p.op {
		case oPrune:
			return "prune"
		case oTrue:
			return "true"
		case oFalse:
			return "false"
		}
		return p.op.String() + "?"
	default:
		return p.op.String() + "?"
	}

}

func (t tok) String() string {
	return fmt.Sprintf("%c", t)
}

type lex struct {
	t     []rune
	debug bool
}

func newLex(s string) *lex {
	return &lex{[]rune(s), false}
}

func (l *lex) peek() (tok, string, error) {
	t := l.t
	x, s, e := l.next()
	l.t = t
	return x, s, e
}

func (l *lex) scan() (tok, string, error) {
	t, s, e := l.next()
	if l.debug {
		fmt.Printf("tok %s\n", s)
	}
	return t, s, e
}

func (l *lex) next() (tok, string, error) {
	for len(l.t) > 0 && isspace(l.t[0]) {
		l.t = l.t[1:]
	}
	if len(l.t) == 0 {
		return tNone, "", io.EOF
	}
	switch c := l.t[0]; c {
	case '~':
		if len(l.t) > 1 && l.t[1] == '~' {
			l.t = l.t[2:]
			return tRexp, "~~", nil
		}
		l.t = l.t[1:]
		return tMatch, "~", nil
	case '(', ')', '|', '&', ',', ':', '≈', '≡', '≠':
		if c == '&' {
			c = ','
		} else if c == '|' {
			c = ':'
		}
		t := string(l.t[:1])
		l.t = l.t[1:]
		return tok(c), t, nil
	case '>', '<', '!', '=':
		if len(l.t) > 1 && l.t[1] == '=' {
			l.t = l.t[2:]
			if c == '!' {
				if len(l.t) > 0 && l.t[0] == '=' {
					l.t = l.t[1:]
					return tNeq, "!==", nil
				}
				return tNeqs, "!=", nil
			}
			if c == '=' {
				return tEq, "==", nil
			}
			if c == '<' {
				return tLe, "<=", nil
			}
			return tGe, ">=", nil
		}
		l.t = l.t[1:]
		return tok(c), fmt.Sprintf("%c", c), nil
	case '"':
		if len(l.t) == 1 {
			return tNone, "", errors.New(`" expected`)
		}
		for i := 1; i < len(l.t); i++ {
			if l.t[i] == '"' {
				v := string(l.t[1:i])
				l.t = l.t[i+1:]
				return tName, v, nil
			}
		}
		return tName, "", errors.New("' expected")
	case '\'':
		if len(l.t) == 1 {
			return tNone, "", errors.New("' expected")
		}
		for i := 1; i < len(l.t); i++ {
			if l.t[i] == '\'' {
				v := string(l.t[1:i])
				l.t = l.t[i+1:]
				return tName, v, nil
			}
		}
		return tName, "", errors.New("' expected")
	default:
		i := 1
		for ; i < len(l.t); i++ {
			if ispunct(l.t[i]) {
				break
			}
			if isspace(l.t[i]) {
				break
			}
		}
		t := string(l.t[:i])
		l.t = l.t[i:]
		if t == "prune" {
			return tPrune, t, nil
		}
		if t == "true" || t == "t" {
			return tTrue, t, nil
		}
		if t == "false" || t == "f" {
			return tFalse, t, nil
		}
		return tName, t, nil
	}
}

func (l *lex) parse() (*Pred, error) {
	p, err := l.parseOrs()
	if err != nil {
		return nil, err
	}
	if _, _, err := l.scan(); err == nil {
		return nil, errors.New("syntax error")
	}
	return p, nil
}

func (l *lex) parseOrs() (*Pred, error) {
	if l.debug {
		defer dbg.Trace(dbg.Call("ors"))
	}
	args := []*Pred{}
	a1, err := l.parseAnds()
	if err != nil {
		return nil, err
	}
	args = append(args, a1)
	for {
		op, _, err := l.peek()
		if err != nil || op != tOr {
			if err == io.EOF || op == tRpar {
				err = nil
			}
			if len(args) == 1 {
				return args[0], err
			}
			return &Pred{op: oOr, args: args}, err
		}
		l.scan()
		a, err := l.parseAnds()
		if err != nil {
			return nil, err
		}
		args = append(args, a)

	}
}

func (l *lex) parseAnds() (*Pred, error) {
	if l.debug {
		defer dbg.Trace(dbg.Call("ands"))
	}
	args := []*Pred{}
	a1, err := l.parsePrim()
	if err != nil {
		return nil, err
	}
	args = append(args, a1)
	for {
		op, _, err := l.peek()
		if err != nil || op != tAnd {
			if err == io.EOF || op == tRpar || op == tOr {
				err = nil
			}
			if len(args) == 1 {
				return args[0], err
			}
			return &Pred{op: oAnd, args: args}, err
		}
		l.scan()
		a, err := l.parsePrim()
		if err != nil {
			return nil, err
		}
		args = append(args, a)
	}
}

/*
	prim ::= prune | t | true | f | false | name op name | unop prim | maxmin name | n | ( ors )
	but handle ~ str as meaning "name~str"
*/
func (l *lex) parsePrim() (*Pred, error) {
	if l.debug {
		defer dbg.Trace(dbg.Call("prim"))
	}
	t, _, err := l.peek()
	if err != nil {
		return nil, err
	}
	switch {
	case t == tPrune, t == tTrue, t == tFalse:
		l.scan()
		return &Pred{op: op(t)}, nil
	case t == tMatch || t == tEqs || t == tRexp:
		// unary usage assumes path op ...
		l.scan()
		t2, v2, err := l.scan()
		if err != nil || t2.class() != cName {
			return nil, errors.New("name expected")
		}
		return &Pred{op: op(t), name: "path", value: v2}, nil
	case t == tName:
		_, v1, _ := l.scan()
		if v1 == "d" || v1 == "-" || v1 == "c" {
			// d/-/c -> type = d/-/c
			return &Pred{op: oEqs, name: "type", value: v1}, nil
		}
		_, err := strconv.Atoi(v1)
		if err == nil {
			// n -> depth<=n
			return &Pred{op: oLe, name: "depth", value: v1}, nil
		}
		x, _, err := l.scan()
		if err != nil || x.class() != cOp {
			return nil, errors.New("op expected")
		}
		t2, v2, err := l.scan()
		if err != nil || t2.class() != cName {
			return nil, errors.New("name expected")
		}
		return &Pred{op: op(x), name: v1, value: v2}, nil
	case t.class() == cUn:
		l.scan()
		arg, err := l.parsePrim()
		if err != nil {
			return nil, err
		}
		return &Pred{op: op(t), args: []*Pred{arg}}, nil
	case t == tLpar:
		l.scan()
		arg, err := l.parseOrs()
		if err != nil {
			return nil, err
		}
		r, _, err := l.scan()
		if err != nil || r != tRpar {
			return nil, errors.New("')' expected")
		}
		return arg, nil
	}
	return nil, errors.New("not a primary expression")
}
