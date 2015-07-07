package sre

import (
	"fmt"
	"unicode"
)

/*
	clear r to return it to the user (if there's no match, it's nil).
*/
func retsel(r []Range) []Range {
	if len(r)==0 || r[0].P0<0 {
		return nil
	}
	return r
}

/*
	lists of states reached in the FNA and their selections
*/
type states  {
	lst []state
}

/*
	NFA state reached (executed by the virtual machine)
*/
type state  {
	i   pinst   // instruction we start with
	sel []Range // selection
}

/*
	Add a new state (ninst, sel) to the list of states
	(unless it's already there, in which case it's used)
*/
func (ss *states) add(ninst pinst, sel []Range) {
	for si, s := range ss.lst {
		if s.i == ninst {
			if sel[0].P0 < s.sel[0].P0 {
				// if ss is not pending when add is called
				// then this is a bug, else it optimizes.
				copy(ss.lst[si].sel, sel)
			}
			return
		}
	}
	s := state{ninst, make([]Range, len(sel))}
	copy(s.sel, sel)
	ss.lst = append(ss.lst, s)
}

func (ss *states) clear() {
	ss.lst = ss.lst[:0]
}

/*
	See if a match in the state is preferred to any
	previous match (perhaps none)
*/
func (prg *ReProg) newmatch(sel, r []Range) {
	if sel[0].P0<0 || r[0].P0<sel[0].P0 ||
		r[0].P0==sel[0].P0 && r[0].P1>sel[0].P1 {
		copy(sel, r)
	}
}

/*
	See if c matches the character class or not
*/
func classMatch(cls []rune, c rune) bool {
	for i := 0; i < len(cls); i++ {
		r := cls[i]
		if r==tWORD && (unicode.IsLetter(c) || unicode.IsNumber(c)) {
			return true
		}
		if r==tBLANK && unicode.IsSpace(c) {
			return true
		}
		if r == cRange {
			if cls[i+1]<=c && c<=cls[i+2] {
				return true
			}
			i += 2
		} else if r == c {
			return true
		}
	}
	return false
}

/*
	Like Exec but for strings.
	See Exec for more details.
*/
func (prg *ReProg) ExecStr(s string, start int, end int) []Range {
	return prg.ExecRunes([]rune(s), start, end)
}

/*
	Like Exec but for []rune
	See Exec for more details.
*/
func (prg *ReProg) ExecRunes(s []rune, start int, end int) []Range {
	return prg.Exec(runestr(s), start, end)
}

// Set to true to enable debug of rexp execution.
var Debug = false

/*
	Execute prg to search in s starting at s[start] and not going past
	s[end], when compiled to search forward.
	When compiled to search backward, it starts at s[start] but goes
	backwards and considers s[end] the end of text.
	Like ExecStr but for a general rune provider.
	Returns nil if there is no match. Otherwise, the returned
	slice contains at 0 the match for the full expression and
	in further elements the matching ranges for subexpressions
	matched (i.e, \1, \2, ...).
	The matched substrings are s[range.P1:range.P1].
*/
func (prg *ReProg) Exec(txt Text, start int, end int) []Range {
	if end < 0 {
		end = txt.Len()
	}
	if prg.back {
		return prg.execBack(txt, start, end)
	}
	var (
		startc, c rune
	)
	if prg.code[prg.entry].op < tOPERATOR {
		startc = prg.code[prg.entry].op
	}
	txtlen := txt.Len()
	if end > txtlen {
		end = txtlen
	}
	statel := &states{}
	nextl := &states{}
	sel := make([]Range, prg.cursubid+1)
	sel[0].P0 = -1
	sempty := make([]Range, prg.cursubid+1)

	if Debug {
		fmt.Printf("%s\n", prg)
	}
	/* Run the regexp machine for each rune in text */
	p := start
	for ; ; p++ {
		if p>end || sel[0].P0>=0 && len(statel.lst)==0 {
			return retsel(sel)
		}
		if p == end {
			/* the string is exhausted but we might have
			 * an accept state pending, so go one more round.
			 */
			c = 0
		} else {
			c = txt.Getc(p)
		}

		if Debug {
			fmt.Printf("c[%d] '%c' %c:\n", p, c, c)
		}

		// skip first char fast
		if startc!=0 && len(statel.lst)==0 && c!=startc {
			if Debug {
				fmt.Printf("\tskip\n")
			}
			continue
		}

		if sel[0].P0 < 0 {
			sempty[0].P0 = p
			statel.add(prg.entry, sempty)
		}

		// Execute the set of states, computing the next set
		for si := 0; si < len(statel.lst); si++ {
			s := statel.lst[si]
			i := s.i
		Exec:
			if i == 0 {
				break
			}
			x := prg.code[i]
			if Debug {
				fmt.Printf("\t->%s\t%v\n", x, s.sel)
			}
			switch op := x.op; op {
			default:
				if op == c {
					nextl.add(x.left, s.sel)
				}
			case tLPAREN:
				s.sel[x.subid].P0 = p
				i = x.left
				goto Exec
			case tRPAREN:
				s.sel[x.subid].P1 = p
				i = x.left
				goto Exec
			case tANY:
				if c!='\n' && c!=0 {
					nextl.add(x.left, s.sel)
				}
			case tWORD:
				if unicode.IsLetter(c) || unicode.IsNumber(c) {
					nextl.add(x.left, s.sel)
				}
			case tBLANK:
				if unicode.IsSpace(c) && c!='\n' {
					nextl.add(x.left, s.sel)
				}
			case tBOL:
				if p==0 || txt.Getc(p-1)=='\n' && p<end {
					i = x.left
					goto Exec
				}
			case tEOL:
				if c=='\n' || c==0 {
					i = x.left
					goto Exec
				}
			case tCCLASS:
				if classMatch(x.class, c) {
					nextl.add(x.left, s.sel)
				}
			case tNCCLASS:
				if !classMatch(x.class, c) {
					nextl.add(x.left, s.sel)
				}
			case tOR:
				statel.add(x.right, s.sel)
				i = x.left
				goto Exec
			case tEND:
				s.sel[0].P1 = p
				prg.newmatch(sel, s.sel)
			}
		}

		statel, nextl = nextl, statel
		nextl.clear()
	}
	return retsel(sel)
}

