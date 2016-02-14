package main

import (
	"clive/cmd"
	"io"
	"os/exec"
)

var (
	// NB: groff does not handle well · which happens a lot in our docs,
	// we just use "." instead by now.

	pscmd = `sed 's/·/./g' | pic  | tbl | eqn | groff -ms -m pspic`

	// Pstopdf does NOT know how to write a pdf to stdout.
	// We might pass to the writer the name of the output file and
	// avoid the file dance.
	pdfcmd = `sed 's/·/./g' | pic  | tbl | eqn | groff -ms -m pspic |pstopdf -i -o /tmp/_x.pdf; cat /tmp/_x.pdf`
)

func pspdf(t *Text, wid int, out io.Writer, cline, outfig string) {
	// pipe the roff writer into a command to output ps and pdf
	xcmd := exec.Command("sh", "-c", cline)
	xcmd.Stdout = out
	stdin, err := xcmd.StdinPipe()
	if err != nil {
		cmd.Fatal("pipe to sh: %s", err)
	}
	if err := xcmd.Start(); err != nil {
		cmd.Fatal("pipe to sh: %s", err)
	}
	wrroff(t, wid, stdin, outfig)
	stdin.Close()
	if err := xcmd.Wait(); err != nil {
		cmd.Warn("pspdf: sh: %s", err)
	}
}

// pdf writer
func wrpdf(t *Text, wid int, out io.Writer, outfig string) {
	pspdf(t, wid, out, pdfcmd, outfig)
}

// ps writer
func wrps(t *Text, wid int, out io.Writer, outfig string) {
	pspdf(t, wid, out, pscmd, outfig)
}
