package zx

import (
	"bytes"
	"clive/nchan"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// Someone that knows go to get to the tree for d given its protocol name.
type Dial func(d Dir) (Tree, error)

var (
	lk   sync.RWMutex
	devs = map[string]Dial{}

	trees   = map[string]Tree{}
	treeslk sync.RWMutex
)

/*
	Define a new protocol.
	(Done by the packages defining new protocols, not for you to call).
*/
func DefProto(proto string, dial Dial) {
	lk.Lock()
	defer lk.Unlock()
	devs[proto] = dial
}

func init() {
	DefProto("proc", dialproc)
}

func dialproc(d Dir) (Tree, error) {
	tpath := d["tpath"]
	if tpath == "" {
		return nil, errors.New("no tpath")
	}

	treeslk.RLock()
	defer treeslk.RUnlock()
	r := trees[tpath]
	if r != nil {
		return r, nil
	}
	return nil, errors.New("no such tree")
}

// Register a zx tree so its Dirs can dial their tree given their tpath
// within a single process by dialing the "proc" protocol.
func RegisterProcTree(t Tree, tpath string) error {
	treeslk.RLock()
	defer treeslk.RUnlock()
	r := trees[tpath]
	if r != nil {
		return errors.New("tree already registered")
	}
	trees[tpath] = t
	return nil
}

// Unregister a zx tree. See RegisterProcTree
func UnregisterProcTree(tpath string) error {
	treeslk.RLock()
	defer treeslk.RUnlock()
	r := trees[tpath]
	if r == nil {
		return errors.New("no such tree")
	}
	delete(trees, tpath)
	return nil
}

/*
	Return the tree for d, perhaps dialing a remote tree or
	building or using cached remote or local tree.
*/
func DirTree(d Dir) (Tree, error) {
	proto := d["proto"]
	lk.RLock()
	defer lk.RUnlock()
	dial, ok := devs[proto]
	if !ok {
		return nil, errors.New("no protocol")
	}
	return dial(d)
}

func RWDirTree(d Dir) (RWTree, error) {
	t, err := DirTree(d)
	if err != nil {
		return nil, err
	}
	wt, ok := t.(RWTree)
	if !ok {
		return nil, errors.New("not a RW tree")
	}
	return wt, nil
}

// Return true if both directory entries have exactly the same attributes and values.
func EqDir(d1, d2 Dir) bool {
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

// sort entries by name
func SortDirs(ds []Dir) {
	sort.Sort(byName(ds))
}

// Make of dup of the dir entry.
func (d Dir) Dup() Dir {
	nd := Dir{}
	for k, v := range d {
		nd[k] = v
	}
	return nd
}

// Is this the name of a user defined attribute (starts with upcase)
func IsUpper(name string) bool {
	r, _ := utf8.DecodeRuneInString(name)
	return unicode.IsUpper(r)
}

// Is this the name of a user settable attribute?
// (mode, mtime, size, and those starting with Upper runes but for Wuid and Sum)
func IsUsr(name string) bool {
	return IsUpper(name) && name != "Wuid" && name != "Sum" ||
		name == "mode" || name == "mtime" || name == "size"
}

// Make a dup of the dir entry with just user settable attributes:
// mode, mtime, size, and those starting with Upper runes but for Wuid
func (d Dir) UsrAttrs() Dir {
	nd := Dir{}
	for k, v := range d {
		if IsUsr(k) {
			nd[k] = v
		}
	}
	return nd
}

var uids = []string{"Uid", "Gid", "Wuid", "Sum"}

// Print d in a format suitable for testing: i.e: no mtime and just important attrs,
// excluding user attributes
func (d Dir) TestFmt() string {
	if d == nil {
		return "<nil dir>"
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "path %s name %s type %s mode %s size %s",
		d["path"], d["name"], d["type"], d["mode"], d["size"])
	return b.String()
}

// Print d in a format suitable for testing: i.e: no mtime and just important attrs,
// but include all user attributes.
func (d Dir) LongTestFmt() string {
	if d == nil {
		return "<nil dir>"
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "path %s name %s type %s mode %s size %s",
		d["path"], d["name"], d["type"], d["mode"], d["size"])
	ks := []string{}
	for _, u := range uids {
		if k := d[u]; k != "" {
			ks = append(ks, u)
		}
	}
	for k := range d {
		if k == "Uid" || k == "Gid" || k == "Wuid" || k == "Sum" || k == "Mode" {
			continue
		}
		if len(k) > 0 && k[0] >= 'A' && k[0] <= 'Z' && k != "Sum" {
			ks = append(ks, k)
		}
	}
	sort.Sort(sort.StringSlice(ks))
	for _, k := range ks {
		fmt.Fprintf(&b, " %s %s", k, d[k])
	}
	return b.String()
}

func nouid(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

const (
	KiB = 1024
	MiB = 1024 * KiB
	GiB = 1024 * MiB
	TiB = 1024 * GiB
)

func szstr(d Dir) string {
	sz := d.Int64("size")
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

// Print d in the conventional long listing format:
//	type mode size path
func (d Dir) Long() string {
	var b bytes.Buffer

	typ := d["type"]
	if typ == "" {
		fmt.Fprintf(&b, "-")
	} else {
		fmt.Fprintf(&b, "%s", typ)
	}
	mode := d.Mode()
	fmt.Fprintf(&b, "%s", os.FileMode(mode))
	fmt.Fprintf(&b, " %s", szstr(d))
	s := d["path"]
	if s == "" {
		s = d["name"]
	}
	fmt.Fprintf(&b, "  %s", s)
	return b.String()
}

// Print d in the conventional very long listing format:
//	type mode size uid gid wuid mtime path
func (d Dir) LongLong() string {
	var b bytes.Buffer

	typ := d["type"]
	if typ == "" {
		fmt.Fprintf(&b, "-")
	} else {
		fmt.Fprintf(&b, "%s", typ)
	}
	mode := d.Mode()
	fmt.Fprintf(&b, "%s", os.FileMode(mode))
	fmt.Fprintf(&b, " %s", szstr(d))
	fmt.Fprintf(&b, " %-8s ", nouid(d["Uid"]))
	fmt.Fprintf(&b, " %-8s ", nouid(d["Gid"]))
	fmt.Fprintf(&b, " %-8s ", nouid(d["Wuid"]))
	if d["mtime"] != "" {
		tm := d.Time("mtime")
		fmt.Fprintf(&b, " %s", tm.Format("2006/0102 15:04"))
	}
	s := d["path"]
	if s == "" {
		s = d["name"]
	}
	return b.String()
}

var longattrs = []string{
	"path", "name", "type", "mode", "size", "Uid", "Gid", "Wuid", "mtime",
}

/*
	For debug and printing dirs.
	Lists all attributes. Those printed in LongLong format are printed first
	in the same order and then all other attributes, sorted by name.
	This format can be parsed using ParseDirString().
*/
func (e Dir) String() string {
	var b bytes.Buffer
	sep := ""
	for _, k := range longattrs {
		if v, ok := e[k]; ok {
			fmt.Fprintf(&b, "%s%s:%q", sep, k, v)
			sep = " "
		}
	}
	nms := []string{}
	for k := range e {
		switch k {
		case "type", "mode", "size", "Uid", "Gid", "Wuid", "mtime", "path", "name":
			continue
		}
		nms = append(nms, k)
	}
	sort.Sort(sort.StringSlice(nms))
	for _, k := range nms {
		if v, ok := e[k]; ok {
			fmt.Fprintf(&b, "%s%s:%q", sep, k, v)
			sep = " "
		}
	}
	return b.String()
}

/*
	Pack the dir into a []byte that could be used later with ParseDir to
	unpack the same dir.
*/
func (e Dir) Pack() []byte {
	buf := make([]byte, 0, 100)
	var hdr [4]byte
	buf = append(buf, hdr[:]...)
	for k, v := range e {
		buf = nchan.PutString(buf, k)
		buf = nchan.PutString(buf, v)
	}
	n := uint32(len(buf) - len(hdr))
	binary.LittleEndian.PutUint32(buf[0:], n)
	return buf
}

/*
	Unpack a dir from b, returning it, the rest of the buffer, and the error status.
	An empty buffer yields Dir{} and it is ok.
*/
func UnpackDir(buf []byte) (Dir, []byte, error) {
	d := Dir{}
	var k, v string
	var err error
	if len(buf) == 0 {
		return Dir{}, buf, nil
	}
	if len(buf) < 4 {
		return nil, buf, errors.New("short buffer")
	}
	n := int(binary.LittleEndian.Uint32(buf[:4]))
	if len(buf) < n+4 {
		return nil, buf, errors.New("short buffer")
	}
	buf = buf[4:]
	b := buf[:n]
	buf = buf[n:]
	for len(b) > 0 {
		k, b, err = nchan.GetString(b)
		if err != nil {
			return nil, buf, err
		}
		v, b, err = nchan.GetString(b)
		if err != nil {
			return nil, buf, err
		}
		d[k] = v
	}
	return d, buf, nil
}

/*
	Build a directory entry from a string with the format used by
	Dir.String. For testing, and also used by nspace.
*/
func ParseDirString(s string) (Dir, int) {
	d := make(Dir, 10)
	tot := 0
	for {
		for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
			s = s[1:]
			tot++
		}
		nv := strings.SplitN(s, ":", 2)
		if len(nv) != 2 {
			return d, tot
		}
		n := nv[0]
		s = nv[1]
		for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
			tot++
			s = s[1:]
		}
		if len(s) == 0 {
			return d, tot
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
			return d, tot
		}
		d[n] = v
		tot += len(n) + 1 + i // +1 for ':'
		s = s[i:]
	}
}

// Send a single Dir through c
func (d Dir) Send(c chan<- []byte) (int, error) {
	dp := d.Pack()
	if ok := c <- dp; !ok {
		return 0, cerror(c)
	}
	return len(dp), nil
}

// Receive a single Dir from c. The Dir is sent using a single message
// with a string built by printing the entry.
func RecvDir(c <-chan []byte) (Dir, error) {
	data, ok := <-c
	if !ok || len(data) == 0 {
		return nil, cerror(c)
	}
	d, _, err := UnpackDir(data)
	return d, err
}

/*
	Does e match the attributes in d? Attributes match if their
	values match. The special value "*" matches any defined
	attribute value.
*/
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

/*
	Helper to build a Dir for a local file provided by the os package.
*/
func NewDir(fi os.FileInfo, nchild int) Dir {
	d := Dir{}
	mode := fi.Mode()
	d.SetMode(uint64(mode))
	d[Size] = strconv.FormatInt(fi.Size(), 10)
	switch {
	case fi.IsDir():
		d[Type] = "d"
		d[Size] = strconv.Itoa(nchild)
	case mode&os.ModeSymlink != 0:
		d[Type] = "l"
	case mode&(os.ModeNamedPipe|os.ModeSocket) != 0:
		d[Type] = "p"
	case mode&os.ModeDevice != 0:
		d[Type] = "c"
	default:
		d[Type] = "-"
	}
	d[Name] = fi.Name()
	d.SetTime(Mtime, fi.ModTime())
	return d
}

// Get the (int) value for an integer attribute at dir.
func (d Dir) Int(attr string) int {
	v, ok := d[attr]
	if !ok {
		return 0
	}
	n, _ := strconv.ParseInt(v, 0, 64)
	return int(n)
}

// Get the (int64) value for an integer attribute at dir.
func (d Dir) Int64(attr string) int64 {
	v, ok := d[attr]
	if !ok {
		return 0
	}
	n, _ := strconv.ParseInt(v, 0, 64)
	return n
}

// Get the (uint64) value for an integer attribute at dir.
func (d Dir) Uint64(attr string) uint64 {
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

// Get the (time) value for a time attribute at dir.
func (d Dir) Time(attr string) time.Time {
	t := d.Int64(attr)
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
	return d.Uint64("mode") & 0777
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

var zsum string

func init() {
	h := sha1.New()
	sum := h.Sum(nil)
	zsum = fmt.Sprintf("%040x", sum)
}

// return the sum for an empty file
func Zsum() string {
	return zsum
}
