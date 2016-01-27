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
//	rlse
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
struct Txt {
	*Ctlr
	t *txt.Text
	tag string
	tagged, noedits bool

	owner string
	held []*Ev
}

// Write the HTML for the text control to a page.
func (t *Txt) WriteTo(w io.Writer) (tot int64, err error) {
	vid := t.newViewId()

	n, err := io.WriteString(w, `
		<div id="`+vid+`" class="`+t.Id+`, ui-widget-content", tabindex="1" style="border:2px solid black; margin:0; overflow:auto;width:95%;height:300">`)
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
	n, err = io.WriteString(w, `
<canvas id="`+vid+`c" class="txt1c" width="100%" height="100%" style="border:1px solid black;"></canvas>
</div>
<script>
	$(function(){
		var d = $("#`+vid+`");
		var t = $("#`+vid+`t");
		var x = $("#`+vid+`c").get(0);
		x.tag = "`+t.tag+`";
		x.lines = [];
		x.lines.push({txt: "", off: 0});
		document.mktxt(d, t, x, "`+t.Id+`", "`+vid+`");
	});
</script>`)
	tot += int64(n)
	return tot, err
}

// Create a new text control with the given tag line and body lines.
func newTxt(tagged bool, tag string, lines ...string) *Txt {
	lns := strings.Join(lines, "\n");
	t := &Txt {
		Ctlr: newCtlr("text"),
		t: txt.NewEditing([]rune(lns)),
		tag: tag,
		tagged: tagged,
	}
	go t.handler()
	return t
}

// Create a new text control with the given tag line and body lines.
func NewTaggedTxt(tag string, lines ...string) *Txt {
	return newTxt(true, tag, lines...)
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

// Prevent user edits
func (t *Txt) NoEdits() {
	t.noedits = true
	t.out <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"noedits"}}
}

// Permit user edits (default)
func (t *Txt) Edits() {
	t.noedits = false
	t.out <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"edits"}}
}

// Return the text so the application can edit it at will,
// further updates from the views will fail due to wrong version,
// and the caller must call EditDone() when done so the views are reloaded
// with the new text.
func (t *Txt) EditTxt() *txt.Text {
	c := make(chan bool, 1)
	rdy := func() {
		c <- true
	}
	t.in <- &Ev{Id: t.Id, Src: "app", Args: []string{"hold"}, fn: rdy}
	<-c
	return t.t
}

// After calling EditTxt() and using the txt.Text to edit by program,
// this must be called to reload the views with the new text.
func (t *Txt) EditDone() {
	c := make(chan bool, 1)
	rdy := func() {
		c <- true
	}
	t.in <- &Ev{Id: t.Id, Src: "app", Args: []string{"rlse"}, fn: rdy}
	<-c
	t.updateAll()
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
	ev = &Ev{Id: t.Id, Src: "", Args: []string{"reloaded", fmt.Sprintf("%d", t.t.Vers())}}
	if ok := to <- ev; !ok {
		return
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
		if e.fn != nil {
			e.fn()
		}
		// TODO: This may lead to a close loop if
		// we want to receive just from one view and have queued events
		// that must be deferred until we do receive from the view.
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
		if wev.Src != "" {
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
		if ev[0] == "end" || ev[0] == "start" {
			t.apply(wev);
			return t.handleLocked
		}
		if ev[0] == "tick" {
			// BUG? We shouldn't get a tick here, but we do.
			return t.handleLocked
		}
		t.held = append(t.held, wev)
		if ev[0] == "hold" {
			to := t.viewOut(t.owner)
			to <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"rlse"}}
			cmd.Dprintf("%s: releasing %s for %s\n", t.Id, t.owner, wev.Src)
			return t.handleReleasing
		}
		return t.handleLocked
	}
	if ev[0] == "rlsed" || ev[0] == "end" {
		t.owner = ""
		if ev[0] == "end" {
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
		if ev[0] == "end" || ev[0] == "start" {
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
	if ev[0] == "end" {
		t.owner = ""
		cmd.Dprintf("%s: unlocked\n", t.Id)
		return t.handleUnlocked
	}
	return t.handleReleasing
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
	case "hold", "held", "rlse", "rlsed":
		cmd.Warn("%s: unexpected %v\n", t.Id, wev)
panic("bug")
	case "tag":
		cmd.Dprintf("%s: %v\n", t.Id, wev)
		t.post(wev)
	case "start":
		cmd.Dprintf("%s: start %v\n", t.Id, wev.Src)
		t.update(wev.Src)
		t.post(wev)
	case "needreload":
		t.update(wev.Src)
	case "end":
		cmd.Dprintf("%s: end %v\n", t.Id, wev.Src)
		t.post(wev)
		t.discard(wev.Src)
	case "tick":
		if len(ev) < 3 {
			cmd.Dprintf("%s: tick: short\n", t.Id)
			return
		}
		t.post(wev)
	case "click1", "click2", "click4":
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
		for {
			var uev *txt.Edit
			if ev[0] == "eundo" {
				uev = t.t.Undo()
			} else {
				uev = t.t.Redo()
			}
			if uev == nil {
				cmd.Dprintf("%s: %s: no more\n", t.Id, ev[0])
				return
			}
			cmd.Dprintf("%s: %s: undo1\n", t.Id, ev[0])
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
				break
			}
		}
	case "intr":
		cmd.Dprintf("%s: intr dump:\n:%s", t.Id, t.t.Sprint())
		cmd.Dprintf("%s: vers %d\n", t.Id, t.t.Vers())
		t.post(wev)
	}
}
