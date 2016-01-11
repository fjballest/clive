package rzx

import (
	"bytes"
	"clive/ch"
	"clive/dbg"
	"clive/zx"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type MsgId byte

const (
	Ttrees MsgId = iota + 66
	Tstat
	Tget
	Tput
	Tmove
	Tlink
	Tremove
	Tremoveall
	Twstat
	Tfind
	Tfindget
	Tend
	Tmin = Ttrees
)

struct Msg {
	Op    MsgId
	Fsys  string // All requests
	Path  string // All requests
	Off   int64  // Get, Put
	Count int64  // Get
	D     zx.Dir // Put, Wstat
	To    string // Move, Liink
	Pred  string // Find, Findget
	Spref string // Find, Findget
	Dpref string // Find, Findget
	Depth int    // Find, Findget
}

var ErrBadMsg = errors.New("bad message type")

func init() {
	ch.DefType(&Msg{})
}

func (o MsgId) String() string {
	switch o {
	case Ttrees:
		return "Ttrees"
	case Tstat:
		return "Tstat"
	case Tget:
		return "Tget"
	case Tput:
		return "Tput"
	case Tmove:
		return "Tmove"
	case Tlink:
		return "Tlink"
	case Tremove:
		return "Tremove"
	case Tremoveall:
		return "Tremoveall"
	case Tfind:
		return "Tfind"
	case Tfindget:
		return "Tfindget"
	case Twstat:
		return "Twstat"
	default:
		return fmt.Sprintf("Tunknown<%d>", o)
	}
}

func (m *Msg) WriteTo(w io.Writer) (n int64, err error) {
	if m.Op < Tmin || m.Op >= Tend {
		err = fmt.Errorf("%s %d", ErrBadMsg, m.Op)
		dbg.Warn("Msg.WriteTo: %s", err)
		return 0, err
	}
	var op [1]byte
	op[0] = byte(m.Op)
	if _, err := w.Write(op[:]); err != nil {
		return 0, err
	}
	n = 1
	if m.Op == Ttrees {
		return n, nil
	}
	nw, err := ch.WriteStringTo(w, m.Fsys)
	n += nw
	if err != nil {
		return n, err
	}
	nw, err = ch.WriteStringTo(w, m.Path)
	n += nw
	if err != nil {
		return n, err
	}
	if m.Op == Tget || m.Op == Tput {
		if err = binary.Write(w, binary.LittleEndian, uint64(m.Off)); err != nil {
			return n, err
		}
		n += 8
	}
	if m.Op == Tget {
		if err = binary.Write(w, binary.LittleEndian, uint64(m.Count)); err != nil {
			return n, err
		}
		n += 8
	}
	if m.Op == Tput || m.Op == Twstat {
		nw, err = m.D.WriteTo(w)
		n += nw
		if err != nil {
			return n, err
		}
	}
	if m.Op == Tmove || m.Op == Tlink {
		nw, err = ch.WriteStringTo(w, m.To)
		n += nw
		if err != nil {
			return n, err
		}
	}
	if m.Op == Tfind || m.Op == Tfindget {
		nw, err = ch.WriteStringTo(w, m.Pred)
		n += nw
		if err != nil {
			return n, err
		}
	}
	if m.Op == Tfind || m.Op == Tfindget {
		nw, err = ch.WriteStringTo(w, m.Spref)
		n += nw
		if err != nil {
			return n, err
		}
		nw, err = ch.WriteStringTo(w, m.Dpref)
		n += nw
		if err != nil {
			return n, err
		}
		if err = binary.Write(w, binary.LittleEndian, uint64(m.Depth)); err != nil {
			return n, err
		}
		n += 8
	}
	return n, nil
}

func (m *Msg) String() string {
	var buf bytes.Buffer
	if m == nil {
		return "<nil msg>"
	}
	if m.Op == Ttrees {
		fmt.Fprintf(&buf, "%s", m.Op)
	} else {
		fmt.Fprintf(&buf, "%s '%s' '%s'", m.Op, m.Fsys, m.Path)
	}
	if m.Op == Tget || m.Op == Tput {
		fmt.Fprintf(&buf, " off %d", m.Off)
	}
	if m.Op == Tget {
		fmt.Fprintf(&buf, " count %d", m.Count)
	}
	if m.Op == Tput || m.Op == Twstat {
		fmt.Fprintf(&buf, " d <%s> ", m.D)
	}
	if m.Op == Tmove || m.Op == Tlink {
		fmt.Fprintf(&buf, " to '%s'", m.To)
	}
	if m.Op == Tfind || m.Op == Tfindget {
		fmt.Fprintf(&buf, " pred '%s'", m.Pred)
	}
	if m.Op == Tfind || m.Op == Tfindget {
		fmt.Fprintf(&buf, " spref '%s' dpref '%s' depth %d",
			m.Spref, m.Dpref, m.Depth)
	}
	return buf.String()

}

func UnpackMsg(buf []byte) ([]byte, *Msg, error) {
	m := &Msg{}
	if len(buf) < 1 {
		return buf, nil, ch.ErrTooSmall
	}
	m.Op = MsgId(buf[0])
	if m.Op < Tmin || m.Op >= Tend {
		return buf, nil, fmt.Errorf("unknown msg type %d", buf[0])
	}
	buf = buf[1:]
	if m.Op == Ttrees {
		return buf, m, nil
	}
	var err error
	buf, m.Fsys, err = ch.UnpackString(buf)
	if err != nil {
		return buf, nil, err
	}
	buf, m.Path, err = ch.UnpackString(buf)
	if err != nil {
		return buf, nil, err
	}
	if m.Op == Tget || m.Op == Tput {
		if len(buf) < 8 {
			return buf, nil, ch.ErrTooSmall
		}
		m.Off = int64(binary.LittleEndian.Uint64(buf[0:]))
		buf = buf[8:]
	}
	if m.Op == Tget {
		if len(buf) < 8 {
			return buf, nil, ch.ErrTooSmall
		}
		m.Count = int64(binary.LittleEndian.Uint64(buf[0:]))
		buf = buf[8:]
	}
	if m.Op == Tput || m.Op == Twstat {
		buf, m.D, err = zx.UnpackDir(buf)
		if err != nil {
			return buf, nil, err
		}
	}
	if m.Op == Tmove || m.Op == Tlink {
		buf, m.To, err = ch.UnpackString(buf)
		if err != nil {
			return buf, nil, err
		}
	}
	if m.Op == Tfind || m.Op == Tfindget {
		buf, m.Pred, err = ch.UnpackString(buf)
		if err != nil {
			return buf, nil, err
		}
	}
	if m.Op == Tfind || m.Op == Tfindget {
		buf, m.Spref, err = ch.UnpackString(buf)
		if err != nil {
			return buf, nil, err
		}
		buf, m.Dpref, err = ch.UnpackString(buf)
		if err != nil {
			return buf, nil, err
		}
		if len(buf) < 8 {
			return buf, nil, ch.ErrTooSmall
		}
		m.Depth = int(binary.LittleEndian.Uint64(buf[0:]))
		buf = buf[8:]
	}
	return buf, m, nil
}

func (m *Msg) Unpack(b []byte) (face{}, error) {
	_, m, err := UnpackMsg(b)
	return m, err
}

func (m *Msg) TypeId() uint16 {
	return ch.Tzx
}
