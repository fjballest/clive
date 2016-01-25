package web

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
struct Text {
	*Ctlr
	t *txt.Text
	tag string
	tagged, noedits bool
}

// Write the HTML for the text control to a page.
func (t *Text) WriteTo(w io.Writer) (tot int64, err error) {
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
		document.mktext(d, t, x, "`+t.Id+`", "`+vid+`");
	});
</script>`)
	tot += int64(n)
	return tot, err
}

// Create a new text control with the given tag line and body lines.
func newText(tagged bool, tag string, lines ...string) *Text {
	lns := strings.Join(lines, "\n");
	t := &Text {
		Ctlr: newCtlr("text"),
		t: txt.NewEditing([]rune(lns)),
		tag: tag,
		tagged: tagged,
	}
	go func() {
		for e := range t.in {
			t.handle(e)
		}
	}()
	return t
}

// Create a new text control with the given tag line and body lines.
func NewTaggedText(tag string, lines ...string) *Text {
	return newText(true, tag, lines...)
}

// Create a new text control with no tag line and the given body lines.
func NewText(lines ...string) *Text {
	return newText(false, "", lines...)
}

// Change the font used.
// Known fonts are "r", "b", "i", "t".
// Known combinations are "rb", "tb", and "ri".
func (t *Text) SetFont(f string) {
	t.out <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"font", f}}
}

// Prevent user edits
func (t *Text) NoEdits() {
	t.noedits = true
	t.out <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"noedits"}}
}

// Permit user edits (default)
func (t *Text) Edits() {
	t.noedits = false
	t.out <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"edits"}}
}


/// Insert in the text and update the views.
// Views might have to reload if they are concurrently editing.
func (t *Text) Ins(data []rune, off int) error {
	if err := t.t.Ins(data, off); err != nil {
		return err
	}
	t.out <- &Ev{Id: t.Id, Src: t.Id+"u", Vers: t.t.Vers(),
		Args: []string{"eins", string(data), fmt.Sprintf("%d", off)},
	}
	return nil
}

// Delete in the text and update the views.
// Views might have to reload if they are concurrently editing.
func (t *Text) Del(off, n int) []rune {
	rs := t.t.Del(off, n)
	if len(rs) == 0 {
		return rs
	}
	t.out <- &Ev{Id: t.Id, Src: t.Id+"u", Vers: t.t.Vers(),
		Args: []string{"edel", fmt.Sprintf("%d", off), fmt.Sprintf("%d", off+len(rs))},
	}
	return rs
}

// Return the text so the application can edit it at will,
// further updates from the views will fail due to wrong version,
// and the caller must call EditDone() when done so the views are reloaded
// with the new text.
func (t *Text) EditText() *txt.Text {
	return t.t
}

// After calling EditText() and using the txt.Text to edit by program,
// this must be called to reload the views with the new text.
func (t *Text) EditDone() {
	t.updateAll()
}

// Retrieve the current text.
// Text is locked while getting the text
func (t *Text) Get(off int, n int) <-chan []rune {
	return t.t.Get(off, n)
}

// Retrieve a rune.
func (t *Text) Getc(off int) rune {
	return t.t.Getc(off)
}

func (t *Text) sendLine(toid string, to chan<- *Ev, buf *bytes.Buffer) bool {
	s := buf.String()
	buf.Reset()
	ev := &Ev{Id: t.Id, Src: "", Args: []string{"reloading", s}}
	ok := to <- ev
	return ok
}

func (t *Text) update(toid string) {
	to := t.viewOut(toid)
	if t.noedits {
		t.out <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"noedits"}}
	} else {
		t.out <- &Ev{Id: t.Id, Src: t.Id+"u", Args: []string{"edits"}}
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

func (t *Text) updateAll() {
	vs := t.Views()
	for _, v := range vs {
		t.update(v)
	}
}

func (t *Text) wrongVers(tag string, wev *Ev) bool {
	vers := t.t.Vers()
	if wev.Vers == vers+1 {
		return false
	}
	cmd.Dprintf("%s: %s: vers %d != %d+1\n", t.Id, tag, wev.Vers, vers)
	t.update(wev.Src)
	return true
}

func (t *Text) p0p1(ev []string) (int, int, error) {
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

func (t *Text) getString(off int, n int) string {
	rc := t.t.Get(off, n)
	var buf bytes.Buffer
	for rs := range rc {
		fmt.Fprintf(&buf, "%s", string(rs))
	}
	return buf.String()
}

func (t *Text) handle(wev *Ev) {
	if wev==nil || len(wev.Args)<1 {
		return
	}

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
	case "tick", "click1", "click2", "click4":
		// t.uevc <- wev
		t.post(wev)
	case "eins":
		if len(ev) < 3 {
			cmd.Dprintf("%s: ins: short\n", t.Id)
			return
		}
		if t.wrongVers("ins", wev) {
			return
		}
		p0, err := strconv.Atoi(ev[2])
		if err != nil {
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
		// t.uevc <- wev
	}
}
