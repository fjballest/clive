package cmd

import (
	"io"
	"sync"
)

const nBuf = 8192

// An interactive reader capable of prompting the user.
// IsTTY can be used to decide if the prompt should be empty,
// in which case it is not written.
type Reader  {
	lk     sync.Mutex
	in     io.Reader
	out    io.Writer
	prompt []byte
	buf    *[nBuf]byte
	saved  []byte
}

func NewReader(in io.Reader, out io.Writer, prompt string) *Reader {
	return &Reader{in: in, out: out, prompt: []byte(prompt),
		buf: new([8192]byte),
	}
}

func (r *Reader) SetPrompt(s string) {
	r.prompt = []byte(s)
}

func (r *Reader) Read(buf []byte) (n int, err error) {
	r.lk.Lock()
	defer r.lk.Unlock()
	if len(r.saved) == 0 {
		if len(r.prompt) > 0 {
			r.out.Write(r.prompt)
		}
		n, err = r.in.Read(r.buf[0:])
		r.saved = r.buf[0:n]
	}
	if len(r.saved) > 0 {
		n = copy(buf, r.saved[0:])
		r.saved = r.saved[n:]
		if n == 0 {
			err = io.EOF
		}
	}
	return n, err
}

func (r *Reader) Flush() {
	r.lk.Lock()
	defer r.lk.Unlock()
	if len(r.saved) > 0 {
		r.saved = r.saved[:0]
	}
}
