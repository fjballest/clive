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
)

// Editable text
struct Text {
	*Ctlr
	*txt.Text
	napplies int
}

// Write the HTML for the text to the page.
func (t *Text) WriteTo(w io.Writer) (int64, error) {
	vid := t.NewViewId()
	html := `<div id="`+vid+`" class="`+t.Id+
		`", tabindex="1" style="padding=0; margin:0; width:100%%;height:100%%;">
<canvas id="`+vid+`c" class="mayresize txt1c hastag" width="300" height="128" style="border:1px solid black;"></canvas>
</div>
<script>
	$(function(){
		var d = $("#`+vid+`");
		var x = $("#`+vid+`c").get(0);
		x.tag = "text";
		x.lines = [];
		x.lines.push({txt: "", off: 0, eol: true});
		document.mktext(d, x, 0, "`+t.Id+`", "`+vid+`");
	});
</script>
`
	n, err := io.WriteString(w, html)
	return int64(n), err
}

func NewText(id string) *Text {
	t := &Text {
		Ctlr: NewCtlr(id),
		Text: txt.NewEditing(nil),
	}
	go func() {
		for e := range t.In() {
			t.handle(e)
		}
	}()
	return t
}

// XXX: The Ctlr should tell us when a new view starts, we could do its load,
// and also when a view ends, should we want to exit, or cleanup or whatever.
// This could be an event "estart", "eend", with the view id.
// And we can then use t.ViewOut() to post events just to it.

func (t *Text) wrongVers(tag string, wev *Ev) bool {
	vers := t.Vers()
	if wev.Vers == vers+1 {
		return false
	}
	cmd.Dprintf("%s: %s: vers %d != %d+1\n", t.Id, tag, wev.Vers, vers)
	nev := *wev
	nev.Args = []string{"reload"}
	t.ViewOut(wev.Src) <- &nev
	// XXX: and send just to that view the full text again
	// so it re-initializes its frame, that requires sending the text vers
	// as well.
	// The same must be done at init time.
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
	rc := t.Get(off, n)
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
		t.DiscontdEdit()
	default:
	}

	switch ev[0] {
	default:
		cmd.Dprintf("%s: unhandled %v\n", t.Id, ev)
		return
	case "start":
		// XXX: must reload the text into the view
		// use ViewOut() to get the web.Src out chan and
		// then send all the stuff to it
	case "end":
		// nothing to do
	case "tick", "click1", "click2", "click4":
		// t.uevc <- wev
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
		if err := t.Ins(data, p0); err != nil {
			cmd.Dprintf("%s: ins: %s\n", t.Id, err)
			return
		}
		t.ContdEdit()
		cmd.Dprintf("%s: vers %d\n", t.Id, t.Vers())
		t.Out() <- wev
	case "edel", "ecut":
		p0, p1, err := t.p0p1(ev)
		if err!=nil || t.wrongVers(ev[0], wev) {
			return
		}
		if p1 <= p0 {
			return
		}
		rs := t.Del(p0, p1-p0)
		if ev[0] == "ecut" {
			if err := snarf.Set(string(rs)); err != nil {
				cmd.Dprintf("%s: %s: snarf: %s\n", t.Id, ev[0], err)
			}
		} else {
			t.ContdEdit()
		}
		cmd.Dprintf("%s: vers %d\n", t.Id, t.Vers())
		ev[0] = "edel"
		t.Out() <- wev
		// t.uevc <- wev
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
		if err := t.Ins([]rune(s), p0); err != nil {
			cmd.Dprintf("%s: %s: ins: %s\n", t.Id, ev[0], err)
			return
		}
		nev := &Ev{Id: t.Id, Src: "", Vers: t.Vers()}
		nev.Args = []string{"eins", s, wev.Args[1]}
		t.Out() <- nev
		// t.uevc <- nev
	case "eundo", "eredo":
		for {
			var uev *txt.Edit
			if ev[0] == "eundo" {
				uev = t.Undo()
			} else {
				uev = t.Redo()
			}
			if uev == nil {
				cmd.Dprintf("%s: %s: no more\n", t.Id, ev[0])
				return
			}
			cmd.Dprintf("%s: %s: undo1\n", t.Id, ev[0])
			nev := &Ev{Id: t.Id, Src: "", Vers: t.Vers()}
			off := fmt.Sprintf("%d", uev.Off)
			s := string(uev.Data)
			if uev.Op == txt.Eins {
				nev.Args = []string{"eins", s, off}
			} else {
				off2 := fmt.Sprintf("%d", uev.Off+len(s))
				nev.Args = []string{"edel", off, off2}
			}
			t.Out() <- nev
			// t.uevc <- nev
			if !uev.Contd {
				break
			}
		}
	case "intr":
		cmd.Dprintf("%s: intr dump:\n:%s", t.Id, t.Sprint())
		cmd.Dprintf("%s: vers %d\n", t.Id, t.Vers())
		// t.uevc <- wev
	}
}
