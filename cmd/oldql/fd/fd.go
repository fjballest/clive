/*
	Ql builtin and external fd command.
	file dump
*/
package fd

import (
	"bytes"
	"clive/cmd"
	"clive/cmd/opt"
	"clive/dbg"
	"clive/zx"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

type fmtfn func(w io.Writer, bs []byte) bool

type fmtdef struct {
	flag  bool
	usage string
	flagc string
	fn    fmtfn
}

type xCmd struct {
	*cmd.Ctx
	*opt.Flags
	debug   bool
	defs    []fmtdef
	chans   []chan byte
	dprintf dbg.PrintFunc
}

func (x *xCmd) RunFile(d zx.Dir, dc <-chan []byte) error {
	name := d["path"]
	if dc == nil {
		return nil
	}
	x.dprintf("fd runfile %s\n", name)
	for data := range dc {
		x.dprintf("pz runfile: [%d]\n", len(data))
		for _, b := range data {
			for _, c := range x.chans {
				c <- b
				<-c
			}
		}
	}
	err := cerror(dc)
	x.dprintf("pz %s sts %v\n", name, err)
	return err
}

var xpref = strings.Repeat(" ", 14)

func pref(nb int, first bool) string {
	if !first {
		return xpref
	}
	return fmt.Sprintf("%012x  ", nb)
}

func bfmt(w io.Writer, b []byte) bool {
	fmt.Fprintf(w, " %02x", b[0])
	return true
}

func cfmt(w io.Writer, b []byte) bool {
	r := rune(b[0])
	if unicode.IsGraphic(r) {
		fmt.Fprintf(w, " %2c", r)
	} else {
		fmt.Fprintf(w, "  .")
	}
	return true
}

func rfmt(w io.Writer, b []byte) bool {
	if !utf8.FullRune(b) {
		return false
	}
	r, _ := utf8.DecodeRune(b)
	if unicode.IsGraphic(r) {
		fmt.Fprintf(w, " %2c", r)
	} else {
		fmt.Fprintf(w, "  .")
	}
	return true
}

func sfmt(w io.Writer, b []byte) bool {
	if len(b) < 2 {
		return false
	}
	nb := binary.LittleEndian.Uint16(b)
	fmt.Fprintf(w, "  %04x", nb)
	return true
}

func ifmt(w io.Writer, b []byte) bool {
	if len(b) < 4 {
		return false
	}
	nb := binary.LittleEndian.Uint32(b)
	fmt.Fprintf(w, "    %08x", nb)
	return true
}

func lfmt(w io.Writer, b []byte) bool {
	if len(b) < 8 {
		return false
	}
	nb := binary.LittleEndian.Uint32(b)
	fmt.Fprintf(w, "        %016x", nb)
	return true
}

func (x *xCmd) xdump(bc chan byte, donec chan bool, first bool, fn fmtfn) {
	n := 0
	var ln bytes.Buffer
	rbytes := make([]byte, 0, 8)
	fmt.Fprintf(&ln, "%s", pref(n, first))
	for b := range bc {
		rbytes = append(rbytes, b)
		if fn(&ln, rbytes) {
			rbytes = rbytes[:0]
			if n%16 == 15 {
				x.Printf("%s\n", ln.String())
				ln.Reset()
				fmt.Fprintf(&ln, "%s", pref(n+1, first))
			}
		}
		n++
		bc <- 0
	}
	if n%16 != 15 {
		x.Printf("%s\n", ln.String())
	}
	donec <- true
}

func Run(c cmd.Ctx) (err error) {
	argv := c.Args
	x := &xCmd{Ctx: &c}
	x.defs = []fmtdef{
		{flagc: "b", usage: "dump bytes", fn: bfmt},
		{flagc: "c", usage: "dump ASCII chars", fn: cfmt},
		{flagc: "r", usage: "dump runes", fn: rfmt},
		{flagc: "s", usage: "dump 2-byte ints (shorts)", fn: sfmt},
		{flagc: "i", usage: "dump 4-byte ints (ints)", fn: ifmt},
		{flagc: "l", usage: "dump 8-byte ints (longs)", fn: lfmt},
	}

	x.Flags = opt.New("{file}")
	x.Argv0 = argv[0]
	x.dprintf = dbg.FlagPrintf(x.Stderr, &x.debug)
	x.NewFlag("D", "debug", &x.debug)
	for i := range x.defs {
		df := &x.defs[i]
		x.NewFlag(df.flagc, df.usage, &df.flag)
	}
	cmd.Debug = x.debug
	args, err := x.Parse(argv)
	if err != nil {
		x.Usage(x.Stderr)
		return err
	}
	if cmd.Ns == nil {
		cmd.MkNS()
	}
	i := 0
	for ; i < len(x.defs); i++ {
		if x.defs[i].flag {
			break
		}
	}
	if i == len(x.defs) {
		x.defs[0].flag = true
	}

	x.chans = []chan byte{}
	donec := make(chan bool, 10)
	first := true
	for _, df := range x.defs {
		if df.flag {
			bc := make(chan byte)
			x.chans = append(x.chans, bc)
			go x.xdump(bc, donec, first, df.fn)
			first = false
		}
	}

	err = cmd.RunFiles(x, args...)
	for _, c := range x.chans {
		close(c)
		<-donec
	}
	return err
}
