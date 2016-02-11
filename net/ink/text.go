package ink

import (
	"errors"
	"strconv"
	"clive/txt"
	"clive/cmd"
	"bytes"
	"fmt"
	"clive/snarf"
	"io"
	"html"
	"strings"
	"sync"
)

// Events sent from the viewer:
//	tag wordclicked
//	click[124]	textclicked	p0 p1	(buttons are 1, 2, 4, 8, 16, ...)
//	tick	p0 p1
//	epaste	p0 p1
//	ecopy	p0 p1
//	ecut	p0 p1
//	eins	text p0
//	edel	p0 p1
//	eundo
//	eredo
//	neeedreload
//	intr	esc|...
//	hold
//	rlsed
//	save
//	quit
//	focus
// Events sent from the viewer but not for the user:
//	id
// Events sent to the viewer (besides all reflected events):
//	reload
//	reloading text
//	reloaded vers
//	eins ...
//	edel ...
//	close
//	noedits
//	edits
//	font name
//	held
//	rlse
//	mark name pos
//	markinsing name str
//	markinsdone name
//	delmark
//	dirty
//	clean
//	tag str
//	show
//	sel pos pos	(sets p0 and p1 marks)
// Events sent to the user (besides those from the viewer):
//	start
//	end

// Editable text control.
// See Ctlr for the common API for controls.
// The events posted to the user are:
//	start
//	end
//	tag wordclicked
//	click[124]	textclicked	p0 p1	(buttons are 1, 2, 4, 8, 16, ...)
//	tick	p0 p1
//	eins	text p0
//	edel	p0 p1
//	intr	esc|...
//
struct Txt {
	*Ctlr
	t *txt.Text
	tag string		// NB: this is not the page element tag.
	tagged, noedits bool	// It was a tag line but we no longer use it.
	cundo bool
	owner string
	held []*Ev
	lastev string
	ngets int
	getslk sync.Mutex
	dirty, istemp bool
}

// Prevent t from getting dirty despite viewer or user calls.
func (t *Txt) DoesntGetDirty() {
	t.istemp = true
}

// If called, undo and redo events are not processed by the text, but
// are forwarded to the client as events instead.
func (t *Txt) ClientDoesUndoRedo() {
	t.cundo = true
}

// Write the HTML for the text control to a page.
func (t *Txt) WriteTo(w io.Writer) (tot int64, err error) {
	vid := t.newViewId()

	n, err := io.WriteString(w, `
		<div id="`+vid+`" class="`+t.Id+` ui-widget-content hasws", tabindex="1" style="border:2px solid black; margin:0; width:100%;height:300; background-color:#ffffea">`)
	tot += int64(n)
	if err != nil {
		return tot, err
	}
	if t.tagged {
		n, err := io.WriteString(w, `<div id="`+vid+`t" class="ui-widget-header">`+ 
			html.EscapeString(t.tag) + `</div>`)
		tot += int64(n)
		if err != nil {
			return tot, err
		}
	}
	ctag := t.Tag()
	ts := ``
	if ctag != "" {
		ctag = html.EscapeString(ctag)
		ts = `
			if(document.settag) {
				document.settag(x, "`+ctag+`");
			}
		`
	}
	if t.dirty {
		ts += `
			if(document.setdirty) {
				document.setdirty(x);
			}
		`
	}
	wsaddr := `wss://localhost:`+servePort
	n, err = io.WriteString(w, `
<canvas id="`+vid+`c" class="`+t.Id+`c" width="100%" height="100%" style="border:1px;"></canvas>
</div>
<script>
	$(function(){
		var d = $("#`+vid+`");
		var t = $("#`+vid+`t");
		var x = $("#`+vid+`c").get(0);
		d.wsaddr = "`+wsaddr+`";
		x.tag = "`+t.tag+`";
		x.lines = [];
		x.lines.push({txt: "", off: 0});
		document.mktxt(d, t, x, "`+t.Id+`", "`+vid+`");
		`+ts+`
	});
</script>`)
	tot += int64(n)
	return tot, err
}

// Create a new text control with the given tag line and body lines.
func newTxt(tagged bool, tag string, lines ...string) *Txt {
	lns := strings.Join(lines, "\n");
	if len(lns) == 0 || lns[len(lns)-1] != '\n' {
		lns += "\n"
	}
	t := &Txt {
		Ctlr: newCtlr("text"),
		t: txt.NewEditing([]rune(lns)),
		tag: tag,
		tagged: tagged,
	}
	t.t.SetMark("p0", 0)
	t.t.SetMark("p1", 0)
	go t.handler()
	return t
}

