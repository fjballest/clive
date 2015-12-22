package main

import (
	"bufio"
	"clive/cmd/picky/gx"
	"clive/cmd/picky/paminstr"
	"clive/cmd/picky/pbytes"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	u32sz = 4

	rMode = 1 << iota
	wMode
)

type Xfile struct {
	b     *bufio.ReadWriter
	bfd   *os.File
	eof   bool
	eol   bool
	err   string
	mode  int
	fname string

	graph *gx.Graphics
}

type Vmem struct {
	tid  int
	elem []byte
}

type Ptr struct {
	tid  uint
	pc   uint32
	p    []byte
	next *Ptr
	idx  int //TODO, array of free ptrs.
}

//
// File I/O is a clear departure from the Pascal heritage.
//
// read(char) may be used to do (buffered) i/o one char at a time.
// Special constants Eol and Eof may be obtained at end of line/file.
// Once Eol is found, it must be read using readeol().
// Eol may not  be written. writeeol() must be used instead.
//
// further calls to read(char) at Eof (after it has been returned once)
// are a fault.
//
// Functions eof() and eol() report if the file is into eol/eof conditions.
// That is, before reading, eof() would report False even if there's
// nothing else to read.
//
// There are two advantages for this scheme:
//	- it works fine for both streams and actual files.
//	- text files are portable
//	(in windows, '\r' may trigger the Eol condition and readeol() may
//	 read the '\n' after it.; in unix, readeol() does nothing.)
//	EOL contains the end of line string ("\n" or "\r\n"), code assumes
//	it has one or two characters.
//

const (
	AIncr = 32
	FD0   = "/fd/0"
	FD1   = "/fd/1"
)

var (
	files []*Xfile
	ptrs  *Ptr
	nheap int
)

func (f *Xfile) isgraphic() bool {
	return f.graph != nil
}

//flush all files, except graphical ones
func flushall() {
	for i := range files {
		f := files[i]
		if f != nil && f.b != nil && (f.mode&wMode) != 0 && !f.isgraphic() {
			f.b.Flush()
		}
	}
}

func xbopen(name string, m string, g *gx.Graphics) int {
	var (
		i   int
		bfd *os.File
		err error
	)
	nfiles := len(files)
	for i = 0; i < nfiles; i++ {
		if files[i] == nil {
			break
		}
	}
	if i == nfiles {
		files = append(files, make([]*Xfile, AIncr)...)
	}
	fmode := os.O_RDONLY
	mode := 0
	switch m {
	case "rw", "wr":
		if g == nil {
			panic("non-graphx 'rw' not supported in this implementation.")
		}
		mode = rMode | wMode
	case "w":
		fmode = os.O_WRONLY | os.O_CREATE
		mode = wMode
	case "r":
		fmode = os.O_RDONLY
		mode = rMode
	}
	files[i] = &Xfile{eof: false, eol: false, err: "", mode: mode, fname: name, graph: nil}
	if g == nil {
		switch name {
		case FD0:
			bfd = os.Stdin
		case FD1:
			bfd = os.Stdout
		default:
			bfd, err = os.OpenFile(name, fmode, 0600)
			if err != nil {
				errs := fmt.Sprintf("open: '%s': %v", name, err)
				panic(errs)
			}
		}
		br := bufio.NewReader(bfd)
		bw := bufio.NewWriter(bfd)
		files[i].bfd = bfd
		files[i].b = bufio.NewReadWriter(br, bw)
	} else {
		files[i].graph = g

	}
	return i
}

func fnsinit() {
	xbopen(FD0, "r", nil)
	xbopen(FD1, "w", nil)
}

func xbfile(i int) (f *Xfile, sc io.RuneScanner) {
	if i > len(files) || (files[i].b == nil && !files[i].isgraphic()) {
		panic("file not open")
	}
	f = files[i]
	if f.isgraphic() {
		sc = f.graph
	} else {
		sc = f.b
	}
	return f, sc
}

func xacos() {
	r := popr()
	if r < -1-paminstr.Eps || r > 1+paminstr.Eps {
		panic("acos argument out of [-1.0, 1.0]")
	}
	pushr(math.Acos(r))
}

func xasin() {
	r := popr()
	if r < -1-paminstr.Eps || r > 1+paminstr.Eps {
		panic("asin argument out of [-1.0, 1.0]")
	}
	pushr(math.Asin(r))
}

func xatan() {
	r := popr()
	if r < -math.Pi/2+paminstr.Eps || r > math.Pi/2-paminstr.Eps {
		panic("atan argument out of [-Pi/2, Pi/2]")
	}
	pushr(math.Atan(r))
}

func xcos() {
	pushr(math.Cos(popr()))
}

func xexp() {
	r := popr()
	if r < paminstr.Eps {
		panic("value out of domain of cos")
	}
	pushr(math.Exp(r))
}

func xlog() {
	r := popr()
	if r < 0.0 {
		panic("log argument < 0")
	}
	pushr(math.Log(r))
}

func xlog10() {
	r := popr()
	if r < 0.0 {
		panic("log10 argument < 0")
	}
	pushr(math.Log10(r))
}

func xpow() {
	r1 := popr()
	r2 := popr()
	pushr(math.Pow(r1, r2))
}

func xsin() {
	pushr(math.Sin(popr()))
}

