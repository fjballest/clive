/*
	print files command
*/
package main

import (
	"clive/ch"
	"clive/mblk"
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx"
	fpath "path"
)

struct wFile {
	d zx.Dir
	dat *mblk.Buffer
}

var (
	opts = opt.New("")
	printf = cmd.Printf
	odir string

	notux, lflag, pflag, nflag, iflag, dflag, aflag, fflag, wflag, wwflag bool
)

func (w *wFile) start(d zx.Dir) error {
	if d == nil {
		return nil
	}
	d = d.Dup()
	delete(d, "gid")
	delete(d, "size")
	if wwflag {
		if d["type"] == "d" {
			d["type"] = "D"
		} else if d["type"] == "-" {
			d["type"] = "F"
		}
	}
	d["Dpath"] = d["path"]
	if odir != "" {
		d["Dpath"] = fpath.Join(odir, d["Rpath"])
	}
	if d["type"] == "d" {
		cmd.Dprintf("writing dir %s\n", w.d["Dpath"])
		rc := cmd.Put(w.d["Dpath"], d, 0, nil)
		<-rc
		err := cerror(rc)
		if zx.IsExists(err) {
			err = nil
		}
		return err
	}
	w.d = d
	w.dat = &mblk.Buffer{}
	return nil
}

func (w *wFile) end() error {
	if w == nil || w.d == nil {
		return nil
	}
	dc := make(chan []byte)
	cmd.Dprintf("writing file %s\n", w.d["Dpath"])
	rc := cmd.Put(w.d["Dpath"], w.d, 0, dc)
	_, _, err := w.dat.SendTo(0, -1, dc);
	close(dc, err)
	if err != nil {
		close(rc, err)
	} else {
		<-rc
		err = cerror(rc)
	}
	w.d = nil
	w.dat = nil
	return err
}

func (w *wFile) write(b []byte) error {
	if w == nil || w.dat == nil {
		return nil
	}
	_, err := w.dat.Write(b)
	if err != nil {
		w.dat = nil
		w.d = nil
	}
	return err
}

func main() {
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("d", "no not print file data", &dflag)
	opts.NewFlag("f", "no not print dir data", &fflag)
	opts.NewFlag("n", "print just names", &nflag)
	opts.NewFlag("p", "print just paths", &pflag)
	opts.NewFlag("l", "long list for dirs", &lflag)
	opts.NewFlag("i", "print also ignored data", &iflag)
	opts.NewFlag("u", "don't use unix out", &notux)
	opts.NewFlag("a", "print addresses", &aflag)
	opts.NewFlag("o", "write destination path", &odir)
	opts.NewFlag("w", "write file data back to disk (-d implied)", &wflag)
	opts.NewFlag("W", "writeall file data back to disk (-d implied)", &wwflag)
	args, err := opts.Parse()
	cmd.UnixIO("err")
	if !notux {
		cmd.UnixIO("out")
	}
	if err != nil {
		cmd.Warn("%s", err)
		opts.Usage()
	}
	if len(args) != 0 {
		opts.Usage()
	}
	wflag = wflag || wwflag
	dflag = dflag || wflag

	in := cmd.IO("in")
	out := cmd.IO("out")
	var w wFile
	for m := range in {
		cmd.Dprintf("got %T\n", m)
		switch m := m.(type) {
		case error:
			if wflag {
				if werr := w.end(); werr != nil {
					cmd.Warn("write: %s", werr)
				}
			}
			if notux {
				if ok := out <- m; !ok {
					close(in, cerror(out))
				}
				continue
			}
			cmd.Warn("%s", m)
		case zx.Dir:
			if wflag {
				if werr := w.end(); werr != nil {
					cmd.Warn("write: %s", werr)
				}
				if werr := w.start(m); werr != nil {
					cmd.Warn("write: %s", werr)
				}
			}
			if fflag {
				continue
			}
			switch {
			case notux:
				if ok := out <- m; !ok {
					close(in, cerror(out))
				}
			case nflag:
				printf("%s\n", m["Upath"])
			case pflag:
				printf("%s\n", m["path"])
			case lflag:
				printf("%s\n", m.LongFmt())
			default:
				printf("%s\n", m.Fmt())
			}
		case []byte:
			if wflag {
				if werr := w.write(m); werr != nil {
					cmd.Warn("write: %s", werr)
				}
			}
			if dflag {
				continue
			}
			if ok := out <- m; !ok {
				close(in, cerror(out))
			}
		case ch.Ign:
			if dflag {
				continue
			}
			b := m.Dat
			if ok := out <- b; !ok {
				close(in, cerror(out))
			}
		case zx.Addr:
			if !aflag {
				continue
			}
			printf("%s\n", m)
		}
	}
	if wflag {
		if err = w.end(); err != nil {
			cmd.Warn("write: %s", err)
		}
	}
	if err == nil {
		err = cerror(in)
	}
	if err != nil {
		cmd.Fatal(err)
	}
}