// Create a new text control with no tag line and the given body lines.
func NewTxt(lines ...string) *Txt {
	return newTxt(false, "", lines...)
}

// Change the font used.
// Known fonts are "r", "b", "i", "t".
// Known combinations are "rb", "tb", and "ri".
func (t *Txt) SetFont(f string) {
	t.out <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"font", f}}
}

func (t *Txt) sendLine(toid string, to chan<- *Ev, buf *bytes.Buffer) bool {
	s := buf.String()
	buf.Reset()
	ev := &Ev{Id: t.Id, Src: "", Args: []string{"reloading", s}}
	ok := to <- ev
	return ok
}

func (t *Txt) update(toid string) {
	to := t.viewOut(toid)
	if t.noedits {
		to <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"noedits"}}
	} else {
		to <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"edits"}}
	}
	ev := &Ev{Id: t.Id, Src: "", Args: []string{"reload"}}
	if ok := to <- ev; !ok {
		return
	}
	var buf bytes.Buffer
	gc := t.t.Get(0, txt.All)
	for rs := range gc {
		for _, r := range rs {
			if r == '\n' {
				if !t.sendLine(toid, to, &buf) {
					close(gc)
					return
				}
			} else {
				buf.WriteRune(r)
			}
		}
	}
	if buf.Len() > 0 {
		t.sendLine(toid, to, &buf)
	}
	for _, mark := range t.t.Marks() {
		m := t.t.Mark(mark)
		if m == nil {
			continue
		}
		ev = &Ev{Id: t.Id, Src: "", Args: []string{"mark", mark, fmt.Sprintf("%d", m.Off)}}
		if ok := to <- ev; !ok {
			return
		}
	}
	ev = &Ev{Id: t.Id, Src: "", Args: []string{"reloaded", fmt.Sprintf("%d", t.t.Vers())}}
	if ok := to <- ev; !ok {
		return
	}
	m0 := t.t.Mark("p0")
	m1 := t.t.Mark("p1")
	if m0 != nil && m1 != nil {
		ev = &Ev{Id: t.Id, Src: "", Args: []string{"sel", strconv.Itoa(m0.Off), strconv.Itoa(m1.Off)}}
		to <- ev
	}
}

func (t *Txt) updateAll() {
	vs := t.Views()
	for _, v := range vs {
		t.update(v)
	}
}

func (t *Txt) wrongVers(tag string, wev *Ev) bool {
	vers := t.t.Vers()
	if wev.Vers == vers+1 {
		return false
	}
	cmd.Dprintf("%s: %s: vers %d != %d+1\n", t.Id, tag, wev.Vers, vers)
	t.update(wev.Src)
	return true
}

func (t *Txt) p0p1(ev []string) (int, int, error) {
	if len(ev) < 3 {
		cmd.Dprintf("%s: %s: short\n", t.Id, ev[0])
		return 0, 0, errors.New("short event")
	}
	p0, err := strconv.Atoi(ev[1])
	if err != nil {
		cmd.Dprintf("%s: %s: p0: %s\n", t.Id, ev[0], err)
		return 0, 0, errors.New("bad p0")
	}
	p1, err := strconv.Atoi(ev[2])
	if err != nil {
		cmd.Dprintf("%s: %s: p1: %s\n", t.Id, ev[0], err)
		return 0, 0, errors.New("bad p1")
	}
	return p0, p1, nil
}

func (t *Txt) getString(off int, n int) string {
	rc := t.t.Get(off, n)
	var buf bytes.Buffer
	for rs := range rc {
		fmt.Fprintf(&buf, "%s", string(rs))
	}
	return buf.String()
}

type handler func(*Ev) handler

