/*
	Clive ZX file system tools.

	The service in ZX is split in two parts: finders are used
	to find directory entries; trees are used to operate on them.
*/
package zx

import (
	"strconv"
	"time"
	"sort"
	"bytes"
	"fmt"
	"encoding/binary"
	"clive/u"
	"clive/ch"
)

const (
	KiB = 1024
	MiB = 1024 * KiB
	GiB = 1024 * MiB
	TiB = 1024 * GiB
)

// A Dir, or directory entry, identifices a file or a resource in the system.
// It is a set of attribute/value pairs, including some conventional attributes
// like "name", "size", etc.
//
// Directory entries are self-describing in many cases, and include the address
// and resource path as known by the server as extra attributes. Thus, programs can
// operate on streams of Dir entries and ask each entry to perform an operation on the
// resource it described.
//
// The purpose of very important interfaces in the system, like ns.Finder and
// zx.Tree is to operate on Dirs.
type Dir map[string]string

// File addresses as handled by some commands
struct Addr {
	Name     string // file or resource name
	Ln0, Ln1 int    // line range or zero
	P0, P1   int    // point (rune) range or zero
}

var (
	// Standard attributes present in most dirs.
	stdAttr = map[string]bool {}
	// Preferred order for prints of std attributes
	StdAttrOrder = [...]string {
		"name",
		"type",
		"mode",
		"size",
		"mtime",
		"uid",
		"gid",
		"wuid",
		"path",
		"addr",
		"err",
	}
	stdShortOrder = [...] string {
		"type",
		"mode",
		"size",
		"path",
		"err",
	}
)

func init() {
	for _, a := range StdAttrOrder {
		stdAttr[a] = true
	}
}

func IsStd(attr string) bool {
	return stdAttr[attr]
}

// Make a dup of the dir entry.
func (d Dir) Dup() Dir {
	nd := Dir{}
	for k, v := range d {
		nd[k] = v
	}
	return nd
}

// Get the value for an integer attribute at dir.
// If the attribute is mode, base 8 is used.
func (d Dir) Uint(attr string) uint64 {
	v, ok := d[attr]
	if !ok {
		return 0
	}
	b := 0
	if attr == "mode" {
		b = 8
	}
	n, _ := strconv.ParseUint(v, b, 64)
	return n
}

// Get the size attribute
func (d Dir) Size() int64 {
	return int64(d.Uint("size"))
}

// Set the size attribute
func (d Dir) SetSize(size int64) {
	if size < 0 {
		size = 0
	}
	d["size"] = strconv.FormatInt(size, 10)
}

// Set the int value cleanly formatted
func (d Dir) SetUint(name string, v uint64) {
	d[name] = strconv.FormatUint(v, 10)
}

// Get the (time) value for a time attribute at dir.
func (d Dir) Time(attr string) time.Time {
	t := int64(d.Uint(attr))
	return time.Unix(t/1e9, t%1e9)
}

// Set a time attribute
func (d Dir) SetTime(name string, t time.Time) {
	d[name] = strconv.FormatInt(t.UnixNano(), 10)
}

// Set a mode attribute (only bits 0777)
func (d Dir) SetMode(mode uint64) {
	d["mode"] = "0" + strconv.FormatUint(mode&0777, 8)
}

// Return mode bits (only 0777)
func (d Dir) Mode() uint64 {
	return d.Uint("mode") & 0777
}

// Adjust mode bits to inherit group bits cleared/set from the parent
func (d Dir) Inherit(parent uint64) {
	mode := d.Mode()
	mode &= parent
	if mode&0440 == 0400 && parent&040 == 040 {
		mode |= 040
	}
	if mode&0220 == 0200 && parent&020 == 020 {
		mode |= 020
	}
	if d["type"] == "d" {
		if mode&0110 == 0100 && parent&010 == 010 {
			mode |= 010
		}
	}
	d.SetMode(mode)
}

// Return true if both directory entries have exactly the same attributes and values.
func Equals(d1, d2 Dir) bool {
	if len(d1) != len(d2) {
		return false
	}
	for k, v := range d1 {
		if d2[k] != v {
			return false
		}
	}
	return true
}

type byName []Dir

func (ds byName) Len() int           { return len(ds) }
func (ds byName) Less(i, j int) bool { return ds[i]["name"] < ds[j]["name"] }
func (ds byName) Swap(i, j int)      { ds[i], ds[j] = ds[j], ds[i] }

