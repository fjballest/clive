/*
	Notify the user or run commands when files change
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/fswatch"
	"os/exec"
	"strings"
)

var (
	opts        = opt.New("file [cmd...]")
	notux, once bool
)

func kq(dir, wcmd string) {
	w, err := fswatch.New()
	if err != nil {
		cmd.Fatal("kqueue: %s", err)
	}

	cmd.Dprintf("(re)read %s\n", dir)
	if err := w.Add(dir); err != nil {
		cmd.Fatal("kqueue: %s", err)
	}
	if err != nil {
		cmd.Fatal("watch: %s", err)
	}
	var pc chan string
	if once {
		w.Once()
	}
	pc = w.Changes()
	for p := range pc {
		if wcmd == "" {
			cmd.Warn("%s", p)
			continue
		}
		cln := strings.Replace(wcmd, "%", p, -1)
		cmd.Dprintf("run %s\n", cln)
		out, err := exec.Command("sh", "-c", cln).CombinedOutput()
		cmd.Out("out") <- out
		if err != nil {
			cmd.Warn("run: %s", err)
		}
	}
	if err := cerror(pc); err != nil {
		cmd.Fatal(err)
	}
	cmd.Exit(nil)
}

func main() {
	cmd.UnixIO("err")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("u", "don't use unix out", &notux)
	opts.NewFlag("1", "terminate after the first change", &once)
	args := opts.Parse()
	if !notux {
		cmd.UnixIO("out")
	}

	wcmd := ""
	if len(args) >= 2 {
		wcmd = strings.Join(args[1:], " ")
		args = args[:1]
	}
	if len(args) != 1 {
		opts.Usage()
	}
	for {
		kq(args[0], wcmd)
	}
}
