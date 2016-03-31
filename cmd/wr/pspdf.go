package main

import (
	"bytes"
	"clive/cmd"
	"io"
	"os/exec"
	"fmt"
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
	outf := fmt.Sprintf("%s.%s%s", outfig, figk[e.Kind], e.Nb)
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
		cmd.Warn("mkpic: %s: %s", e.Data, err)
		return "none.pdf"
	}
	cmd.Warn("pic: %s", outf)
	return outf
}

func (e *Elem) pdffig() string {
	fn := e.Data
	if strings.HasSuffix(fn, ".pdf") {
		return fn
	}
	fn = e.epsfig()
	return epstopdf(fn)
}

func (e *Elem) epsfig() string {
	fn := e.Data
	if strings.HasSuffix(fn, ".eps") {
		return fn
	}
	outf := fmt.Sprintf("%s.%s%s", outfig, figk[e.Kind], e.Nb)
	outf = strings.Replace(outf, ".", "_", -1) + ".eps"
	xcmd := exec.Command("sh", "-c", "convert " + fn + " " + outf)
	errs, err := xcmd.CombinedOutput()
	if err != nil {
		if len(errs) > 0 {
			cmd.Warn("%s", string(errs))
		}
		cmd.Warn("fig2eps: %s: %s", e.Data, err)
		return "none.eps"
	}
	cmd.Warn("pic: %s", outf)
	return outf
}

func (e *Elem) htmlfig() string {
	fn := e.Data
	if strings.HasSuffix(fn, ".png") {
		return fn
	}
	if strings.HasSuffix(fn, ".gif") {
		return fn
	}
	if strings.HasSuffix(fn, ".jpg") {
		return fn
	}
	return e.pdffig()
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
		return "none.pdf"
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
