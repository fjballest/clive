package e

import (
	"clive/cmd"
	"clive/txt"
	"clive/zx"
	"fmt"
	"path"
	"path/filepath"
)

// BUG: This is interface is not good enough now, we should use the same of zx.RWTree
type Fsys interface {
	Get(name string) ([]rune, error)
	Put(name string, rc <-chan []rune) error
}

type opKind int

const (
	eNone = opKind(iota)
	eIns
	eDel
)

type logOp  {
	kind  opKind
	data  []rune
	p0    int
	n     int
	dirty bool
}

type log  {
	ops []*logOp
}

type file  {
	path           string
	dirty          bool
	temp           bool
	focus          bool
	t              *txt.Text
	p0, p1, m0, m1 *txt.Mark
	log            *log
}

// Use zx to fetch files but understand "-" and stdin for get, stdout for put
type lfs  {
	stdin     []rune
	out       chan<- string
	dontwrite bool
}

// it's actually a ramfs for cmd.Ns, understanding "-" as stdin/stdout.
var LocalFS = &lfs{}

// BUG: this should return a chan of runes.
func (fs *lfs) Get(name string) ([]rune, error) {
	if name == "-" {
		dprintf("get -: %s\n", string(fs.stdin))
		return fs.stdin, nil
	}
	fpath, err := filepath.Abs(name)
	if err != nil {
		return nil, err
	}
	dc := cmd.Ns.Find(fpath, "depth<1", "/", "/", 0)
	d := <-dc
	close(dc, "done")
	if d == nil {
		return nil, cerror(dc)
	}
	t, err := zx.RWDirTree(d)
	if err != nil {
		return nil, err
	}
	fdata, err := zx.GetAll(t, d["spath"])
	if err != nil {
		return nil, err
	}
	return []rune(string(fdata)), nil
}

func (fs *lfs) Put(name string, rc <-chan []rune) error {
	if name == "-" {
		fs.stdin = []rune{}
	}
	if name=="-" || fs.dontwrite {
		for r := range rc {
			fs.out <- string(r)
			fs.stdin = append(fs.stdin, r...)
		}
		close(rc)
		return nil
	}
	fpath, err := filepath.Abs(name)
	if err != nil {
		close(rc)
		return err
	}
	parent := filepath.Dir(fpath)
	dc := cmd.Ns.Find(parent, "depth<1", "/", "/", 0)
	d := <-dc
	close(dc, "done")
	if d == nil {
		close(rc)
		return cerror(dc)
	}
	t, err := zx.RWDirTree(d)
	if err != nil {
		close(rc)
		return err
	}
	datc := make(chan []byte)
	npath := zx.Path(d["spath"], path.Base(fpath))
	ec := t.Put(npath, zx.Dir{"mode": "0664"}, 0, datc, "")
	for rs := range rc {
		if ok := datc <- []byte(string(rs)); !ok {
			close(rc, cerror(dc))
			return err
		}
	}
	err = cerror(rc)
	close(datc, err)
	<-ec
	if err == nil {
		err = cerror(ec)
	}
	return err
}

/*
 * if path is "" the file is unnamed
 */
func (s *Sam) newFile(path string) (*file, error) {
	fs := s.fs
	var rs []rune
	var err error
	if path != "" {
		rs, err = fs.Get(path)
		if err != nil {
			return nil, err
		}
	}
	t := txt.NewEditing(rs)
	dprintf("new editing '%s' <%s>\n", path, string(rs))
	return &file{
		path: path,
		t:    t,
		p0:   t.Mark(0, false),
		p1:   t.Mark(0, true),
		log:  newLog(),
		m0:   t.Mark(0, false),
		m1:   t.Mark(0, true),
	}, nil
}

func (f *file) Contd() {
	f.t.ContdEdit()
}

func (f *file) Undo() bool {
	e := f.t.Undo()
	if e == nil {
		return false
	}
	for e!=nil && e.Contd {
		e = f.t.Undo()
	}
	return true
}