// Event handling goes from these states:
//	unlocked -> locked
//	locked -> unlocked
//	locked -> releasing
//	releasing - > unlocked
//
// Held events are kept in t.held. 
func (t *Txt) handler() {
	h := t.handleUnlocked
	for {
		select {
		case e, ok := <-t.in:
			if !ok {
				cmd.Dprintf("%s: handler done\n", t.Id)
				return
			}
			t.held = append(t.held, e)
		default:
			if len(t.held) == 0 {
				e, ok := <-t.in
				if !ok {
					cmd.Dprintf("%s: handler done\n", t.Id)
					return
				}
				t.held = append(t.held, e)
			}
		}
		e := t.held[0]
		if len(t.held) == 1 {
			t.held = nil
		} else {
			t.held = t.held[1:]
		}
		if e == nil {
			continue
		}
		cmd.Dprintf("-> %d %v\n", len(t.held), e)
		if len(e.Args) > 0 {
			h = h(e)
		}
		if e.fn != nil && (len(t.held) == 0 || t.held[len(t.held)-1] != e) {
			// call fn, the even was not requeued; it's done.
			e.fn()
		}
		// TODO: This may lead to a tight loop if
		// we want to receive just from one view and have queued events
		// that must be deferred until we do receive from that view.
		// Should this be a problem, we must teach Ctlr how to receive
		// from just one view for a while.
	}
}

func (t *Txt) discard(src string) {
	for i := 0; i < len(t.held); {
		if t.held[i].Src == src {
			copy(t.held[i:], t.held[i+1:])
			t.held = t.held[:len(t.held)-1]
		} else  {
			i++
		}
	}
}

func (t *Txt) handleUnlocked(wev *Ev) handler {
	ev := wev.Args
	switch ev[0] {
	case "hold":
		if t.owner != "" {
			panic("owner for a free text")
		}
		t.owner = wev.Src
		if wev.Src != "" && wev.Src != "app" {
			to := t.viewOut(wev.Src)
			to <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"held"}}
		}
		cmd.Dprintf("%s: locked by %s (%d)\n", t.Id, wev.Src, len(t.held))
		return t.handleLocked
	}
	t.apply(wev)
	return t.handleUnlocked
}

func (t *Txt) handleLocked(wev *Ev) handler {
	ev := wev.Args
	if t.owner == "" {
		panic("no owner for a locked text")
	}
	if wev.Src != t.owner {
		if ev[0] == "end" || ev[0] == "quit"  || ev[0] == "start" || ev[0] == "intr" {
			t.apply(wev);
			return t.handleLocked
		}
		if ev[0] == "tick" {
			// BUG? We shouldn't get a tick here, but we do.
			return t.handleLocked
		}
		t.held = append(t.held, wev)
		if ev[0] == "hold" {
			if t.owner != "app" {
				to := t.viewOut(t.owner)
				to <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"rlse"}}
			}
			cmd.Dprintf("%s: releasing %s for %s\n", t.Id, t.owner, wev.Src)
			return t.handleReleasing
		}
		return t.handleLocked
	}
	if ev[0] == "rlsed" || ev[0] == "end" || ev[0] == "quit" {
		t.owner = ""
		if ev[0] == "end"  || ev[0] == "quit" {
			t.apply(wev)
		}
		cmd.Dprintf("%s: unlocked\n", t.Id)
		return t.handleUnlocked
	}
	t.apply(wev)
	return t.handleLocked
}

func (t *Txt) handleReleasing(wev *Ev) handler {
	ev := wev.Args
	if t.owner == "" {
		panic("no owner for a releasing text")
	}
	if wev.Src != t.owner {
		if ev[0] == "end" || ev[0] == "quit"  || ev[0] == "start" || ev[0] == "intr" {
			t.apply(wev)
		} else {
			t.held = append(t.held, wev)
		}
		return  t.handleReleasing
	}
	if ev[0] == "rlsed" {
		t.owner = ""
		cmd.Dprintf("%s: unlocked\n", t.Id)
		return t.handleUnlocked
	}
	t.apply(wev)
	if ev[0] == "end"  || ev[0] == "quit" {
		t.owner = ""
		cmd.Dprintf("%s: unlocked\n", t.Id)
		return t.handleUnlocked
	}
	return t.handleReleasing
}

func (t *Txt) undoRedo(isredo bool) bool {
	some := false
	o := "undo"
	if isredo {
		o = "redo"
	}
	for {
		var uev *txt.Edit
		if !isredo {
			uev = t.t.Undo()
		} else {
			uev = t.t.Redo()
		}
		if uev == nil {
			cmd.Dprintf("%s: %s: no more\n", t.Id, o)
			return some
		}
		some = true
		cmd.Dprintf("%s: %s: undo1\n", t.Id, o)
		nev := &Ev{Id: t.Id, Src: "", Vers: t.t.Vers()}
		off := fmt.Sprintf("%d", uev.Off)
		s := string(uev.Data)
		if uev.Op == txt.Eins {
			nev.Args = []string{"eins", s, off}
		} else {
			off2 := fmt.Sprintf("%d", uev.Off+len(s))
			nev.Args = []string{"edel", off, off2}
		}
		t.out <- nev
		t.post(nev)
		if !uev.Contd {
			return some
		}
	}
}

