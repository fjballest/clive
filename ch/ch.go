/*
	Channels that can go through external I/O devices for Clive.

	The protocol used in the device permits muxing of multiple
	channels within a single connection.

	In the connection, messages exchanged use the format:

	size[4] tag[2] type[2] data[size]

	Here, a tag identifies a channel and type identifies the type
	of data exchanged.
*/
package ch

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// Message types.
// It is ok for the user to define new types.
// By convention, unknown message types are to be forwarded as-is
// by processes piping data along with actual data being processed.
// For any unknown type, Bytes(), WriteTo(), and String() are used
// if they exist, if they don't, the type is discarded.
// In this case, the type is Tign unless a TypeId() method exists to
// return the type id to be sent.
const (
	Tnone  uint16 = iota
	Tbytes        // byte[], used as data
	Tign          // byte[], ignored as data
	Tstr          // string
	Terr          // error string
	Taddr         // file address (name, ln, ch)
	Tdir          // map[string]string, directory entry
	Tusr          // first user defined type value
)

const (
	hdrSz = 4 + 4 + 2

	// Maximum supported msg sz
	MaxMsgSz = 64 * 1024
	// Maximum supported len(Dir)
	MaxDirSz = 1024
)

// byte[] messages ignored as data.
type Ign []byte

// addresses
struct Addr {
	Name     string // file or resource name
	Ln0, Ln1 int    // line range or zero
	P0, P1   int    // point (rune) range or zero
}

// directory entry
type Dir map[string]string

interface byteser {
	Bytes() []byte
}

interface typer {
	TypeId() uint16
}

interface writerTo {
	Len() int
	WriteTo(w io.Writer) (int64, error)
}

// For user defined types, implementors of this interface
// use their own make function to make values of the message type
// upon reception.
interface Unpacker {
	TypeId() uint16
	Unpack([]byte) (face{}, error)
}

var (
	ErrTooLarge  = errors.New("message size is too large")
	ErrTooSmall  = errors.New("truncated message")
	ErrAlready   = errors.New("type already defined")
	ErrDiscarded = errors.New("msg write discarded")

	// Msg size for []byte readers
	MsgSz = 16 * 1024

	empty = []byte{} // it must be a slice

	unpackers = map[uint16]Unpacker{
		Taddr: Addr{},
		Tdir:  Dir{},
	}
)

// Define a user type to be sent through chans
func DefType(x Unpacker) error {
	id := x.TypeId()
	if id < Tusr || unpackers[id] != nil {
		return ErrAlready
	}
	unpackers[id] = x
	return nil
}

func (a Addr) TypeId() uint16 {
	return Taddr
}

func (a Addr) Bytes() []byte {
	var buf bytes.Buffer

	binary.Write(&buf, binary.LittleEndian, uint32(a.Ln0))
	binary.Write(&buf, binary.LittleEndian, uint32(a.Ln1))
	binary.Write(&buf, binary.LittleEndian, uint32(a.P0))
	binary.Write(&buf, binary.LittleEndian, uint32(a.P1))
	buf.WriteString(a.Name)
	return buf.Bytes()
}

func parseAddr(b []byte) (Addr, error) {
	var a Addr

	if len(b) < 16 {
		return a, ErrTooSmall
	}
	a.Ln0 = int(binary.LittleEndian.Uint32(b[0:]))
	a.Ln1 = int(binary.LittleEndian.Uint32(b[4:]))
	a.P0 = int(binary.LittleEndian.Uint32(b[8:]))
	a.P1 = int(binary.LittleEndian.Uint32(b[12:]))
	a.Name = string(b[16:])
	return a, nil
}

func (a Addr) Unpack(b []byte) (face{}, error) {
	a, err := parseAddr(b)
	return a, err
}

func (d Dir) Bytes() []byte {
	var buf bytes.Buffer

	binary.Write(&buf, binary.LittleEndian, uint32(len(d)))
	for k, v := range d {
		binary.Write(&buf, binary.LittleEndian, uint16(len(k)))
		buf.WriteString(k)
		binary.Write(&buf, binary.LittleEndian, uint16(len(v)))
		buf.WriteString(v)
	}
	return buf.Bytes()
}

func (d Dir) TypeId() uint16 {
	return Tdir
}

func UnpackString(b []byte) ([]byte, string, error) {
	if len(b) < 2 {
		return nil, "", ErrTooSmall
	}
	sz := int(binary.LittleEndian.Uint16(b[0:]))
	b = b[2:]
	if len(b) < sz {
		return nil, "", ErrTooSmall
	}
	return b[sz:], string(b[:sz]), nil
}

func parseDir(b []byte) (Dir, error) {
	if len(b) < 4 {
		return nil, ErrTooSmall
	}
	d := map[string]string{}
	n := int(binary.LittleEndian.Uint32(b[0:]))
	if n < 0 || n > MaxDirSz {
		return nil, ErrTooLarge
	}
	b = b[4:]
	var err error
	var k, v string
	for i := 0; i < n; i++ {
		b, k, err = UnpackString(b)
		if err != nil {
			return nil, err
		}
		b, v, err = UnpackString(b)
		if err != nil {
			return nil, err
		}
		d[k] = v
	}
	return d, nil
}

func (d Dir) Unpack(b []byte) (face{}, error) {
	d, err := parseDir(b)
	return d, err
}

func writeBytes(w io.Writer, tag uint32, typ uint16, b []byte) (int, error) {
	var hdr [hdrSz]byte

	if b == nil {
		b = empty[:]
	}
	n := len(b)
	binary.LittleEndian.PutUint32(hdr[0:], uint32(n))
	binary.LittleEndian.PutUint32(hdr[4:], tag)
	binary.LittleEndian.PutUint16(hdr[8:], typ)
	tot, err := w.Write(hdr[:])
	if err != nil || n == 0 {
		return tot, err
	}
	tot, err = w.Write(b)
	if err != nil {
		tot += len(hdr)
	}
	return tot, err
}