func xsqrt() {
	r := popr()
	if r < 0.0 {
		panic("sqrt argument < 0")
	}
	pushr(math.Sqrt(r))
}

func xtan() {
	r := popr()
	if r < paminstr.Eps {
		panic("value out of domain of tan")
	}
	pushr(math.Tan(r))
}

func xnew() {
	t := tfetch(int(pop32()))
	sz := mabs.tents[t.etid].sz
	p := popduaddr()
	pt := new(Ptr)
	pt.pc = mst.pc
	pt.tid = t.etid
	xp := make([]byte, sz)
	if xp == nil {
		panic("no more memory")
	}
	poison(xp)
	pt.p = xp
	nheap += int(sz)
	pt.next = ptrs
	ptrs = pt
	Ptrptr(pt, p)
	if debug['M'] != 0 {
		fmt.Fprintf(os.Stderr, "newptr: Ptr %v\n", pt)
	}
}

func (pt *Ptr) String() string {
	s := fmt.Sprintf("tid: %#x, pc: %#x, p: ", pt.tid, pt.pc)
	for _, v := range pt.p {
		s += fmt.Sprintf("%#x, ", v)
	}
	return s
}

func xdispose() {
	var (
		pt *Ptr
	)
	pp := popduaddr()
	if pp&uintptr(1) != 0 {
		panic("dispose of a not initialized pointer")
	}
	if pp == uintptr(0) {
		panic("dispose of a dangling pointer")
	}
	pt = ptrPtr(pp)
	if pt.p == nil {
		panic("memory already disposed")
	}
	if debug['M'] != 0 {
		fmt.Fprintf(os.Stderr, "freeptr: Ptr %v\n", pt)
	}
	pt.p = nil

	if int(pt.tid) > len(mabs.tents) {
		panic("memory already disposed ??")
	}
	nheap -= int(mabs.tents[pt.tid].sz)
}

func xptr(pi interface{}) *byte {
	pt := pi.(*Ptr)
	if pt == nil {
		panic("dereferencing a nil pointer")
	}
	if pt == Uninit {
		panic("dereferencing a not initialized pointer")
	}
	if debug['M'] != 0 {
		fmt.Fprintf(os.Stderr, "xptr: Ptr %v\n", pt)
	}
	if pt.p == nil {
		panic("attempt to use disposed memory")
	}
	return &pt.p[0]
}

func undisposed() {
	var pe, leaks *Pc
	once := 0
	for ; ptrs != nil; ptrs = ptrs.next {
		if ptrs.p != nil {
			if once == 0 {
				fmt.Fprintf(os.Stderr, "memory leaks:\n")
			}
			once++
			pe = mabs.findpc(ptrs.pc)
			if pe == nil {
				fmt.Fprintf(os.Stderr, "pc %#x\n", ptrs.pc)
			} else {
				if pe.n == 0 {
					pe.next = leaks
					leaks = pe
				}
				pe.n++
			}
		}
	}
	for pe = leaks; pe != nil; pe = pe.next {
		fmt.Fprintf(os.Stderr, "%s:%d\t(%d times)\n", pe.fname, pe.lineno, pe.n)
	}
}

func xfatal() {
	flushall()
	t := tfetch(int(pop32()))
	cp := popn(int(t.sz))
	fmt.Fprintf(os.Stderr, "fatal: %s\n", cp)
	done(cp)
}

var randsrc *rand.Rand

func xrand() {
	n := int(pop32())
	if n <= 0 || n > paminstr.Maxint {
		panic("rand: n should be in (0, Maxint] ")
	}
	i := randsrc.Intn(n)
	ip := popslice(u32sz)
	err := pbytes.MarshalBinary(ip, i)
	if err != nil {
		panic("rand marshal")
	}
}

/* some windows propagate C-z instead of sending EOF, agh */
func iseof(c rune, err error) bool {
	return (c == 0x1a && runtime.GOOS == "windows") || err == io.EOF
}

func xfpeek() {
	var (
		c   rune
		err error
	)
	fid := pop32()
	f, sc := xbfile(int(fid))
	if !f.isgraphic() {
		flushall()
	}
	cp := popslice(u32sz)
	if f.eof {
		panic("peek: eof met")
	}
	if f.eol {
		c = rune(paminstr.EOL[0])
	} else {
		c, _, err = sc.ReadRune()
		if iseof(c, err) {
			c = 0xFF
			f.eof = true
		} else if err != nil {
			panic("fpeek: bad file")
		} else {
			if c == rune(paminstr.EOL[0]) {
				f.eol = true
			} else {
				sc.UnreadRune()
			}
		}
	}
	err = pbytes.MarshalBinary(cp, c)
	if err != nil {
		panic("xfpeek marshal")
	}
	if debug['I'] != 0 {
		fmt.Fprintf(os.Stderr, "fpeek: c '%c'[%x] file %d: eol %veof %v\n",
			c, c, fid, f.eol, f.eof)
	}
}

