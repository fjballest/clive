/*
	Ql builtin and external frmt command.
	format text
*/
package frmt

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/nchan"
	"clive/zx"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"
)

type parFmt  {
	rc chan string
	ec chan bool
}

func startPar(out io.Writer, indent0, indent string, max int) *parFmt {
	rc := make(chan string)
	ec := make(chan bool, 1)
	wc := make(chan string)
	pf := &parFmt{rc, ec}
	go func() {
		for s := range rc {
			if s == "\n" {
				wc <- s
				continue
			}
			words := strings.Fields(strings.TrimSpace(s))
			for _, w := range words {
				wc <- w
			}
		}
		close(wc)
	}()
	go func() {
		pos, _ := fmt.Fprintf(out, "%s", indent0)
		firstword := true
		lastword := "x"
		for w := range wc {
			if len(w) == 0 {
				continue
			}
			if w == "\n" {
				fmt.Fprintf(out, "\n")
				firstword = true
				pos = 0
				continue
			}
			if pos+len(w)+1 > max {
				fmt.Fprintf(out, "\n")
				pos, _ = fmt.Fprintf(out, "%s", indent)
				firstword = true
			}
			if !firstword && len(w)>0 && !unicode.IsPunct(rune(w[0])) {
				lastr := rune(lastword[len(lastword)-1])
				if !strings.ContainsRune("([{", lastr) {
					fmt.Fprintf(out, " ")
					pos++
				}
			}
			fmt.Fprintf(out, "%s", w)
			pos += len(w)
			firstword = false
			lastword = w
		}
		if !firstword {
			fmt.Fprintf(out, "\n")
		}
		close(ec)
	}()
	return pf
}

func (pf *parFmt) WriteString(s string) {
	pf.rc <- s
}

func (pf *parFmt) Close() {
	if pf == nil {
		return
	}
	close(pf.rc)
	<-pf.ec
}

type xCmd  {
	*cmd.Ctx
	*opt.Flags
	wid int
}

func tabsOf(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != '\t' {
			return i
		}
	}
	return 0
}

func (x *xCmd) RunFile(d zx.Dir, dc <-chan []byte) error {
	if dc == nil {
		return nil
	}
	rc := nchan.Lines(dc, '\n')
	var pf *parFmt
	ntabs := 0
	tabs := ""
	doselect {
	case <-x.Intrc:
		close(rc, "interrupted")
		pf.Close()
		return errors.New("interrupted")
	case s, ok := <-rc:
		if !ok {
			pf.Close()
			return cerror(rc)
		}
		if s=="\n" || s=="" {
			pf.Close()
			pf = nil
			x.Printf("\n")
			continue
		}
		nt := tabsOf(s)
		if nt != ntabs {
			pf.Close()
			pf = nil
			ntabs = nt
			tabs = strings.Repeat("\t", ntabs)
		}
		if pf == nil {
			pf = startPar(x.Stdout, tabs, tabs, x.wid)
		}
		pf.WriteString(s)
	}
	pf.Close()
	if err := cerror(rc); err != nil {
		return err
	}
	return nil
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("{file}")
	x.Argv0 = argv[0]
	x.NewFlag("w", "wid: set max line width", &x.wid)
	args, err := x.Parse(argv)
	if err != nil {
		x.Usage(x.Stderr)
		return err
	}
	if x.wid < 10 {
		x.wid = 70
	}
	if cmd.Ns == nil {
		cmd.MkNS()
	}
	return cmd.RunFiles(x, args...)
}
