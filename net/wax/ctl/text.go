package ctl

import (
	"bytes"
	"clive/dbg"
	"clive/net/wax"
	"clive/snarf"
	"clive/txt"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"strconv"
)

type Text struct {
	*wax.Conn
	uevc     chan *wax.Ev
	tag      string
	t        *txt.Text
	napplies int
}

var (
	Debug   bool
	dprintf = dbg.FlagPrintf(os.Stderr, &Debug)
)

func NewText(in, out chan *wax.Ev, tag string) *Text {
	inc := make(chan *wax.Ev)
	if out == nil {
		out = make(chan *wax.Ev)
	}
	t := &Text{
		Conn: &wax.Conn{Evc: inc, Updc: out},
		uevc: in,
		tag:  tag,
		t:    txt.NewEditing(nil),
	}
	go t.handler()
	return t
}

func (t *Text) handler() {
	dprintf("%s: handler started\n", t.Id)
	for ev := range t.Evc {
		dprintf("%s: %v\n", t.Id, ev)
		t.handle(ev)
	}
	if t.uevc != nil {
		close(t.uevc, cerror(t.Evc))
	}
	dprintf("%s: handler done\n", t.Id)
}

/*
	Text implements wax.Controller. Not for you to call.
*/
func (t *Text) ShowAt(w io.Writer, nm string) error {
	if t.Id == "" {
		t.Id = nm
	} else {
		nm = t.Id
	}
	id := fmt.Sprintf("%s_%d", t.Id, t.napplies)
	cid := fmt.Sprintf("%s_0", t.Id)
	fmt.Fprintf(w, `<div id="%s" class="%s" tabindex="1" style="padding=0; margin:0; width:100%%;height:100%%;">
		`+"\n", id, cid)
	fmt.Fprintf(w, `<canvas id="%sc" class="mayresize %sc hastag" width="%d" height="%d" style="border:1px solid black;"></canvas>`+"\n", id, cid, 300, 128)
	var buf bytes.Buffer
	fmt.Fprintf(w, "</div>\n")
	fmt.Fprintf(w, `<script>
		$(function(){
			var d = $("#`+id+`");
			var x = $("#`+id+`c").get(0);
			x.tag = "`+html.EscapeString(t.tag)+`";
			x.lines = [];`+"\n")
	off := 0
	vers := t.t.Vers()
	for rs := range t.Get(0, txt.All) {
		for _, r := range rs {
			if r == '\n' {
				s := html.EscapeString(buf.String())
				fmt.Fprintf(w,
					"x.lines.push({txt:\"%s\", off:%d, eol:true});\n", s, off)
				off += buf.Len() + 1
				buf.Reset()
				continue
			}
			fmt.Fprintf(&buf, "%c", r)
		}
		if buf.Len() > 0 {
			s := html.EscapeString(buf.String())
			fmt.Fprintf(w, "x.lines.push({txt:\"%s\", off:%d});\n", s, off)
			off += buf.Len() + 1
			buf.Reset()
		} else {
			fmt.Fprintf(w, "x.lines.push({txt:\"\", off:%d});\n", off)
		}
	}
	fmt.Fprintf(w, `document.mktext(d, x, %d, "`+cid+`", "`+id+`");`, vers)
	fmt.Fprintf(w, "\n});\n</script>\n")
	return nil
}

func (t *Text) Label() string {
	return t.tag
}

// like that of txt.Text.
func (t *Text) Len() int {
	return t.t.Len()
}

// like that of txt.Text.
func (t *Text) Undo() *txt.Edit {
	// XXX: and send update
	return t.t.Undo()
}

// like that of txt.Text.
func (t *Text) Redo() *txt.Edit {
	// XXX: and send update
	return t.t.Redo()
}

// like that of txt.Text.
func (t *Text) ContdEdit() {
	t.t.ContdEdit()
}

// like that of txt.Text.
func (t *Text) Ins(data []rune, off int) error {
	// XXX: and send update
	return t.t.Ins(data, off)
}

// like that of txt.Text.
func (t *Text) Del(off, n int) []rune {
	// XXX: and send update
	return t.t.Del(off, n)
}

// like that of txt.Text.
func (t *Text) Get(off int, n int) <-chan []rune {
	return t.t.Get(off, n)
}

func (t *Text) GetString(off int, n int) string {
	rc := t.t.Get(off, n)
	var buf bytes.Buffer
	for rs := range rc {
		fmt.Fprintf(&buf, "%s", string(rs))
	}
	return buf.String()
}

// like that of txt.Text.
func (t *Text) Getc(off int) rune {
	return t.t.Getc(off)
}

// like that of txt.Text.
func (t *Text) Sprint() string {
	return t.t.Sprint()
}

// like that of txt.Text.
func (t *Text) DelAll() {
	// XXX: and send update
	t.t.DelAll()
}

// like that of txt.Text.
func (t *Text) Vers() int {
	return t.t.Vers()
}

