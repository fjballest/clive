package web

import (
	"io"
	"fmt"
	"html"
	"clive/cmd"
	"strconv"
)

// Events sent from the viewer:
//	click name nb
//	Set  name nb on|off
// Events sent from the viewer but not for the user:
//	id
// Events sent to the user (besides those from the viewer):
//	start
//	end

// A set of radio buttons
// See Ctlr for the common API for controls.
// The events posted to the user are:
//	start
//	end
//	Set  name idx on
struct RadioSet {
	*Ctlr
	Value *string	// current value
	els []*Button
}

// Create a Radio button Set
// The buttons are check buttons if they have a pointer to a bool
func NewRadioSet(value *string, button ...*Button) *RadioSet {
	bs := &RadioSet {
		Value: value,
		Ctlr: newCtlr("buttons"),
		els: button,
	}
	for _, b := range button {
		if *value == b.Name {
			b.value = true
			break
		}
	}
	go func() {
		for e := range bs.in {
			bs.handle(e)
		}
	}()
	return bs
}

// Write the HTML for the radio set control to a page.
func (bs *RadioSet) WriteTo(w io.Writer) (tot int64, err error) {
	vid := bs.newViewId()
	n, err := io.WriteString(w,
		`<form><div id="`+vid+`" class="`+bs.Id+`">`)
	tot += int64(n)
	if err != nil {
		return tot, err
	}
	bids := []string{}
	for i, b := range bs.els {
		bid := fmt.Sprintf("%s_b%d", vid, i)
		bids = append(bids, bid)
		n, err = io.WriteString(w, `<input type="radio" id="`+bid+`" name="`+vid+`">` +
				`<label for = "`+bid+`">` + 
				html.EscapeString(b.Tag) + `</label>` + "\n")
		tot += int64(n)
		if err != nil {
			return tot, err
		}
	}
	n, err = io.WriteString(w, `</div></form><script>
		$(function(){
			var d = $("#`+vid+`");
			document.mkradio(d, "`+bs.Id+`", "`+vid+`");
			$("#`+vid+`").buttonset();` + "\n")
	tot += int64(n)
	if err != nil {
		return tot, err
	}
	for i, b := range bids {
		n, err = io.WriteString(w, `$("#`+b+`").change(function() {
			console.log("change", "`+b+`");
			d.post(["Set", "`+bs.els[i].Name+`", "`+fmt.Sprintf("%d", i)+`", "on"]);
		});`)
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

func (bs *RadioSet) update(id string) {
	out := bs.viewOut(id)
	for i, b := range bs.els {
		v := "on"
		if !b.value {
			v = "off"
		}
		ev := &Ev{Id: bs.Id, Src: id+"u", Args: []string{
			"Set", b.Name, fmt.Sprintf("%d", i), v}}
		out <- ev
	}
}

func (bs *RadioSet) handle(wev *Ev) {
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
	case "Set":
		if len(ev) < 4 {
			return
		}
		n, _ := strconv.Atoi(ev[2])
		if n < 0 || n >= len(bs.els) {
			return
		}
		for i, b := range bs.els {
			b.value = i == n
			if b.Value != nil  {
				*b.Value = b.value
			}
			if b.value && bs.Value != nil {
				*bs.Value = b.Name
			}
		}
		bs.post(wev)
	default:
		cmd.Dprintf("%s: unhandled %v\n", bs.Id, ev)
		return
	}
}