func (t *Txt) apply(wev *Ev) {

	ev := wev.Args
	switch ev[0] {
	case "tick", "ecut", "epaste", "ecopy", "eintr", "eundo", "eredo":
		t.t.DiscontdEdit()
	default:
	}
	switch ev[0] {
	default:
		cmd.Dprintf("%s: unhandled %v\n", t.Id, ev)
		return
	case "save", "quit", "tag", "click1", "click2", "click4", "focus":
		cmd.Dprintf("%s: %v\n", t.Id, wev)
		t.post(wev)
	case "hold", "held", "rlse", "rlsed":
		cmd.Warn("%s: unexpected %v\n", t.Id, wev)
		// If we get a hold it might be a race on the javascript code,
		// let's see if that happens.
		panic("javascript hold bug?")
	case "start":
		cmd.Dprintf("%s: start %v\n", t.Id, wev.Src)
		p0 := t.t.Mark("p0")
		t.t.SetMark(wev.Src+"p0", p0.Off)
		p1 := t.t.Mark("p1")
		t.t.SetMark(wev.Src+"p1", p1.Off)
		t.update(wev.Src)
		t.post(wev)
	case "needreload":
		t.update(wev.Src)
	case "end":
		cmd.Dprintf("%s: end %v\n", t.Id, wev.Src)
		t.t.DelMark(wev.Src+"p0")
		t.t.DelMark(wev.Src+"p1")
		t.post(wev)
		t.out <- &Ev{Id: t.Id, Src: wev.Src, Args: []string{
			"delmark", wev.Src+"p0",
		}}
		t.out <- &Ev{Id: t.Id, Src: wev.Src, Args: []string{
			"delmark", wev.Src+"p1",
		}}
		t.discard(wev.Src)
	case "tick":
		if len(ev) < 3 {
			cmd.Dprintf("%s: tick: short\n", t.Id)
			return
		}
		p0, err := strconv.Atoi(ev[1])
		if err!=nil {
			cmd.Dprintf("%s: ins: %s\n", t.Id, err)
			return
		}
		p1, err := strconv.Atoi(ev[2])
		if err!=nil {
			cmd.Dprintf("%s: ins: %s\n", t.Id, err)
			return
		}
		t.t.SetMark(wev.Src+"p0", p0)
		t.t.SetMark(wev.Src+"p1", p1)
		t.t.SetMark("p0", p0)
		t.t.SetMark("p1", p1)
		t.out <- &Ev{Id: t.Id, Src: wev.Src, Args: []string{
			"mark", wev.Src+"p0", ev[1],
		}}
		t.out <- &Ev{Id: t.Id, Src: wev.Src, Args: []string{
			"mark", wev.Src+"p1", ev[2],
		}}
		t.post(wev)
	case "eins":
		if len(ev) < 3 {
			cmd.Dprintf("%s: ins: short\n", t.Id)
			return
		}
		p0, err := strconv.Atoi(ev[2])
		if err!=nil || t.wrongVers(ev[0], wev) {
			cmd.Dprintf("%s: ins: %s\n", t.Id, err)
			return
		}
		data := []rune(ev[1])
		if len(data) == 0 {
			return
		}
		if err := t.t.Ins(data, p0); err != nil {
			cmd.Dprintf("%s: ins: %s\n", t.Id, err)
			return
		}
		t.t.ContdEdit()
		cmd.Dprintf("%s: vers %d\n", t.Id, t.t.Vers())
		t.out <- wev
		t.post(wev)
	case "edel", "ecut":
		p0, p1, err := t.p0p1(ev)
		if err!=nil || t.wrongVers(ev[0], wev) {
			return
		}
		if p1 <= p0 {
			return
		}
		rs := t.t.Del(p0, p1-p0)
		if ev[0] == "ecut" {
			if err := snarf.Set(string(rs)); err != nil {
				cmd.Dprintf("%s: %s: snarf: %s\n", t.Id, ev[0], err)
			}
		} else {
			t.t.ContdEdit()
		}
		cmd.Dprintf("%s: vers %d\n", t.Id, t.t.Vers())
		ev[0] = "edel"
		t.out <- wev
		t.post(wev)
	case "ecopy":
		p0, p1, err := t.p0p1(ev)
		if err!=nil || t.wrongVers(ev[0], wev) {
			return
		}
		s := ""
		if p1 > p0 {
			s = t.getString(p0, p1-p0)
		}
		if err := snarf.Set(s); err != nil {
			cmd.Dprintf("%s: %s: snarf: %s\n", t.Id, ev[0], err)
		}
	case "epaste":
		p0, _, err := t.p0p1(ev)
		if err!=nil || t.wrongVers(ev[0], wev) {
			return
		}
		s, err := snarf.Get()
		if err != nil {
			cmd.Dprintf("%s: %s: snarf: %s\n", t.Id, ev[0], err)
			return
		}
		if s == "" {
			return
		}
		if err := t.t.Ins([]rune(s), p0); err != nil {
			cmd.Dprintf("%s: %s: ins: %s\n", t.Id, ev[0], err)
			return
		}
		nev := &Ev{Id: t.Id, Src: "", Vers: t.t.Vers()}
		nev.Args = []string{"eins", s, wev.Args[1]}
		t.out <- nev
		t.post(nev)

	case "eundo", "eredo":
		if t.cundo {
			t.post(wev)
		} else {
			t.undoRedo(ev[0] == "eredo")
		}
	case "intr":
		cmd.Dprintf("%s: intr dump:\n:%s", t.Id, t.t.Sprint())
		cmd.Dprintf("%s: vers %d\n", t.Id, t.t.Vers())
		t.post(wev)
		if t.lastev == ev[0] {
			t.post(&Ev{Id: t.Id, Src: wev.Src, Vers: t.t.Vers(), Args: []string{"clear"}})
		}
	}
	t.lastev = ev[0]
}

