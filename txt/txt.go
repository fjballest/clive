/*
	In-memory text with insertion, removal, selection, and marks
*/
package txt

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
)

// edit type
type Tedit int

const (
	Eins = Tedit(iota) // insert
	Edel               // delete

	// Edit flag (part of the last edit, new edit)
	Esame = true
	Enew  = false

	// size arg for get
	All = -1
)

/*
	Edition operation
*/
type Edit struct {
	Op    Tedit  // Eins | Edel
	Off   int    // offset for the edit
	Data  []rune // data inserted or deleted
	Contd bool   // part of the previous edit regarding undo/redo
}

/*
	A position kept in text despite insertions/removals
*/
type Mark struct {
	Off      int
	equaltoo bool
}

/*
	The basic text interface as supplied by this package.
*/
type Interface interface {
	Len() int
	Ins(data []rune, off int) error
	Del(off, n int) []rune
	Get(off int, n int) <-chan []rune
	Getc(off int) rune
	Vers() int
}

/*
	Undoable text
*/
type Edition interface {
	Interface
	Undo() *Edit
	Redo() *Edit
	ContdEdit()
}

/*
	Text kept in a series of rune slices with insert, delete,
	marks, undo, and redo.
*/
type Text struct {
	data   [][]rune
	edits  []*Edit
	nedits int // edits applied in edits
	sz     int
	marks  []*Mark
	seek   seek
	contd  bool
	vers   int
	sync.Mutex
}

type seek struct {
	off, i, n int
}

/*
	Return the text length
*/
func (t *Text) Len() int {
	t.Lock()
	defer t.Unlock()
	return t.sz
}

func (t *Text) Vers() int {
	t.Lock()
	defer t.Unlock()
	return t.vers
}

func pins(old, insp, inssz int) int {
	if old < insp {
		return old
	}
	return old + inssz
}

func pdel(old, delp0, delp1 int) int {
	if old < delp0 {
		return old
	}
	if old < delp1 {
		return delp0
	}
	return old - (delp1 - delp0)
}

func (t *Text) markins(p0, n int) {
	for _, m := range t.marks {
		if m.Off != p0 || m.equaltoo {
			m.Off = pdel(m.Off, p0, n)
		}
	}
}

func (t *Text) markdel(p0, p1 int) {
	for _, m := range t.marks {
		m.Off = pdel(m.Off, p0, p1)
	}
}

func (t *Text) markEdit(e *Edit) {
	if e.Op == Eins {
		t.markins(e.Off, len(e.Data))
	} else {
		t.markdel(e.Off, e.Off+len(e.Data))
	}
}

func (te Tedit) String() string {
	if te == Eins {
		return "Eins"
	}
	return "Edel"
}

/*
	Debug: return a printable edit string
*/
func (e Edit) String() string {
	if e.Contd {
		return fmt.Sprintf("+%s %d '%s'", e.Op, e.Off, string(e.Data))
	}
	return fmt.Sprintf("%s %d '%s'", e.Op, e.Off, string(e.Data))
}

/*
	Create a new text with support for undo and redo
*/
func NewEditing(txt []rune) *Text {
	t := &Text{
		data:  make([][]rune, 0, 128),
		edits: make([]*Edit, 0, 128),
		marks: make([]*Mark, 0, 4),
		seek:  seek{off: -2},
	}
	if len(txt) > 0 {
		t.Ins(txt, 0)
	}
	return t
}

/*
	Create a new text with no support for undo and redo
*/
func New(txt []rune) *Text {
	t := &Text{
		data:  make([][]rune, 0, 128),
		marks: make([]*Mark, 0, 4),
		seek:  seek{off: -2},
	}
	if len(txt) > 0 {
		t.Ins(txt, 0)
	}
	return t
}

func (t *Text) addEdit(op Tedit, pos int, data []rune, same bool) *Edit {
	if t.edits == nil {
		return &Edit{op, pos, data, same}
	}
	if t.nedits < len(t.edits) {
		t.edits = t.edits[:t.nedits]
	}
	if op == Eins && t.nedits > 0 {
		e := t.edits[t.nedits-1]
		if e.Op == Eins && e.Off+len(e.Data) == pos &&
			len(e.Data) < 1024 {
			e.Data = append(e.Data, data...)
			return &Edit{op, pos, data, e.Contd}
		}
	} else if op == Edel && t.nedits > 0 {
		e := t.edits[t.nedits-1]
		if e.Op == Edel && e.Off+len(e.Data) == pos &&
			len(e.Data) < 1024 {
			e.Data = append(e.Data, data...)
			return &Edit{op, pos, data, e.Contd}
		}
	}
	ndata := make([]rune, len(data))
	copy(ndata, data)
	e := &Edit{op, pos, ndata, same}
	t.edits = append(t.edits, e)
	t.nedits++
	return e
}

