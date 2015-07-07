package main

import (
	"clive/dbg"
	"io"
	"os"
)

type Tee  {
	In  io.Reader
	out []io.WriteCloser
}

func (t *Tee) New() io.Reader {
	r, w, err := os.Pipe()
	if err != nil {
		dbg.Fatal("tee: %s", err)
	}
	t.out = append(t.out, w)
	return r
}

func (t *Tee) IO() error {
	var buf [8192]byte
	var err error
	for {
		n, rerr := t.In.Read(buf[0:])
		if n == 0 {
			err = rerr
			break
		}
		for i, o := range t.out {
			if o == nil {
				continue
			}
			nw, werr := o.Write(buf[:n])
			if nw != n {
				err = werr
				o.Close()
				t.out[i] = nil
			}
		}
	}
	for _, o := range t.out {
		if o != nil {
			o.Close()
		}
	}
	return err
}