// Is the text dirty (as indicated by calls to Dirty/Clean)?
func (t *Txt) IsDirty() bool {
	t.Lock()
	defer t.Unlock()
	return !t.istemp && t.dirty
}

// Flag the text as dirty; it's a nop if t.DoesntGetDirty() has been called.
func (t *Txt) Dirty() {
	t.Lock()
	if t.istemp {
		t.Unlock()
		return
	}
	t.dirty = true
	t.Unlock()
	t.out <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"dirty"}}
}

// Flag the text as clean
func (t *Txt) Clean() {
	t.Lock()
	t.dirty = false
	t.Unlock()
	t.out <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"clean"}}
}

// Prevent user edits
func (t *Txt) NoEdits() {
	t.Lock()
	t.noedits = true
	t.Unlock()
	t.out <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"noedits"}}
}

// Permit user edits (default)
func (t *Txt) Edits() {
	t.Lock()
	t.noedits = false
	t.Unlock()
	t.out <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"edits"}}
}

func (t *Txt) getText() {
	t.getslk.Lock()
	defer t.getslk.Unlock()
	if t.ngets == 0 {
		t.t.DiscontdEdit()
		c := make(chan bool)
		done := func() {
			c <- true
		}
		t.in <- &Ev{Id: t.Id, Src: "app", Args:[]string{"hold"}, fn: done}
		<-c
	}
	t.ngets++
}

func (t *Txt) putText() {
	t.getslk.Lock()
	defer t.getslk.Unlock()
	t.ngets--
	if t.ngets == 0 {
		c := make(chan bool)
		done := func() {
			c <- true
		}
		t.in <- &Ev{Id: t.Id, Src: "app", Args:[]string{"rlsed"}, fn: done}
		<-c
	}
}

// Return the text so the application can edit it at will,
// further updates from the views will fail due to wrong version,
// and the caller must call EditDone() when done so the views are reloaded
// with the new text.
func (t *Txt) GetText() *txt.Text {
	t.getText()
	return t.t
}

// Undo a GetText w/o putting the new text (no text was changed)
func (t *Txt) UngetText() {
	t.putText()
}

// After calling GetText() and using the txt.Text to edit by program,
// this must be called to unlock the text and reload the views with the new text.
func (t *Txt) PutText() {
	t.putText()
	t.updateAll()
}

// Get the text length.
func (t *Txt) Len() int {
	return t.t.Len()
}

// Retrieve the current text.
// Txt is locked while getting the text
func (t *Txt) Get(off int, n int) <-chan []rune {
	return t.t.Get(off, n)
}

// Retrieve a rune.
func (t *Txt) Getc(off int) rune {
	return t.t.Getc(off)
}