func (t *Text) wrongVers(tag string, wev *wax.Ev) bool {
	vers := t.t.Vers()
	if wev.Vers == vers+1 {
		return false
	}
	dprintf("%s: %s: vers %d != %d+1\n", t.Id, tag, wev.Vers, vers)
	nev := *wev
	nev.Args = []string{"reload"}
	t.Updc <- &nev
	return true
}

func (t *Text) p0p1(ev []string) (int, int, error) {
	if len(ev) < 3 {
		dprintf("%s: %s: short\n", t.Id, ev[0])
		return 0, 0, errors.New("short event")
	}
	p0, err := strconv.Atoi(ev[1])
	if err != nil {
		dprintf("%s: %s: p0: %s\n", t.Id, ev[0], err)
		return 0, 0, errors.New("bad p0")
	}
	p1, err := strconv.Atoi(ev[2])
	if err != nil {
		dprintf("%s: %s: p1: %s\n", t.Id, ev[0], err)
		return 0, 0, errors.New("bad p1")
	}
	return p0, p1, nil
}

func (t *Text) handle(wev *wax.Ev) {
	if wev == nil || len(wev.Args) < 1 {
		return
	}

	ev := wev.Args
	switch ev[0] {
	case "tick", "ecut", "epaste", "ecopy", "eintr", "eundo", "eredo":
		t.t.DiscontdEdit()
	default:
		t.t.ContdEdit()
	}

	switch ev[0] {
	default:
		dprintf("%s: unhandled %v\n", t.Id, ev)
		return
	case "tick", "click1", "click2", "click4":
		if t.uevc != nil {
			t.uevc <- wev
		}
	case "eins":
		if len(ev) < 3 {
			dprintf("%s: ins: short\n", t.Id)
			return
		}
		if t.wrongVers("ins", wev) {
			return
		}
		p0, err := strconv.Atoi(ev[2])
		if err != nil {
			dprintf("%s: ins: %s\n", t.Id, err)
			return
		}
		data := []rune(ev[1])
		if len(data) == 0 {
			return
		}
		if err := t.Ins(data, p0); err != nil {
			dprintf("%s: ins: %s\n", t.Id, err)
			return
		}
		dprintf("vers %d\n", t.t.Vers())
		t.Updc <- wev
		if t.uevc != nil {
			t.uevc <- wev
		}
	case "edel", "ecut":
		p0, p1, err := t.p0p1(ev)
		if err != nil || t.wrongVers(ev[0], wev) {
			return
		}
		if p1 <= p0 {
			return
		}
		rs := t.t.Del(p0, p1-p0)
		if ev[0] == "ecut" {
			if err := snarf.Set(string(rs)); err != nil {
				dprintf("%s: %s: snarf: %s\n", t.Id, ev[0], err)
			}
		}
		dprintf("vers %d\n", t.t.Vers())
		ev[0] = "edel"
		t.Updc <- wev
		if t.uevc != nil {
			t.uevc <- wev
		}
	case "ecopy":
		p0, p1, err := t.p0p1(ev)
		if err != nil || t.wrongVers(ev[0], wev) {
			return
		}
		s := ""
		if p1 > p0 {
			s = t.GetString(p0, p1-p0)
		}
		if err := snarf.Set(s); err != nil {
			dprintf("%s: %s: snarf: %s\n", t.Id, ev[0], err)
		}
	case "epaste":
		p0, _, err := t.p0p1(ev)
		if err != nil || t.wrongVers(ev[0], wev) {
			return
		}
		s, err := snarf.Get()
		if err != nil {
			dprintf("%s: %s: snarf: %s\n", t.Id, ev[0], err)
			return
		}
		if s == "" {
			return
		}
		if err := t.t.Ins([]rune(s), p0); err != nil {
			dprintf("%s: %s: ins: %s\n", t.Id, ev[0], err)
			return
		}
		nev := &wax.Ev{Id: wev.Id, Src: "", Vers: t.t.Vers()}
		nev.Args = []string{"eins", s, wev.Args[1]}
		t.Updc <- nev
		if t.uevc != nil {
			t.uevc <- nev
		}
	case "eundo", "eredo":
		for {
			var uev *txt.Edit
			if ev[0] == "eundo" {
				uev = t.t.Undo()
			} else {
				uev = t.t.Redo()
			}
			if uev == nil {
				dprintf("%s: %s: no more\n", t.Id, ev[0])
				return
			}
			nev := &wax.Ev{Id: wev.Id, Src: "", Vers: t.t.Vers()}
			off := fmt.Sprintf("%d", uev.Off)
			s := string(uev.Data)
			if uev.Op == txt.Eins {
				nev.Args = []string{"eins", s, off}
			} else {
				off2 := fmt.Sprintf("%d", uev.Off+len(s))
				nev.Args = []string{"edel", off, off2}
			}
			t.Updc <- nev
			if t.uevc != nil {
				t.uevc <- nev
			}
			if !uev.Contd {
				break
			}
		}
	case "intr":
		dprintf("%s intr dump:\n:%s", t.Id, t.t.Sprint())
		dprintf("vers %d\n", t.t.Vers())
		if t.uevc != nil {
			t.uevc <- wev
		}
	}
}
