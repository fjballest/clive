/*
	write documents, man pages, and html pages
	using a single formatting language.
*/
package main

import (
	"bytes"
	"clive/cmd"
	"clive/cmd/opt"
	"clive/cmd/wr/refs"
	"clive/zx"
	"fmt"
	"io"
	"os"
	fpath "path"
	"path/filepath"
)

struct xCmd {
	*opt.Flags
	*cmd.Ctx
}

var (
	opts               = opt.New("{file}")
	debugPars          bool
	debugIndent        bool
	debugSplit         bool
	outdir             = "."
	outfig             = "./wrfig"
	outpdf             string
	uname, oname, oext string
	max                = 70
	refsdir            = ""
	wrs                = map[string]func(*Text, int, io.Writer, string){
		".man":  wrtxt,
		".ms":   wrroff,
		".ps":   wrps,
		".pdf":  wrpdf,
		".tex":  wrtex,
		".html": wrhtml,
	}

	eflag, hflag, tflag, lflag, mflag, pflag, psflag, notux bool

	labels = map[Kind]string{
		Kfig:  "Figure",
		Kpic:  "Figure",
		Kgrap: "Figure",
		Ktbl:  "Table",
		Keqn:  "Eqn.",
		Kcode: "Listing",
		Kchap: "Chapter",
	}

	splabels = map[Kind]string{
		Kfig:  "Figura",
		Kpic:  "Figura",
		Kgrap: "Figura",
		Ktbl:  "Tabla",
		Keqn:  "Ec.",
		Kcode: "Listado",
		Kchap: "Capítulo",
	}
)

func outExt() string {
	switch {
	case hflag, sect != "":
		if tflag || lflag || mflag || pflag || psflag {
			opts.Usage()
		}
		hflag = true
		return ".html"
	case tflag:
		if hflag || lflag || mflag || pflag || psflag {
			opts.Usage()
		}
		return ".ms"
	case lflag:
		if hflag || tflag || mflag || pflag || psflag {
			opts.Usage()
		}
		return ".tex"
	case mflag, tflag:
		if hflag || tflag || lflag || pflag || psflag {
			opts.Usage()
		}
		return ".man"
	case pflag:
		if hflag || tflag || lflag || mflag || psflag {
			opts.Usage()
		}
		return ".pdf"
	case psflag:
		if hflag || tflag || lflag || mflag || pflag {
			opts.Usage()
		}
		return ".ps"
	default:
		mflag = true
		cliveMan = true
		return ".man"
	}
}

func out(t *Text) error {
	wr, ok := wrs[oext]
	if !ok {
		cmd.Fatal("no writer for %s", oext)
	}
	var b bytes.Buffer
	if oext == ".ms" {
		fmt.Fprintf(&b, `.\" grap %s | pic  | tbl | eqn | `+
			`groff  -ms -m pspic  |pstopdf -i -o  %s`+"\n",
			oname, outpdf)
	}
	wr(t, max, &b, outfig)
	cmd.Dprintf("output to %s\n", oname)
	out := cmd.Out("out")
	var fout chan []byte
	var rc <-chan zx.Dir
	if oname != "-" {
		fout = make(chan []byte)
		rc = cmd.Put(oname, zx.Dir{"type": "-", "mode": "0644"}, 0, fout)
	}
	dat := b.Bytes()
	for len(dat) > 0 {
		n := len(dat)
		if n > 16*1024 {
			n = 16 * 1024
		}
		if fout != nil {
			if ok := fout <- dat[:n]; !ok {
				return cerror(fout)
			}
		} else {
			if ok := out <- dat[:n]; !ok {
				return cerror(out)
			}
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

func startFile(d zx.Dir) (chan<- string, <-chan *Text) {
	cmd.Dprintf("file %s\n", d["path"])
	iname := d["name"]
	uname = d["Upath"]
	iext := filepath.Ext(iname)
	ibase := iname[:len(iname)-len(iext)]
	outdir = filepath.Dir(d["path"])
	if oname == "" {
		if oext == ".man" {
			oname = "-"
		} else {
			oname = ibase + oext
		}
	} else if oname != "-" {
		if a, err := filepath.Abs(oname); err == nil {
			outdir = filepath.Dir(a)
		}
	}
	outpdf = ibase + ".pdf"
	outfig = fpath.Join(outdir, ibase)
	cmd.Dprintf("oname %s\n", oname)
	cmd.Dprintf("outfig %s\n", outfig)
	cmd.Dprintf("outdir %s\n", outdir)
	return Parse()
}

func endFile(lnc chan<- string, tc <-chan *Text) error {
	close(lnc)
	t := <-tc
	if err := cerror(tc); err != nil {
		return err
	}
	return out(t)
}

func wr(in <-chan face{}) error {
	var lnc chan<- string
	var tc <-chan *Text
	singleout := oname != "" && oname != "-"
	var sts error
	stdin := zx.Dir{"name": "stdin", "uname": "stdin"}
	for m := range in {
		switch m := m.(type) {
		case error:
			cmd.Warn("%s", m)
			sts = m
		case zx.Dir:
			if lnc != nil && !singleout {
				if e := endFile(lnc, tc); e != nil {
					sts = e
				}
				lnc, tc = nil, nil
			}
			if m["type"] == "d" {
				continue
			}
			if lnc == nil || !singleout {
				lnc, tc = startFile(m)
			}
		case []byte:
			if lnc == nil {
				lnc, tc = startFile(stdin)
			}
			lnc <- string(m)
		}
	}
	if lnc != nil {
		if e := endFile(lnc, tc); e != nil {
			sts = e
		}
	}
	if sts != nil {
		return sts
	}
	return cerror(in)
}

func main() {
	cmd.UnixIO("err")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("w", "wid: text width for text formats", &max)
	opts.NewFlag("h", "generate html", &hflag)
	opts.NewFlag("r", "generate roff", &tflag)
	opts.NewFlag("l", "generate latex", &lflag)
	opts.NewFlag("m", "generate man page", &mflag)
	opts.NewFlag("c", "sect: with -h, generate a man page in the given section", &sect)
	opts.NewFlag("s", "generate ps", &psflag)
	opts.NewFlag("p", "generate pdf", &pflag)
	opts.NewFlag("o", "file: generate a single output file", &oname)
	opts.NewFlag("I", "debug indents", &debugIndent)
	opts.NewFlag("S", "debug split", &debugSplit)
	opts.NewFlag("P", "debug paragraphs", &debugPars)
	opts.NewFlag("b", "dir: change the default refer bib dir", &refsdir)
	opts.NewFlag("u", "do not generate output for unix", &notux)
	opts.NewFlag("e", "use spanish for labels", &eflag)

	args := opts.Parse()
	if !notux {
		cmd.UnixIO("out")
	}
	if oname == "stdout" {
		oname = "-"
	}
	if refsdir == "" {
		refsdir = refs.Dir
		if _, err := os.Stat("/u/bib"); err == nil {
			refsdir = "/u/bib"
		}
	}
	hflag = hflag || sect != ""
	cliveMan = sect != "" || mflag
	if len(args) != 0 {
		cmd.SetIn("in", cmd.Files(args...))
	}
	oext = outExt()
	if eflag {
		labels = splabels
	}
	sts := wr(cmd.Lines(cmd.In("in")))
	if sts != nil {
		cmd.Fatal(sts)
	}
}
