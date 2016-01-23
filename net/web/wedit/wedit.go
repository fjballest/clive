/*
	Simple editor mostly to test the web UI framework
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/net/web"
)

func edit(t *web.Text) {
	in := t.Events()
	for ev := range in {
		cmd.Warn("got %v", ev.Args)
		switch ev.Args[0] {
		case "start":
			// Example: keep only a single view
			vs := t.Views()
			for _, v := range vs {
				if v != ev.Src {
					t.CloseView(v)
				}
			}
		case "end":
			// Example: delete the text when all views are gone
			vs := t.Views()
			cmd.Dprintf("views %v", t.Views())
			if len(vs) == 0 {
				t.Close()
				return
			}
		}
	}
}

func main() {
	opts := opt.New("")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	cmd.UnixIO()
	opts.Parse()
	t := web.NewText("txt1 Del", "1234", "abc")
	go edit(t)
	web.NewPg("/", t)
	web.Serve()
}
