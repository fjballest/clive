package wr

import (
	"clive/app/wr/frmt"
	"fmt"
)

const (
	cmdEsc   = frmt.Esc
	cmdNoEsc = frmt.NoEsc
)

func (f *par) newPar() {
	sc, wc := frmt.Words()
	lnc := frmt.Fmt(wc, f.wid, f.right, frmt.NoBlankLines)
	donec := make(chan bool)
	go func() {
		i := f.i0
		for ln := range lnc {
			if len(ln) >= 0 {
				fmt.Fprintf(f.out, i)
				i = f.in
				s := string(ln)
				if f.fn != nil {
					s = f.fn(s)
				}
				fmt.Fprintf(f.out, "%s\n", s)
			}
		}
		close(donec, cerror(lnc))
	}()
	f.sc, f.dc = sc, donec
}

func (f *par) closePar() {
	if f.sc != nil {
		close(f.sc)
		<-f.dc
	}
	f.sc, f.dc = nil, nil
}

// close and reset also i0, in, fn
func (f *par) endPar() {
	f.closePar()
	f.fn = nil
	f.i0 = ""
	f.in = ""
}

func (f *par) printPar(ss ...string) {
	if f.sc == nil {
		f.newPar()
	}
	for _, s := range ss {
		f.sc <- s
	}
}

func (f *par) printParCmd(ss ...string) {
	if f.sc == nil {
		f.newPar()
	}
	for _, s := range ss {
		f.sc <- cmdEsc+s+cmdNoEsc
	}
}

func (f *par) printCmd(fmts string, arg ...interface{}) {
	f.closePar()
	fmt.Fprintf(f.out, fmts, arg...)
}
