package e

import (
	"clive/sre"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	dbg "runtime/debug"
	"strings"
)

type eDir int

const (
	eAbs = eDir(iota) // absolute position
	eFwd              // forward
	eBck              // backward
)

func (s *Sam) runCmd(c *Cmd) (err error) {
	//defer un(trz("runcmd"))
	defer func() {
		if e := recover(); e != nil {
			if re, ok := e.(runtime.Error); ok {
				xprintf("cmd panic:\n%s\n", dbg.Stack())
				panic(re)
			}
			err = fmt.Errorf("%s", e)
		}
	}()

	defer func() {
		if c.tab.flag&CFsetdot != 0 {
			s.dot = c.dot
			s.dot.setFileSel()
			/* operations made on the dot's file log
			 * processed later by the main loop will also
			 * update the file selection on all the files
			 * with insertions and removals.
			 */
		}
	}()

	if (c.Adr==nil || c.Adr.Kind!='"') && c.dot.F==nil &&
		c.tab.flag&CFnowin==0 {
		return fmt.Errorf("cmd '%c': no current panel", c.Ch)
	}
	if c.tab.flag&CFinsist != 0 {
		s.insist = s.lastc == c.Ch
		s.insist = true // XXX: for e, we always insist
		s.lastc = c.Ch
	} else {
		s.lastc = rune(0)
	}

	if c.tab.flag&CFnl != 0 {
		if c.Adr == nil {
			c.dot = c.dot.lnAddr(1, eFwd)
			c.dot.setFileSel()
		} else {
			c.dot = s.cmdSel(c.Adr, c.dot, eAbs)
			c.dot.setFileSel()
		}
	} else if c.tab.flag&CFaddrmask != 0 {
		switch {
		case c.Adr == nil:
			a := &Addr{Kind: '.'}
			if c.tab.flag&CFall != 0 {
				a.Kind = '*'
			}
			c.Adr = a
		case c.Adr.Kind=='"' && c.Adr.Left==nil:
			a := &Addr{Kind: '.'}
			if c.tab.flag&CFall != 0 {
				a.Kind = '*'
			}
			c.Adr.Left = a
		}

		if c.Adr != nil {
			if c.dot.F != nil {
				c.dot = c.dot.F.Sel()
			} else {
				c.dot = eSel{}
			}
			c.dot = s.cmdSel(c.Adr, c.dot, eAbs)
			c.dot.setFileSel()
		}
	}
	dprintf("exec %s\n", c)
	c.tab.xfn(s, c)
	return nil
}

func (s *Sam) show(sel eSel) {
	for rs := range sel.F.Get(sel.P0, sel.P1-sel.P0) {
		s.Out <- string(rs)
	}
}

func (s *Sam) cmdSel(a *Addr, sel eSel, d eDir) eSel {
	var s1, s2 eSel
	f := sel.F
	for {
		switch a.Kind {
		case 'l':
			sel = sel.lnAddr(a.N, d)
		case '#':
			sel = sel.chrAddr(a.N, d)
		case '.':
			sel = f.Sel()
		case '$':
			sel.P0 = f.Len()
			sel.P1 = sel.P0
		case '\'':
			sel = f.Mark()
		case '?':
			if d != eBck {
				d = eBck
			} else {
				d = eFwd
			}
			fallthrough
		case '/':
			var m []sre.Range
			if d != eBck {
				m = sel.matchFwd(a.Rexp)
			} else {
				m = sel.matchBck(a.Rexp)
			}
			if len(m) > 0 {
				sel.P0, sel.P1 = m[0].P0, m[0].P1
			}
		case '"':
			f = s.matchFile(a.Rexp)
			sel = f.Sel()
		case '*':
			sel.P0, sel.P1 = 0, f.Len()
			return sel
		case ',', ';':
			s1 = eSel{F: sel.F}
			if a.Left != nil {
				s1 = s.cmdSel(a.Left, sel, eAbs)
			}
			if a.Kind == ';' {
				f = s1.F
				sel = s1
			}
			s2 = eSel{F: sel.F}
			if a.Right != nil {
				s2 = s.cmdSel(a.Right, sel, eAbs)
			} else {
				s2.P0 = f.Len()
				s2.P1 = s2.P0
				s2.F = f
			}
			if s2.P1 < s1.P0 {
				panic("reversed selection")
			}
			if s1.F != s2.F {
				panic("cross file selection")
			}
			sel.P0, sel.P1 = s1.P0, s2.P1
			return sel
		case '+', '-':
			d = eFwd
			if a.Kind == '-' {
				d = eBck
			}
			if a.Left==nil ||
				a.Left.Kind=='+' || a.Left.Kind=='-' {
				sel = sel.lnAddr(1, d)
			}
		default:
			panic(fmt.Sprintf("unknown address type '%c'", a.Kind))
		}
		if a = a.Left; a == nil {
			break
		}
	}
	return sel
}

