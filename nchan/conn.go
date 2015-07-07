package nchan

import (
	"bufio"
	"io"
	"reflect"
)

// See NewSplitConn.
type CloseReader interface {
	CloseRead() error
}

// See NewSplitConn.
type CloseWriter interface {
	CloseWrite() error
}

/*
	A Conn is a channel-pair used as a duplex connection.
	The tag may be used for debugging or to convey the address
	of the other end of the connection.
*/
type Conn  {
	Tag string // debug
	In  <-chan []byte
	Out chan<- []byte
}

/*
	Create a pipe with a Conn interface, using channels with
	nbuf elements of buffering.

	When Buffering is false, messages are sent directly.
	When Buffering is true, buffering is used for writing and
	w.Flush() is called after each message (message header and data
	are sent in a single write) and buffering is used for reading.
*/
func NewPipe(nbuf int) Conn {
	r, w := io.Pipe()
	in := make(chan []byte, nbuf)
	out := make(chan []byte, nbuf)
	c := Conn{In: in, Out: out}

	go func() {
		var wr io.Writer = w
		if Buffering {
			wr = bufio.NewWriter(w)
		}
		_, _, err := WriteMsgsTo(wr, out)
		close(out, err)
		w.CloseWithError(err)
	}()
	go func() {
		_, _, err := ReadMsgsFrom(r, in)
		close(in, err)
		r.CloseWithError(err)
	}()
	return c
}

/*
	Return two Conns piped to each other.
	The chans involved all have nbuf elements of buffering.
	Useful for debugging or to replace the network.
*/
func NewConnPipe(nbuf int) (Conn, Conn) {
	p1 := NewPipe(nbuf)
	p2 := NewPipe(nbuf)
	c1 := Conn{In: p1.In, Out: p2.Out}
	c2 := Conn{In: p2.In, Out: p1.Out}
	return c1, c2
}

// Like NewSplitConn(rw, rw, nbuf, wout).
func NewConn(rw io.ReadWriter, nbuf int, win, wout chan bool) Conn {
	return NewSplitConn(rw, rw, nbuf, win, wout)
}

/*
	Create a Conn to perform I/O through the given input and output devices
	(perhaps the same). Messages sent through the Conn.Out are
	written to w using WriteMsgsTo. Those read with ReadMsgsFrom from r are
	sent to Conn.In. Errors are also propagated. If win/wout is not nil,
	it is closed when the reader/write process exits with the error causing the exit.

	if r/w implement Closer, Close is called when done, but,
	if r/w implement CloseReader/CloseWriter, half closes are used.

	When Buffering is false, messages sent are written directly and messages
	received are read by reading exactly the message size and its data.

	When Buffering is true, readers are always buffered; if w implements Flusher,
	w.Flush() is called after each message, unless w implements DontFlusher
	(which means that the caller is handling buffering itself).
*/
func NewSplitConn(r io.Reader, w io.Writer, nbuf int, win, wout chan bool) Conn {
	in := make(chan []byte, nbuf)
	out := make(chan []byte, nbuf)
	c := Conn{In: in, Out: out}
	rcloser, _ := r.(io.Closer)
	closereader, _ := r.(CloseReader)
	wcloser, _ := w.(io.Closer)
	closewriter, _ := w.(CloseWriter)

	go func() {
		var wr io.Writer = w
		_, buffered := w.(Flusher)
		if Buffering && !buffered {
			wr = bufio.NewWriter(w)
		}
		_, _, err := WriteMsgsTo(wr, out)
		if closewriter != nil {
			closewriter.CloseWrite()
		} else if reflect.ValueOf(r)!=reflect.ValueOf(w) && wcloser!=nil && rcloser!=nil {
			wcloser.Close()
		}
		close(out, err)
		if wout != nil {
			close(wout, err)
		}
	}()
	go func() {
		_, _, err := ReadMsgsFrom(r, in)
		if closereader != nil {
			closereader.CloseRead()
		} else if rcloser != nil {
			rcloser.Close()
		}
		close(in, err)
		if win != nil {
			close(win, err)
		}
	}()
	return c
}
