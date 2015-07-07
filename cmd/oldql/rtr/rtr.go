/*
	Ql builtin and external trc command.
	translate characters
*/
package rtr

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/nchan"
	"clive/zx"
	"errors"
	"strings"
)

type xCmd  {
	*cmd.Ctx
	*opt.Flags
	debug                 bool
	up, low, title, vflag bool
	del, from, to         []rune
}

func matches(r rune, from []rune, to []rune) rune {
	if len(to) == 0 {
		to = from
	}
	for i := 0; i < len(from); i++ {
		if r == from[i] {
			if len(to) == 1 {
				return to[0]
			}
			return to[i]
		}
		if i+2<len(from) && from[i+1]=='-' {
			if r>=from[i] && r<=from[i+2] {
				if len(to) == 1 {
					return to[0]
				}
				return r - from[i] + to[i]
			}
			i += 2 // next range
		}
	}
	return -1
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
		switch {
		case x.up:
			s = strings.ToUpper(s)
		case x.low:
			s = strings.ToLower(s)
		case x.title:
			s = strings.Title(s)
		case len(x.del) > 0:
			if x.vflag {
				s = strings.Map(func(r rune) rune {
					if matches(r, x.del, nil) >= 0 {
						return r
					}
					return -1
				}, s)
			} else {
				s = strings.Map(func(r rune) rune {
					if matches(r, x.del, nil) >= 0 {
						return -1
					}
					return r
				}, s)
			}
		case len(x.from) > 0:
			s = strings.Map(func(r rune) rune {
				if rr := matches(r, x.from, x.to); rr >= 0 {
					return rr
				}
				return r
			}, s)
		}
		x.Printf("%s", s)
	}
	if err := cerror(rc); err != nil {
		return err
	}
	return nil
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("[from to] {file}")
	x.Argv0 = argv[0]
	var del, from, to string
	x.NewFlag("u", "convert to uppercase", &x.up)
	x.NewFlag("l", "convert to lowercase", &x.low)
	x.NewFlag("t", "convert to title", &x.title)
	x.NewFlag("d", "delete", &del)
	x.NewFlag("v", "invert matches for deletes", &x.vflag)
	x.NewFlag("D", "debug", &x.debug)
	args, err := x.Parse(argv)
	if err != nil {
		x.Usage(x.Stderr)
		return err
	}
	if !x.up && !x.low && !x.title && del=="" {
		if len(args) < 2 {
			x.Usage(x.Stderr)
			return err
		}
		from, to = args[0], args[1]
		args = args[2:]
	}
	if x.up && x.low || x.up && x.title || x.low && x.title {
		x.Usage(x.Stderr)
		return err
	}
	if (x.up || x.low || x.title || del!="") && from!="" {
		x.Usage(x.Stderr)
		return err
	}
	x.del = []rune(del)
	x.from = []rune(from)
	x.to = []rune(to)
	if len(x.from)!=len(x.to) && len(x.to)!=1 {
		x.Usage(x.Stderr)
		return err
	}
	if cmd.Ns == nil {
		cmd.MkNS()
	}
	return cmd.RunFiles(x, args...)
}