func (f *file) Redo() bool {
	e := f.t.Redo()
	if e == nil {
		return false
	}
	for e!=nil && e.Contd {
		e = f.t.Undo()
	}
	return true
}

func (f *file) Len() int {
	return f.t.Len()
}

func (f *file) Getc(n int) rune {
	return f.t.Getc(n)
}

func (f *file) Get(p0, n int) <-chan []rune {
	return f.t.Get(p0, n)
}

func (f *file) P0() int {
	return f.p0.Off
}

func (f *file) P1() int {
	return f.p1.Off
}

func (f *file) Sel() eSel {
	return eSel{P0: f.p0.Off, P1: f.p1.Off, F: f}
}

func (f *file) SetSel(p0, p1 int) {
	f.p0.Off = p0
	f.p1.Off = p1
}

func (f *file) Mark() eSel {
	return eSel{P0: f.m0.Off, P1: f.m1.Off, F: f}
}

func (f *file) SetMark(p0, p1 int) {
	f.m0.Off = p0
	f.m1.Off = p1
}

func (f *file) MenuLine() string {
	drty := ' '
	if f.dirty || f.path=="-" {
		drty = '\''
	}
	foc := ' '
	if f.focus {
		foc = '.'
	}
	view := '-'
	// TODO:  use '+' if one viewer, '*' if multiple viewers
	return fmt.Sprintf("%c%c%c %s", drty, view, foc, f.path)
}

func (f *file) Ins(txt []rune, p0 int) error {
	return f.t.Ins(txt, p0)
}

func (f *file) Del(p0, n int) []rune {
	return f.t.Del(p0, n)
}

func newLog() *log {
	return &log{ops: make([]*logOp, 0)}
}

func (lg *log) Clear() {
	lg.ops = lg.ops[:0]
}

func (lg *log) Ins(data []rune, p0 int, dirty bool) {
	if len(lg.ops) > 0 {
		last := lg.ops[len(lg.ops)-1]
		if last.kind==eIns && last.p0+len(last.data)==p0 {
			last.data = append(last.data, data...)
			last.dirty = last.dirty || dirty
			return
		}
		if last.kind==eIns && p0+len(data)==last.p0 {
			last.data = append(data, last.data...)
			last.p0 = p0
			last.dirty = last.dirty || dirty
			return
		}
	}
	op := &logOp{kind: eIns, data: data, p0: p0, dirty: dirty}
	lg.ops = append(lg.ops, op)
}

func (lg *log) Del(p0, n int, dirty bool) {
	if len(lg.ops) > 0 {
		last := lg.ops[len(lg.ops)-1]
		if last.kind==eDel && last.p0+last.n==p0 {
			last.n += n
			last.dirty = last.dirty || dirty
			return
		}
		if last.kind==eDel && p0+n==last.p0 {
			last.n += n
			last.p0 = 0
			last.dirty = last.dirty || dirty
			return
		}
	}
	op := &logOp{kind: eDel, p0: p0, n: n, dirty: dirty}
	lg.ops = append(lg.ops, op)
}

func (lg *log) Repl(p0, n int, data []rune, dirty bool) {
	/* log will be applied in reverse order, so ins then del */
	lg.Ins(data, p0, dirty)
	lg.Del(p0, n, dirty)
}

/*
	returns the new name for the file, if changed, or ""
*/
func (f *file) ApplyLog() string {
	lg := f.log
	nm := ""
	contd := false
	//defer un(trz("applylog"))
	for i := len(lg.ops) - 1; i >= 0; i-- {
		op := lg.ops[i]
		switch op.kind {
		case eIns:
			if op.p0 > f.Len() {
				op.p0 = f.Len()
			}
			if contd {
				f.Contd()
			}
			f.Ins(op.data, op.p0)
			f.SetSel(op.p0, op.p0+len(op.data))
			f.dirty = op.dirty
		case eDel:
			if contd {
				f.Contd()
			}
			f.Del(op.p0, op.n)
			f.SetSel(op.p0, op.p0)
			f.dirty = op.dirty
		}
		contd = true
		//f.t.Dump("apply")
	}
	lg.ops = lg.ops[:0]
	return nm
}