// Sort dir entries by name
func SortDirs(ds []Dir) {
	sort.Sort(byName(ds))
}

func szstr(sz uint64) string {
	if sz < KiB {
		return fmt.Sprintf("%6d", sz)
	}
	var u rune
	div := int64(1)
	switch {
	case sz >= GiB:
		div = GiB
		u = 'G'
	case sz >= MiB:
		div = MiB
		u = 'M'
	default:
		div = KiB
		u = 'k'
	}
	return fmt.Sprintf("%5.1f%c", float64(sz)/float64(div), u)
}

// This is stolen from go/src/os/types.go
func modeString(m uint64) string {
	var buf [32]byte // Mode is uint32.
	const rwx = "rwxrwxrwx"
	m &=0777
	w := 0
	for i, c := range rwx {
		if m&(1<<uint(9-1-i)) != 0 {
			buf[w] = byte(c)
		} else {
			buf[w] = '-'
		}
		w++
	}
	return string(buf[:w])
}

func (d Dir) fmt(attrs []string, quoteall bool) string {
	if d == nil {
		return "<nil dir>"
	}
	var b bytes.Buffer
	sep := ""
	for _, a := range attrs {
		v := d[a]
		if IsStd(a) && !quoteall {
			switch a {
			case "size":
				n, _ := strconv.ParseUint(v, 10, 64)
				v = szstr(n)
			case "mode":
				n, _ := strconv.ParseUint(v, 8, 64)
				v = modeString(n)
			case "mtime":
				v = fmt.Sprintf("%12s", v)
			case "name":
				if d["path"] != "" {
					continue
				}
			case "uid", "gid", "wuid":
				v = fmt.Sprintf("%6s", v)
			case "addr":
				continue
			case "err":
				if v == "" {
					continue
				}
			}
			fmt.Fprintf(&b, "%s%s", sep, v)
		} else {
			fmt.Fprintf(&b, "%s%s:%q", sep, a, v)
		}
		sep = " "
	}
	return b.String()
}

// Return the set of attributes with values, sorted in std order.
func (d Dir) Attrs() []string {
	ss := make([]string, 0, len(d))
	for _, k := range StdAttrOrder {
		if _, ok := d[k]; ok {
			ss = append(ss, k)
		}
	}
	us := make([]string, 0, len(d))
	for k := range d {
		if !IsStd(k) {
			us = append(us, k)
		}
	}
	sort.Sort(sort.StringSlice(us))
	return append(ss, us...)
	
}

// Print d in test format.
// All values are quoted, mtime is removed.
// u.Uid is replaced with "elf" in all uids
func (d Dir) TestFmt() string {
	nd := d.Dup()
	for _, usr := range []string{"uid", "gid", "wuid"} {
		if nd[usr] == u.Uid {
			nd[usr] = "elf"
		}
	}
	delete(nd, "mtime")
	return nd.fmt(nd.Attrs(), true)
}

// Print d in std format
func (d Dir) Fmt() string {
	return d.fmt(stdShortOrder[:], false)
}

// Print d in long std format
func (d Dir) LongFmt() string {
	return d.fmt(d.Attrs(), false)
}

// Return a string that can be parsed later.
func (d Dir) String() string {
	return d.fmt(d.Attrs(), true)
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

func UnpackDir(b []byte) (Dir, []byte, error) {
	if len(b) < 4 {
		return nil, b, ch.ErrTooSmall
	}
	d := map[string]string{}
	n := int(binary.LittleEndian.Uint32(b[0:]))
	if n < 0 || n > ch.MaxMsgSz {
		return nil, b, ch.ErrTooLarge
	}
	b = b[4:]
	var err error
	var k, v string
	for i := 0; i < n; i++ {
		b, k, err = ch.UnpackString(b)
		if err != nil {
			return nil, b, err
		}
		b, v, err = ch.UnpackString(b)
		if err != nil {
			return nil, b, err
		}
		d[k] = v
	}
	return d, b, nil
}

func (d Dir) Unpack(b []byte) (face{}, error) {
	d, _, err := UnpackDir(b)
	return d, err
}


// Does e match the attributes in d? Attributes match if their
// values match. The special value "*" matches any defined
// attribute value.
func (e Dir) Matches(d Dir) bool {
	if d == nil {
		return true
	}
	for k, v := range d {
		dv, ok := e[k]
		if !ok || dv == "" {
			if v != "" {
				return false
			}
		}
		if dv != v && v != "*" {
			return false
		}
	}
	return true
}
