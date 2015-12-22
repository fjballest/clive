/*
	Ql builtin and external cols command.
	columnate text
*/
package cols

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/nchan"
	"clive/zx"
	"errors"
	"fmt"
	"strings"
)

type xCmd struct {
	*cmd.Ctx
	*opt.Flags
	wid, ncols int
	words      []string
	maxwid     int
}

func (x *xCmd) col() {
	if x.wid == 0 {
		x.wid, _ = cmd.Cols()
		if x.wid == 0 {
			x.wid = 70
		}
	}
	colwid := x.maxwid + 1
	if x.ncols == 0 {
		x.ncols = x.wid / colwid
	}
	if x.ncols == 0 {
		x.ncols = 1
	}
	fmts := fmt.Sprintf("%%-%ds ", x.maxwid)
	for i, w := range x.words {
		x.Printf(fmts, w)
		if (i+1)%x.ncols == 0 {
			x.Printf("\n")
		}
	}
	if len(x.words)%x.ncols != 0 {
		x.Printf("\n")
	}
}

func (x *xCmd) RunFile(d zx.Dir, dc <-chan []byte) error {
	if dc == nil {
		return nil
	}
	rc := nchan.Lines(dc, '\n')
	doselect {
	case <-x.Intrc:
		close(rc, "interrupted")
		return errors.New("interrupted")
	case s, ok := <-rc:
		if !ok {
			return cerror(rc)
		}
		words := strings.Fields(strings.TrimSpace(s))
		for _, w := range words {
			x.words = append(x.words, w)
			if len(w) > x.maxwid {
				x.maxwid = len(w)
			}
		}
	}
	if err := cerror(rc); err != nil {
		return err
	}
	return nil
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("{file}")
	x.Argv0 = argv[0]
	x.NewFlag("w", "wid: set max line width", &x.wid)
	x.NewFlag("n", "ncols: set number of columns", &x.ncols)
	args, err := x.Parse(argv)
	if err != nil {
		x.Usage(x.Stderr)
		return err
	}
	if cmd.Ns == nil {
		cmd.MkNS()
	}
	if err := cmd.RunFiles(x, args...); err != nil {
		return err
	}
	x.col()
	return nil
}
