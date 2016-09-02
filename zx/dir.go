/*
	Clive ZX file system tools.

	The service in ZX is split in two parts: finders are used
	to find directory entries; trees are used to operate on them.
*/
package zx

import (
	"bytes"
	"clive/ch"
	"clive/u"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
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
// Attributes starting with upper-case are considered as temporary and won't be updated
// by any file system.
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
	stdAttr = map[string]bool{}
	// Preferred order for prints of std attributes
	StdAttrOrder = [...]string{
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
	stdShortOrder = [...]string{
		"type",
		"mode",
		"size",
		"path",
		"err",
	}
)

func init() {
	ch.DefType(Dir{})
	ch.DefType(Addr{})
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

// Is this the name of a temporary attribute (starts with upcase)
func IsTemp(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

// Make a dup of the dir entry w/o temporary attributes
func (d Dir) SysDup() Dir {
	nd := Dir{}
	for k, v := range d {
		if !IsTemp(k) {
			nd[k] = v
		}
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
// the addr attribute is ignored.
func EqualDirs(d1, d2 Dir) bool {
	if len(d1) != len(d2) {
		return false
	}
	for k, v := range d1 {
		if k != "addr" && d2[k] != v {
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
	m &= 0777
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

func nouid(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

// Print d in a format suitable for keeping a db of file metadata.
func (d Dir) DbFmt() string {
	var b bytes.Buffer

	fmt.Fprintf(&b, "%-14s", d["path"])
	typ := d["type"]
	if typ == "" {
		fmt.Fprintf(&b, " -")
	} else {
		fmt.Fprintf(&b, " %s", typ)
	}
	if d["rm"] != "" {
		fmt.Fprintf(&b, " GONE")
	} else {
		fmt.Fprintf(&b, " 0%o", d.Mode())
	}
	uid := nouid(d["Uid"])
	gid := nouid(d["Gid"])
	wuid := nouid(d["Wuid"])
	fmt.Fprintf(&b, " %-8s %-8s %-8s", uid, gid, wuid)
	fmt.Fprintf(&b, " %8d", d.Uint("size"))
	if d["type"] != "d" {
		fmt.Fprintf(&b, " %d", d.Uint("mtime"))
	}
	if d["err"] != "" {
		fmt.Fprintf(&b, " %s", d["err"])
	}
	return b.String()
}

// Return a string that can be parsed later.
func (d Dir) String() string {
	return d.fmt(d.Attrs(), true)
}

func (d Dir) WriteTo(w io.Writer) (n int64, err error) {
	if err := binary.Write(w, binary.LittleEndian, uint32(len(d))); err != nil {
		return 0, err
	}
	n = 4
	for k, v := range d {
		nw, err := ch.WriteStringTo(w, k)
		n += nw
		if err != nil {
			return n, err
		}
		nw, err = ch.WriteStringTo(w, v)
		n += nw
		if err != nil {
			return n, err
		}
	}
	return n, err
}

func (d Dir) Bytes() []byte {
	var buf bytes.Buffer
	d.WriteTo(&buf)
	return buf.Bytes()
}

func UnpackDir(b []byte) ([]byte, Dir, error) {
	if len(b) < 4 {
		return b, nil, ch.ErrTooSmall
	}
	d := map[string]string{}
	n := int(binary.LittleEndian.Uint32(b[0:]))
	if n < 0 || n > ch.MaxMsgSz {
		return b, nil, ch.ErrTooLarge
	}
	b = b[4:]
	var err error
	var k, v string
	for i := 0; i < n; i++ {
		b, k, err = ch.UnpackString(b)
		if err != nil {
			return b, nil, err
		}
		b, v, err = ch.UnpackString(b)
		if err != nil {
			return b, nil, err
		}
		d[k] = v
	}
	return b, d, nil
}

func (d Dir) TypeId() uint16 {
	return ch.Tdir
}

func (d Dir) Unpack(b []byte) (face{}, error) {
	_, d, err := UnpackDir(b)
	return d, err
}

func (a Addr) TypeId() uint16 {
	return ch.Taddr
}

func parseDot(s string) (int, int) {
	if s == "" {
		return 0, 0
	}
	els := strings.SplitN(s, ",", 2)
	if els[0][0] == '#' {
		els[0] = els[0][1:]
	}
	p0, _ := strconv.Atoi(els[0])
	if len(els) == 1 {
		return p0, p0
	}
	if els[1][0] == '#' {
		els[1] = els[1][1:]
	}
	p1, _ := strconv.Atoi(els[1])
	return p0, p1
}

func parseLns(s string) (int, int) {
	if s == "" {
		return 0, 0
	}
	els := strings.SplitN(s, ",", 2)
	p0, _ := strconv.Atoi(els[0])
	if len(els) == 1 {
		return p0, p0
	}
	p1, _ := strconv.Atoi(els[1])
	return p0, p1
}

func ParseAddr(s string) Addr {
	var a Addr
	els := strings.Split(s, ":")
	if len(els) == 0 {
		return a
	}
	a.Name = els[0]
	for _, addr := range els[1:] {
		if len(addr) > 0 && addr[0] == '#' {
			a.P0, a.P1 = parseDot(addr)
			continue
		}
		if len(addr) > 0 {
			a.Ln0, a.Ln1 = parseLns(addr)
		}
	}
	return a
}

func (a Addr) String() string {
	if a.Name == "" {
		a.Name = "in"
	}
	addr := a.Name
	if a.Ln0 != 0 || a.Ln1 != 0 {
		if a.Ln0 == a.Ln1 {
			addr += fmt.Sprintf(":%d", a.Ln0)
		} else {
			addr += fmt.Sprintf(":%d,%d", a.Ln0, a.Ln1)
		}
		if a.P0 == 0 && a.P1 == 0 {
			return addr
		}
	}
	return fmt.Sprintf("%s:#%d,#%d", addr, a.P0, a.P1)
}

func UnpackAddr(b []byte) ([]byte, Addr, error) {
	var a Addr
	var err error
	b, a.Name, err = ch.UnpackString(b)
	if err != nil {
		return b, a, err
	}
	if len(b) < 4*4 {
		return b, a, ch.ErrTooSmall
	}
	a.Ln0 = int(binary.LittleEndian.Uint32(b[0:]))
	a.Ln1 = int(binary.LittleEndian.Uint32(b[4:]))
	a.P0 = int(binary.LittleEndian.Uint32(b[8:]))
	a.P1 = int(binary.LittleEndian.Uint32(b[12:]))
	b = b[16:]
	return b, a, nil
}

func (a Addr) Unpack(b []byte) (face{}, error) {
	_, a, err := UnpackAddr(b)
	return a, err
}

func (a Addr) WriteTo(w io.Writer) (n int64, err error) {
	n, err = ch.WriteStringTo(w, a.Name)
	if err != nil {
		return n, err
	}
	if err := binary.Write(w, binary.LittleEndian, uint32(a.Ln0)); err != nil {
		return n, err
	}
	n += 4
	if err := binary.Write(w, binary.LittleEndian, uint32(a.Ln1)); err != nil {
		return n, err
	}
	n += 4
	if err := binary.Write(w, binary.LittleEndian, uint32(a.P0)); err != nil {
		return n, err
	}
	n += 4
	if err := binary.Write(w, binary.LittleEndian, uint32(a.P1)); err != nil {
		return n, err
	}
	n += 4
	return n, nil
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

// Parse a string as produced by Dir.String() and return the dir
func ParseDir(s string) (Dir, error) {
	d := make(Dir, 10)
	tot := 0
	for {
		for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
			s = s[1:]
			tot++
		}
		if len(s) == 0 {
			return d, nil
		}
		nv := strings.SplitN(s, ":", 2)
		if len(nv) != 2 {
			return d, errors.New("missing ':' in dir string")
		}
		n := nv[0]
		s = nv[1]
		for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
			tot++
			s = s[1:]
		}
		if len(s) == 0 {
			return d, nil
		}
		quoted := s[0] == '"'
		p0 := 0
		i := 1
		for i < len(s) {
			r, n := utf8.DecodeRuneInString(s[i:])
			if !quoted {
				if r == ' ' || r == '\t' || r == '\n' {
					i++
					break
				}
				i += n
				continue
			}
			if r == '\\' {
				if i+1 < len(s) && s[i+1] == '"' {
					i += 2
					continue
				}
			} else if r == '"' {
				i++
				break
			}
			i += n
		}
		if !quoted {
			v := s[p0:i]
			if i < len(s) {
				v = s[p0 : i-1] // remove blank
			}
			d[n] = v
			tot += len(n) + 1 + i
			s = s[i:]
			continue
		}
		v, err := strconv.Unquote(s[p0:i])
		if err != nil {
			return d, err
		}
		d[n] = v
		tot += len(n) + 1 + i // +1 for ':'
		s = s[i:]
	}
}

// Return the server path for the resource
// Returns the path if there's no server path
func (d Dir) SPath() string {
	a := d["addr"]
	n := strings.LastIndexByte(a, '!')
	if n < 0 {
		return d["path"]
	}
	return a[n+1:]
}

// Return the address for the resource server, including the protocol.
// Returns "" if no address is set.
func (d Dir) SAddr() string {
	a := d["addr"]
	n := strings.LastIndexByte(a, '!')
	if n < 0 {
		return ""
	}
	return a[:n]
}

// Return the protocol to reach the resource,
// return "" if no address is set.
func (d Dir) Proto() string {
	a := d["addr"]
	n := strings.IndexByte(a, '!')
	if n < 0 {
		return ""
	}
	return a[:n]
}

// Return true if this entry refers to a server supporting the
// finder protocol
func (d Dir) IsFinder() bool {
	p := d.Proto()
	switch p {
	case "lfs", "zxc", "zx", "finder":
		return true
	default:
		return false
	}
}