func (s *Sam) matchFile(re []rune) *file {
	prg, err := sre.Compile(re, sre.Fwd)
	if err != nil {
		panic(fmt.Sprintf("match file regexp: %s", err))
	}
	for _, f := range s.f {
		nm := f.MenuLine()
		if rg := prg.ExecStr(nm, 0, len(nm)); len(rg) > 0 {
			return f
		}
	}
	panic("no file matches")
	return nil
}

func cmdNop(s *Sam, c *Cmd) {
	s.Out <- fmt.Sprintf("cmd %c not implemented\n", c.Ch)
}

func cmdcd(s *Sam, c *Cmd) {
	dir := strings.TrimSpace(string(c.Txt))
	if len(dir) == 0 {
		home := os.Getenv("HOME")
		if home == "" {
			home = os.Getenv("home")
		}
		if home == "" {
			panic("you are homeless")
		}
		dir = home
	}
	if err := os.Chdir(dir); err != nil {
		panic(fmt.Sprintf("cd %s: %s", dir, err))
	}
}

func cmdBlock(s *Sam, c *Cmd) {
	for _, cc := range c.Child {
		cc.dot = c.dot
		cc.dot.F.SetSel(cc.dot.P0, cc.dot.P1)
		if err := s.runCmd(cc); err != nil {
			panic(err)
		}
	}
}

func cmdnl(s *Sam, c *Cmd) {
	s.show(c.dot)
}

func (s *Sam) setFocus(f *file) {
	if s.dot.F != nil {
		s.dot.F.focus = false
	}
	if f == nil {
		s.dot = eSel{}
	} else {
		f.focus = true
		s.dot = f.Sel()
	}
}

/*
 * don't panic here, a single command might call this multiple times
 */
func (s *Sam) newWin(name string, quiet bool) error {
	if _, ok := s.names[name]; ok {
		if !quiet {
			s.Out <- fmt.Sprintf("B: %s: already loaded\n", name)
		}
		return nil
	}
	f, err := s.newFile(name)
	if err != nil {
		if !quiet {
			s.Out <- fmt.Sprintf("B: %s\n", err)
		}
		return err
	}
	s.f = append(s.f, f)
	s.setFocus(f)
	if !quiet {
		s.Out <- fmt.Sprintf("%s\n", f.MenuLine())
	}
	return nil
}

/*
 * don't panic here, a single command might call this multiple times
 */
func (s *Sam) delWin(name string) {
	f, ok := s.names[name]
	if !ok {
		s.Out <- fmt.Sprintf("B: %s: not loaded\n", name)
		return
	}
	if f.dirty && !s.insist {
		s.Out <- fmt.Sprintf("B: %s: has changes\n", name)
		return
	}
	if f.focus {
		s.setFocus(nil)
	}
	delete(s.names, name)
	for i := 0; i < len(s.f); i++ {
		if s.f[i] == f {
			s.f = append(s.f[:i], s.f[i+1:]...)
			break
		}
	}
}

func cmdB(s *Sam, c *Cmd) {
	str := string(c.Txt)
	str = strings.TrimSpace(str)
	if len(str) == 0 {
		s.newWin("", false)
		return
	}
	if str[0] == '<' {
		cmd := exec.Command("rc", "-c", str[1:])
		data, err := cmd.Output()
		if err != nil {
			panic(fmt.Sprintf("B: %s", err))
		}
		str = string(data)
	}
	for _, fname := range strings.Fields(str) {
		if len(fname) > 0 {
			s.newWin(fname, false)
		}
	}

}