func readword(f *Xfile, isnum bool) string {
	var (
		barr [512]rune
		sp   int
		c    rune
		err  error
		sc   io.RuneScanner
	)
	if f.isgraphic() {
		sc = f.graph
	} else {
		sc = f.b
	}
	buf := barr[0:0]
	sp = 0
	for {
		c, _, err = sc.ReadRune()
		if err != nil || !unicode.IsSpace(c) {
			break
		}
	}
	for {
		if err != nil && sp == 0 {
			panic("read: eof met")
		}
		buf = append(buf, c)
		sp++
		c, _, err = sc.ReadRune()
		if err != nil || (isnum && !strings.ContainsRune("0123456789+-eE.", c)) {
			break
		}
		if err != nil || unicode.IsSpace(c) {
			break
		}
	}
	if err != nil {
		if iseof(c, err) {
			f.eof = true
		}
	} else {
		f.eol = (c == rune(paminstr.EOL[0]))
		if !f.eol {
			sc.UnreadRune()
		}
	}
	if len(buf) == 0 {
		panic("read: eof met")
	}
	return string(buf)
}

const p32sz = 4

func _xfread(tid int, f *Xfile) {
	var sc io.RuneScanner
	if f.isgraphic() {
		sc = f.graph
	} else {
		sc = f.b
		flushall()
	}
	if f.eof {
		panic("read: eof met")
	}
	if (f.mode & rMode) == 0 {
		panic("read: file not open for reading")
	}
	switch mabs.tents[tid].fmt {
	case 'i', 'u', 'h':
		d := popslice(int(mabs.tents[tid].sz))
		s := readword(f, true)
		n, err := strconv.ParseInt(s, 0, 64)
		if err != nil {
			panic("read: no int value found")
		}

		if int(n) < mabs.tents[tid].first || int(n) > mabs.tents[tid].last {
			panic("read: value is out of range")
		}
		err = pbytes.MarshalBinary(d, int(n))
		if err != nil {
			panic("_xfread marshal")
		}
	case 'e':
		d := popslice(mabs.tents[tid].nitems * p32sz)
		s := readword(f, false)
		for i := 0; i < mabs.tents[tid].nitems; i++ {
			if strings.EqualFold(s, mabs.tents[tid].lits[i]) {
				err := pbytes.MarshalBinary(d, mabs.tents[tid].first+i)
				if err != nil {
					panic("_xfread marshal")
				}
				return
			}
		}
		panic("read: no enumerated value found")
	case 'c':
		d := popslice(int(mabs.tents[tid].sz))
		if f.eol {
			panic("read: at end of line")
		}
		n, _, err := sc.ReadRune()
		if err != nil {
			if iseof(n, err) {
				n = 0xFF
				f.eof = true
				f.eol = false
			} else {
				panic("read error")
			}
		} else {
			f.eol = n == rune(paminstr.EOL[0])
			if int(n) < mabs.tents[tid].first || int(n) > mabs.tents[tid].last {
				panic("read: value is out of range")
			}
		}
		err = pbytes.MarshalBinary(d, n)
		if err != nil {
			panic("_xfread marshal")
		}
	case 'b':
		d := popslice(int(mabs.tents[tid].sz))
		s := readword(f, false)
		n := 0
		if strings.EqualFold(s, "True") {
			n = 1
		} else if !strings.EqualFold(s, "False") {
			panic("read: no bool value found")
		}
		err := pbytes.MarshalBinary(d, n)
		if err != nil {
			panic("_xfread marshal")
		}
	case 'l':
		/* opacity is like a float with range */
		d := popslice(int(mabs.tents[tid].sz))
		s := readword(f, true)
		df, err := strconv.ParseFloat(s, 64)
		if err != nil {
			panic("read: no float value found")
		}
		if df < float64(mabs.tents[tid].first) || df > float64(mabs.tents[tid].last) {
			panic("read: value is out of range")
		}
		err = pbytes.MarshalBinary(d, float32(df))
		if err != nil {
			panic("_xfread marshal")
		}
	case 'r':
		d := popslice(int(mabs.tents[tid].sz))
		s := readword(f, true)
		df, err := strconv.ParseFloat(s, 64)
		if err != nil {
			panic("read: no float value found")
		}
		err = pbytes.MarshalBinary(d, float32(df))
		if err != nil {
			panic("_xfread marshal")
		}
	case 'a':
		d := popslice(4 * mabs.tents[tid].nitems)
		str := make([]rune, 0)
		spad := make([]rune, 3)
		for i := 0; i < mabs.tents[tid].nitems; i += 1 {
			n, w, err := sc.ReadRune()
			str = append(str, n)
			str = append(str, spad[0:4-w]...)
			if byte(n) == paminstr.EOL[0] {
				panic("read: eol")
			}
			if err != nil {
				if iseof(n, err) {
					panic("read: eof met")
				} else {
					panic("error reading")
				}
			}
		}
		err := pbytes.MarshalBinary(d, string(str))
		if err != nil {
			panic("_xfread marshal")
		}
	default:
		panic("read: can't read variables of this type")
	}
}

func xfread() {
	tid := int(pop32())
	if tid < 0 || tid >= len(mabs.tents) {
		panic("bad tid")
	}
	fid := int(pop32())
	f, _ := xbfile(fid)
	_xfread(tid, f)
	if debug['I'] != 0 {
		fmt.Fprintf(os.Stderr, "fread: file %d: eol %v eof %v\n", fid, f.eol, f.eof)
	}
}

