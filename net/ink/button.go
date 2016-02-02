package ink

import (
	"io"
	"fmt"
	"html"
	"clive/cmd"
	"strconv"
)

// A single button
struct Button {
	Name string	// reported in events
	Tag string		// shown in the button
	Value *bool	// nil, or on/off value for check buttons
	value bool
}

// Events sent from the viewer:
//	click name nb
//	Set  name nb on|off
// Events sent from the viewer but not for the user:
//	id
//	tag str
// Events sent to the user (besides those from the viewer):
//	start
//	end

// A set of buttons
// See Ctlr for the common API for controls.
// The events posted to the user are:
//	start
//	end
//	click name nb	(nb is the index in the button array)
//	Set  name nb on|off
struct ButtonSet {
	*Ctlr
	els []*Button
}

// Create a Button Set
// The buttons are check buttons if they have a pointer to a bool
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

// Write the HTML for the button set control to a page.
func (bs *ButtonSet) WriteTo(w io.Writer) (tot int64, err error) {
	vid := bs.newViewId()
	n, err := io.WriteString(w,
		`<div id="`+vid+`" class="`+bs.Id+` ui-widget-header, ui-corner-all">`)
	tot += int64(n)
	if err != nil {
		return tot, err
	}
	bids := []string{}
	for i, b := range bs.els {
		bid := fmt.Sprintf("%s_b%d", vid, i)
		bids = append(bids, bid)
		if b.Value != nil {
			n, err = io.WriteString(w, `<input type="checkbox" id="`+bid+`">` +
				`<label for = "`+bid+`">` + 
				html.EscapeString(b.Tag) + `</label>` + "\n")
		} else {
			n, err = io.WriteString(w, `<button id="`+bid+`">` +
				html.EscapeString(b.Tag) + `</button>` + "\n")
		}
		tot += int64(n)
		if err != nil {
			return tot, err
		}
	}
	n, err = io.WriteString(w, `</div><script>
		$(function(){
			var d = $("#`+vid+`");
			document.mkbuttons(d, "`+bs.Id+`", "`+vid+`");` + "\n")
	tot += int64(n)
	if err != nil {
		return tot, err
	}
	for i, b := range bids {
		if bs.els[i].Value != nil {
			n, err = io.WriteString(w, `$("#`+b+`").button().click(function(e) {
				var checked = $("#`+b+`").is(':checked');
				if(checked) {
					d.post(["Set", "`+bs.els[i].Name+`", "`+fmt.Sprintf("%d", i)+`", "on"]);
				} else {
					d.post(["Set", "`+bs.els[i].Name+`", "`+fmt.Sprintf("%d", i)+`", "off"]);
				}
			});`)
		} else {
			n, err = io.WriteString(w, `$("#`+b+`").button().click(function() {
				d.post(["click", "`+bs.els[i].Name+`", "`+fmt.Sprintf("%d", i)+`"]);
			});`)
		}
		tot += int64(n)
		if err != nil {
			return tot, err
		}
	}
	n, err = io.WriteString(w, `});
		</script>`+"\n")
	tot += int64(n)
	return tot, err
}

func (bs *ButtonSet) update(id string) {
	out := bs.viewOut(id)
	for i, b := range bs.els {
		if b.Value == nil {
			continue
		}
		v := "on"
		if !b.value {
			v = "off"
		}
		ev := &Ev{Id: bs.Id, Src: id+"u", Args: []string{
			"Set", b.Name, fmt.Sprintf("%d", i), v}}
		out <- ev
	}
}

func (bs *ButtonSet) handle(wev *Ev) {
	if wev==nil || len(wev.Args)<1 {
		return
	}
	ev := wev.Args
	switch ev[0] {
	case "start":
		bs.update(wev.Src)
		bs.post(wev)
	case "end":
		bs.post(wev)
	case "click", "Set":
		if len(ev) < 3 {
			return
		}
		n, _ := strconv.Atoi(ev[2])
		if n < 0 || n >= len(bs.els) {
			return
		}
		b := bs.els[n]
		if b.Value != nil && len(ev) > 3 {
			b.value = ev[3] == "on"
			*b.Value = b.value
		}
		bs.post(wev)
	default:
		cmd.Dprintf("%s: unhandled %v\n", bs.Id, ev)
		return
	}
}