func (prg *ReProg) newbackmatch(sel, r []Range) {
	if sel[0].P0<0 || r[0].P0>sel[0].P1 ||
		r[0].P0==sel[0].P1 && r[0].P1<sel[0].P0 {
		for i := range r {
			sel[i].P0, sel[i].P1 = r[i].P1, r[i].P0
		}
	}
}

/*
	exactly like Exec, but searching backwards.
*/
func (prg *ReProg) execBack(txt Text, start int, end int) []Range {
	var (
		startc, c rune
	)
	if prg.code[prg.entry].op < tOPERATOR {
		startc = prg.code[prg.entry].op
	}
	statel := &states{}
	nextl := &states{}
	sel := make([]Range, prg.cursubid+1)
	sel[0].P0 = -1
	sempty := make([]Range, prg.cursubid+1)
	/* Run the regexp machine for each rune in text */
	onemore := false
	for p := start; ; p-- {
		if (!onemore && p<0) || sel[0].P0>=0 && len(statel.lst)==0 {
			return retsel(sel)
		}

		if p==0 || onemore {
			/* the string is exhausted but we might have
			 * an accept state pending, so go one more round.
			 */
			c = 0
		} else {
			c = txt.Getc(p - 1)
		}
		onemore = false
		if Debug {
			fmt.Printf("c[%d] '%c' %x:\n", p, c, c)
		}
		// skip first char fast
		if startc!=0 && len(statel.lst)==0 && c!=startc {
			if Debug {
				fmt.Printf("\tskip\n")
			}
			continue
		}

		if sel[0].P0 < 0 {
			/* -p to make list[].add() work */
			sempty[0].P0 = -p
			statel.add(prg.entry, sempty)
		}
		/* Execute everything in this list */
		for si := 0; si < len(statel.lst); si++ {
			s := statel.lst[si]
			i := s.i
		Exec:
			if i == 0 {
				break
			}
			x := prg.code[i]
			if Debug {
				fmt.Printf("\t%s\t%v\n", x, s.sel)
			}
			switch op := x.op; op {
			default:
				if op == c {
					nextl.add(x.left, s.sel)
				}
			case tLPAREN:
				// BUG?: see the xxx kludge
				s.sel[x.subid].P0 = p
				i = x.left
				goto Exec
			case tRPAREN:
				s.sel[x.subid].P1 = p
				i = x.left
				goto Exec
			case tANY:
				if c!='\n' && c!=0 {
					nextl.add(x.left, s.sel)
				}
			case tWORD:
				if unicode.IsLetter(c) || unicode.IsNumber(c) {
					nextl.add(x.left, s.sel)
				}
			case tBLANK:
				if unicode.IsSpace(c) && c!='\n' {
					nextl.add(x.left, s.sel)
				}
			case tBOL:
				if c==0 || p>0 && txt.Getc(p-1)=='\n' && p<end {
					i = x.left
					if c == 0 {
						// if we are at the start of text (c == 0)
						// we want to go through exec and then
						// go even one more round to resolve any pending
						// state.
						onemore = true
					}
					goto Exec
				}
			case tEOL:
				if p==start || txt.Getc(p)=='\n' {
					i = x.left
					goto Exec
				}
			case tCCLASS:
				if classMatch(x.class, c) {
					nextl.add(x.left, s.sel)
				}
			case tNCCLASS:
				if !classMatch(x.class, c) {
					nextl.add(x.left, s.sel)
				}
			case tOR:
				statel.add(x.right, s.sel)
				i = x.left
				goto Exec
			case tEND:
				s.sel[0].P0 = -s.sel[0].P0
				s.sel[0].P1 = p
				prg.newbackmatch(sel, s.sel)
			}
		}
		statel, nextl = nextl, statel
		nextl.clear()
	}
	return retsel(sel)
}