func xfreadeol() {
	fid := int(pop32())
	f, sc := xbfile(fid)
	if f.eof {
		panic("read: eof met")
	}
	if !f.eol {
		panic("read: not at end of line")
	}
	if !f.isgraphic() {
		flushall()
	}
	if len(paminstr.EOL) > 1 {
		c, _, err := sc.ReadRune()
		if err != nil || c != rune(paminstr.EOL[1]) {
			panic("read: broken end of line")
		}
	}
	f.eol = false
	if debug['I'] != 0 {
		fmt.Fprintf(os.Stderr, "freadeol: file %d: eol %v eof %v\n", fid, f.eol, f.eof)
	}
}

func xfreadln() {
	var (
		c   rune
		err error
	)
	tid := int(pop32())
	if tid < 0 || tid >= len(mabs.tents) {
		panic("bad tid")
	}
	fid := int(pop32())
	f, sc := xbfile(fid)
	_xfread(tid, f)
	if !f.eol {
		for {
			c, _, err = sc.ReadRune()
			if err != nil || c == rune(paminstr.EOL[0]) {
				break
			}
		}
	}
	// perhaps an empty line
	if f.eol || c == rune(paminstr.EOL[0]) {
		if len(paminstr.EOL) > 1 && paminstr.EOL[1] != 0 {
			c, _, err = sc.ReadRune()
			if err != nil || c != rune(paminstr.EOL[1]) {
				panic("freadln: broken end of line")
			}
		}
		f.eol = false
	} else {
		f.eof = true
	}
	if debug['I'] != 0 {
		fmt.Fprintf(os.Stderr, "freadln: file %d: eol %v eof %v\n", fid, f.eol, f.eof)
	}
}

func xfrewind() {
	fid := pop32()
	f, _ := xbfile(int(fid))
	if f.isgraphic() {
		return //should it panic?
	}
	f.b.Writer.Flush()
	f.bfd.Seek(0, 0)
	f.b.Reader.Reset(f.bfd)
	f.b.Writer.Reset(f.bfd)
	f.eof = false
	f.eol = false
	if debug['I'] != 0 {
		fmt.Fprintf(os.Stderr, "frewind: file %d: eol %v eof %v\n", fid, f.eol, f.eof)
	}
}

func floatfmt(f float64) string {
	s := fmt.Sprintf("%.5g", f)
	if strings.Contains(s, ".") {
		return s
	}
	se := strings.Split(s, "e")
	s = se[0] + ".0"
	if len(se) > 1 {
		s += "e" + se[1]
	}
	return s
}

func tfmt(t *Tent, fname string) string {
	s := ""
	switch t.fmt {
	case 'u':
		b := int32(pop32())
		if b == 0 {
			s = fmt.Sprintf("NoBut")
		} else {
			s = fmt.Sprintf("%d", b)
		}
	case 'i', 'h':
		s = fmt.Sprintf("%d", int32(pop32()))
	case 'c':
		i := rune(pop32())
		if i == rune(paminstr.EOL[0]) {
			errs := fmt.Sprintf("%s: end of line", fname)
			panic(errs)
		}
		//
		// NB: This check prevents using the file data type
		// to handle binary files.
		//

		if i == paminstr.Eof {
			errs := fmt.Sprintf("%s: can't write eof", fname)
			panic(errs)
		}
		if (i & 0x80) != 0 {
			errs := fmt.Sprintf("%s: can't write special char", fname)
			panic(errs)
		}
		if i > 0xFF {
			panic("char value out of range")
		}
		s = fmt.Sprintf("%c", i)
	case 'b':
		if pop32() != 0 {
			s = fmt.Sprintf("True")
		} else {
			s = fmt.Sprintf("False")
		}
	case 'r', 'l':
		r := popr()
		s = fmt.Sprintf(floatfmt(r))
	case 's':
		d := popn(int(t.sz))
		s = fmt.Sprintf("%s", d)
	case 'e':
		i := int(pop32())
		if i < t.first || i > t.last {
			panic("can't print a value out of range")
		}
		s = fmt.Sprintf("%s", t.lits[i-t.first])
	case 'a':
		if mabs.tents[t.etid].fmt == 'c' {
			d := popn(int(t.sz))
			s = fmt.Sprintf("%s", d)
			break
		}
		fallthrough
	default:
		panic(fmt.Sprintf("%s: can't write variables of type '%c'", fname, t.fmt))
	}
	return s
}

func _xfwrite(nl bool) {
	var wr io.Writer
	t := tfetch(int(pop32()))
	f, _ := xbfile(int(pop32()))
	if (f.mode & wMode) == 0 {
		panic("write: file not open for writing")
	}
	s := tfmt(t, "write")
	if f.isgraphic() {
		wr = f.graph
	} else {
		wr = f.b
	}
	fmt.Fprintf(wr, "%s", s)
	if nl {
		fmt.Fprint(wr, paminstr.EOL)
		if !f.isgraphic() {
			f.b.Flush()
		}
	}
}

func xopen() {
	t1 := tfetch(int(pop32()))
	t2 := tfetch(int(pop32()))
	fp := popslice(u32sz)
	name := popn(int(t1.sz))
	mode := popn(int(t2.sz))
	if name == "stdgraph" {
		panic("use gopen for stdgraph")
	}
	n := uint32(xbopen(name, mode, nil))
	err := pbytes.MarshalBinary(fp, n)
	if err != nil {
		panic("xopen marshal")
	}
}

