/*
	write documents, man pages, and html pages
	using a single formatting language.
*/
package wr

import (
	"bytes"
	"clive/app"
	"clive/app/nsutil"
	"clive/app/opt"
	"clive/app/wr/refs"
	"clive/dbg"
	"clive/zx"
	"fmt"
	"io"
	"path/filepath"
)

type xCmd struct {
	*opt.Flags
	*app.Ctx

	debugPars                                 bool
	debugIndent                               bool
	debugSplit                                bool
	hflag, tflag, lflag, mflag, pflag, psflag bool
	outdir, outfig, outpdf                    string
	uname, oname, oext                        string
	max                                       int
	refsdir                                   string
}

var (
	wrs = map[string]func(*Text, int, io.Writer, string){
		".man":  wrtxt,
		".ms":   wrroff,
		".ps":   wrps,
		".pdf":  wrpdf,
		".tex":  wrtex,
		".html": wrhtml,
	}
)

func (x *xCmd) outExt() string {
	switch {
	case x.hflag, sect != "":
		if x.tflag || x.lflag || x.mflag || x.pflag || x.psflag {
			x.Usage()
			app.Exits("usage")
		}
		x.hflag = true
		return ".html"
	case x.tflag:
		if x.hflag || x.lflag || x.mflag || x.pflag || x.psflag {
			x.Usage()
			app.Exits("usage")
		}
		return ".ms"
	case x.lflag:
		if x.hflag || x.tflag || x.mflag || x.pflag || x.psflag {
			x.Usage()
			app.Exits("usage")
		}
		return ".tex"
	case x.mflag, x.tflag:
		if x.hflag || x.tflag || x.lflag || x.pflag || x.psflag {
			x.Usage()
			app.Exits("usage")
		}
		return ".man"
	case x.pflag:
		if x.hflag || x.tflag || x.lflag || x.mflag || x.psflag {
			x.Usage()
			app.Exits("usage")
		}
		return ".pdf"
	case x.psflag:
		if x.hflag || x.tflag || x.lflag || x.mflag || x.pflag {
			x.Usage()
			app.Exits("usage")
		}
		return ".ps"
	default:
		x.mflag = true
		cliveMan = true
		return ".man"
	}
}

func (x *xCmd) startFile(d zx.Dir) (chan<- string, <-chan *Text) {
	app.Dprintf("file %s\n", d["path"])
	iname := d["name"]
	x.uname = d["upath"]
	iext := filepath.Ext(iname)
	ibase := iname[:len(iname)-len(iext)]
	x.outdir = filepath.Dir(d["path"])
	if x.oname == "" {
		if x.oext == ".man" {
			x.oname = "-"
		} else {
			x.oname = ibase + x.oext
		}
	} else {
		if a, err := filepath.Abs(x.oname); err == nil {
			x.outdir = filepath.Dir(a)
		}
	}
	x.outpdf = ibase + ".pdf"
	x.outfig = zx.Path(x.outdir, ibase)
	app.Dprintf("oname %s\n", x.oname)
	app.Dprintf("outfig %s\n", x.outfig)
	app.Dprintf("outdir %s\n", x.outdir)
	return x.Parse()
}

func (x *xCmd) endFile(lnc chan<- string, tc <-chan *Text) error {
	close(lnc)
	t := <-tc
	if err := cerror(tc); err != nil {
		return err
	}
	return x.out(t)
}

func (x *xCmd) out(t *Text) error {
	wr, ok := wrs[x.oext]
	if !ok {
		app.Fatal("no writer for %s", x.oext)
	}
	var b bytes.Buffer
	if x.oext == ".ms" {
		fmt.Fprintf(&b, `.\" pic %s | tbl | eqn | `+
			`groff -ms -m pspic  |pstopdf -i -o  %s`+"\n",
			x.oname, x.outpdf)
	}
	wr(t, x.max, &b, x.outfig)
	app.Dprintf("output to %s\n", x.oname)
	out := app.Out()
	var fout chan []byte
	var rc chan zx.Dir
	if x.oname != "-" {
		fout = make(chan []byte)
		rc = nsutil.Put(x.oname, zx.Dir{"mode": "0644"}, 0, fout, "")
	}
	dat := b.Bytes()
	for len(dat) > 0 {
		n := len(dat)
		if n > 16*1024 {
			n = 16 * 1024
		}
		var ok bool
		if fout != nil {
			ok = fout <- dat[:n]
		} else {
			ok = out <- dat[:n]
		}
		if !ok {
			return cerror(out)
		}
		dat = dat[n:]
	}
	if fout != nil {
		close(fout)
		<-rc
		return cerror(rc)
	}
	return nil
}

func (x *xCmd) wr(in chan interface{}) error {
	var lnc chan<- string
	var tc <-chan *Text
	singleout := x.oname != ""
	var sts error
	stdin := zx.Dir{"name": "stdin", "uname": "stdin"}
	doselect {
	case <-x.Sig:
		close(lnc, dbg.ErrIntr)
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-in:
		if !ok {
			break
		}
		switch m := m.(type) {
		case zx.Dir:
			if lnc != nil && !singleout {
				if e := x.endFile(lnc, tc); e != nil {
					sts = e
				}
				lnc, tc = nil, nil
			}
			if m["type"] == "d" {
				continue
			}
			if lnc == nil || !singleout {
				lnc, tc = x.startFile(m)
			}
		case []byte:
			if lnc == nil {
				lnc, tc = x.startFile(stdin)
			}
			lnc <- string(m)
		}
	}
	if lnc != nil {
		if e := x.endFile(lnc, tc); e != nil {
			sts = e
		}
	}
	if sts != nil {
		return sts
	}
	return cerror(in)
}

func Run() {
	x := &xCmd{
		Ctx:     app.AppCtx(),
		max:     70,
		refsdir: refs.Dir,
		outdir:  ".",
		outfig:  "./wrfig",
	}
	x.Flags = opt.New("{file}")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("w", "wid: text width for text formats", &x.max)
	x.NewFlag("h", "generate html", &x.hflag)
	x.NewFlag("r", "generate roff", &x.tflag)
	x.NewFlag("l", "generate latex", &x.lflag)
	x.NewFlag("t", "generate plaint text", &x.tflag)
	x.NewFlag("m", "generate man page", &x.mflag)
	x.NewFlag("c", "sect: generate html for a clive man page in the given section", &sect)
	x.NewFlag("s", "generate ps", &x.psflag)
	x.NewFlag("p", "generate pdf", &x.pflag)
	x.NewFlag("o", "file: generate a single output file", &x.oname)
	x.NewFlag("I", "debug indents", &x.debugIndent)
	x.NewFlag("S", "debug split", &x.debugSplit)
	x.NewFlag("P", "debug paragraphs", &x.debugPars)
	x.NewFlag("b", "dir: change the default refer bib dir", &x.refsdir)
	args, err := x.Flags.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}

	x.hflag = x.hflag || sect != ""
	cliveMan = sect != "" || x.mflag
	if len(args) != 0 {
		in := app.Files(args...)
		app.SetIO(in, 0)
	}
	x.oext = x.outExt()
	sts := x.wr(app.Lines(app.In()))
	app.Exits(sts)
}
