/*
	print each file as a single msg.
*/
package main

import (
	"bytes"
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx"
)

var (
	opts = opt.New("{file}")
	one  bool
)

// Run print lines in the current app context.
func main() {
	c := cmd.AppCtx()
	cmd.UnixIO("err")
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("1", "collect all files (not one msg per file)", &one)
	ux := false
	opts.NewFlag("u", "use unix out", &ux)
	args := opts.Parse()
	if ux {
		cmd.UnixIO("out")
	}
	if len(args) != 0 {
		cmd.SetIn("in", cmd.Files(args...))
	}
	buf := &bytes.Buffer{}
	in := cmd.In("in")
	out := cmd.Out("out")
	for m := range in {
		switch m := m.(type) {
		case []byte:
			buf.Write(m)
		case zx.Dir:
			if !one && buf.Len() > 0 {
				if ok := out <- buf.Bytes(); !ok {
					close(in, cerror(out))
					break
				}
				buf = &bytes.Buffer{}
			}
			if !one && !ux {
				if ok := out <- m; !ok {
					close(in, cerror(out))
					break
				}
			}
		case error:
			cmd.Warn("%s", m)
		default:
			cmd.Dprintf("ignored %T\n", m)
			if !ux {
				if ok := out <- m; !ok {
					close(in, cerror(out))
					break
				}
			}
		}
	}
	if buf.Len() > 0 {
		out <- buf.Bytes()
	}
	if err := cerror(in); err != nil {
		cmd.Fatal(err)
	}
}