func xfwriteeol() {
	var wr io.Writer
	f, _ := xbfile(int(pop32()))
	if (f.mode & wMode) == 0 {
		panic("write: file not open for writing")
	}
	if f.isgraphic() {
		wr = f.graph
	} else {
		wr = f.b
	}
	fmt.Fprint(wr, paminstr.EOL)
	if !f.isgraphic() {
		f.b.Flush()
	}
}

func xfflush() {
	f, _ := xbfile(int(pop32()))
	if f.isgraphic() {
		f.graph.Flush()
	} else {
		f.b.Flush()
	}
}

func xfwrite() {
	_xfwrite(false)
}

func xfwriteln() {
	_xfwrite(true)
}

func xbclose(i int) {
	if i >= len(files) || files[i] == nil {
		panic("file not open")
	}
	files[i] = nil
}

func xbgclose(i int) {
	xbclose(i)
}

func xbgopen(g *gx.Graphics) int {
	return xbopen("g:"+g.Name(), "rw", g)
}

func xclose() {
	fid := int(pop32())
	f, _ := xbfile(fid)
	if f.isgraphic() {
		panic("fclose: cannot fclose a graphx file")
	}
	f.b.Flush()
	f.bfd.Close()
	xbclose(fid)
}

func xfeof() {
	fid := int(pop32())
	f, _ := xbfile(fid)
	if f.eof {
		push32(1)
	} else {
		push32(0)
	}
	if debug['I'] != 0 {
		fmt.Fprintf(os.Stderr, "feof: file %d: eol %v eof %v\n", fid, f.eol, f.eof)
	}
}

func xfeol() {
	fid := int(pop32())
	f, _ := xbfile(fid)
	if f.eol {
		push32(1)
	} else {
		push32(0)
	}
	if debug['I'] != 0 {
		fmt.Fprintf(os.Stderr, "feol: file %d: eol %v eof %v\n", fid, f.eol, f.eof)
	}
}

//
// BUG: pred/1 and succ/1 should become pred/2 and succ/2,
// and receive the type id for the argument. This is needed
// to perform range checks in them.
// Otherwise, if the result is not assigned to a variable,
// an out of range condition might be hidden, like in:
//	writeln(succ(Maxint));
//
func xpred() {
	push32(pop32() - 1)
}

func xsucc() {
	push32(pop32() + 1)
}

func tabs(lvl int) string {
	s := ""
	for i := 0; i < lvl; i++ {
		s += "\t"
	}
	return s
}

func dumpc(r rune) string {
	switch r {
	case '\b':
		return "\\b"
	case '\r':
		return "\\r"
	case '\n':
		return "\\n"
	case '\t':
		return "\\t"
	default:
		if r >= 0 && r < 0x20 {
			return fmt.Sprintf("'\\%03o'", r)
		} else if r >= 0x20 && r < 0xFFFF {
			return fmt.Sprintf("'%c'", r)
		} else {
			return "'\\???'"
		}
	}
}

var vlvl int

func (v *Vmem) String() string {
	var (
		nv  Vmem
		err error
		ifc interface{}
	)
	t := &mabs.tents[v.tid]
	s := ""
	switch t.fmt {
	case 'b':
		var e uint32
		ifc, err = pbytes.UnmarshalBinary(v.elem, e)
		if err != nil {
			panic("vmem umarshal")
		}
		e = ifc.(uint32)
		if e != 0 {
			s += fmt.Sprintf("True")
		} else {
			s += fmt.Sprintf("False")
		}
	case 'c':
		var e rune
		ifc, err = pbytes.UnmarshalBinary(v.elem, e)
		if err != nil {
			panic("vmem umarshal")
		}
		e = ifc.(rune)
		s += dumpc(e)
	case 'i':
		var e uint32
		ifc, err = pbytes.UnmarshalBinary(v.elem, e)
		if err != nil {
			panic("vmem umarshal")
		}
		e = ifc.(uint32)
		ee := int(int32(e))
		if ee < t.first || ee > t.last {
			s += fmt.Sprintf("out of range")
		} else {
			s += fmt.Sprintf("%d", ee)
		}
		break
	case 'e', 'h':
		var e int
		ifc, err = pbytes.UnmarshalBinary(v.elem, e)
		if err != nil {
			panic("vmem umarshal")
		}
		e = ifc.(int)
		if e < t.first || e > t.last {
			s += fmt.Sprintf("out of range")
		} else {
			s += fmt.Sprintf("%s", t.lits[e-t.first])
		}
	case 'r', 'l':
		var e float32
		ifc, err = pbytes.UnmarshalBinary(v.elem, e)
		if err != nil {
			panic("vmem umarshal")
		}
		e = ifc.(float32)
		s += floatfmt(float64(e))
	case 'p':
		var e uintptr
		ifc, err = pbytes.UnmarshalBinary(v.elem, e)
		if err != nil {
			panic("vmem umarshal")
		}
		e = ifc.(uintptr)
		if e != 0 {
			s += fmt.Sprintf("%#x", e)
		} else {
			s += "nil"
		}
	case 'a':
		if mabs.tents[t.etid].fmt == 'c' {
			var e string
			ifc, err = pbytes.UnmarshalBinary(v.elem, e)
			if err != nil {
				panic("vmem umarshal")
			}
			e = ifc.(string)
			s += fmt.Sprintf("\"")
			s += fmt.Sprintf("%s", e)
			s += fmt.Sprintf("\"")
			break
		}
		s += fmt.Sprintf("{\n")
		vlvl++

		nv.tid = int(t.etid)
		st := uint(0)
		for i := 0; i < t.nitems; i++ {
			s += tabs(vlvl)
			nv.elem = v.elem[st : st+mabs.tents[t.etid].sz]
			s += fmt.Sprintf("[%d] %v,\n", i, nv)
			st += mabs.tents[t.etid].sz
		}
		vlvl--
		s += tabs(vlvl)
		s += fmt.Sprintf("}")
	case 'R':
		s += fmt.Sprintf("{\n")
		vlvl++
		st := uint(0)
		for i := 0; i < t.nitems; i++ {
			fld := t.fields[i]
			nv.tid = int(fld.tid)
			nv.elem = v.elem[st : st+mabs.tents[nv.tid].sz]
			s += tabs(vlvl)
			s += fmt.Sprintf(".%s = %v,\n", fld.name, nv)
		}
		vlvl--
		s += tabs(vlvl)
		s += fmt.Sprintf("}")
	case 'X':
		s += fmt.Sprintf("<procedure>")
	case 'f':
		var e int
		ifc, err = pbytes.UnmarshalBinary(v.elem, e)
		if err != nil {
			panic("vmem umarshal")
		}
		e = ifc.(int)
		if e < 0 || e >= len(files) {
			s += fmt.Sprintf("invalid file")
			return s
		}
		if files[e] == nil {
			s += fmt.Sprintf("closed file")
			return s
		}
		switch e {
		case 0:
			s += fmt.Sprintf("<stdin>")
			break
		case 1:
			s += fmt.Sprintf("<stdout>")
			break
		default:
			s += fmt.Sprintf("<file #%d %s>", e, files[e])
		}
	case 'F':
		s += fmt.Sprintf("<function>")
		break
	case 's':
		var e string
		ifc, err = pbytes.UnmarshalBinary(v.elem, e)
		if err != nil {
			panic("vmem umarshal")
		}
		e = ifc.(string)
		s += fmt.Sprintf("\"")
		s += fmt.Sprintf("%s", e)
		s += fmt.Sprintf("\"")
	default:
		errs := fmt.Sprintf("vfmt: fmt %x", t.fmt)
		panic(errs)
	}
	return s
}

