/*
	Format paragraphs
*/

package frmt

import (
	"unicode"
	"strings"
)

// 1st arg to Fmt
const (
	Left = false	// left-justify only
	Both = true	// justify both margins
)

// 2nd arg to Fmt
type ParFmt int

const (
	NoBlankLines ParFmt = iota		// format everything as a single par
	OneBlankLine			// output one empty line between pars
	AllBlankLines			// keep all empty lines between pars
)

// Escapes can be used to handle a portion of text as a single word.
// They are also written to the output and, if used, requires the
// caller to remove them before presenting data to the user.
const (
	Esc = "\001"		// start of raw word escape
	NoEsc = "\002"	// end of raw word escape
)

// Given a chan where we send strings, return a chan to
// get separate words and white-space runs.
func Words() (chan<-string, <-chan []rune) {

	sc := make(chan string)
	rc := make(chan []rune)
	go func() {
		first := false
		var inword bool
		w := []rune{}
		esc := rune(Esc[0])
		noesc := rune(NoEsc[0])
		escing := false
		for s := range sc {
			for _, r := range s {
				spc := unicode.IsSpace(r) && !escing
				if r == esc {
					escing = true
				} else if r == noesc {
					escing = false
				}
				if first {
					inword = !spc
					first = false
				}
				if spc == inword {
					if ok := rc <- w; !ok {
						close(sc, cerror(rc))
					}
					w = nil
					inword = !spc
				}
				w = append(w, r)
			}
		}
		if len(w) > 0 {
			rc<-w
		}
		close(rc, cerror(sc))
	}()
	return sc, rc
}

func rightJust(ln [][]rune, wid int) {
	n := lnLen(ln)
	if n >= wid || len(ln) < 3 {
		return
	}
	gaps := len(ln)/2
	add := wid - n
	fix := add/gaps
	rem := add%gaps
	for i := 1; i < len(ln); i += 2 {
		ns := fix
		if rem > 0 {
			ns++
			rem--
		}
		spc := []rune(strings.Repeat(" ", ns))
		ln[i] = append(ln[i], spc...)
	}
}

func trimWord(ln [][]rune, wid int, right bool) ([]rune, [][]rune) {
	ln = trimSpc(ln)
	switch n := len(ln); n {
	case 0:
		return nil, nil
	case 1:
		return ln[0], nil
	default:
		nln := trimSpc(ln[:n-1])
		if right {
			rightJust(nln, wid)
		}
		return join(nln), ln[n-1:]
	}
}

func trimSpc(ln [][]rune) [][]rune {
	if len(ln) == 0 {
		return ln
	}
	last := ln[len(ln)-1]
	if unicode.IsSpace(last[0]) {
		return ln[:len(ln)-1]
	}
	return ln
}

func join(ln [][]rune) []rune {
	o := []rune{}
	for _, w := range ln {
		o = append(o, w...)
	}
	return o
}

func lnLen(ln [][]rune) int {
	tot := 0
	for _, w := range ln {
		tot += len(w)
	}
	return tot
}

func sendLn(ln [][]rune, tot int, rc chan []rune, wid int, right bool) ([][]rune, int, error) {
	var fmtln []rune
	if tot > wid {
		fmtln, ln = trimWord(ln, wid, right)
	} else {
		if right {
			rightJust(ln, wid)
		}
		fmtln, ln = join(ln), nil
	}
	if ok := rc <- fmtln; !ok {
		return nil, 0, cerror(rc)
	}
	return ln, lnLen(ln), nil
}

func nlines(w []rune) int {
	tot := 0
	for _, r := range w {
		if r == '\n' {
			tot++
		}
	}
	return tot
}

// Given a chan as the output of Words(), return a chan to read lines from it
// so that each line is at most n runes wide.
// This eats most of the white space including empty lines.
// Justify also the right margin if right is true.
// If keeplines is true, runs of empty lines are not eated, but
// replaced with a single empty line.
func Fmt(wc <-chan []rune, wid int, right bool, keeplines ParFmt) <-chan []rune {
	rc := make(chan []rune)
	go func() {
		first := true
		ln := [][]rune{}
		tot := 0
		for w := range wc {
			var err error
			if len(w) == 0 {
				continue
			}
			isspc := unicode.IsSpace(w[0])
			if keeplines != NoBlankLines && isspc {
				n := nlines(w)
				if first && n == 1 {
					n++
				}
				if n > 1 {
					if len(ln) > 0 {
						ln, tot, err = sendLn(ln, tot, rc, wid, right)
						if err != nil {
							close(wc, cerror(rc))
							return
						}
					}
					for i := 0; i < n-1; i++ {
						if ok := rc <- []rune{}; !ok {
							close(wc, cerror(rc))
							return
						}
						if keeplines == OneBlankLine {
							break
						}
					}
					first = false
					continue
				}
			}
			if first && isspc {
				continue
			}
			first = false
			if isspc {
				w[0] = ' '
				ln = append(ln, w[:1])
				tot++
				continue
			}
			ln = append(ln, w)
			tot += len(w)
			if tot > wid {
				ln, tot, err = sendLn(ln, tot, rc, wid, right)
				if err != nil {
					close(wc, cerror(rc))
					return
				}
			}
		}
		if ln = trimSpc(ln); len(ln) > 0 {
			rc <- join(ln)
		}
		close(rc, cerror(wc))
	}()
	return rc
}
