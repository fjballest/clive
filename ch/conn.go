package ch

import (
	"io"
	"sync"
)

// A Conn is a channel-pair used as a duplex connection.
// The tag may be used for debugging or to convey the address
// of the other end of the connection.
struct Conn {
	Tag string // debug
	In  <-chan face{}
	Out chan<- face{}
}

// Creates an io.Pipe with a Conn interface, using channels with
// nbuf elements of buffering.
func NewPipe(nbuf int) Conn {
	// We use io.Pipe so we can close with errors
	r, w := io.Pipe()
	in := make(chan face{}, nbuf)
	out := make(chan face{}, nbuf)
	c := Conn{Tag: "pipe", In: in, Out: out}
	go func() {
		_, _, err := WriteMsgs(w, 1, out)
		close(out, err)
		w.CloseWithError(err)
	}()
	go func() {
		_, _, err := ReadMsgs(r, in)
		close(in, err)
		r.CloseWithError(err)
	}()
	return c
}

// Returns two Conns piped to each other.
// The chans involved all have nbuf elements of buffering.
// Useful for debugging or to replace the network.
func NewPipePair(nbuf int) (Conn, Conn) {
	p1 := NewPipe(nbuf)
	p2 := NewPipe(nbuf)
	c1 := Conn{Tag: "pipe", In: p1.In, Out: p2.Out}
	c2 := Conn{Tag: "pipe", In: p2.In, Out: p1.Out}
	return c1, c2
}

interface (
	// See NewConn.
	CloseReader {
		CloseRead() error
	}

	// See NewConn.
	CloseWriter {
		CloseWrite() error
	}
)

// Create a Conn to perform msg I/O through the given device.
// If r/w implements CloseReader/CloseWriter, half closes are used.
// Note that TCP has half closes but TLS does not.
// Otherwise, if Close() is implemented, the end of the reading or
// writing processes cause a close on the entire connection.
// Error messages are propagated like everybody else and do not
// cause a break.
// I/O errors (and the like) on the device do cause the connection to break
// and the error is propagated if possible.
// If hup is not nil, it is closed when rw is closed.
func NewConn(rw io.ReadWriter, nbuf int, hup chan bool) Conn {
	in := make(chan face{}, nbuf)
	out := make(chan face{}, nbuf)
	c := Conn{Tag: "conn", In: in, Out: out}
	closereader, _ := rw.(CloseReader)
	closewriter, _ := rw.(CloseWriter)
	closer, _ := rw.(io.Closer)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		_, _, err := WriteMsgs(rw, 1, out)
		if closewriter != nil {
			cerr := closewriter.CloseWrite()
			if err == nil {
				err = cerr
			}
		} else if closer != nil {
			closer.Close()
		}
		close(out, err)
		wg.Done()
	}()
	go func() {
		_, _, err := ReadMsgs(rw, in)
		close(in, err)
		if closereader != nil {
			closereader.CloseRead()
		} else if closer != nil {
			closer.Close()
		}
		wg.Done()
	}()
	go func() {
		wg.Wait()
		if closereader == nil || closewriter == nil {
			if closer != nil {
				closer.Close()
			}
		}
		if hup != nil {
			close(hup)
		}
	}()
	return c
}