func dumplocals(tag string, n int, vents []Vent, lp int) {
	var v Vmem
	for i := 0; i < vlvl; i++ {
		fmt.Fprintf(os.Stderr, "\t")
	}
	fmt.Fprintf(os.Stderr, "%s:\n", tag)
	for i := 0; i < n; i++ {
		v.tid = int(vents[i].tid)
		st := uint(lp + int(vents[i].addr))
		sz := mabs.tents[v.tid].sz
		v.elem = mst.stack[st : st+sz]
		for j := 0; j < vlvl; j++ {
			fmt.Fprintf(os.Stderr, "\t")
		}
		if debug['S'] != 0 || debug['D'] != 0 || debug['M'] != 0 {
			fmt.Fprintf(os.Stderr, "%v  ", v.elem)
		}
		fmt.Fprintf(os.Stderr, "%s = %v\n", vents[i].name, &v)
	}
}

func dumpglobals() {
	if len(mabs.vents) > 0 {
		dumplocals("global variables", len(mabs.vents), mabs.vents, 0)
	}
}

func dumpheap() {
	var v Vmem

	if ptrs == nil {
		return
	}
	for i := 0; i < vlvl; i++ {
		fmt.Fprintf(os.Stderr, "\t")
	}
	fmt.Fprintf(os.Stderr, "heap:\n")
	for pt := ptrs; pt != nil; pt = pt.next {
		if pt.p != nil {
			v.tid = int(pt.tid)
			v.elem = pt.p
			for i := 0; i < vlvl; i++ {
				fmt.Fprintf(os.Stderr, "\t")
			}
			fmt.Fprintf(os.Stderr, "%p = %v\n", pt, &v)
		}
	}
}

type SFrame struct {
	pc  uint32
	pid uint32
	fp  int
	vp  int
	ap  int
}

func proctrace(fr SFrame) {
	pent := &mabs.pents[fr.pid]
	fmt.Fprintf(os.Stderr, "%s() pid %d\tpc %#8.8x", pent.name, fr.pid, fr.pc)
	if false {
		fmt.Fprintf(os.Stderr, " args %d vars %d", pent.nargs, pent.nvars)
	}
	pe := mabs.findpc(fr.pc)
	if pe != nil {
		fmt.Fprintf(os.Stderr, " %s:%d\n", pe.fname, pe.lineno)
	} else {
		fmt.Fprintf(os.Stderr, "\n")
	}
	vlvl++
	if len(pent.args) > 0 {
		dumplocals("arguments", len(pent.args), pent.args, fr.ap)
	}
	if len(pent.vars) > 0 {
		dumplocals("local variables", len(pent.vars), pent.vars, fr.vp)
	}
	vlvl--
	fmt.Fprintf(os.Stderr, "\n")
}

