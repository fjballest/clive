/*
	read files for clive pipes
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/ns"
)

var opts = opt.New("{file}")

func main() {
	ns.AddLfsPath("/", nil)
	cmd.UnixIO("err")
	cmd.UnixIO("in")
	args, err := opts.Parse()
	if err != nil {
		cmd.Warn("%s", err)
		opts.Usage()
	}
	if len(args) != 0 {
		in := cmd.Files(args...)
		cmd.SetIO("in", in)
	}
	out := cmd.IO("out")
	in := cmd.IO("in")
	for m := range in {
		if ok := out <- m; !ok {
			close(in, cerror(out))
		}
	}
	close(out, cerror(in))
}
