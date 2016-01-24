package web

import (
	"io"
	"fmt"
	"html"
	"clive/cmd"
)

// A single button
struct Button {
	Tag string
	Enabled bool
	Value *bool
}

// TODO: complete this
// XXX: Must open the web socket and post events just like everybody else.
// Make a mkbuttonset function in button.js to post the events
// Add handlers to the buttons, to post the relevant events.
// Clicks are easy
// Checks must report on/off events
// Radio must report one of the listed buttons each time.
// Update for checks must set on/off
// Update for radio must set the one currently set

// A set of buttons
struct ButtonSet {
	*Ctlr
	els []*Button
}

func NewButtonSet(button ...*Button) *ButtonSet {
	bs := &ButtonSet {
		Ctlr: newCtlr("buttons"),
		els: button,
	}
	go func() {
		for e := range bs.in {
			bs.handle(e)
		}
	}()
	return bs
}

func (bs *ButtonSet) WriteTo(w io.Writer) (tot int64, err error) {
	vid := bs.newViewId()
	n, err := io.WriteString(w,
		`<div id="`+vid+`" class="`+bs.Id+`, ui-widget-header, ui-corner-all">`)
	tot += int64(n)
	if err != nil {
		return tot, err
	}
	bids := []string{}
	for i, b := range bs.els {
		bid := fmt.Sprintf("%s_b%d", bs.Id, i)
		bids = append(bids, bid)
		n, err := io.WriteString(w, `<button id="`+bid+`">` +
			html.EscapeString(b.Tag) + `</button>` + "\n")
		tot += int64(n)
		if err != nil {
			return tot, err
		}
	}
	n, err = io.WriteString(w, `</div>`+"<script>\n")
	for _, b := range bids {
		n, err := io.WriteString(w, `$("#`+b+`").button();`+"\n")
		tot += int64(n)
		if err != nil {
			return tot, err
		}
	}
	n, err = io.WriteString(w, `</script>`+"\n")
	tot += int64(n)
	return tot, err
}

func (bs *ButtonSet) update(toid string) {
	to := bs.viewOut(toid)
	for i, b := range bs.els {
		ev := &Ev{Id: bs.Id, Src: "", Args: []string{
			"set", fmt.Sprintf("%d", i), fmt.Sprintf("%v", *b.Value),
		}}
		if ok := to <- ev; !ok {
			return
		}
	}
}

func (bs *ButtonSet) handle(wev *Ev) {
	if wev==nil || len(wev.Args)<1 {
		return
	}
	ev := wev.Args
	switch ev[0] {
	case "start", "end", "set":
		bs.update(wev.Src)
		bs.post(wev)
	default:
		cmd.Dprintf("%s: unhandled %v\n", bs.Id, ev)
		return
	}
}
