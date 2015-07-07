/*
	Ql builtin and external chc command.
	change attributes for files
*/
package chc

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/dbg"
	"clive/zx"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type xCmd  {
	*cmd.Ctx
	debug bool
	*opt.Flags
	dprintf dbg.PrintFunc
}

func bit(s string) uint64 {
	b := uint64(0)
	if strings.ContainsRune(s, 'r') {
		b |= 4
	}
	if strings.ContainsRune(s, 'w') {
		b |= 2
	}
	if strings.ContainsRune(s, 'x') {
		b |= 1
	}
	return b
}

func ugoa(s string, bit uint64) uint64 {
	b := uint64(0)
	all := s[0]=='+' || s[0]=='-'
	if strings.ContainsRune(s, 'u') || strings.ContainsRune(s, 'a') || all {
		b |= bit<<6
	}
	if strings.ContainsRune(s, 'g') || strings.ContainsRune(s, 'a') || all {
		b |= bit<<3
	}
	if strings.ContainsRune(s, 'o') || strings.ContainsRune(s, 'a') || all {
		b |= bit
		if s[0] == '+' { // +w does not switch on w for others
			b &^= 2
		}
	}
	return b
}

func chmod(d zx.Dir, val string) string {
	omode := d.Uint64("mode")
	nb, err := strconv.ParseUint(val, 8, 64)
	if err == nil {
		omode = nb
	} else {
		bits := ugoa(val, bit(val))
		if strings.Contains(val, "+") {
			omode |= bits
		} else {
			omode &^= bits
		}
	}
	return "0" + strconv.FormatUint(omode&0777, 8)
}

var units = map[uint8]uint64{
	'b': 1,
	'B': 1,
	'k': 1024,
	'K': 1024,
	'm': 1024*1024,
	'M': 1024*1024,
	'g': 1024*1024,
	'G': 1024*1024*1024,
}

func size(val string) string {
	if len(val) <= 1 {
		return val
	}
	n := len(val)
	unit := val[n-1]
	u, ok := units[unit]
	if !ok {
		return val
	}
	nb, err := strconv.ParseUint(val[:n-1], 0, 64)
	if err != nil {
		return val
	}
	return strconv.FormatUint(nb*u, 10)
}

func (x *xCmd) run(d zx.Dir, attr, val string) error {
	p := d["spath"]
	if attr=="name" || attr=="path" || attr=="spath" || len(val)==0 {
		return fmt.Errorf("%s: won't change '%s'", d["path"], attr)
	}
	if attr == "uids" {
		if err := x.run(d, "Uid", val); err != nil {
			return err
		}
		if err := x.run(d, "Gid", val); err != nil {
			return err
		}
		return x.run(d, "Wuid", val)
	}
	if p == "" {
		return fmt.Errorf("%s: no server path", d["path"])
	}
	switch attr {
	case "mode":
		val = chmod(d, val)
	case "size":
		val = size(val)
	}
	nd := zx.Dir{attr: val}
	fs, err := zx.RWDirTree(d)
	if err != nil {
		x.dprintf("rwdirtree: %s\n", err)
		return fmt.Errorf("%s: %s", d["path"], err)
	}
	err = <-fs.Wstat(p, nd)
	x.dprintf("chc %s %s -> sts %v\n", p, nd, err)
	return err
}

func ismode(a string) bool {
	_, err := strconv.ParseInt(a, 8, 32)
	isnum := err == nil
	return strings.Contains(a, "+") || strings.Contains(a, "-") || isnum
}

func (x *xCmd) usage() {
	x.Usage(x.Stderr)
	x.Eprintf("\tattr uids means all of Uid, Gid, Wuid\n")
	x.Eprintf("\tattr mode can use +rwx or -rwx as value; name optional here\n")
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.Flags = opt.New("attr value {file}")
	x.Argv0 = argv[0]
	x.dprintf = dbg.FlagPrintf(x.Stderr, &x.debug)
	x.NewFlag("D", "debug", &x.debug)
	args, err := x.Parse(argv)
	if err != nil {
		x.usage()
		return err
	}
	if len(args)==2 && ismode(args[0]) {
		args = append([]string{"mode"}, args...)
	}
	if len(args) < 3 {
		x.usage()
		return errors.New("missing argument")
	}
	x.dprintf("dry run\n")
	attr, val := args[0], args[1]
	args = args[2:]
	cmd.Debug = x.debug
	if cmd.Ns == nil {
		cmd.MkNS()
	}
	dirc := cmd.Files(args...)
	for dir := range dirc {
		if dir["err"] != "" {
			err = errors.New("errors")
			continue
		}
		if xerr := x.run(dir, attr, val); xerr != nil {
			x.Warn("%s", xerr)
			err = errors.New("errors")
		}
	}
	return cerror(dirc)
}
