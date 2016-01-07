/*
	ch command
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx"
	"strconv"
	"strings"
)

var units = map[uint8]uint64{
	'b': 1,
	'B': 1,
	'k': 1024,
	'K': 1024,
	'm': 1024 * 1024,
	'M': 1024 * 1024,
	'g': 1024 * 1024,
	'G': 1024 * 1024 * 1024,
}

var (
	opts = opt.New("{attr value}")
)

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

// return the dir to be written.
// mode is left as indicated (eg. +r) and processed later on a per-dir basis
func chd(args ...string) zx.Dir {
	d := zx.Dir{}
	for len(args) >= 2 {
		attr, val := args[0], args[1]
		args = args[2:]
		if zx.IsTemp(attr) {
			cmd.Fatal("%s: won't change '%s'", d["path"], attr)
		}
		if attr == "uids" {
			d["uid"] = val
			d["gid"] = val
		}
		if attr == "size" {
			val = size(val)
		}
		d[attr] = val
	}
	return d
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
	all := s[0] == '+' || s[0] == '-'
	if strings.ContainsRune(s, 'u') || strings.ContainsRune(s, 'a') || all {
		b |= bit << 6
	}
	if strings.ContainsRune(s, 'g') || strings.ContainsRune(s, 'a') || all {
		b |= bit << 3
	}
	if strings.ContainsRune(s, 'o') || strings.ContainsRune(s, 'a') || all {
		b |= bit
		if s[0] == '+' { // +w does not switch on w for others
			b &^= 2
		}
	}
	return b
}

func chmod(d zx.Dir, val string) {
	omode, _ := strconv.ParseUint(d["mode"], 8, 64)
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
	d["mode"] = "0" + strconv.FormatUint(omode&0777, 8)
}

func ch(d, nd zx.Dir) error {
	if nd["mode"] != "" {
		nd = nd.Dup()
		chmod(d, nd["mode"])
	}
	_, err := cmd.Wstat(d["path"], nd)
	return err
}

func ismode(a string) bool {
	_, err := strconv.ParseInt(a, 8, 32)
	isnum := err == nil
	return strings.Contains(a, "+") || strings.Contains(a, "-") || isnum
}

func main() {
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	cmd.UnixIO("err")
	args, err := opts.Parse()
	if err != nil {
		cmd.Warn("%s", err)
		opts.Usage()
	}
	if len(args) == 1 && ismode(args[0]) {
		args = append([]string{"mode"}, args...)
	}
	if len(args) == 0 || len(args)%2 != 0 {
		cmd.Warn("wrong number of arguments")
		opts.Usage()
	}
	nd := chd(args...)

	in := cmd.IO("in")
	for m := range in {
		switch d := m.(type) {
		case zx.Dir:
			cmd.Dprintf("got %T %s\n", d, d["Upath"])
			if cerr := ch(d, nd); cerr != nil {
				cmd.Warn("%s", cerr)
				err = cerr
			}
		default:
			// ignored
			cmd.Dprintf("got %T\n", m)
		}
	}
	if err != nil {
		cmd.Exit(err)
	}
}
