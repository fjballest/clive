/*
	print fields in input
*/
package main

import (
	"bytes"
	"clive/cmd"
	"clive/cmd/opt"
	"fmt"
	"strings"
)

var (
	opts = opt.New("{file}")

	ranges []string
	one    bool
	seps   string
	osep   string
	addrs  []opt.Range
	all    bool
)

func parseRanges() error {
	for _, r := range ranges {
		a, err := opt.ParseRange(r)
		if err != nil {
			return err
		}
		if a.P0 == 1 && a.P1 == -1 {
			all = true
		}
		addrs = append(addrs, a)
	}
	return nil
}

func flds(in <-chan face{}, out chan<- face{}) {
	osep := "\t"
	if osep != "" {
		osep = osep
	} else {
		if seps == "" {
			osep = "\t"
		} else {
			osep = seps
		}
		if !one {
			osep = osep[:1]
		}
	}
	for m := range in {
		dat, ok := m.([]byte)
		if !ok {
			cmd.Dprintf("got %T\n", m)
			out <- m
			continue
		}
		s := string(dat)
		if len(s) > 0 && s[len(s)-1] == '\n' {
			s = s[:len(s)-1]
		}
		var fields []string
		if one {
			if seps == "" {
				seps = "\t"
			}
			fields = strings.Split(s, seps)
		} else {
			if seps == "" {
				fields = strings.Fields(s)
			} else {
				fields = strings.FieldsFunc(s, func(r rune) bool {
					return strings.ContainsRune(seps, r)
				})
			}
		}
		if all {
			cmd.Printf("%s\n", strings.Join(fields, osep))
			continue
		}
		b := &bytes.Buffer{}
		sep := ""
		for na, a := range addrs {
			for i, fld := range fields {
				nfld := i + 1
				cmd.Dprintf("tl match %d of %d in a%d %s\n", nfld, len(fields), na, a)
				if a.Matches(nfld, len(fields)) {
					fmt.Fprintf(b, "%s%s", sep, fld)
					sep = osep
				}
			}
		}
		fmt.Fprintf(b, "\n")
		if ok := out <- b.Bytes(); !ok {
			cmd.Fatal(cerror(out))
		}
	}
}

// Run print fields in the current app context.
func main() {
	cmd.UnixIO("err")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("r", "range: print this range", &ranges)
	opts.NewFlag("F", "sep: input field delimiter character(s) (or string under -1)", &seps)
	opts.NewFlag("o", "sep: output field delimiter string", &osep)
	opts.NewFlag("1", "fields separated by 1 run of the field delimiter string", &one)
	ux := false
	opts.NewFlag("u", "use unix out", &ux)
	args := opts.Parse()
	if ux {
		cmd.UnixIO("out")
	}
	if len(args) != 0 {
		cmd.SetIn("in", cmd.Files(args...))
	}
	if len(ranges) == 0 {
		ranges = append(ranges, ",")
	}
	if err := parseRanges(); err != nil {
		cmd.Fatal(err)
	}
	in := cmd.Lines(cmd.In("in"))
	flds(in, cmd.Out("out"))
	if err := cerror(in); err != nil {
		cmd.Fatal(err)
	}
}