func (t *Text) edit(e *Edit) error {
	if e.Op == Eins {
		if err := t.ins(e.Data, e.Off); err != nil {
			return err
		}
	} else {
		t.del(e.Off, len(e.Data))
	}
	return nil
}

/*
	Return the next edit in the undo list, nil if none.
	Contd is set to true in the returned edit if the edit continues.
	(and apply the edit to the text)
*/
func (t *Text) Undo() *Edit {
	t.Lock()
	defer t.Unlock()
	if t.edits == nil || t.nedits == 0 {
		return nil
	}
	t.vers++
	t.nedits--
	e := *t.edits[t.nedits]
	if e.Op == Eins {
		e.Op = Edel
	} else {
		e.Op = Eins
	}
	t.edit(&e)
	t.markEdit(&e)
	return &e
}

/*
	Return the next edit in the redo list, nil if none.
	Contd is set to true in the returned edit if the edit continues.
	(and apply the edit to the text).
*/
func (t *Text) Redo() *Edit {
	t.Lock()
	defer t.Unlock()
	if t.edits == nil || t.nedits == len(t.edits) {
		return nil
	}
	t.vers++
	e := *t.edits[t.nedits]
	e.Contd = t.nedits < len(t.edits)-1 && t.edits[t.nedits+1].Contd
	t.nedits++
	t.edit(&e)
	t.markEdit(&e)
	return &e
}

/*
	Insert data at off
*/
func (t *Text) ins(data []rune, off int) error {
	// defer t.dump("ins")
	t.seek.off = -2 // invalidate
	d := t.data
	if off > t.sz {
		return errors.New("text can't have holes")
	}
	if off == t.sz {
		if len(d) > 0 {
			i := len(d) - 1
			if len(d[i]) < 512 {
				d[i] = append(d[i], data...)
				t.sz += len(data)
				return nil
			}
		}
		nd := make([]rune, len(data), len(data)+64)
		copy(nd, data)
		t.data = append(t.data, nd)
		t.sz += len(data)
		return nil
	}
	for i := range d {
		if off < len(d[i]) {
			d = append(d, nil)
			t.data = d
			if i < len(d)-1 {
				copy(d[i+2:], d[i+1:])
			}
			d[i+1] = make([]rune, len(d[i])-off, len(d[i]))
			copy(d[i+1][0:], d[i][off:])
			d[i] = d[i][:off]
		}
		if off == len(d[i]) {
			d[i] = append(d[i], data...)
			t.sz += len(data)
			break
		}
		off -= len(d[i])
	}
	return nil
}

/*
	Delete n runes at off and return the deleted text
*/
func (t *Text) del(off int, n int) []rune {
	// defer t.dump("del")
	t.seek.off = -2 // invalidate
	b := make([]rune, 0, 64)
	d := t.data
	if off >= t.sz {
		return b
	}
	var i int
	for i = 0; i < len(d); i++ {
		if off < len(d[i]) {
			break
		}
		off -= len(d[i])
	}
	nd, tot := 0, 0
	for ; i < len(d) && tot < n; tot += nd {
		nd = len(d[i]) - off
		if nd > n-tot {
			nd = n - tot
		}
		if nd == len(d[i]) {
			b = append(b, d[i][0:]...)
			if i < len(d)-1 {
				copy(d[i:], d[i+1:])
			}
			d = d[:len(d)-1]
			t.data = d
		} else if off+nd == len(d[i]) {
			b = append(b, d[i][off:]...)
			d[i] = d[i][:off]
			i++
		} else {
			b = append(b, d[i][off:off+nd]...)
			d[i] = append(d[i][:off], d[i][off+nd:]...)
			i++
		}
		t.sz -= nd
		off = 0
	}
	return b
}

/*
	Flag that the next Ins or Del is to be considered part of
	the last edition regarding undo and redo.
	The edit added to the undo list will have Contd == true
*/
func (t *Text) ContdEdit() {
	t.Lock()
	defer t.Unlock()
	t.contd = true
}

/*
	Flag that the next Ins or Del not to be considered part of
	the last edition regarding undo and redo.
	That is, undo the effect of a previous call to ContdEdit().
*/
func (t *Text) DiscontdEdit() {
	t.Lock()
	defer t.Unlock()
	t.contd = false
}

/*
	Insert text at off
*/
func (t *Text) Ins(data []rune, off int) error {
	t.Lock()
	defer t.Unlock()
	contd := t.contd
	t.contd = false
	if err := t.ins(data, off); err != nil {
		return err
	}
	t.vers++
	e := t.addEdit(Eins, off, data, contd)
	t.markEdit(e)
	return nil
}

/*
	Delete n runes at off
*/
func (t *Text) Del(off, n int) []rune {
	t.Lock()
	defer t.Unlock()
	if n == 0 {
		return []rune{}
	}
	t.vers++
	contd := t.contd
	t.contd = false
	rs := t.del(off, n)
	e := t.addEdit(Edel, off, rs, contd)
	t.markEdit(e)
	return rs
}

