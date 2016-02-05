/*
	Simple editor mostly to test the ink UI framework
	Creates a text control to edit the text and then prints the text
	when it exits.
*/
package main

import (
	"clive/cmd"
	"clive/zx"
	"clive/cmd/opt"
	"clive/net/ink"
	"time"
	"fmt"
	"bytes"
)

// Example of how to update the text from the API while the user edits it,
// again, mostly for testing.
func edits(t *ink.Txt) {
	time.Sleep(5*time.Second)
	t.Ins([]rune("ZZZ\n"), 3)
	time.Sleep(1*time.Second)
	rs := t.Del(3, 4)
	cmd.Dprintf("did del %s\n", string(rs))
	time.Sleep(1*time.Second)
	x := t.GetText()
	x.Ins([]rune("XXX\n"), 2)
	x.Ins([]rune("XXX\n"), x.Len())
	t.SetMark("xx", 30)
	t.PutText()
	for i := 0; i < 30; i++ {
		time.Sleep(time.Second)
		t.MarkIns("xx", []rune(fmt.Sprintf("--%d--\n", i)))
	}
}

func edit(t *ink.Txt) {
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

func buttons(bs *ink.ButtonSet, rs *ink.RadioSet, t *ink.Txt) {
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
	doedits = false
)

func main() {
	opts := opt.New("[file]")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	rdonly := false
	opts.NewFlag("r", "read only", &rdonly)
	opts.NewFlag("e", "do edits for testing", &doedits)
	cmd.UnixIO("err")
	args := opts.Parse()
	var t *ink.Txt
	inkout := cmd.Out("ink")
	if inkout == nil {
		cmd.UnixIO()
	}
	if len(args) == 0 {
		t = ink.NewTxt("1234", "abc")
	} else {
		dat, err := zx.GetAll(cmd.NS(), cmd.AbsPath(args[0]))
		if err != nil {
			cmd.Fatal(err)
		}
		t = ink.NewTxt(args[0] + " Del", string(dat))
	}
	go edit(t)
	if rdonly {
		t.NoEdits()
	}
	ink.UsePort("8182")
	bs := ink.NewButtonSet(&ink.Button{Tag: "One", Name: "one"},
		&ink.Button{Tag: "Two", Name: "two"},
		&ink.Button{Tag: "B", Name: "b", Value: &bold},
		&ink.Button{Tag: "I", Name: "i", Value: &italic})
	rs := ink.NewRadioSet(&style, &ink.Button{Tag: "R", Name: "r"},
		&ink.Button{Tag: "T", Name: "t"})
	go buttons(bs, rs, t)

	pg := ink.NewPg("/", "Example text editing:", bs, rs, t)
	pg.Tag = "Clive's iedit"
	if doedits {
		go edits(t)
	}
	go ink.Serve()
	if inkout != nil {
		// TODO: ctlrs must use unique ids sytem-wide, or
		// controls written won't work because of their ids.
		inkout <- []byte(`<tt>Hi there, this is HTML</tt>`)
		var buf bytes.Buffer
		bs.WriteTo(&buf)
		inkout <- buf.Bytes()
		inkout <- []byte("https://localhost:8182")
	}
	t.Wait()
	for rs := range t.Get(0, -1) {
		cmd.Printf("%s", string(rs))
	}
}
