/*
	Provides controls that implement wax.Controller and
	can be used within wax parts.

	See the implementation of Entry as an example of how
	to add new controls.
*/
package ctl

import (
	"bytes"
	"clive/net/wax"
	"fmt"
	"html"
	"io"
)

/*
	A button, to be used in Bars
*/
type Button string

/*
	A check button, to be used in Bars
*/
type Check string

/*
	A radio button, to be used in Bars
*/
type Radio string

/*
	A bar of buttons. Runs of radio buttons correspond to a single
	radio box.
	Once created and applied to a wax page the channels can be used
	to receive events an post udpates.
*/
type Bar  {
	*wax.Conn
	Buttons  []interface{} // must contain only Button, Radio, and Check
	napplies int
}

/*
	Convenience: create a Bar
*/
func NewBar(in, out chan *wax.Ev, buttons ...interface{}) *Bar {
	return &Bar{
		Conn:    &wax.Conn{Evc: in, Updc: out},
		Buttons: buttons,
	}
}

const (
	normalB = iota
	checkB
	radioB
)

func handle(w io.Writer, wsid, id, cid string, kind int) {
	js := `
		$( "#` + id + `" )
		.on("click", function (e) {
			var ws = partws;
			if(!partws){
				console.log("no ws");
				return;
			}
			var ev = {};
			ev.Id = "` + cid + `";
			if(e.target.checked === undefined || e.target.checked){
				ev.Args = ["Set", "on"];
			} else {
				ev.Args = ["Set", "off"];
			}
			var msg = JSON.stringify(ev);
			console.log("click " + msg);
			ws.send(msg);
		})
	`
	switch kind {
	case checkB:
		fmt.Fprintf(w, "%s", js)
		js = `.get(0).update = function(e) {
			if(e && e.Args) {
				console.log("click " + e.Args[1]);
				this.checked = (e.Args[1] == "on");
			}
		};`
		fmt.Fprintf(w, "%s\n", js)
	case radioB:
		fmt.Fprintf(w, "%s", js)
		js = `.get(0).update = function(e) {
			console.log("click " + e.Args[1] + " " + this.checked);
			if(e && e.Args) {
				var v = 'checked';
				this.checked = e.Args[1] == "on";
				$("#"+this.id).closest(".ui-buttonset").buttonset("refresh");
			}
		};`
		fmt.Fprintf(w, "%s\n", js)
	default:
		fmt.Fprintf(w, "%s;\n", js)
	}
}

/*
	Bar implements wax.Controller. Not for you to call.
*/
func (bar *Bar) ShowAt(w io.Writer, nm string) error {
	var js bytes.Buffer
	if bar.Id == "" {
		bar.Id = nm
	} else {
		nm = bar.Id
	}
	nm = fmt.Sprintf("%s_%d", bar.Id, bar.napplies)
	cnm := fmt.Sprintf("%s_0", bar.Id)
	set := bar.Buttons
	fmt.Fprintf(w, `<div id="%s" class="ui-widget-header ui-corner-all">`+"\n", nm)
	for i := 0; i < len(set); {
		switch b := set[i].(type) {
		default:
			return fmt.Errorf("bar show: unknown type %T", b)
		case Button:
			id := fmt.Sprintf("%s_%d", nm, i)
			cid := fmt.Sprintf("%s_%d", cnm, i)
			fmt.Fprintf(w, "\t<button id=\"%s\" class=\"%s\">%s</button>\n",
				id, cid, html.EscapeString(string(b)))
			handle(&js, nm, id, cid, normalB)
			i++
		case Radio:
			rid := fmt.Sprintf("%s_r%d", nm, i)
			fmt.Fprintf(w, "\t<span id=\"%s\">\n", rid)
			for ; i < len(set); i++ {
				rb, ok := set[i].(Radio)
				if !ok {
					break
				}
				id := fmt.Sprintf("%s_%d", nm, i)
				cid := fmt.Sprintf("%s_%d", cnm, i)
				fmt.Fprintf(w, "\t\t"+
					`<input type="radio" id="%s" name="%s" class="%s">`,
					id, rid, cid)
				fmt.Fprintf(w, `<label for="%s">%s</label>`+"\n",
					id, html.EscapeString(string(rb)))
				handle(&js, nm, id, cid, radioB)
			}
			fmt.Fprintf(w, "\t</span>\n")
			fmt.Fprintln(&js, `$("#`+rid+`").buttonset();`)
		case Check:
			id := fmt.Sprintf("%s_%d", nm, i)
			cid := fmt.Sprintf("%s_%d", cnm, i)
			fmt.Fprintf(w, "\t"+`<input type="checkbox" id="%s" class="%s">`,
				id, cid)
			fmt.Fprintf(w, `<label for="%s">%s</label>`+"\n",
				id, html.EscapeString(string(b)))
			handle(&js, nm, id, cid, checkB)
			i++
		}
	}
	fmt.Fprintf(w, "</div>\n")
	fmt.Fprintf(w, "<script>\n%s\n</script>\n", js.String())
	bar.napplies++
	return nil
}