func nextproc(fr *SFrame) error {
	sp := mst.sp
	mst.sp = fr.fp
	fr.pc = ptrU32(&mst.stack[mst.sp-p32sz]) // saved pc
	if fr.pc == ^uint32(0) {
		return errors.New("bad pc")
	}
	fr.pid = ptrU32(&mst.stack[mst.sp-2*p32sz])
	fr.fp = int(ptrU32(&mst.stack[mst.sp-3*p32sz]))
	fr.vp = int(ptrU32(&mst.stack[mst.sp-4*p32sz]))
	fr.ap = int(ptrU32(&mst.stack[mst.sp-5*p32sz]))
	mst.sp = sp
	return nil
}

func xstack() {
	var fr SFrame

	fr.pid = mst.procid
	fr.pc = mst.pc
	fr.vp = mst.vp
	fr.ap = mst.ap
	fr.fp = mst.fp
	fmt.Fprintf(os.Stderr, "stack trace at:\n")
	for {
		proctrace(fr)
		if nextproc(&fr) != nil {
			break
		}
		fmt.Fprintf(os.Stderr, "called from:\n")
	}
}

func xdata() {
	dumpglobals()
	dumpheap()
}

func xgrx(fid int, fname string) *gx.Graphics {
	f, _ := xbfile(fid)
	g := f.graph
	if g == nil {
		errs := fmt.Sprintf("%s: graphx operation on non-graphx file", fname)
		panic(errs)
	}
	return g
}

func xgellipse() {
	g := xgrx(int(pop32()), "gellipse")
	x := int(pop32())
	y := int(pop32())
	rx := int(pop32())
	ry := int(pop32())
	angle := float32(popr())
	g.Ellipse(x, y, rx, ry, angle)
}

func xgloc() {
	g := xgrx(int(pop32()), "gloc")
	x := int(pop32())
	y := int(pop32())
	angle := float32(popr())
	g.PosText(x, y, angle)
}

func xgline() {
	g := xgrx(int(pop32()), "gline")
	x1 := int(pop32())
	y1 := int(pop32())
	x2 := int(pop32())
	y2 := int(pop32())
	g.Line(x1, y1, x2, y2)
}

func xgpolygon() {
	g := xgrx(int(pop32()), "gpolygon")
	x := int(pop32())
	y := int(pop32())
	r := int(pop32())
	nsides := int(pop32())
	angle := float32(popr())
	g.Polygon(x, y, r, nsides, angle)
}

//Cursor is set inmediately
func xgshowcursor() {
	g := xgrx(int(pop32()), "gshowcursor")
	isvisible := int(pop32())
	g.Cursor(isvisible == 1)
}

var opencmd = map[string]string{
	"darwin":  "open",
	"windows": "cmd",
	"linux":   "xdg-open",
}

func urlopen(input string) {
	if runtime.GOOS == "windows" {
		exec.Command(opencmd[runtime.GOOS], "/C start "+input).Run()
	} else {
		exec.Command(opencmd[runtime.GOOS], input).Run()
	}
}

func xgopen() {
	t1 := tfetch(int(pop32()))
	ip := popslice(u32sz)
	name := popn(int(t1.sz))
	g := gx.OpenGraphics(name)
	if g == nil {
		panic("unable to open graphx")
	}
	i := xbgopen(g)
	err := pbytes.MarshalBinary(ip, i)
	if err != nil {
		panic("gopen marshal")
	}
	urlopen("http://localhost:4242/" + name)
}

func xgclear() {
	g := xgrx(int(pop32()), "gclear")
	g.Clear()
}

func xgclose() {
	i := int(pop32())
	g := xgrx(i, "gclose")
	g.Close()
	xbgclose(i)
}

func xgkeypress() {
	t1 := tfetch(int(pop32()))
	fid := int(pop32())
	f, _ := xbfile(fid)
	g := xgrx(fid, "greadkeys")
	switch t1.fmt {
	case 'c':
		cp := popslice(int(t1.sz))
		for i := 0; i < len(cp); i++ {
			cp[i] = 0
		}
		c := g.ReadKeyPress()
		if c == 0xff {
			f.eof = true
		}
		err := pbytes.MarshalBinary(cp, c)
		if err != nil {
			panic("xgreadkeys: marshal")
		}
	case 'a':
		nc := t1.nitems
		cp := popslice(4 * nc)
		cpp := make([]byte, nc)
		g.ReadKeyPresses(cpp)
		if cpp[0] == 0xff {
			f.eof = true
		}
		for i := 0; i < len(cp); i++ {
			if i%4 == 0 {
				cp[i] = cpp[i/4]
			} else {
				cp[i] = 0
			}
		}
	default:
		panic("xgreadkeys: can't read variables of this type")
	}
}

func xgreadmouse() {
	var x, y, nbut int
	g := xgrx(int(pop32()), "greadmouse")
	xp := popslice(u32sz)
	yp := popslice(u32sz)
	nbutp := popslice(u32sz)
	g.ReadMouse(&x, &y, &nbut)
	err := pbytes.MarshalBinary(xp, x)
	if err != nil {
		panic("xgreadmouse marshal")
	}
	err = pbytes.MarshalBinary(yp, y)
	if err != nil {
		panic("xgreadmouse marshal")
	}
	err = pbytes.MarshalBinary(nbutp, nbut)
	if err != nil {
		panic("xgreadmouse marshal")
	}
}