func cmdb(s *Sam, c *Cmd) {
	str := string(c.Txt)
	str = strings.TrimSpace(str)
	if len(str) == 0 {
		panic("b: empty name")
	}
	flds := strings.Fields(str)
	if len(flds)==0 || len(flds[0])==0 {
		panic("b: empty name")
	}
	str = flds[0]
	f, ok := s.names[str]
	if !ok {
		panic(fmt.Sprintf("b: %s: not loaded\n", str))
	}
	s.setFocus(f)
	s.Out <- fmt.Sprintf("%s\n", f.MenuLine())
}

func cmdf(s *Sam, c *Cmd) {
	str := string(c.Txt)
	str = strings.TrimSpace(str)
	f := c.dot.F
	if len(str) > 0 {
		if _, ok := s.names[str]; ok {
			panic(fmt.Sprintf("f: %s: already loaded\n", str))
		}
		delete(s.names, f.path)
		s.names[str] = f
		f.dirty = true
		f.path = str
	}
	s.Out <- fmt.Sprintf("%s\n", f.MenuLine())
}

func cmdn(s *Sam, c *Cmd) {
	for _, f := range s.f {
		s.Out <- fmt.Sprintf("%s\n", f.MenuLine())
	}
}

func cmdq(s *Sam, c *Cmd) {
	for _, f := range s.f {
		if f.dirty && !s.insist {
			panic("q: unsaved changes\n")
		}
	}
	s.exiting = true
}

func cmdD(s *Sam, c *Cmd) {
	str := string(c.Txt)
	str = strings.TrimSpace(str)
	if len(str) == 0 {
		if c.dot.F == nil {
			panic("D: no current panel")
		}
		s.delWin(c.dot.F.path)
		return
	}
	if str[0] == '<' {
		cmd := exec.Command("rc", "-c", str[1:])
		data, err := cmd.Output()
		if err != nil {
			panic(fmt.Sprintf("D: %s", err))
		}
		str = string(data)
	}
	for _, fname := range strings.Fields(str) {
		if len(fname) > 0 {
			s.delWin(fname)
		}
	}

}

func cmde(s *Sam, c *Cmd) {
	f := c.dot.F
	if f.dirty && !s.insist {
		panic(fmt.Sprintf("e: %s: has changes\n", f.path))
	}
	str := string(c.Txt)
	str = strings.TrimSpace(str)
	if len(str) == 0 {
		str = f.path
	} else if _, ok := s.names[str]; ok {
		panic(fmt.Sprintf("f: %s: already loaded\n", str))
	}
	data, err := s.fs.Get(str)
	if err != nil {
		panic(err)
	}
	f.log.Repl(0, f.Len(), data, false)
	if str != f.path {
		delete(s.names, f.path)
		s.names[str] = f
		f.path = str
	}
	c.dot.P0, c.dot.P1 = 0, 0
}

func cmdr(s *Sam, c *Cmd) {
	f := c.dot.F
	str := string(c.Txt)
	str = strings.TrimSpace(str)
	if len(str) == 0 {
		panic("r: empty file name")
	}
	data, err := s.fs.Get(str)
	if err != nil {
		panic(err)
	}
	dirty := c.dot.P0==0 && c.dot.P1==len(data) && str==f.path
	f.log.Repl(c.dot.P0, c.dot.P1-c.dot.P0, data, dirty)
	c.dot.P1 = c.dot.P1 + len(data)
}

func cmda(s *Sam, c *Cmd) {
	f := c.dot.F
	f.log.Ins(c.Txt, c.dot.P1, true)
	c.dot.P0, c.dot.P1 = c.dot.P1, c.dot.P1+len(c.Txt)
}

func cmdi(s *Sam, c *Cmd) {
	f := c.dot.F
	f.log.Ins(c.Txt, c.dot.P0, true)
	c.dot.P1 = c.dot.P0 + len(c.Txt)
}

func cmdc(s *Sam, c *Cmd) {
	f := c.dot.F
	f.log.Repl(c.dot.P0, c.dot.P1-c.dot.P0, c.Txt, true)
	c.dot.P1 = c.dot.P0 + len(c.Txt)
}

