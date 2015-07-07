package rfs

import (
	"bytes"
	"clive/dbg"
	"clive/nchan"
	"clive/zx"
	"encoding/binary"
	"errors"
	"fmt"
)

type MsgId byte

const (
	Tstat MsgId = iota + 66
	Tget
	Tput
	Tmkdir
	Tmove
	Tremove
	Tremoveall
	Twstat
	Tfind
	Tfindget
	Tfsys
	Tend
	Tmin = Tstat
)

/*
	Messages used in the zx protocol used to talk to a remote ns.Finder/zx.RWTree.
*/
type Msg  {
	Op         MsgId
	Rid        string // All requests (but for Move)
	Off, Count int64  // Get
	D          zx.Dir // Put, Mkdir, Wstat
	To         string // Move
	Pred       string // Find, Get, Put, Findget
	Depth      int    // Find, Findget
	Spref      string // Find, Findget
	Dpref      string // Find, Findget
}

func (o MsgId) String() string {
	switch o {
	case Tstat:
		return "Tstat"
	case Tget:
		return "Tget"
	case Tput:
		return "Tput"
	case Tmkdir:
		return "Tmkdir"
	case Tmove:
		return "Tmove"
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
	case Tfsys:
		return "Tfsys"
	default:
		return fmt.Sprintf("Tunknown<%d>", o)
	}
}

func (m *Msg) Pack() []byte {
	buf := make([]byte, 0, 100)
	var n [8]byte

	if m.Op<Tmin || m.Op>=Tend {
		dbg.Fatal("unknown msg type %d", m.Op)
	}
	buf = append(buf, byte(m.Op))
	buf = nchan.PutString(buf, m.Rid)
	if m.Op==Tget || m.Op==Tput {
		binary.LittleEndian.PutUint64(n[0:], uint64(m.Off))
		buf = append(buf, n[:8]...)
	}
	if m.Op == Tget {
		binary.LittleEndian.PutUint64(n[0:], uint64(m.Count))
		buf = append(buf, n[:8]...)
	}
	if m.Op==Tput || m.Op==Tmkdir || m.Op==Twstat {
		buf = append(buf, m.D.Pack()...)
	}
	if m.Op == Tmove {
		buf = nchan.PutString(buf, m.To)
	}
	if m.Op==Tfind || m.Op==Tget || m.Op==Tput || m.Op==Tfindget {
		buf = nchan.PutString(buf, m.Pred)
	}
	if m.Op==Tfind || m.Op==Tfindget {
		buf = nchan.PutString(buf, m.Spref)
		buf = nchan.PutString(buf, m.Dpref)
		binary.LittleEndian.PutUint64(n[0:], uint64(m.Depth))
		buf = append(buf, n[:8]...)
	}
	return buf
}

func (m *Msg) String() string {
	var buf bytes.Buffer
	if m == nil {
		return "<nil msg>"
	}
	fmt.Fprintf(&buf, "%s rid '%s'", m.Op, m.Rid)
	if m.Op==Tget || m.Op==Tput {
		fmt.Fprintf(&buf, " off %d", m.Off)
	}
	if m.Op == Tget {
		fmt.Fprintf(&buf, " count %d", m.Count)
	}
	if m.Op==Tput || m.Op==Tmkdir || m.Op==Twstat {
		fmt.Fprintf(&buf, " stat <%s> ", m.D)
	}
	if m.Op == Tmove {
		fmt.Fprintf(&buf, " to '%s'", m.To)
	}
	if m.Op==Tfind || m.Op==Tget || m.Op==Tput || m.Op==Tfindget {
		fmt.Fprintf(&buf, " pred '%s'", m.Pred)
	}
	if m.Op==Tfind || m.Op==Tfindget {
		fmt.Fprintf(&buf, " spref '%s' dpref '%s' depth %d", m.Spref, m.Dpref, m.Depth)
	}
	return buf.String()

}

func UnpackMsg(buf []byte) (*Msg, error) {
	m := &Msg{}

	if len(buf) < 1 {
		return nil, errors.New("short msg")
	}
	m.Op = MsgId(buf[0])
	if m.Op<Tmin || m.Op>=Tend {
		return nil, fmt.Errorf("unknown msg type %d", buf[0])
	}
	buf = buf[1:]
	var err error
	m.Rid, buf, err = nchan.GetString(buf)
	if err != nil {
		return nil, err
	}
	if m.Op==Tget || m.Op==Tput {
		if len(buf) < 8 {
			return nil, errors.New("short msg")
		}
		m.Off = int64(binary.LittleEndian.Uint64(buf[0:]))
		buf = buf[8:]
	}
	if m.Op == Tget {
		if len(buf) < 8 {
			return nil, errors.New("short msg")
		}
		m.Count = int64(binary.LittleEndian.Uint64(buf[0:]))
		buf = buf[8:]
	}
	if m.Op==Tput || m.Op==Tmkdir || m.Op==Twstat {
		m.D, buf, err = zx.UnpackDir(buf)
		if err != nil {
			return nil, err
		}
	}
	if m.Op == Tmove {
		m.To, buf, err = nchan.GetString(buf)
		if err != nil {
			return nil, err
		}
	}
	if m.Op==Tfind || m.Op==Tget || m.Op==Tput || m.Op==Tfindget {
		m.Pred, buf, err = nchan.GetString(buf)
		if err != nil {
			return nil, err
		}
	}
	if m.Op==Tfind || m.Op==Tfindget {
		m.Spref, buf, err = nchan.GetString(buf)
		if err != nil {
			return nil, err
		}
		m.Dpref, buf, err = nchan.GetString(buf)
		if err != nil {
			return nil, err
		}
		if len(buf) < 8 {
			return nil, errors.New("short msg")
		}
		m.Depth = int(binary.LittleEndian.Uint64(buf[0:]))
		buf = buf[8:]
	}
	return m, nil
}
