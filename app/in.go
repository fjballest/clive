package app

import (
	"os"
	"io"
	"clive/app/tty"
	"bytes"
	"unicode/utf8"
	"clive/dbg"
)

type Prompter interface {
	Prompt() string
}

// Start reading OS stdin and return a chan that can be used as a clive in.
// Upon reception of a intr signal, dbg.ErrIntr is sent through the input.
func OSIn() chan interface{} {
	return rdlines(nil)
}

// Start reading OS stdin with the given prompt and return a chan that can be
// used as In in any context.
// No prompt is written if stdin is not a tty.
func PromptIn(p Prompter) chan interface{} {
	if !tty.IsTTY(os.Stdin) {
		p = nil
	}
	return rdlines(p)
}

func rdlines(p Prompter) chan interface{} {
	c := make(chan interface{})
	inr, inw, err := os.Pipe()
	if err != nil {
		close(c, err)
		return c
	}
	go func() {
		io.Copy(inw, os.Stdin)
		inw.Close()
	}()
	/*
	stop := func(sig string) bool {
		close(c, "interrupted")
		inw.Close()
		return false
	}
	*/
	stop := func(sig string) bool {
		go func() {
			c <- dbg.ErrIntr
		}()
		return false
	}
	Handle("intr", stop)
	dprintf("reading stdin\n")
	go func() {
		defer dprintf("not reading stdin\n")
		defer DontHandle("intr", stop)
		for {
			buf := make([]byte, 4096)
			if p != nil {
				ps := p.Prompt()
				if ps != "" {
					os.Stdout.WriteString(ps)
				}
			}
			nr, err := inr.Read(buf)
			if nr > 0 {
				ok := c <- buf[:nr]
				if !ok {
					break
				}
			}
			if err != nil {
				if err == io.EOF {
					err = nil
				}
				close(c, err)
				break
			}
		}
	}()
	return c
}

// pipe an input chan and make sure the output
// issues one message per file in the input containing all data.
// non []byte messages forwarded as-is.
func FullFiles(c chan interface{}) chan interface{} {
	rc := make(chan interface{})
	go func() {
		var b *bytes.Buffer
		for m := range c {
			switch d := m.(type) {
			case []byte:
				if b == nil {
					b = &bytes.Buffer{}
				}
				b.Write(d)
			default:
				if b != nil {
					if ok := rc <- b.Bytes(); !ok {
						return
					}
					b = nil
				}
				if ok := rc <- m; !ok {
					return
				}
			}
		}
		if b != nil {
			rc <- b.Bytes()
		}
		close(rc, cerror(c))
	}()
	return rc
}

// pipe an input chan and make sure the output
// issues one message per line in the input.
// non []byte messages are forwarded as-is.
func Lines(c chan interface{}) chan interface{} {
	sep := '\n'
	rc := make(chan interface{})
	go func() {
		var buf bytes.Buffer
		saved := []byte{}
		for m := range c {
			d, ok := m.([]byte)
			if !ok {
				if len(saved) > 0 {
					rc <- saved
					saved = nil
				}
				if ok := rc <- m; !ok {
					return
				}
				continue
			}
			if len(saved) > 0 {
				nb := []byte{}
				nb = append(nb, saved...)
				nb = append(nb, d...)
				d = nb
				saved = nil
			}
			for len(d)>0 && utf8.FullRune(d) {
				r, n := utf8.DecodeRune(d)
				d = d[n:]
				buf.WriteRune(r)
				if r == sep {
					nb := make([]byte, buf.Len())
					copy(nb, buf.Bytes())
					if ok := rc <- nb; !ok {
						return
					}
					buf.Reset()
				}
			}
			saved = d
		}
		if len(saved) > 0 {
			buf.Write(saved)
		}
		if buf.Len() > 0 {
			rc <- buf.Bytes()
		}
		close(rc, cerror(c))
	}()
	return rc
}

type uxrd {
	inc chan interface{}
	left []byte
}

// Read from the input only when a unix app calls read
func InReader() io.ReadCloser {
	return &uxrd{inc: In()}
}

func (u *uxrd) Read(buf []byte) (int, error) {
	for len(u.left) == 0 {
		x, ok := <-u.inc
		if !ok {
			err := cerror(u.inc)
			if err == nil {
				err = io.EOF
			}
			return 0,err
		}
		if e, ok := x.(error); ok {
			return 0, e
		}
		if b, ok := x.([]byte); ok {
			u.left = b
			break
		}
	}
	n := copy(buf, u.left)
	u.left = u.left[n:]
	return n, nil
}

func (u *uxrd) Close() error {
	return nil
}