// Write []byte, or Ign, string, error, Stringer, Byteser or discard the write.
// If the write is discarded, ErrDiscarded is returned.
func WriteMsg(w io.Writer, tag uint32, m face{}) (int, error) {
	switch m := m.(type) {
	case []byte:
		return writeBytes(w, tag, Tbytes, m)
	case Ign:
		return writeBytes(w, tag, Tign, []byte(m))
	case string:
		return writeBytes(w, tag, Tstr, []byte(m))
	case error:
		if m == nil {
			return writeBytes(w, tag, Terr, nil)
		}
		return writeBytes(w, tag, Terr, []byte(m.Error()))
	case fmt.Stringer:
		typ := Tign
		if ti, ok := m.(typer); ok {
			typ = ti.TypeId()
		}
		return writeBytes(w, tag, typ, []byte(m.String()))
	case byteser:
		typ := Tign
		if ti, ok := m.(typer); ok {
			typ = ti.TypeId()
		}
		return writeBytes(w, tag, typ, m.Bytes())
	}
	return 0, ErrDiscarded
}

func decHdr(hdr []byte) (int, uint32, uint16) {
	return int(binary.LittleEndian.Uint32(hdr[0:])),
		binary.LittleEndian.Uint32(hdr[4:]),
		binary.LittleEndian.Uint16(hdr[8:])
}

func readBytes(r io.Reader, sz int) (d []byte, err error) {
	dat := make([]byte, sz, sz)
	nr, err := io.ReadFull(r, dat)
	if err != nil && nr != sz {
		return nil, err
	}
	return dat, nil
}

// Read a message and return the number of bytes, the msg, and its tag.
// If the message is an error, it is returned in in the interface.
// Errors while reading from r are returned using the error instead.
// EOF is reported using io.EOF; but it's not an error.
func ReadMsg(r io.Reader) (n int, tag uint32, m face{}, err error) {
	var hdr [hdrSz]byte

	nr, err := io.ReadFull(r, hdr[:])
	if err != nil {
		return nr, 0, nil, err
	}
	sz, tag, typ := decHdr(hdr[:])
	if sz < 0 || sz > MaxMsgSz {
		return nr, tag, nil, ErrTooLarge
	}
	var b []byte
	if sz > 0 {
		b, err = readBytes(r, sz)
		sz += hdrSz
		if err != nil {
			return sz, tag, nil, err
		}
	} else {
		sz += hdrSz
	}
	switch typ {
	case Tbytes:
		return sz, tag, b, nil
	case Tign:
		return sz, tag, Ign(b), nil
	case Tstr:
		return sz, tag, string(b), nil
	case Terr:
		err := errors.New(string(b))
		return sz, tag, err, nil
	default:
		if mk := unpackers[typ]; mk != nil {
			m, err = mk.Unpack(b)
			return sz, tag, m, err
		}
		return sz, tag, Ign(b), nil
	}
}

// Read messages from a external reader and send them through c
// Error messages are forwarded.
// The chan is not closed, the caller may close(c, err) upon return.
func ReadMsgs(r io.Reader, c chan<- face{}) (nbytes int64, nmsgs int, err error) {
	for {
		n, _, m, rerr := ReadMsg(r)
		err = rerr
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}
		nbytes += int64(n)
		nmsgs++
		if ok := c <- m; !ok {
			err = cerror(c)
			return
		}
	}
}

// Write messages received from c through an external writer with the given tag.
// The chan is not closed, the caller may close(c, err) upon return.
// Error messages are also propagated.
// The cerror of c, if not nil, is also written as an error message.
func WriteMsgs(w io.Writer, tag uint32, c <-chan face{}) (nbytes int64, nmsgs int, err error) {
	fl, _ := w.(flusher)
	for m := range c {
		n, rerr := WriteMsg(w, tag, m)
		if rerr == ErrDiscarded {
			rerr = nil
		}
		err = rerr
		if err == nil  && fl != nil {
			err = fl.Flush()
		} 
		if err != nil {
			return
		}
		nbytes += int64(n)
		nmsgs++
	}
	err = cerror(c)
	if err != nil {
		n, _ := WriteMsg(w, tag, err)
		nbytes += int64(n)
	}
	return
}

// Read bytes from a external reader and send them as messages through c
// The chan is not closed, the caller may close(c, err) upon return.
func ReadBytes(r io.Reader, c chan<- face{}) (nbytes int64, nmsgs int, err error) {
	err = nil
	buf := make([]byte, MsgSz)
	for {
		n, rerr := r.Read(buf[0:])
		if rerr != nil {
			if rerr != io.EOF && err == nil {
				err = rerr
			}
			return
		}
		nbytes += int64(n)
		nmsgs++
		m := make([]byte, n)
		copy(m, buf[:n])
		if ok := c <- m; !ok {
			if err == nil {
				err = cerror(c)
			}
			return
		}
	}

}

// Write []byte messages to an external writer, ignoring everything else.
// Error messages are ignored (the first one is used as the return sts).
func WriteBytes(w io.Writer, c <-chan face{}) (nbytes int64, nmsgs int, err error) {
	err = nil
	for {
		m, ok := <-c
		if !ok {
			break
		}
		if e, ok := m.(error); ok {
			if err == nil {
				err = e
			}
			continue
		}
		b, ok := m.([]byte)
		if !ok {
			continue
		}
		n, werr := w.Write(b)
		nbytes += int64(n)
		nmsgs++
		if werr != nil {
			if err == nil {
				err = werr
			}
			return
		}
	}
	if err == nil {
		err = cerror(c)
	}
	return
}
