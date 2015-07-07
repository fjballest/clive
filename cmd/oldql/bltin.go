package main

import (
	"clive/cmd"
	"clive/cmd/oldql/chc"
	"clive/cmd/oldql/cols"
	"clive/cmd/oldql/comp"
	"clive/cmd/oldql/cpc"
	"clive/cmd/oldql/e"
	"clive/cmd/oldql/fd"
	"clive/cmd/oldql/flds"
	"clive/cmd/oldql/frmt"
	"clive/cmd/oldql/gr"
	"clive/cmd/oldql/jn"
	"clive/cmd/oldql/lf"
	"clive/cmd/oldql/lns"
	"clive/cmd/oldql/mvc"
	"clive/cmd/oldql/pc"
	"clive/cmd/oldql/rmc"
	"clive/cmd/oldql/rtr"
	"clive/cmd/oldql/srt"
	"clive/cmd/oldql/xp"
	"clive/dbg"
	"errors"
	"os"
	"strings"
)

type X func(c cmd.Ctx) error

var (
	bltin = map[string]X{
		"flag": bflag,
		"echo": becho,
		"exit": bexit,
		"wait": bwait,
		"pwd":  bpwd,
		"cd":   bcd,

		// these should also be available as external commands
		":":    xp.Run,
		"chc":  chc.Run,
		"cols": cols.Run,
		"cpc":  cpc.Run,
		"flds": flds.Run,
		"frmt": frmt.Run,
		"gr":   gr.Run,
		"jn":   jn.Run,
		"lf":   lf.Run,
		"lns":  lns.Run,
		"mvc":  mvc.Run,
		"pc":   pc.Run,
		"rmc":  rmc.Run,
		"rtr":  rtr.Run,
		"srt":  srt.Run,
		"xp":   xp.Run,
		"fd":   fd.Run,

		// these are deleted by init because they are large or they don't handle C-c
		// correctly.
		"e":    e.Run,
		"comp": comp.Run,
	}
)

func init() {
	bltin["type"] = btype
	delete(bltin, "e")
	delete(bltin, "comp")
}

func bflag(c cmd.Ctx) error {
	args := c.Args
	xprintf("bflag...\n")
	if len(args)!=2 || len(args[1])==0 || args[1][0]!='-' && args[1][0]!='+' {
		return errors.New("usage: flag ±flags")
	}
	flg := args[1]
	set := flg[0] == '+'
	for _, r := range flg[1:] {
		switch r {
		case 'D':
			debug = set
		case 'V':
			verb = set
			debug = set
		case 'N':
			debugNs = set
		default:
			c.Warn("unknown flag '%c'", r)
			return errors.New("unknown flag")
		}
	}
	debugYacc = debug
	debugExec = verb
	cmd.Debug = debugNs
	return nil
}

func bexit(c cmd.Ctx) error {
	args := c.Args
	xprintf("exiting...\n")
	if len(args)<=1 || args[1]=="" {
		dbg.Fatal("")
	} else {
		dbg.Fatal(args[1])
	}
	return nil
}

func bwait(c cmd.Ctx) error {
	args := c.Args
	xprintf("waiting...\n")
	what := ""
	if len(args)>1 && len(args[1])>0 {
		what = args[1]
	}
	// BUG: select on intrc, but check out that Wait is ok with that
	ec := bg.Wait(what)
	<-ec
	err := cerror(ec)
	xprintf("waiting...done (%v)\n", err)
	return err
}

func ntype(nm string) string {
	if bltin[nm] != nil {
		return "builtin"
	}
	if funcs[nm] != nil {
		return "func"
	}
	if os.Getenv(nm) != "" {
		return "env"
	}
	return LookCmd(nm)
}

func btype(c cmd.Ctx) error {
	args := c.Args
	if len(args) == 1 {
		return errors.New("usage: type name...")
	}
	for _, nm := range args[1:] {
		c.Printf("%s: %s\n", nm, ntype(nm))
	}
	return nil
}

func becho(c cmd.Ctx) error {
	args := c.Args[1:]
	nl := "\n"
	if len(args)>=1 && args[0]=="-n" {
		nl = ""
		args = args[1:]
	}
	str := strings.Join(args, " ")
	c.Printf("%s%s", str, nl)
	return nil
}

func bpwd(c cmd.Ctx) error {
	args := c.Args
	if len(args) > 1 {
		return errors.New("extra args to pwd")
	}
	s, err := os.Getwd()
	if err != nil {
		return err
	}
	c.Printf("%s\n", s)
	return nil
}

func bcd(c cmd.Ctx) error {
	args := c.Args
	if len(args) != 2 {
		return errors.New("usage is cd dir")
	}
	return os.Chdir(args[1])
}
