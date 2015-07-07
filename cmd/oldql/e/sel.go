package e

import (
	"clive/sre"
	"fmt"
)

func (sel eSel) String() string {
	if sel.F != nil {
		return fmt.Sprintf("'%s':%d,%d", sel.F.path, sel.P0, sel.P1)
	}
	return fmt.Sprintf("-:%d,%d", sel.P0, sel.P1)
}

func (sel eSel) setFileSel() {
	if sel.F != nil {
		sel.F.SetSel(sel.P0, sel.P1)
	}
}

func (sel eSel) chrAddr(n int, d eDir) eSel {
	switch d {
	case eAbs:
		sel.P0 = n
		sel.P1 = n
	case eFwd:
		sel.P0 += n
		sel.P1 += n
	case eBck:
		sel.P0 -= n
		sel.P1 -= n
	}
	if sel.P0<0 || sel.P1>sel.F.Len() {
		panic("address out of range")
	}
	return sel
}

func (sel eSel) Get() []rune {
	rs := []rune{}
	for r := range sel.F.Get(sel.P0, sel.P1-sel.P0) {
		rs = append(rs, r...)
	}
	return rs
}

func (sel eSel) lnAddr(n int, d eDir) eSel {
	if d == eBck {
		return sel.backLine(n)
	}
	if d == eAbs {
		sel.P0 = 0
		sel.P1 = 0
	} else {
		/* go to end of last line in dot, right after \n */
		for sel.P1>0 && sel.P1<sel.F.Len() &&
			sel.F.Getc(sel.P1-1)!='\n' {
			sel.P1++
		}
		sel.P0 = sel.P1
	}
	/* move after the \n, n lines fwd (current counts as 1)  */
	for i := 1; i<n && sel.P1<sel.F.Len(); sel.P1++ {
		if sel.F.Getc(sel.P1) == '\n' {
			sel.P0 = sel.P1 + 1
			i++
		}
	}
	/* if address is 0 or +0 we don't must select the line */
	if n == 0 {
		return sel
	}
	/* now advance P1 to the end of that line, unless n was 0 */
	for sel.P1<sel.F.Len() && sel.F.Getc(sel.P1)!='\n' {
		sel.P1++
	}
	/* and include the \n */
	if sel.P1 < sel.F.Len() {
		sel.P1++
	}
	return sel
}

func (sel eSel) backLine(n int) eSel {
	/* go to start of first line in dot, right after \n */
	for sel.P0>0 && sel.F.Getc(sel.P0-1)!='\n' {
		sel.P0--
	}
	sel.P1 = sel.P0
	/* go before the \n */
	if sel.P0 > 0 {
		sel.P0--
	}
	/* now go back n lines, including this one, right before the \n */
	for i := 0; i<n && sel.P0>0; sel.P0-- {
		if sel.F.Getc(sel.P0-1) == '\n' {
			i++
			if i < n {
				sel.P1 = sel.P0
			}
		}
	}
	/* and skip the \n right before the first line */
	if sel.P0 > 0 {
		sel.P0++
	}
	/* -0 must select just the start, not the line */
	if n == 0 {
		sel.P1 = sel.P0
	}
	return sel
}

func (s eSel) matchFwd(re []rune) []sre.Range {
	prg, err := sre.Compile(re, sre.Fwd)
	if err != nil {
		panic(err)
	}
	rg := prg.Exec(s.F, s.P1, s.F.Len())
	if len(rg)==0 && s.P1>=s.F.Len() {
		rg = prg.Exec(s.F, s.P0, s.F.Len())
	}
	return rg
}

func (s eSel) matchBck(re []rune) []sre.Range {
	prg, err := sre.Compile(re, sre.Bck)
	if err != nil {
		panic(err)
	}
	rg := prg.Exec(s.F, s.P0, s.F.Len())
	if len(rg)==0 && s.P0==0 {
		rg = prg.Exec(s.F, s.F.Len(), s.F.Len())
	}
	return rg
}

func (s eSel) nlines() int {
	n := 0
	for ; s.P0 < s.P1; s.P0++ {
		if s.F.Getc(s.P0) == '\n' {
			n++
		}
	}
	return n
}

func (sel eSel) matches(rexp []rune, neg bool) ([][]sre.Range, error) {
	re, err := sre.Compile(rexp, sre.Fwd)
	if err != nil {
		return nil, err
	}
	sels := make([][]sre.Range, 0)
	for sel.P0 < sel.P1 {
		rgs := re.Exec(sel.F, sel.P0, sel.P1)
		dprintf("match sel %s rgs %v\n", sel, rgs)
		if len(rgs) == 0 {
			if neg {
				s := sre.Range{sel.P0, sel.P1}
				sels = append(sels, []sre.Range{s})
			}
			break
		}
		if neg {
			s := sre.Range{sel.P0, rgs[0].P0}
			sels = append(sels, []sre.Range{s})
		} else {
			sels = append(sels, rgs)
		}
		sel.P0 = rgs[0].P1
		if rgs[0].P0 == rgs[0].P1 {
			sel.P0++
		}
	}
	return sels, nil
}

func (sel eSel) backRefs(txt []rune, rg []sre.Range) []rune {
	if len(rg) == 0 {
		return txt
	}
	sel.P0, sel.P1 = rg[0].P0, rg[0].P1
	repl := make([]rune, 0, len(txt))
	for i := 0; i < len(txt); i++ {
		if txt[i]=='\\' && i<len(txt)-1 && txt[i+1]=='\\' {
			repl = append(repl, txt[i:i+2]...)
			i++
		} else if txt[i]=='\\' && i<len(txt)-1 &&
			txt[i+1]>='0' && txt[i+1]<='9' {
			i++
			n := int(txt[i] - '0')
			if n>=0 && n<len(rg) {
				msel := sel
				msel.P0, msel.P1 = rg[n].P0, rg[n].P1
				match := msel.Get()
				repl = append(repl, match...)
			}
		} else if txt[i] == '&' {
			match := sel.Get()
			repl = append(repl, match...)
		} else {
			repl = append(repl, txt[i])
		}
	}
	return repl
}
