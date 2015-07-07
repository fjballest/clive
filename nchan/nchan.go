/*
	Package clive/nchan provides utilities to bridge I/O devices and channels.

	Channels may be closed by senders to indicate EOF or to indicate other
	errors if an error is supplied. Channels may be closed by receivers to indicate
	that they do not want more data through the channel.

	Nchan propagates channel errors through the I/O device used and vice-versa.
*/
package nchan


import (
	"bufio"
	"bytes"
	"clive/bufs"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"unicode/utf8"
)

// BUG(x): Messages are not buffered, hence no flush.

// REFERENCE(x): clive/net, for helpers to dial and listen using Conns.

var (
	/*
		Buffering controls how reads and writes are
		performed by processes relaying to and from
		channels.

		This affects
		If set to true (default), NewConn, NewSplitConn,
		NewPipe, etc. will use buffering for reads and
		for writing each message in a single write
		(if possible, as controlled by the buffer used).
		Otherwise, no buffering is used and each message
		written will lead to two writes.
	*/
	Buffering = true

	// A closed channel
	Null chan []byte
)

func init() {
	Null = make(chan []byte)
	close(Null)
}

func Cprintf(c chan<- []byte, str string, args ...interface{}) error {
	ok := c <- []byte(fmt.Sprintf(str, args...))
	if ok {
		return nil
	}
	if c == nil {
		return errors.New("nil chan")
	}
	return cerror(c)
}

/*
	Receive everything sent to c and return it all as a single []byte
*/
func Bytes(c <-chan []byte) ([]byte, error) {
	var req bytes.Buffer
	for d := range c {
		req.Write(d)
	}
	return req.Bytes(), cerror(c)
}

/*
	Receive everything sent to c and return it as a single string
*/
func String(c <-chan []byte) (string, error) {
	var req bytes.Buffer
	for d := range c {
		req.Write(d)
	}
	return req.String(), cerror(c)
}

/*
	Pack a string into a []byte, appending to buff, which might grow as a result.
*/
func PutString(buf []byte, s string) []byte {
	n := uint32(len(s))
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[0:], n)
	buf = append(buf, hdr[:]...)
	buf = append(buf, s...)
	return buf
}

/*
	Unpack a string packed with PutString from a []byte and
	return the rest of the []byte along with the string.
	Unpacking from an empty buffer yields "" and is ok.
*/
func GetString(buf []byte) (string, []byte, error) {
	if len(buf) == 0 {
		return "", buf, nil
	}
	if len(buf) < 4 {
		return "", nil, errors.New("short buffer")
	}
	n := int(binary.LittleEndian.Uint32(buf[:4]))
	buf = buf[4:]
	if len(buf) < n {
		return "", nil, errors.New("short string")
	}
	s := string(buf[:n])
	buf = buf[n:]
	return s, buf, nil
}

/*
	Write everything received from c to w.
	Stops on an empty write.
	Returns the number of messages and the number of bytes
*/
func WriteBytesTo(w io.Writer, c <-chan []byte) (int64, int64, error) {
	var tot, n int64
	for data := range c {
		n++
		if len(data) == 0 {
			break
		}
		nw, err := w.Write(data)
		tot += int64(nw)
		if err != nil {
			return n, tot, err
		}
	}
	return n, tot, cerror(c)
}

// Special size indicating that an error indication follows.
// This places a limit on the max message size we can handle.
var errSize = uint64(0xFFFFFFFFFF)

// Write a header to w (either msg size or ErrSize).
func writeHdr(w io.Writer, sz uint64) (int, error) {
	var hdr [8]byte
	binary.LittleEndian.PutUint64(hdr[0:], sz)
	nw, err := w.Write(hdr[0:])
	if err != nil {
		return nw, err
	}
	if nw != len(hdr) {
		return nw, errors.New("short header write")
	}
	return nw, nil
}

