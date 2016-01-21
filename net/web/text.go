package web

import (
	"errors"
	"strconv"
	"clive/txt"
	"clive/cmd"
	"bytes"
	"fmt"
	"clive/snarf"
	"sync"
)

// Editable text
struct Text {
	sync.Mutex
	nb int
	*Ctlr
	*txt.Text
	napplies int
}

func (t *Text) HTML() string {
	t.Lock()
	defer t.Unlock()
	t.nb++
	id := fmt.Sprintf("%s", t.Id)
	return `<div id="`+id+`" class="`+t.Id+
		`", tabindex="1" style="padding=0; margin:0; width:100%%;height:100%%;">
<canvas id="`+t.Id+
		`c" class="mayresize txt1c hastag" width="300" height="128" style="border:1px solid black;"></canvas>
</div>
<script>
	$(function(){
		var d = $("#`+t.Id+`");
		var x = $("#`+t.Id+`c").get(0);
		x.tag = "text";
		x.lines = [];
		x.lines.push({txt: "", off: 0, eol: true});
		document.mktext(d, x, 0, "`+t.Id+`", "`+t.Id+`");
	});
</script>
`
}

func NewText(id string) *Text {
	t := &Text {
		Ctlr: NewCtlr(id),
		Text: txt.NewEditing(nil),
	}
	go func() {
		for e := range t.In {
			t.handle(e)
		}
	}()
	return t
}

func (t *Text) wrongVers(tag string, wev *Ev) bool {
	vers := t.Vers()
	if wev.Vers == vers+1 {
		return false
	}
	cmd.Dprintf("%s: %s: vers %d != %d+1\n", t.Id, tag, wev.Vers, vers)
	nev := *wev
	XXX: send all the text with the reload
	use the same event at the start to pre-charge the text.
	nev.Args = []string{"reload"}
	t.Out <- &nev
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
		t.ContdEdit()
	}

	switch ev[0] {
	default:
		cmd.Dprintf("%s: unhandled %v\n", t.Id, ev)
		return
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
		cmd.Dprintf("%s: vers %d\n", t.Id, t.Vers())
		// t.Updc <- wev
		// t.uevc <- wev
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
		}
		cmd.Dprintf("%s: vers %d\n", t.Id, t.Vers())
		ev[0] = "edel"
		// t.Updc <- wev
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
		// t.Updc <- nev
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
			t.Out <- nev
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
