/*
	Turn reservation system for jcenteno

	TODO
*/
package main

import (
	"time"
	"clive/app"
	"clive/app/opt"
	"strings"
	"strconv"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
)

const (
	Nerrs = 10
)


type xCmd {
	*opt.Flags
	*app.Ctx
}

// a reserve file has:
// N date time min title	// time frame #0 with n min slots starting at date/time
// N date time min title	// time frame #1 with n min slots starting at date/time
// ....	These are titled time frames with N slots as indicated.
// N title
// < i name	// add name to the frame i as early as possible
// > i name	// add name to the frame i as late as possible
// ! name		// remove name from the reservation
//
// This is append only and the last entry for name wins regarding the name

type Frame {
	Id int
	N int
	Start time.Time
	Ival time.Duration
	Title string

	Tot int
}

type File {
	Name string
	Frames []*Frame
	Ents []*Ent
}

const (
	Before = '<'
	After = '>'
	Not = '!'
)

type Ent {
	Order byte
	Frid int
	Who string
}

func parseFrame(ln string) (*Frame, error) {
	toks := strings.Fields(ln)
	if len(toks) < 5 {
		return nil, errors.New("format is 'N date time min title'")
	}
	fr := &Frame{}
	if n, err := strconv.Atoi(toks[0]); err != nil {
		return nil, errors.New("not a number for the number of slots")
	} else {
		fr.N = n
	}
	tm := toks[1] + " " + toks[2]
	if t, err := opt.ParseTime(tm); err != nil {
		return nil, errors.New("bad date/time format")
	} else {
		fr.Start = t
	}
	if n, err := time.ParseDuration(toks[3]); err != nil {
		return nil, errors.New("the slot size is not a duration")
	} else {
		fr.Ival = n
		if n < time.Minute {
			return nil, errors.New("slot duration is less than a minute")
		}
	}
	toks = toks[4:]
	fr.Title = strings.Join(toks, " ")
	return fr, nil
}

func parseEntry(ln string) (*Ent, error) {
	words := strings.Fields(ln)
	if len(words) < 3 || words[0] == "" {
		return nil, fmt.Errorf("bad entry '%s", ln)
	}
	
	e := &Ent{}
	switch words[0][0] {
	case Before, After, Not:
		e.Order = ln[0]
	default:
		return nil , fmt.Errorf("bad entry '%s'", ln)
	}
	if n, err := strconv.Atoi(words[1]); err != nil {
		return nil, fmt.Errorf("bad entry '%s'", ln)
	} else {
		e.Frid = n
	}
	e.Who = strings.Join(words[2:], "")
	return e, nil
}

func (e *Ent) String() string {
	return fmt.Sprintf("%c %d %s", e.Order, e.Frid, e.Who)
}

func (f *File) addEnt(e *Ent) error {
	if e == nil {
		return errors.New("null entry")
	}
	if e.Frid < 0 || e.Frid >= len(f.Frames) {
		return fmt.Errorf("bad entry frame id %d", e.Frid)
	}
	for i := range f.Ents {
		if f.Ents[i] != nil && f.Ents[i].Who == e.Who {
			if e.Order == Not {
				f.Ents[i] = nil
				copy(f.Ents[i:], f.Ents[i+1:])
				f.Ents = f.Ents[:len(f.Ents)-1]
				return nil
			}
			f.Ents[i] = e
			return nil
		}
	}
	if e.Order == Not {
		return nil
	}
	f.Ents = append(f.Ents, e)
	return nil
}

func (f *File) totals() {
	for _, e := range f.Ents {
		f.Frames[e.Frid].Tot++
	}
}

func (fr *Frame) String() string {
	return fmt.Sprintf("%d %s %v %s", fr.N, fr.Start.Format("2006-01-02 15:04"), fr.Ival, fr.Title)
}

func readFile(fname string) (*File, error) {
	dat, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	lns := strings.Split(string(dat), "\n")
	f := &File{
		Name: fname,
	}
	inentries := false
	nerrs := 0
	for i, ln := range lns {
		if nerrs > Nerrs {
			app.Warn("too many errors in '%s'; drop file", fname)
			return f, errors.New("too many errors")
		}
		if ln == "" || ln[0] == '#' {
			continue
		}
		switch ln[0] {
		case Before, After, Not:
			inentries = true
			e, err := parseEntry(ln)
			if err == nil {
				err = f.addEnt(e)
			}
			if err != nil {
				nerrs++
				app.Warn("%s:%d: %s", fname, i+1, err)
				continue
			}
		default:
			if !inentries {
				fr, err := parseFrame(ln)
				if err != nil {
					nerrs++
					app.Warn("%s:%d: %s", fname, i+1, err)
					continue
				}
				f.Frames = append(f.Frames, fr)
			}
		}
	}
	f.totals()
	return f, nil
}

func readFiles(names ...string) []*File {
	var files []*File
	for _, a := range names {
		if f, err := readFile(a); err != nil {
			app.Warn("%s", err)
		} else {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		app.Fatal("no reservation files")
	}
	return files
}

func (f *File) Dprint() {
	app.Dprintf("file %s\n", f.Name)
	for _, fr := range f.Frames {
		app.Dprintf("%s\n%d ents\n", fr, fr.Tot)
		
	}
	for _, e := range f.Ents {
		app.Dprintf("%s\n", e)
	}
}

func main() {
	app.New()
	x := &xCmd{Ctx: app.AppCtx()}
	x.Args = os.Args
	x.Flags = opt.New("file...")
	x.NewFlag("D", "debug", &x.Debug)
	args, err := x.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}
	if len(args) == 0 {
		app.Warn("wrong number of arguments")
		x.Usage()
		app.Exits("usage")
	}
	files := readFiles(args...)
	if x.Debug {
		for _, f := range files {
			f.Dprint()
		}
	}
	app.Exits(nil)
}