// Write a msg to w for the given data
func writeMsg(w io.Writer, data []byte) (int, error) {
	nw, err := writeHdr(w, uint64(len(data)))
	if err != nil {
		return nw, err
	}
	if len(data) > 0 {
		nw2, err := w.Write(data)
		nw += nw2
		if err != nil {
			return nw, err
		}
		if nw2 != len(data) {
			return nw, errors.New("short message write")
		}
	}
	return nw, nil
}

func writeErr(w io.Writer, err error) (int64, error) {
	nw, werr := writeHdr(w, errSize)
	if werr != nil {
		return int64(nw), werr
	}
	tot := int64(nw)
	data := []byte(err.Error())
	nw, werr = writeMsg(w, data)
	tot += int64(nw)
	return tot, werr
}

// see WriteMsgsTo.
type Flusher interface {
	Flush() error
}

// see WriteMsgsTo.
type DontFlusher interface {
	DontFlush()
}

/*
	Write everything received from c to w,
	preserving message delimiters.
	Does not stop on an empty write.
	if c is closed with an error indication, the
	error is sent through the writer.

	Returns the number of messages, bytes, and error.

	When Buffering is false, messages sent are written directly.

	When Buffering is true, if w implements Flusher,
	w.Flush() is called after each message, unless w implements DontFlusher
	(which means that the caller is handling buffering itself).
*/
func WriteMsgsTo(w io.Writer, c <-chan []byte) (int64, int64, error) {
	var tot, n int64
	flusher, _ := w.(Flusher)
	_, dont := w.(DontFlusher)
	for data := range c {
		n++
		nw, err := writeMsg(w, data)
		tot += int64(nw)
		if err != nil {
			return n, tot, err
		}
		if Buffering && flusher!=nil && !dont {
			err = flusher.Flush()
			if err != nil {
				return n, tot, err
			}
		}
	}
	err := cerror(c)
	if err!=nil && err.Error()!="" {
		writeErr(w, err)
		if flusher != nil {
			flusher.Flush()
		}
	}
	return n, tot, err
}

/*
	Send everything read from r to c,
	preserving message delimiters.
	The chan capacity determines how many buffers are allocated.

	If an error was sent to the device we are reading from (eg.,
	by closing the chan given to WriteMsgsTo),
	it is retrieved and the c is closed with the same error.
*/
func ReadMsgsFrom(r io.Reader, c chan<- []byte) (nmsgs int64, nbytes int64, err error) {
	if Buffering {
		br := bufio.NewReader(r)
		r = br
	}
	var n, tot int64
	var hdr [8]byte
	for {
		_, err := io.ReadFull(r, hdr[0:])
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return n, tot, err
		}
		n++
		tot += 8
		sz := binary.LittleEndian.Uint64(hdr[0:])
		if sz == 0 {
			if ok := c <- []byte{}; !ok {
				return n, tot, cerror(c)
			}
			continue
		}
		haserr := sz == errSize
		if haserr {
			// error indication
			_, err := io.ReadFull(r, hdr[0:])
			if err != nil {
				return n, tot, err
			}
			tot += 8
			sz = binary.LittleEndian.Uint64(hdr[0:])
		}
		if sz > uint64(64*1024) {
			//XXX BUG
			fmt.Printf("TOO LARGE: max 64k sz %d\n", sz)
			panic("message too large")
			return n, tot, errors.New("message too large")
		}
		buf := make([]byte, sz)
		nr, err := io.ReadFull(r, buf[:sz])
		tot += int64(nr)
		if err != nil {
			return n, tot, err
		}
		buf = buf[:nr]
		if haserr {
			err := string(buf)
			close(c, err)
			return n, tot, cerror(c)
		}
		if ok := c <- buf; !ok {
			return n, tot, cerror(c)
		}
	}
}