// Insert
func (t *Txt) Ins(data []rune, off int) error {
	if len(data) == 0 {
		return nil
	}
	t.getText()
	defer t.putText()
	if err := t.t.Ins(data, off); err != nil {
		cmd.Dprintf("%s: ins: %s\n", t.Id, err)
		return err
	}
	cmd.Dprintf("%s: vers %d\n", t.Id, t.t.Vers())
	// Sending 4k or so in a single event makes Safari
	// take a very long time (30s) to post the event.
	// It seems it's not prepared to handle ws messages that are not small.
	// So send multiple chunks, which is faster.
	v := t.t.Vers()
	for tot, nw := 0, 0; tot < len(data); tot += nw {
		nw = len(data) - tot
		if nw > 128 {
			nw = 128
		}
		dat := data[tot:tot+nw]
		t.out <- &Ev{Id: t.Id, Src: "app", Args: []string{"einsing", string(dat)}}
	}
	t.out <- &Ev{Id: t.Id, Src: "app", Vers: v,
			Args: []string{"einsdone", strconv.Itoa(off)}}
	return nil
}

// Delete
func (t *Txt) Del(off, n int) []rune {
	t.getText()
	defer t.putText()
	rs := t.t.Del(off, n)
	cmd.Dprintf("%s: vers %d\n", t.Id, t.t.Vers())
	wev := &Ev{Id: t.Id, Src: "app", Vers: t.t.Vers(),
		Args: []string{"edel", strconv.Itoa(off), strconv.Itoa(off+len(rs))}}
	t.out <- wev
	t.post(wev)
	return nil
}

func (t *Txt) Vers() int {
	return t.t.Vers()
}

func (t *Txt) Undo() bool {
	t.getText()
	defer t.putText()
	return t.undoRedo(false)
}

func (t *Txt) Redo() bool {
	t.getText()
	defer t.putText()
	return t.undoRedo(true)
}

func (t *Txt) ContdEdit() {
	t.getText()
	defer t.putText()
	t.t.ContdEdit()
}

func (t *Txt) SetSel(p0, p1 int) {
	t.getText()
	defer t.putText()
	m0 := t.t.SetMark("p0", p0)
	m1 := t.t.SetMark("p1", p1)
	if m0 != nil && m1 != nil {
		t.out <- &Ev{Id: t.Id, Src: "", Args: []string{"sel", strconv.Itoa(m0.Off), strconv.Itoa(m1.Off)}}
	}
}

func (t *Txt) SetMark(name string, off int) *txt.Mark {
	t.getText()
	defer t.putText()
	m := t.t.SetMark(name, off)
	if m != nil {
		t.out <- &Ev{Id: t.Id, Src: "", Args: []string{"mark", name, strconv.Itoa(m.Off)}}
	}
	return m
}

func (t *Txt) DelMark(name string) {
	t.getText()
	defer t.putText()
	t.out <- &Ev{Id: t.Id, Src: "", Args: []string{"delmark", name}}
	t.t.DelMark(name)
}

func (t *Txt) Mark(name string) *txt.Mark {
	return t.t.Mark(name)
}

func (t *Txt) LineAt(off int) int {
	return t.t.LineAt(off)
}

func (t *Txt) LineOff(n int) int {
	return t.t.LineOff(n)
}

func (t *Txt) LinesAt(off0, off1 int) (int, int) {
	return t.t.LinesAt(off0, off1)
}

func (t *Txt) LinesOff(ln0, ln1 int) (int, int) {
	return t.t.LinesOffs(ln0, ln1)
}

func (t *Txt) Marks() []string {
	return t.t.Marks()
}

func (t *Txt) MarkIns(mark string, data []rune) error {
	// Sending 4k or so in a single event makes Safari
	// take a very long time (30s) to post the event.
	// It seems it's not prepared to handle ws messages that are not small.
	// So send multiple chunks, which is faster.
	t.getText()
	defer t.putText()
	if err := t.t.MarkIns(mark, data); err != nil {
		return err
	}
	for tot, nw := 0, 0; tot < len(data); tot += nw {
		nw = len(data) - tot
		if nw > 128 {
			nw = 128
		}
		t.out <- &Ev{Id: t.Id, Src: "app", Vers: t.t.Vers(),
			Args: []string{"markinsing", mark, string(data[tot:tot+nw])},
		}
	}
	t.out <- &Ev{Id: t.Id, Src: "app", Vers: t.t.Vers(),
			Args: []string{"markinsdone", mark},
	}
	return nil
}
