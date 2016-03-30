package main

import (
	"bytes"
	"clive/cmd"
	"io"
	"os/exec"
	"fmt"
	fpath "path"
	"strings"
)

var (
	// NB: groff does not handle well · which happens a lot in our docs,
	// we just use "." instead by now.

	pscmd = `grap | pic  | tbl | eqn | groff  -ms -m pspic`

	// Pstopdf does NOT know how to write a pdf to stdout.
	// We might pass to the writer the name of the output file and
	// avoid the file dance.
	pdfcmd = `grap | pic  | tbl | eqn | groff -ms -m pspic |pstopdf -i -o /tmp/_x.pdf; cat /tmp/_x.pdf`

	pic2pdf = `grap | pic | tbl | eqn | groff -ms -m pspic -P-b16 >/tmp/_x.ps ; ps2epsi /tmp/_x.ps /tmp/_x.eps; epstopdf /tmp/_x.eps -o=`
	pic2eps = `grap | pic | tbl | eqn | groff -ms -m pspic >/tmp/_x.ps ; pstoepsi /tmp/_x.ps `
)

var figk = map[Kind]string{
	Kfig:  "pic",
	Kgrap: "grap",
	Kpic:  "pic",
	Keqn:  "eqn",
}

func (e *Elem) pic(outfig string) string {
	outf := fmt.Sprintf("%s.%s%s", fpath.Base(outfig), figk[e.Kind], e.Nb)
	outf = strings.Replace(outf, ".", "_", -1) + ".pdf"
	var b bytes.Buffer
	b.WriteString(figstart[e.Kind] + "\n")
	b.WriteString(e.Data)
	b.WriteString(figend[e.Kind] + "\n")
	xcmd := exec.Command("sh", "-c", pic2pdf+outf)
	xcmd.Stdin = &b
	errs, err := xcmd.CombinedOutput()
	if err != nil {
		if len(errs) > 0 {
			cmd.Warn("%s", string(errs))
		}
		cmd.Warn("mkpic: %s: %s", outfig, err)
		return "none.png"
	}
	cmd.Warn("pic: %s", outf)
	return outf
}

func epstopdf(fn string) string {
	if strings.HasSuffix(fn, ".pdf") {
		return fn
	}
	outf := fn
	if strings.HasSuffix(outf, ".eps") {
		outf = outf[:len(outf)-4]
	}
	outf += ".pdf"
	xcmd := exec.Command("pstopdf", fn, outf)
	errs, err := xcmd.CombinedOutput()
	if err != nil {
		if len(errs) > 0 {
			cmd.Warn("%s", string(errs))
		}
		cmd.Warn("epstopdf: %s:, %s", outf, err)
		return "none.png"
	}
	cmd.Warn("epspic: %s", outf)
	return outf
}


func pspdf(t *Text, wid int, out io.Writer, cline, outfig string) {
	// pipe the roff writer into a command to output ps and pdf
	xcmd := exec.Command("sh", "-c", cline)
	xcmd.Stdout = out
	stdin, err := xcmd.StdinPipe()
	if err != nil {
		cmd.Fatal("pipe to sh: %s", err)
	}
	stderr, err := xcmd.StderrPipe()
	if err != nil {
		cmd.Fatal("pipe to sh: %s", err)
	}
	if err := xcmd.Start(); err != nil {
		cmd.Fatal("pipe to sh: %s", err)
	}

	wrroff(t, wid, stdin, outfig)
	stdin.Close()
	var buf bytes.Buffer
	io.Copy(&buf, stderr)
	if buf.Len() > 0 {
		cmd.Eprintf("%s", buf)
	}
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
