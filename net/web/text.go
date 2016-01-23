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

// Editable text
struct Text {
	*Ctlr
	t *txt.Text
	tag string
	napplies int
}

// Write the HTML for the text to the page.
func (t *Text) WriteTo(w io.Writer) (int64, error) {
	vid := t.NewViewId()
	html := `
		<div id="`+vid+`" class="`+t.Id+`, ui-widget-content", tabindex="1" style="border:2px solid black; margin:0; overflow:auto;width:95%;height:300">
<div id="`+vid+`t" class="ui-widget-header">`+ html.EscapeString(t.tag) + `</div>
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
</script>
`
	n, err := io.WriteString(w, html)
	return int64(n), err
}

func NewText(tag string, lines ...string) *Text {
	lns := strings.Join(lines, "\n");
	t := &Text {
		Ctlr: NewCtlr("text"),
		t: txt.NewEditing([]rune(lns)),
		tag: tag,
	}
	go func() {
		for e := range t.In() {
			t.handle(e)
		}
	}()
	return t
}

// TODO: methods to call Undo(), Redo() ContdEdit() Ins(), Del(), DelAll()
// Mark(), Unmark().
// They must call the same method on t.t and they must t.Post() an event
// to ins/del the text to reflect the change.

// Text locked while getting the text
func (t *Text) Get(off int, n int) <-chan []rune {
	return t.t.Get(off, n)
}

func (t *Text) Getc(off int) rune {
	return t.t.Getc(off)
}

func (t *Text) sendLine(toid string, to chan<- *Ev, buf *bytes.Buffer) bool {
	s := html.EscapeString(buf.String())
	buf.Reset()
	ev := &Ev{Id: t.Id, Src: "", Args: []string{"reloading", s}}
	ok := to <- ev
	return ok
}

func (t *Text) update(toid string) {
	to := t.ViewOut(toid)
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
		t.Post(wev)
	case "start":
		cmd.Dprintf("%s: start %v\n", t.Id, wev.Src)
		t.update(wev.Src)
		t.Post(wev)
	case "end":
		cmd.Dprintf("%s: end %v\n", t.Id, wev.Src)
		t.Post(wev)
	case "tick", "click1", "click2", "click4":
		// t.uevc <- wev
		t.Post(wev)
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
		t.Post(wev)
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
		t.Post(wev)
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
		t.Post(nev)
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
			t.Post(nev)
			if !uev.Contd {
				break
			}
		}
	case "intr":
		cmd.Dprintf("%s: intr dump:\n:%s", t.Id, t.t.Sprint())
		cmd.Dprintf("%s: vers %d\n", t.Id, t.t.Vers())
		t.Post(wev)
		// t.uevc <- wev
	}
}
