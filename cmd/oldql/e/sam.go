/*
	Variant of the sam editor
*/
package e

import (
	"clive/sre"
)

type eSel struct {
	P0, P1 int
	F      *file
}

// Editor in the Plan 9's sam style.
type Sam struct {
	fs      Fsys
	In      chan string
	Out     chan string
	runec   chan rune
	waitc   chan int
	dot     eSel
	insist  bool // to insist on dangerous commands
	lastc   rune // to insist on dangerous commands
	exiting bool
	inXY    bool
	names   map[string]*file
	f       []*file
}

// Create a new editor
func New(fname ...string) *Sam {
	s := &Sam{
		fs:    LocalFS,
		In:    make(chan string),
		Out:   make(chan string),
		runec: make(chan rune),
		waitc: make(chan int, 1),
		names: make(map[string]*file),
	}
	go func() {
		for ln := range s.In {
			for _, r := range ln {
				s.runec <- r
			}
			if s.exiting {
				break
			}
		}
		close(s.runec)
	}()
	go s.loop()
	for _, f := range fname {
		s.newWin(f, true)
	}
	return s
}

func (s *Sam) loop() {
	defer close(s.Out)
	l := lex{c: s.runec}
	// s.newWin("", true)
	sre.Debug = debug
	for !s.exiting {
		if s.dot.F != nil {
			s.dot = s.dot.F.Sel()
		}
		c, err := l.parseCmd(0)
		if err != nil {
			eprintf("error: %s\n", err)
			continue
		} else if c == nil {
			dprintf("EOF\n")
			break // eof
		}
		c.dot = s.dot
		dprintf("dot %s cmd: %s\n", s.dot, c)
		if err := s.runCmd(c); err != nil {
			eprintf("error: %s\n", err)
		}
		for _, f := range s.f {
			f.ApplyLog()
		}
	}
	dprintf("exiting %v\n", s.exiting)
	s.waitc <- 0
	close(s.waitc)
	close(s.In)
	close(s.Out)
}

// Wait for sam to finish
func (s *Sam) Wait() {
	<-s.waitc
}
