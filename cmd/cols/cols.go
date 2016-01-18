/*
	columnate text
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/cmd/tty"
	"clive/zx"
	"unicode/utf8"
	"strings"
	"bytes"
	"fmt"
)

var (
	opts = opt.New("{file}")
	wid, ncols int
	words      []string
	maxwid     int
	ux bool
)

func col() {
	if wid == 0 {
		wid, _ = tty.Cols()
		if wid == 0 {
			wid = 70
		}
	}
	colwid := maxwid + 2
	if ncols == 0 {
		ncols = wid / colwid
	}
	if ncols == 0 {
		ncols = 1
	}
	var buf bytes.Buffer
	for i, w := range words {
		nw := utf8.RuneCountInString(w)
		spcs := ""
		if nw < colwid-1 {
			spcs = strings.Repeat(" ", colwid-1-nw)
		}
		fmt.Fprintf(&buf, "%s%s", w, spcs)
		if (i+1)%ncols == 0 {
			fmt.Fprintf(&buf, "\n")
		}
	}
	if len(words)%ncols != 0 {
		fmt.Fprintf(&buf,"\n")
	}
	cmd.Out("out") <- buf.Bytes()
}

func add(ws ...string) {
	for _, w := range ws {
		if len(w) > maxwid {
			maxwid = len(w)
		}
	}
	words = append(words, ws...)
}

// Run cols in the current app context.
func main() {
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("w", "wid: set max line width", &wid)
	opts.NewFlag("n", "ncols: set number of columns", &ncols)
	opts.NewFlag("u", "use unix output", &ux)
	cmd.UnixIO("err")
	args := opts.Parse()
	if ux {
		cmd.UnixIO("out")
	}
	if len(args) != 0 {
		cmd.SetIn("in", cmd.Files(args...))
	}
	in := cmd.In("in")
	for m := range in {
		switch m := m.(type) {
		default:
			// ignored & forwarded
			cmd.Dprintf("got %T\n", m)
			continue
		case zx.Dir:
			cmd.Dprintf("got %T %s\n", m, m["Upath"])
			add(strings.TrimSpace(m["name"]))
		case error:
			if m != nil {
				cmd.Warn("%s", m)
			}
		case []byte:
			cmd.Dprintf("got %T [%d]\n", m, len(m))
			words := strings.Fields(strings.TrimSpace(string(m)))
			add(words...)
		}
	}
	col()
	if err := cerror(in); err != nil {
		cmd.Fatal("in %s", err)
	}
}