func cmdd(s *Sam, c *Cmd) {
	f := c.dot.F
	f.log.Del(c.dot.P0, c.dot.P1-c.dot.P0, true)
	c.dot.P1 = c.dot.P0
}

func cmdk(s *Sam, c *Cmd) {
	f := c.dot.F
	f.SetMark(c.dot.P0, c.dot.P1)
}

func cmdgv(s *Sam, c *Cmd) {
	re, err := sre.Compile(c.Rexp, sre.Fwd)
	if err != nil {
		panic(fmt.Sprintf("g: regexp: %s", err))
	}
	rg := re.Exec(c.dot.F, c.dot.P0, c.dot.P1)
	if (len(rg)==0 && c.Ch=='v') || (len(rg)>0 && c.Ch=='g') {
		for _, child := range c.Child {
			child.dot = c.dot
			child.dot.setFileSel()
			s.runCmd(child)
		}
	}
}

func cmdmt(s *Sam, c *Cmd) {
	f := c.dot.F
	sel := s.cmdSel(c.Xaddr, c.dot, eAbs)
	txt := c.dot.Get()
	if c.dot.P0==sel.P1 && c.dot.F==sel.F {
		return
	}
	/*
	 * the log applies changes from the end to the start,
	 * must make changes in that order
	 */
	if c.dot.P0 < sel.P1 {
		if c.Ch == 'm' {
			f.log.Del(c.dot.P0, c.dot.P1-c.dot.P0, true)
		}
		sel.F.log.Ins(txt, sel.P1, true)
	} else {
		sel.F.log.Ins(txt, sel.P1, true)
		if c.Ch == 'm' {
			f.log.Del(c.dot.P0, c.dot.P1-c.dot.P0, true)
		}
	}
	sel.P0, sel.P1 = sel.P1, sel.P1+len(txt)
	c.dot = sel
}

func cmdp(s *Sam, c *Cmd) {
	txt := c.dot.Get()
	if c.Ch=='p' || (len(txt)>0 && txt[len(txt)-1]=='\n') {
		s.Out <- string(txt)
	} else {
		s.Out <- fmt.Sprintf("%s\n", string(txt))
	}
}

func cmdEq(s *Sam, c *Cmd) {
	dot := c.dot
	f := dot.F
	out := f.path + ":"
	n0 := dot.P0
	n1 := dot.P1
	if len(c.Txt)>0 && c.Txt[0]=='#' {
		out += "#"
	} else {
		sel := eSel{P0: 0, P1: dot.P0, F: f}
		n0 = sel.nlines() + 1
		n1 = n0 + dot.nlines()
		if dot.P1>0 && dot.P1>dot.P0 && f.Getc(dot.P1-1)=='\n' {
			n1--
		}
	}
	if n0 == n1 {
		s.Out <- fmt.Sprintf("%s%d\n", out, n0)
	} else {
		s.Out <- fmt.Sprintf("%s%d,%d\n", out, n0, n1)
	}
}

func cmds(s *Sam, c *Cmd) {
	sels, err := c.dot.matches(c.Rexp, false)
	if err != nil {
		panic(fmt.Sprintf("s: %s", err))
	}
	if c.Flag==0 && c.N==0 {
		c.N = 1
	}
	nth := 0
	sel := c.dot
	for _, rg := range sels {
		nth++
		if c.N!=nth && c.Flag==0 {
			continue
		}
		sel = c.dot
		repl := sel.backRefs(c.Txt, rg)
		sel.F.log.Repl(sel.P0, sel.P1-sel.P0, repl, true)
		sel.P0, sel.P1 = sel.P0, len(repl)
	}
	c.dot = sel
}

func loopCmd(s *Sam, c *Cmd, sels [][]sre.Range) {
	for _, child := range c.Child {
		for _, selx := range sels {
			sel := selx[0]
			dprintf("loop %s\n", sel)
			c.dot.P0 = sel.P0
			c.dot.P1 = sel.P1
			c.dot.setFileSel()
			child.dot = c.dot
			if err := s.runCmd(child); err != nil {
				panic(fmt.Sprintf("%c: %s", c.Ch, err))
			}
		}
	}
}

