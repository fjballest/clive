/*
	Ql builtin and external e command.
	edit file streams

	This is taken from the old lsub wax sam command,
	adapted a little bit for clive.
	The lsub wax sam command was derived from
	the one true Sam editor, as found in Plan 9 from Bell Labs.
*/
package e

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/dbg"
	"errors"
	"io/ioutil"
)

type xCmd struct {
	*cmd.Ctx
	*opt.Flags
	nflag, wflag  bool
	debug, debugL bool
	dprintf       dbg.PrintFunc
}

var (
	debug            bool
	dprintf          dbg.PrintFunc
	eprintf, xprintf func(string, ...interface{})
)

// Run e as a command, for ql and the external command.
func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("cmd {file}")
	x.Argv0 = argv[0]
	x.dprintf = dbg.FlagPrintf(x.Stderr, &x.debug)
	x.NewFlag("n", "do not write resulting files", &x.nflag)
	x.NewFlag("w", "update files given as arguments instead of printing to stdout", &x.wflag)
	x.NewFlag("D", "debug", &x.debug)
	args, err := x.Parse(argv)
	if err != nil {
		x.Usage(x.Stderr)
		return err
	}
	if len(args) < 1 {
		x.Usage(x.Stderr)
		return errors.New("usage")
	}

	debug = x.debug
	dprintf = x.dprintf
	eprintf = x.Eprintf
	xprintf = x.Printf

	if cmd.Ns == nil {
		cmd.MkNS()
	}

	scmd := args[0] + "\n"
	args = args[1:]

	files := []string{}
	if len(args) == 0 {
		files = append(files, "-")
		dat, err := ioutil.ReadAll(c.Stdin)
		if err != nil {
			x.Warn("stdin: %s", err)
		}
		LocalFS.stdin = []rune(string(dat))
	}
	dirc := cmd.Files(args...)
	var sts error
	for dir := range dirc {
		files = append(files, dir["path"])
	}
	if err := cerror(dirc); err != nil {
		x.Warn("%s", err)
		sts = err
	}
	if len(files) == 0 {
		x.Warn("no files to edit")
		return sts
	}

	delete(ctab, 'B')
	delete(ctab, 'q')

	sam := New()
	LocalFS.out = sam.Out
	LocalFS.dontwrite = !x.wflag
	outdone := make(chan bool, 1)
	go func() {
		for s := range sam.Out {
			xprintf("%s", s)
		}
		outdone <- true
	}()
	for _, f := range files {
		sam.newWin(f, true)
	}
	sam.In <- scmd
	if !x.nflag {
		sam.In <- "X w\n"
	}
	close(sam.In)
	sam.Wait()
	<-outdone
	return nil
}