/*
	Get n runes starting at off.
	They will be sent as slices to the chan returned.
	Updating the runes returned will change the text
	without it knowing, beware.
	The text is locked while we are getting the runes
*/
func (t *Text) Get(off int, n int) <-chan []rune {
	t.Lock()
	defer t.Unlock()
	c := make(chan []rune)
	if n < 0 {
		n = t.sz
	}
	go func() {
		t.Lock()
		defer t.Unlock()
		defer close(c)
		// defer t.dump("get")
		d := t.data
		if off >= t.sz {
			c <- []rune{}
			return
		}
		var i int
		for i = 0; i < len(d); i++ {
			if off < len(d[i]) {
				break
			}
			off -= len(d[i])
		}
		nd, tot := 0, 0
		for ; i < len(d) && tot < n; tot += nd {
			nd = len(d[i]) - off
			if nd > n-tot {
				nd = n - tot
			}
			if ok := c <- d[i][off:off+nd]; !ok {
				return
			}
			i++
			off = 0
		}
	}()
	return c
}

/*
	Get a single rune at off (0 if off-limits)
*/
func (t *Text) Getc(off int) rune {
	t.Lock()
	defer t.Unlock()
	d := t.data
	switch off {
	case t.seek.off:
		return d[t.seek.i][t.seek.n]
	case t.seek.off - 1:
		if t.seek.n > 0 {
			t.seek.n--
		} else if t.seek.i > 0 {
			t.seek.i--
			t.seek.n = len(d[t.seek.i]) - 1
		} else {
			return rune(0)
		}
		t.seek.off--
		return d[t.seek.i][t.seek.n]
	case t.seek.off + 1:
		if t.seek.i >= len(d) {
			return rune(0)
		}
		if t.seek.n >= len(d[t.seek.i])-1 {
			if t.seek.i >= len(d)-1 {
				return rune(0)
			}
			t.seek.i++
			t.seek.n = 0
		} else {
			t.seek.n++
		}
		t.seek.off++
		return d[t.seek.i][t.seek.n]
	}
	if off < 0 || off >= t.sz {
		t.seek.off = -2
		return rune(0)
	}
	soff := off
	var i int
	for i = 0; i < len(d); i++ {
		if soff < len(d[i]) {
			break
		}
		soff -= len(d[i])
	}
	if i >= len(d) {
		return rune(0) // can't happen
	}
	t.seek.i = i
	t.seek.n = soff
	t.seek.off = off
	return d[t.seek.i][t.seek.n]
}

/*
	Debug: print the tag followed by the state of text
*/
func (t *Text) Sprint() string {
	var w bytes.Buffer
	fmt.Fprintf(&w, "%d runes\n", t.sz)
	for i, d := range t.data {
		fmt.Fprintf(&w, "%d: [%d]'", i, len(d))
		for j := 0; j < len(d); j++ {
			if d[j] == '\n' {
				fmt.Fprintf(&w, "\\n")
			} else if d[j] == '	' {
				fmt.Fprintf(&w, "\\t")
			} else if d[j] < 32 {
				fmt.Fprintf(&w, "[%x]", d[j])
			} else {
				fmt.Fprintf(&w, "%c", d[j])
			}
		}
		fmt.Fprintf(&w, "'\n")
	}
	for m, p := range t.marks {
		fmt.Fprintf(&w, "mark[%d] = %v\n", m, p)
	}
	fmt.Fprintf(&w, "\n")
	if t.edits == nil {
		return w.String()
	}
	for i, e := range t.edits {
		s := " "
		if i == t.nedits {
			s = ">"
		}
		fmt.Fprintf(&w, "%s%d: %s\n", s, i, e)
	}
	fmt.Fprintf(&w, "\n")
	return w.String()
}

/*
	Delete all text (undoable)
*/
func (t *Text) DelAll() {
	t.Lock()
	defer t.Unlock()
	contd := t.contd
	t.contd = false
	if t.sz == 0 {
		return
	}
	t.vers++
	dat := t.del(0, t.sz)
	e := t.addEdit(Edel, 0, dat, contd)
	t.markEdit(e)
}

/*
	Place a mark in the text, keeping its position despite
	further inserts and removes.
	If equaltoo is true, the mark advances due to insertions
	exactly at off. Otherwise, an insert at off does not
	advance the mark.
*/
func (t *Text) Mark(off int, equaltoo bool) *Mark {
	t.Lock()
	defer t.Unlock()
	m := &Mark{off, equaltoo}
	t.marks = append(t.marks, m)
	return m
}

/*
	Remove a mark from the text
*/
func (t *Text) Unmark(m *Mark) {
	t.Lock()
	defer t.Unlock()
	for i := 0; i < len(t.marks); i++ {
		if t.marks[i] == m {
			t.marks = append(t.marks[0:i], t.marks[i+1:]...)
			return
		}
	}
}