/*
	Send everything read from r to c.
	The buffers sent through c can be given to
	bufs.Recycle to reuse them when no longer needed.
	The chan capacity limits how many buffers are allocated.
*/
func ReadBytesFrom(r io.Reader, c chan<- []byte) (nmsgs int64, nbytes int64, err error) {
	if Buffering {
		br := bufio.NewReader(r)
		r = br
	}
	var n, tot int64
	buf := bufs.New()
	for {
		if buf == nil {
			buf = bufs.New()
		}
		if len(buf) == 0 {
			panic("zero buf")
		}
		n++
		nr, err := r.Read(buf)
		if nr==0 && err==nil {
			err = io.EOF
		}
		if nr > 0 {
			err = nil
			tot += int64(nr)
			if nr < 512 {
				nbuf := make([]byte, nr)
				copy(nbuf, buf)
				if ok := c <- nbuf[:nr]; !ok {
					bufs.Recycle(buf)
					return n, tot, cerror(c)
				}
			} else {
				if ok := c <- buf[:nr]; !ok {
					bufs.Recycle(buf)
					return n, tot, cerror(c)
				}
				buf = nil
			}
		}
		if err == io.EOF {
			return n, tot, nil
		} else if err != nil {
			bufs.Recycle(buf)
			return n, tot, err
		}
	}
}

type wr  {
	c chan<- []byte
}

/*
	Return a writer that wraps a chan.
*/
func Writer(c chan<- []byte) io.WriteCloser {
	return wr{c}
}

func (w wr) Write(data []byte) (int, error) {
	dat := make([]byte, len(data))
	n := copy(dat, data)
	if ok := w.c <- dat; !ok {
		return n, cerror(w.c)
	}
	return n, nil
}

func (w wr) Close() error {
	close(w.c)
	return nil
}

type rd  {
	c   <-chan []byte
	m   sync.Mutex
	buf []byte
}

/*
	Return a reader that wraps a chan.
*/
func Reader(c <-chan []byte) io.ReadCloser {
	return &rd{c: c}
}

func (r *rd) Read(data []byte) (int, error) {
	r.m.Lock()
	defer r.m.Unlock()

	if len(r.buf) > 0 {
		n := copy(data, r.buf)
		if n == len(r.buf) {
			r.buf = nil
		} else {
			r.buf = r.buf[n:]
		}
		return n, nil
	}
	dat, ok := <-r.c
	if !ok {
		err := cerror(r.c)
		if err == nil {
			err = io.EOF
		}
		return 0, err
	}
	if len(dat) == 0 {
		return 0, io.EOF
	}
	n := copy(data, dat)
	if n < len(dat) {
		r.buf = dat[n:]
	} else {
		r.buf = nil
	}
	if len(dat) == 0 {
		return n, io.EOF
	}
	return n, nil
}

func (r *rd) Close() error {
	close(r.c, "hangup")
	return nil
}

// Snoop data sent through c, and print it for debugging at w.
// The returned channel should be used by the caller instead of c.
func Snoop(w io.Writer, c <-chan []byte, tag string) <-chan []byte {
	nc := make(chan []byte)
	go func() {
		for {
			x, ok := <-c
			if !ok {
				err := cerror(c)
				if tag != "" {
					fmt.Fprintf(w, "%sclosed %v\n", tag, err)
				}
				close(nc, cerror(c))
				break
			}
			if tag != "" {
				w.Write([]byte(tag))
			}
			w.Write(x)
			if tag != "" {
				w.Write([]byte("\n"))
			}
			if ok := nc <- x; !ok {
				if tag != "" {
					fmt.Fprintf(w, "%sdst closed %v\n", tag, cerror(nc))
				}
				close(c, cerror(nc))
				break
			}
		}
	}()
	return nc
}

// Take a chan to receive []byte from and return a chan to
// receive lines from it, separated by the sep character.
// The sep character (if any) is included in the strings returned.
func Lines(dc <-chan []byte, sep rune) <-chan string {
	rc := make(chan string)
	go func() {
		var buf bytes.Buffer
		saved := []byte{}
		for d := range dc {
			if len(saved) > 0 {
				saved = append(saved, d...)
				d = saved
			}
			for len(d)>0 && utf8.FullRune(d) {
				r, n := utf8.DecodeRune(d)
				d = d[n:]
				buf.WriteRune(r)
				if r == sep {
					if ok := rc <- buf.String(); !ok {
						close(dc, cerror(rc))
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
			rc <- buf.String()
		}
		close(rc, cerror(dc))
	}()
	return rc
}