func cmdxy(s *Sam, c *Cmd) {
	if c.Rexp == nil {
		if c.Ch == 'y' {
			panic("cmdxy: bug: y requires a rexp")
		}
		sel := c.dot.lnAddr(0, eBck)
		sels := make([][]sre.Range, 0)
		for sel.P0 < c.dot.P1 {
			nsel := sel.lnAddr(1, eFwd)
			if nsel.P0 == nsel.P1 {
				break
			}
			sel = nsel
			if sel.P1 > sel.P0 {
				sels = append(sels,
					[]sre.Range{{sel.P0, sel.P1}})
			}
		}
		loopCmd(s, c, sels)
		return
	}
	dprintf("loop dot %s\n", c.dot)
	sels, err := c.dot.matches(c.Rexp, c.Ch=='y')
	if err != nil {
		panic(fmt.Sprintf("%c: %s", c.Ch, err))
	}
	loopCmd(s, c, sels)
}

func cmdsh(s *Sam, c *Cmd) {
	cline := string(c.Txt)
	proc := exec.Command("rc", "-c", cline)
	ifd, err := proc.StdinPipe()
	if err != nil {
		panic(err)
	}
	if c.Ch=='|' || c.Ch=='>' {
		txt := c.dot.Get()
		data := []byte(string(txt))
		go func() {
			defer ifd.Close()
			if _, err := ifd.Write(data); err != nil {
				s.Out <- fmt.Sprintf("%s\n", err)
			}
		}()
	} else {
		ifd.Close()
	}
	data, err := proc.CombinedOutput()
	if err != nil {
		panic(err)
	}
	if c.Ch=='|' || c.Ch=='<' {
		txt := []rune(string(data))
		f := c.dot.F
		f.log.Repl(c.dot.P0, c.dot.P1-c.dot.P0, txt, true)
		c.dot.P1 = c.dot.P0 + len(txt)
	}
	if c.Ch=='!' || c.Ch=='>' {
		s.Out <- string(data)
	}
}

func cmdw(s *Sam, c *Cmd) {
	f := c.dot.F
	str := string(c.Txt)
	str = strings.TrimSpace(str)
	if len(str) == 0 {
		str = f.path
	}
	rc := c.dot.F.Get(c.dot.P0, c.dot.P1-c.dot.P0)
	if err := s.fs.Put(str, rc); err != nil {
		panic(err)
	}
	if str==f.path && c.dot.P0==0 && c.dot.P1==f.Len() {
		f.dirty = false
	}
}

func cmdXY(s *Sam, c *Cmd) {
	if s.inXY {
		panic("can't next X or Y commands")
	}
	s.inXY = true
	defer func() { s.inXY = false }()

	var re *sre.ReProg
	var err error
	if c.Rexp != nil {
		if re, err = sre.Compile(c.Rexp, sre.Fwd); err != nil {
			panic(fmt.Sprintf("%c: regexp: %s", c.Ch, err))
		}
	} else if c.Ch == 'Y' {
		panic("Y: no regexp given")
	}
	nf := make([]*file, len(s.f))
	copy(nf, s.f)
	for _, f := range nf {
		matches := true
		if re != nil {
			nm := f.MenuLine() + "\n"
			rg := re.ExecStr(nm, 0, len(nm))
			matches = len(rg) > 0
		}
		if (c.Ch=='X' && !matches) || (c.Ch=='Y' && matches) {
			continue
		}
		c.dot = f.Sel()
		for _, child := range c.Child {
			child.dot = c.dot
			if err := s.runCmd(child); err != nil {
				s.Out <- fmt.Sprintf("%s\n", err)
			}
		}
	}
}

/*
 * TODO?: undo operates on the dot, would we want to undo globally
 * no matter the file? Perhaps a new command U?
 * TODO: u does not undo file rename, we should warn about a rename with
 * dirty changes or let the user undo the rename.
 */
func cmdu(s *Sam, c *Cmd) {
	if c.N == 0 {
		c.N = 1
	}
	isundo := c.N > 0
	if !isundo {
		c.N = -c.N
	}
	for ; c.N > 0; c.N-- {
		var ok bool
		if isundo {
			ok = c.dot.F.Undo()
		} else {
			ok = c.dot.F.Redo()
		}
		if !ok {
			s.Out <- "no more edits\n"
			break
		}
	}
	c.dot = c.dot.F.Sel()
}
