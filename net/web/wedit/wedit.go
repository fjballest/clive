/*
	Simple editor mostly to test the web UI framework
	Creates a text control to edit the text and then prints the text
	when it exits.
*/
package main

import (
	"clive/cmd"
	"clive/zx"
	"clive/cmd/opt"
	"clive/net/web"
	"time"
)

func edits(t *web.Text) {
	time.Sleep(3)
	t.Ins([]rune("bar"), 0)
	t.Ins([]rune("foo"), 8)
	t.Del(8, 3)
}

func edit(t *web.Text) {
	in := t.Events()
	for ev := range in {
		cmd.Warn("got text: %v", ev.Args)
		switch ev.Args[0] {
		case "start":
			continue
			// Example: keep only a single view
			vs := t.Views()
			for _, v := range vs {
				if v != ev.Src {
					t.CloseView(v)
				}
			}
			// Example: do some edits from the program.
			go edits(t)
		case "tag":
			if len(ev.Args) == 1 || ev.Args[1] != "Del" {
				continue
			}
			t.Close()
		case "end":
			// Example: delete the text when all views are gone
			vs := t.Views()
			cmd.Dprintf("views %v\n", t.Views())
			if len(vs) == 0 {
				t.Close()
				return
			}
		}
	}
}

func buttons(bs *web.ButtonSet, rs *web.RadioSet, t *web.Text) {
	in := bs.Events()
	rs.SendEventsTo(in)
	for ev := range in {
		cmd.Warn("buttons: %v", ev.Args)
		if ev.Args[0] == "Set" {
			s := style
			if bold {
				s += "b"
			}
			if italic {
				s += "i"
			}
			t.SetFont(s);
		}
	}
}

var (
	bold, italic bool
	style = "r"
)

func main() {
	opts := opt.New("[file]")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	rdonly := false
	opts.NewFlag("r", "read only", &rdonly)
	cmd.UnixIO()
	args := opts.Parse()
	var t *web.Text
	if len(args) == 0 {
		t = web.NewText("1234", "abc")
	} else {
		dat, err := zx.GetAll(cmd.NS(), cmd.AbsPath(args[0]))
		if err != nil {
			cmd.Fatal(err)
		}
		t = web.NewTaggedText(args[0] + " Del", string(dat))
	}
	go edit(t)
	if rdonly {
		t.NoEdits()
	}

	bs := web.NewButtonSet(&web.Button{Tag: "One", Name: "one"},
		&web.Button{Tag: "Two", Name: "two"},
		&web.Button{Tag: "B", Name: "b", Value: &bold},
		&web.Button{Tag: "I", Name: "i", Value: &italic})
	rs := web.NewRadioSet(&style, &web.Button{Tag: "R", Name: "r"},
		&web.Button{Tag: "T", Name: "t"})
	go buttons(bs, rs, t)

	web.NewPg("/", "Example text editing:", bs, rs, t)
	web.ServeLoginFor("/")

	go web.Serve(":8181")
	t.Wait()
	for rs := range t.Get(0, -1) {
		cmd.Printf("%s", string(rs))
	}
}
