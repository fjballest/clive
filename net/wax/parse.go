package wax

import (
	"fmt"
)

type cmd struct {
	op   tok
	txt  string
	e    *el
	cmds []*cmd
}

/*
	cmds	::= cmd cmds | cmd
*/
func (p *Part) parse(s string) (*cmd, error) {
	l := newLex(s)
	l.debug = Debug
	p.l = l
	_, c, err := p.parseBlock(tEOF)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (p *Part) parseBlock(sep ...tok) (tok, *cmd, error) {
	l := p.l
	c := &cmd{op: tDo, cmds: []*cmd{}}
	for {
		t := l.next()
		for _, e := range sep {
			if t == e || t == tEOF || t == tErr {
				dprintf("%s ", l.val())
				return e, c, nil
			}
		}
		l.undo()
		cc, err := p.parseCmd()
		if err != nil {
			return tNone, nil, err
		}
		c.cmds = append(c.cmds, cc)
	}
}

/*
	cmd	::= TEXT			show this raw text
		| elem				show the named item
		| SHOW elem			show the named item ro
		| EDIT elem			show the named item rw
		| for
		| if
*/
func (p *Part) parseCmd() (*cmd, error) {
	l := p.l
	switch t := l.next(); t {
	case tErr:
		return nil, l.err
	case tText:
		dprintf("%s ", l.val())
		return &cmd{op: tText, txt: l.val()}, nil
	case tShow, tEdit:
		op := t
		v := l.val()
		dprintf("%s ", v)
		e, err := p.parseElem()
		if err != nil {
			return nil, err
		}
		return &cmd{op: op, e: e}, nil
	case tId:
		l.undo()
		e, err := p.parseElem()
		if err != nil {
			return nil, err
		}
		return &cmd{op: tShow, e: e}, err
	case tFor:
		l.undo()
		return p.parseFor()
	case tIf:
		l.undo()
		return p.parseIf()
	default:
		return nil, fmt.Errorf("unexpected %s", t)
	}
}

/*
	for	::= FOR name IN elem DO cmds END
*/
func (p *Part) parseFor() (*cmd, error) {
	l := p.l
	if l.next() != tFor {
		panic("wax for parse bug")
	}
	dprintf("%s ", l.val())
	if t := l.next(); t != tId {
		return nil, fmt.Errorf("id expected at %s in for", t)
	}
	id := l.val()
	dprintf("%s ", id)
	if t := l.next(); t != tIn {
		return nil, fmt.Errorf("in expected at %s in for", t)
	}
	dprintf("%s ", l.val())
	e, err := p.parseElem()
	if err != nil {
		return nil, err
	}
	if t := l.next(); t != tDo {
		return nil, fmt.Errorf("do expected at %s in for", t)
	}
	dprintf("%s ", l.val())

	c := &cmd{op: tFor, txt: id, e: e}
	t, bdy, err := p.parseBlock(tEnd)
	if err != nil {
		return nil, err
	}
	if t != tEnd {
		return nil, fmt.Errorf("end expected")
	}
	c.cmds = bdy.cmds
	return c, nil
}

/*
	if	::= IF elem DO cmds else
	else	::= END | ELSE cmds END
*/
func (p *Part) parseIf() (*cmd, error) {
	l := p.l
	if l.next() != tIf {
		panic("wax if parse bug")
	}
	dprintf("%s ", l.val())
	e, err := p.parseElem()
	if err != nil {
		return nil, err
	}
	if t := l.next(); t != tDo {
		return nil, fmt.Errorf("do expected at %s in if", t)
	}
	dprintf("%s ", l.val())
	c := &cmd{op: tIf, e: e}
	t, bdy, err := p.parseBlock(tEnd, tElse)
	if err != nil {
		return nil, err
	}
	c.cmds = []*cmd{bdy}
	if t == tEnd {
		return c, nil
	}
	if t != tElse {
		return nil, fmt.Errorf("end or else expected")
	}
	t, bdy, err = p.parseBlock(tEnd)
	if err != nil {
		return nil, err
	}
	if t != tEnd {
		return nil, fmt.Errorf("end expected")
	}
	c.cmds = append(c.cmds, bdy)
	return c, nil
}

/*
	elem ::= ID sels
	sels ::= . ID sels | [ ID ] sels | <empty>
*/
func (p *Part) parseElem() (*el, error) {
	l := p.l
	t := l.next()
	if t != tId {
		return nil, fmt.Errorf("id expected at %s", t)
	}
	e := &el{op: tId, name: l.val()}
	last := e
	for {
		switch t := l.next(); t {
		case tDot:
			t = l.next()
			if t != tId {
				return nil, fmt.Errorf("id expected at %s", t)
			}
			last.next = &el{op: tDot, name: l.val()}
			last = last.next
		case tLbra:
			t = l.next()
			if t != tId {
				return nil, fmt.Errorf("id expected at %s", t)
			}
			id := l.val()
			t = l.next()
			if t != tRbra {
				return nil, fmt.Errorf("] expected at %s", t)
			}
			last.next = &el{op: tLbra, name: id}
			last = last.next
		default:
			l.undo()
			dprintf("%s ", e)
			return e, nil
		}
	}
}
