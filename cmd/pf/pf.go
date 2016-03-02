/*
	print files command
*/
package main

import (
	"bytes"
	"clive/ch"
	"clive/cmd"
	"clive/cmd/opt"
	"clive/mblk"
	"clive/zx"
	"fmt"
	"net/http"
	"os"
	fpath "path"
	"time"
)

struct wFile {
	d   zx.Dir
	dat *mblk.Buffer
}

var (
	opts   = opt.New("{file}")
	printf = cmd.Printf
	odir   string

	hflag, notux, lflag, pflag, nflag, iflag, dflag, aflag, fflag, sflag, wflag, wwflag bool
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
	_, _, err := w.dat.SendTo(0, -1, dc)
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

var buf bytes.Buffer

func bufHandler(w http.ResponseWriter, r *http.Request) {
	w.Write(buf.Bytes())
	go func() {
		time.Sleep(time.Second) // http doesn't let serve just once
		os.Exit(0)
	}()
}

func main() {
	cmd.UnixIO("err")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("d", "no not print file data", &dflag)
	opts.NewFlag("f", "no not print dir data", &fflag)
	opts.NewFlag("n", "print just names", &nflag)
	opts.NewFlag("p", "print just paths", &pflag)
	opts.NewFlag("l", "long list for dirs", &lflag)
	opts.NewFlag("i", "print also ignored data", &iflag)
	opts.NewFlag("u", "don't use unix out", &notux)
	opts.NewFlag("h", "serve the output as a page sent to ink", &hflag)
	opts.NewFlag("a", "print addresses", &aflag)
	opts.NewFlag("o", "write destination path", &odir)
	opts.NewFlag("s", "separate messages, print each message in its own line.", &sflag)
	opts.NewFlag("w", "write file data back to disk (-d implied)", &wflag)
	opts.NewFlag("W", "writeall file data back to disk (-d implied)", &wwflag)
	args := opts.Parse()
	if !notux {
		cmd.UnixIO("out")
	}
	if len(args) != 0 {
		cmd.SetIn("in", cmd.Files(args...))
	}
	wflag = wflag || wwflag
	dflag = dflag || wflag

	in := cmd.In("in")
	out := cmd.Out("out")
	var w wFile
	var err error
	if hflag {
		printf = func(fmts string, arg ...face{}) (int, error) {
			return fmt.Fprintf(&buf, fmts, arg...)
		}
	}
	for m := range in {
		cmd.Dprintf("got %T\n", m)
		ok := true
		switch m := m.(type) {
		case error:
			err = m
			if wflag {
				if werr := w.end(); werr != nil {
					err = werr
					cmd.Warn("write: %s", werr)
				}
			}
			cmd.Warn("%s", m)
			if notux {
				out <- m
			}
		case zx.Dir:
			if wflag {
				if werr := w.end(); werr != nil {
					err = werr
					cmd.Warn("write: %s", werr)
				}
				if werr := w.start(m); werr != nil {
					err = werr
					cmd.Warn("write: %s", werr)
				}
			}
			if fflag {
				continue
			}
			var werr error
			switch {
			case nflag:
				_, werr = printf("%s\n", m["Upath"])
			case pflag:
				_, werr = printf("%s\n", m["path"])
			case lflag:
				_, werr = printf("%s\n", m.LongFmt())
			default:
				_, werr = printf("%s\n", m.Fmt())
			}
			if werr != nil {
				ok = false
			}
		case []byte:
			if wflag {
				if werr := w.write(m); werr != nil {
					err = werr
					cmd.Warn("write: %s", werr)
				}
			}
			if dflag {
				continue
			}
			if sflag && len(m) > 0 && m[len(m)-1] != '\n' {
				m = append(m, '\n')
			}
			if _, werr := printf("%s", string(m)); werr != nil {
				ok = false
			}
		case ch.Ign:
			if dflag || !iflag {
				continue
			}
			b := m.Dat
			ok = out <- b
		case string:
			if dflag || !iflag {
				continue
			}
			if _, werr := printf("%s", m); werr != nil {
				ok = false
			}
		case zx.Addr:
			if !aflag {
				continue
			}
			if _, werr := printf("%s\n", m); werr != nil {
				err = werr
				ok = false
			}
		}
		if !ok {
			close(in, cerror(out))
		}
	}
	if wflag {
		if werr := w.end(); werr != nil {
			err = werr
			cmd.Warn("write: %s", err)
		}
	}
	if hflag {
		ink := cmd.Out("ink")
		if ink == nil {
			cmd.Fatal("no ink out chan")
		}
		cert := "/zx/lib/webcert.pem"
		key := "/zx/lib/webkey.pem"
		http.HandleFunc("/", bufHandler)
		go http.ListenAndServeTLS(":10001", cert, key, nil)
		go http.ListenAndServe(":10000", nil)
		// The dance here to serve the page and
		// die once it's served is ugly, but it will be http
		// the one reponsible for listening before writing to ink
		// and then dying after the page has been served.
		// For now, this will do.
		time.Sleep(100 * time.Millisecond)
		cmd.Dprintf("serving: https://localhost:10001/\n")
		ink <- []byte("https://localhost:10001/")
		c := make(chan bool)
		<-c
	}
	if err := cerror(in); err != nil {
		cmd.Fatal(err)
	}
	cmd.Exit(err)
}
