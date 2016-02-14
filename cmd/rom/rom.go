/*
	Eat files and generate static Go data for them.
	Used to include external data files into server binaries.
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx"
)

var (
	opts = opt.New("{file}")
	name = "rom"
)

func bytes(m []byte) {
	if len(m) == 0 {
		cmd.Printf("[]byte{},\n")
		return
	}
	cmd.Printf("[]byte{\n\t\t")
	defer cmd.Printf("\t\t},\n")
	for i, b := range m {
		cmd.Printf("%d,", b)
		if i%16 == 0 && i > 0 && i < len(m)-1 {
			cmd.Printf("\n\t\t")
		}
	}
	if len(m)%16 != 0 {
		cmd.Printf("\n")
	}
}

func rom(in <-chan face{}) {
	cmd.Printf("package %s\n", name)
	cmd.Printf("var Files = map[string][]byte{\n")
	defer cmd.Printf("}\n")
	open := false
	for m := range in {
		cmd.Dprintf("got %T\n", m)
		switch m := m.(type) {
		case error:
			cmd.Fatal("errors: %s", m)
		case zx.Dir:
			if open {
				cmd.Printf("[]byte{},\n")
			}
			nm := m["Upath"]
			cmd.Printf("\t%q: ", nm)
			cmd.Warn("%s", nm)
			open = true
		case []byte:
			if !open {
				cmd.Fatal("bad input stream: missing dir for []byte")
			}
			bytes(m)
			open = false
		}
	}
	if err := cerror(in); err != nil {
		cmd.Fatal(err)
	}
}

// Run rom in the current app context.
func main() {
	c := cmd.AppCtx()
	cmd.UnixIO("err")
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("n", "name: use this name as the package name", &name)
	ux := false
	opts.NewFlag("u", "use unix out", &ux)
	args := opts.Parse()
	if ux {
		cmd.UnixIO("out")
	}
	if len(args) != 0 {
		cmd.SetIn("in", cmd.Files(args...))
	}
	rom(cmd.FullFiles(cmd.In("in")))
}
