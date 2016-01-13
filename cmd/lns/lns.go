/*
	print lines in input
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/zx"
)

var (
	opts = opt.New("{file}")
	all             bool
	pflag, nflag    bool
	nhd, ntl, nfrom int
	ranges          []string
	addrs           []opt.Range
)

func parseRanges() error {
	for _, r := range ranges {
		a, err := opt.ParseRange(r)
		if err != nil {
			return err
		}
		from, to := a.P0, a.P1
		addrs = append(addrs, a)
		if from > 0 && nhd < from {
			nhd = from
		}
		if to > 0 && nhd < to {
			nhd = to
		}
		if from < 0 && ntl < -from {
			ntl = -from
		}
		if to < 0 && ntl < -to {
			ntl = -to
		}
		if from > 0 && to < 0 {
			nfrom = from
		}
		if from < 0 && to > 0 {
			nfrom = to
		}
		if from == 1 && to == -1 {
			all = true
		}
	}
	return nil
}

func lns(nm string, in chan []byte, donec chan bool) {
	last := []string{}
	nln := 0
	cmd.Dprintf("nhd %d ntl %d nfrom %d\n", nhd, ntl, nfrom)
	var err error
	for m := range in {
		s := string(m)
		lout := false
		nln++
		if all {
			if pflag {
				_, err = cmd.Printf("%s:%-5d %s", nm, nln, s)
			} else if nflag {
				_, err = cmd.Printf("%-5d %s", nln, s)
			} else {
				_, err = cmd.Printf("%s", s)
			}
			if err != nil {
				close(in, err)
			}
			continue
		}
		if ntl == 0 && nfrom == 0 && nhd > 0 && nln > nhd {
			close(in)
			close(donec)
			return
		}
		for _, a := range addrs {
			cmd.Dprintf("tl match %d of ? in %s\n", nln, a)
			if a.Matches(nln, 0) {
				lout = true
				if pflag {
					_, err = cmd.Printf("%s:%-5d %s", nm, nln, s)
				} else if nflag {
					_, err = cmd.Printf("%-5d %s", nln, s)
				} else {
					_, err = cmd.Printf("%s", s)
				}
				if err != nil {
					close(in, err)
				}
				break
			}
		}
		if nln >= nfrom || ntl > 0 {
			if lout {
				s = "" /*already there */
			}
			if nln >= nfrom || ntl > 0 && len(last) < ntl {
				last = append(last, s)
			} else {
				copy(last, last[1:])
				last[len(last)-1] = s
			}
		}

	}

	if !all && (ntl > 0 || nfrom > 0) {
		// if len(last) == 3 and nln is 10
		// last[0] is -3 or 10-2
		// last[1] is -2 or 10-1
		// last[2] is -1 or 10
		for i := 0; i < len(last); i++ {
			for _, a := range addrs {
				if a.P0 > 0 && a.P1 > 0 { // done already
					continue
				}
				cmd.Dprintf("tl match %d of %d in %s\n", nln-len(last)+1+i, nln, a)
				if a.Matches(nln-len(last)+1+i, nln) && last[i] != "" {
					if pflag {
						_, err = cmd.Printf("%s:%-5d %s",
							nm, nln-len(last)+1+i, last[i])
					} else if nflag {
						_, err = cmd.Printf("%-5d %s", nln-len(last)+1+i, last[i])
					} else {
						_, err = cmd.Printf("%s", last[i])
					}
					if err != nil {
						close(donec, err)
						return
					}
					last[i] = "" /* because if empty it still contains \n */
					break
				}
			}
		}
	}
	close(donec)
}

func runFiles(fn func(nm string, c chan []byte, dc chan bool)) {
	var lnc chan []byte
	var dc chan bool
	in := cmd.Lines(cmd.In("in"))
	out := cmd.Out("out")
	nm := "in"
	for m := range in {
		cmd.Dprintf("got %T\n", m)
		switch m := m.(type) {
		case zx.Dir:
			if pflag {
				p := m["Upath"]
				if p == "" {
					p = m["path"]
				}
				nm = p
			}
			if dc != nil {
				close(lnc)
				<-dc
			}
			dc = nil
			lnc = nil
			if ok := out <- m; !ok {
				close(in, cerror(out))
			}
		case []byte:
			if dc == nil {
				lnc = make(chan []byte)
				dc = make(chan bool, 1)
				go fn(nm, lnc, dc)
			}
			if ok := lnc <- m; !ok {
				close(in, cerror(lnc))
				close(out, cerror(lnc))
			}
		default:
			if dc != nil {
				close(lnc)
				<-dc
			}
			dc = nil
			lnc = nil
			if ok := out <- m; !ok {
				close(lnc, cerror(out))
				close(in, cerror(out))
			}
		}
	}
	if dc != nil {
		close(lnc, cerror(in))
		<-dc
	}
	cmd.Exit(cerror(in))
}

// Run print lines in the current app context.
func main() {
	c := cmd.AppCtx()
	cmd.UnixIO("err")
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("r", "range: print this range", &ranges)
	opts.NewFlag("n", "print line numbers", &nflag)
	opts.NewFlag("p", "print file names and line numbers", &pflag)
	ux := false
	opts.NewFlag("u", "use unix out", &ux)
	args, err := opts.Parse()
	if err != nil {
		cmd.Warn("%s", err)
		opts.Usage()
	}
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
	runFiles(lns)
}
