/*
	read files for clive pipes
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
)

var opts = opt.New("{file}")

func main() {
	cmd.UnixIO("err")
	cmd.UnixIO("in")
	args := opts.Parse()
	if len(args) != 0 {
		in := cmd.Files(args...)
		cmd.SetIn("in", in)
	}
	out := cmd.Out("out")
	in := cmd.In("in")
	for m := range in {
		if ok := out <- m; !ok {
			close(in, cerror(out))
		}
	}
	if err := cerror(in); err != nil {
		cmd.Fatal(err)
	}
}