func col(c uint32) uint {
	var crgb = []uint{
		paminstr.Black:  gx.BLACK,
		paminstr.White:  gx.WHITE,
		paminstr.Green:  gx.GREEN,
		paminstr.Red:    gx.RED,
		paminstr.Blue:   gx.BLUE,
		paminstr.Yellow: gx.YELLOW,
		paminstr.Orange: gx.ORANGE,
	}
	if int(c) > len(crgb) {
		panic("unknown color")
	}
	return crgb[c]
}

func xgplay() {
	g := xgrx(int(pop32()), "gplay")
	snd := int(pop32())
	g.Play(snd)
}

func xgstop() {
	g := xgrx(int(pop32()), "gstop")
	g.Stop()
}

func xgfillcol() {
	g := xgrx(int(pop32()), "gfillcol")
	col := col(pop32())
	op := float32(popr())
	g.SetFillCol(col, op)
}

func xgfillrgb() {
	g := xgrx(int(pop32()), "gfillrgb")
	r := byte(pop32())
	gr := byte(pop32())
	b := byte(pop32())
	op := float32(popr())
	g.SetFillRGB(r, gr, b, op)
}

func xgpencol() {
	g := xgrx(int(pop32()), "gpencol")
	col := col(pop32())
	op := float32(popr())
	g.SetPenCol(col, op)
}

func xgpenrgb() {
	g := xgrx(int(pop32()), "gpenrgb")
	r := byte(pop32())
	gr := byte(pop32())
	b := byte(pop32())
	op := float32(popr())
	g.SetPenRGB(r, gr, b, op)
}

func xgpenwidth() {
	g := xgrx(int(pop32()), "gpenwidth")
	w := int(pop32())
	g.SetPenWidth(w)
}

func xgtextheight() {
	g := xgrx(int(pop32()), "gtextheight")
	push32(uint32(g.TextHeight()))
}

func xsleep() {
	t := pop32()
	time.Sleep(time.Duration(t) * time.Millisecond)
}

type Bfn func()

var builtin = []Bfn{
	paminstr.PBacos:        xacos,        // real
	paminstr.PBasin:        xasin,        // real
	paminstr.PBatan:        xatan,        // real
	paminstr.PBclose:       xclose,       // file
	paminstr.PBcos:         xcos,         // real
	paminstr.PBdata:        xdata,        // void
	paminstr.PBdispose:     xdispose,     // ptr
	paminstr.PBexp:         xexp,         // real
	paminstr.PBfatal:       xfatal,       // tid array of char
	paminstr.PBfeof:        xfeof,        // file
	paminstr.PBfeol:        xfeol,        // file
	paminstr.PBfflush:      xfflush,      // file
	paminstr.PBfpeek:       xfpeek,       // file char
	paminstr.PBfread:       xfread,       // tid file &arg_of_type_tid
	paminstr.PBfreadeol:    xfreadeol,    // file
	paminstr.PBfreadln:     xfreadln,     // tid file &arg_of_type_tid
	paminstr.PBfrewind:     xfrewind,     // file
	paminstr.PBfwrite:      xfwrite,      // tid file &arg_of_type_tid
	paminstr.PBfwriteeol:   xfwriteeol,   // file
	paminstr.PBfwriteln:    xfwriteln,    // tid file &arg_of_type_tid
	paminstr.PBgclear:      xgclear,      //file
	paminstr.PBgclose:      xgclose,      //file
	paminstr.PBgshowcursor: xgshowcursor, //cursor
	paminstr.PBgellipse:    xgellipse,    //file, x int, y int, radiusx int, radiusy int, angle real
	paminstr.PBgfillcol:    xgfillcol,    //file, color, opacity
	paminstr.PBgfillrgb:    xgfillrgb,    //file, strength, strength, strength, opacity
	paminstr.PBgline:       xgline,       //file, x1 int, y1 int, x2 int, y2 int,
	paminstr.PBgloc:        xgloc,        //file, x int, y int, angle real
	paminstr.PBgopen:       xgopen,       //file
	paminstr.PBgpencol:     xgpencol,     //file, color, opacity
	paminstr.PBgpenrgb:     xgpenrgb,     //file, strength, strength, strength, opacity
	paminstr.PBgpenwidth:   xgpenwidth,   //file, int
	paminstr.PBgplay:       xgplay,       //file, sound
	paminstr.PBgpolygon:    xgpolygon,    //file, x int, y int, radius int, nsides int, angle real
	paminstr.PBgreadmouse:  xgreadmouse,  //file, x int, y int, nbut int
	paminstr.PBgstop:       xgstop,       //file
	paminstr.PBgtextheight: xgtextheight, //file
	paminstr.PBgkeypress:   xgkeypress,   //file, char or array of char
	paminstr.PBlog10:       xlog10,       // real
	paminstr.PBlog:         xlog,         // real
	paminstr.PBnew:         xnew,         // tid ptr
	paminstr.PBopen:        xopen,        // tid x2 file array of char x2
	paminstr.PBpow:         xpow,         // real real
	paminstr.PBpred:        xpred,        // int
	paminstr.PBrand:        xrand,        //rand, int
	paminstr.PBsin:         xsin,         // real
	paminstr.PBsleep:       xsleep,       //int miliseconds
	paminstr.PBsqrt:        xsqrt,        // real
	paminstr.PBstack:       xstack,       // void
	paminstr.PBsucc:        xsucc,        // int
	paminstr.PBtan:         xtan,         // real

}
