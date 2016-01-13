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
	Tzx	// zx protocol msg
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
struct Ign {
	Typ uint16
	Dat []byte
}

interface Byteser {
	Bytes() []byte
}

interface Typer {
	TypeId() uint16
}

// For user defined types, implementors of this interface
// use their own make function to make values of the message type
// upon reception.
interface Unpacker {
	Typer
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

	unpackers = map[uint16]Unpacker{}
)

// Define a user type to be sent through chans
// Should be used only at init time.
func DefType(x Unpacker) {
	unpackers[x.TypeId()] = x
}

func WriteStringTo(w io.Writer, s string) (n int64, err error) {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(s))); err != nil {
		return 0, err
	}
	n = 4
	nw, err := io.WriteString(w, s)
	n += int64(nw)
	return n, err
}

func UnpackString(b []byte) ([]byte, string, error) {
	if len(b) < 4 {
		return nil, "", ErrTooSmall
	}
	sz := int(binary.LittleEndian.Uint32(b[0:]))
	b = b[4:]
	if len(b) < sz {
		return nil, "", ErrTooSmall
	}
	return b[sz:], string(b[:sz]), nil
}


func writeBytes(w io.Writer, tag uint32, typ uint16, b []byte) (int64, error) {
	var hdr [hdrSz]byte

	if b == nil {
		b = empty[:]
	}
	n := len(b)
	// do a single write, at the cost of an extra copy
	var buf bytes.Buffer
	binary.LittleEndian.PutUint32(hdr[0:], uint32(n))
	binary.LittleEndian.PutUint32(hdr[4:], tag)
	binary.LittleEndian.PutUint16(hdr[8:], typ)
	buf.Write(hdr[:])
	buf.Write(b)
	tot, err := w.Write(buf.Bytes())
	return int64(tot), err
}

// Write []byte, or Ign, string, error, Stringer, Byteser or discard the write.
// If the write is discarded, ErrDiscarded is returned.
func WriteMsg(w io.Writer, tag uint32, m face{}) (int64, error) {
	switch m := m.(type) {
	case []byte:
		return writeBytes(w, tag, Tbytes, m)
	case Ign:
		return writeBytes(w, tag, m.Typ, m.Dat)
	case string:
		return writeBytes(w, tag, Tstr, []byte(m))
	case error:
		if m == nil {
			return writeBytes(w, tag, Terr, nil)
		}
		return writeBytes(w, tag, Terr, []byte(m.Error()))
	case io.WriterTo:
		var buf bytes.Buffer
		n, err := m.WriteTo(&buf)
		if err != nil {
			return n, err
		}
		typ := Tign
		if ti, ok := m.(Typer); ok {
			typ = ti.TypeId()
		}
		return writeBytes(w, tag, typ, buf.Bytes())
	case fmt.Stringer:
		typ := Tign
		if ti, ok := m.(Typer); ok {
			typ = ti.TypeId()
		}
		return writeBytes(w, tag, typ, []byte(m.String()))
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
		return sz, tag, Ign{typ, b}, nil
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