/*
	A menu label (option)
*/
type Label string

/*
	A menu with labels (options) and sub-menus.
	Once created and applied to a wax page, the channels are used
	to retrieve events and post updates.
*/
type Menu  {
	*wax.Conn
	Label    string
	Opts     []interface{} // Must contain only Label or Menu entries
	napplies int
}

/*
	Convenience: create a Menu
*/
func NewMenu(in, out chan *wax.Ev, opts ...interface{}) *Menu {
	return &Menu{
		Conn: &wax.Conn{Evc: in, Updc: out},
		Opts: opts,
	}
}

func (m *Menu) showAt(w io.Writer, nm, cnm string) error {
	fmt.Fprintf(w, `<ul id="%s">`+"\n", nm)
	for i := 0; i < len(m.Opts); i++ {
		id := fmt.Sprintf("%s_%d", nm, i)
		cid := fmt.Sprintf("%s_%d", cnm, i)
		switch b := m.Opts[i].(type) {
		default:
			return fmt.Errorf("unknown type %T", b)
		case Label:
			fmt.Fprintf(w, "\t<li id=\"%s\" class=\"%s\"><a href=\"#\">%s</a></li>\n",
				id, cid, html.EscapeString(string(b)))
		case Menu:
			fmt.Fprintf(w, "\t<li id=\"%s\"><a href=\"#\">%s</a>\n",
				id, html.EscapeString(b.Label))
			b.showAt(w, id, cid)
			fmt.Fprintf(w, "</li>\n")
		}
	}
	fmt.Fprintf(w, "</ul>\n")
	return nil
}

/*
	Menu implements wax.Controller. Not for you to call.
*/
func (m *Menu) ShowAt(w io.Writer, nm string) error {
	if m.Id == "" {
		m.Id = nm
	} else {
		nm = m.Id
	}
	id := fmt.Sprintf("%s_%d", m.Id, m.napplies)
	cid := fmt.Sprintf("%s_0", m.Id)
	m.showAt(w, id, cid)
	js := `
	<script>
		$(function() {
			$( "#` + id + `" ).menu()
			.click(function(e) {
				var ws = partws;
				if(!ws){
					console.log("no ws ");
					return;
				}
				var ev = {}
				ev.Id = "` + cid + `";
				ev.Args = ["exec", e.target.text];
				var msg = JSON.stringify(ev)
				console.log("menu " + msg);
				ws.send(msg);
			});
		});
	</script>`
	fmt.Fprintf(w, "%s\n", js)
	m.napplies++
	return nil
}

/*
	A standalone entry.
	Once created and applied to a wax page the channels can be used
	to receive events an post udpates.
*/
type Entry  {
	*wax.Conn
	Label    string
	napplies int
}

/*
	Convenience: create a new Entry
*/
func NewEntry(in, out chan *wax.Ev, lbl string) *Entry {
	return &Entry{
		Conn:  &wax.Conn{Evc: in, Updc: out},
		Label: lbl,
	}
}

/*
	Entry implements wax.Controller. Not for you to call.
*/
func (b *Entry) ShowAt(w io.Writer, nm string) error {
	if b.Id == "" {
		b.Id = nm
	} else {
		nm = b.Id
	}
	id := fmt.Sprintf("%s_%d", b.Id, b.napplies)
	cid := fmt.Sprintf("%s_0", b.Id)
	fmt.Fprintf(w, `<span>%s:<input id="%s" class="%s"></span>`,
		html.EscapeString(b.Label), id, cid)
	js := `
		<script>
		$(function() {
			$( "#` + id + `" ).change(function (e) {
				var ws = partws;
				if(!ws){
					console.log("no ws");
					return
				}
				console.log("entry " );
				console.log("change " + " id " + e.target.id +
					" data: " + e.target.value);
				var ev = {}
				ev.Id = "` + cid + `";
				ev.Args = ["Set", e.target.value];
				var msg = JSON.stringify(ev)
				console.log("entry " + msg);
				ws.send(msg)
			}).get(0).update = function(e) {
				if(e && e.Args) {
					this.value = e.Args[1];
				}
			};
		});
		</script>
	`
	fmt.Fprintf(w, "%s\n", js)
	b.napplies++
	return nil
}

func init() {

	var m, b, e interface{}
	m = &Menu{}
	if _, ok := m.(wax.Controller); !ok {
		panic("menu is not a controller")
	}
	b = &Bar{}
	if _, ok := b.(wax.Controller); !ok {
		panic("bar is not a controller")
	}
	e = &Entry{}
	if _, ok := e.(wax.Controller); !ok {
		panic("entry is not a controller")
	}
}
