/*
	print lines in input
*/
package lns

import (
	"clive/dbg"
	"clive/app"
	"clive/app/opt"
	"clive/zx"
)

type xCmd {
	*opt.Flags
	*app.Ctx

	all             bool
	pflag, nflag           bool
	nhd, ntl, nfrom int
	ranges          []string
	addrs           []opt.Range
}

func (x *xCmd) parseRanges() error {
	for _, r := range x.ranges {
		a, err := opt.ParseRange(r)
		if err != nil {
			return err
		}
		from, to := a.P0, a.P1
		x.addrs = append(x.addrs, a)
		if from>0 && x.nhd<from {
			x.nhd = from
		}
		if to>0 && x.nhd<to {
			x.nhd = to
		}
		if from<0 && x.ntl< -from {
			x.ntl = -from
		}
		if to<0 && x.ntl< -to {
			x.ntl = -to
		}
		if from>0 && to<0 {
			x.nfrom = from
		}
		if from<0 && to>0 {
			x.nfrom = to
		}
		if from==1 && to==-1 {
			x.all = true
		}
	}
	return nil
}

func (x *xCmd) lns(in chan []byte, donec chan bool) {
	last := []string{}
	nln := 0
	app.Dprintf("nhd %d ntl %d nfrom %d\n", x.nhd, x.ntl, x.nfrom)
	doselect {
	case <-x.Sig:
		close(donec, dbg.ErrIntr)
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-in:
		if !ok {
			break
		}
		s := string(m)
		lout := false
		nln++
		if x.all {
			if x.nflag {
				app.Printf("%-5d %s", nln, s)
			} else {
				app.Printf("%s", s)
			}
			continue
		}
		if x.ntl==0 && x.nfrom==0 && x.nhd>0 && nln>x.nhd {
			close(in, "done")
			close(donec)
			return
		}
		for _, a := range x.addrs {
			app.Dprintf("tl match %d of ? in %s\n", nln, a)
			if a.Matches(nln, 0) {
				lout = true
				if x.nflag {
					app.Printf("%-5d %s", nln, s)
				} else {
					app.Printf("%s", s)
				}
				break
			}
		}
		if nln>=x.nfrom || x.ntl>0 {
			if lout {
				s = "" /*already there */
			}
			if nln>=x.nfrom || x.ntl>0 && len(last)<x.ntl {
				last = append(last, s)
			} else {
				copy(last, last[1:])
				last[len(last)-1] = s
			}
		}

	}

	if !x.all && (x.ntl>0 || x.nfrom>0) {
		// if len(last) == 3 and nln is 10
		// last[0] is -3 or 10-2
		// last[1] is -2 or 10-1
		// last[2] is -1 or 10
		for i := 0; i < len(last); i++ {
			for _, a := range x.addrs {
				if a.P0>0 && a.P1>0 { // done already
					continue
				}
				app.Dprintf("tl match %d of %d in %s\n", nln-len(last)+1+i, nln, a)
				if a.Matches(nln-len(last)+1+i, nln) && last[i]!="" {
					if x.nflag {
						app.Printf("%-5d %s", nln-len(last)+1+i, last[i])
					} else {
						app.Printf("%s", last[i])
					}
					last[i] = "" /* because if empty it still contains \n */
					break
				}
			}
		}
	}
	close(donec)
}

func (x *xCmd) runFiles(fn func(c chan []byte, dc chan bool), in chan interface{}) {
	var lnc chan []byte
	var dc chan bool
	out := app.Out()
	doselect {
	case <-x.Sig:
		close(lnc, dbg.ErrIntr)
		app.Fatal(dbg.ErrIntr)
	case m, ok := <-in:
		if !ok {
			break
		}
		app.Dprintf("got %T\n", m)
		switch m := m.(type) {
		case zx.Dir:
			if x.pflag {
				app.Printf("%s:\n", m["upath"])
			}
			if dc != nil {
				close(lnc)
				<-dc
			}
			dc = nil
			lnc = nil
			out <- m
		case []byte:
			if dc == nil {
				lnc = make(chan []byte)
				dc = make(chan bool, 1)
				go fn(lnc, dc)
			}
			lnc <- m
		default:
			if dc != nil {
				close(lnc)
				<-dc
			}
			dc = nil
			lnc = nil
			out <- m
		}
	}
	if dc != nil {
		close(lnc, cerror(in))
		<-dc
	}
}

// Run print lines in the current app context.
func Run() {
	x := &xCmd{Ctx: app.AppCtx()}
	x.Flags = opt.New("{file}")
	x.NewFlag("D", "debug", &x.Debug)
	x.NewFlag("r", "range: print this range", &x.ranges)
	x.NewFlag("n", "print line numbers", &x.nflag)
	x.NewFlag("p", "print file names and line numbers", &x.pflag)
	args, err := x.Parse(x.Args)
	if err != nil {
		app.Warn("%s", err)
		x.Usage()
		app.Exits("usage")
	}
	if len(args) != 0 {
		in := app.Files(args...)
		app.SetIO(in, 0)
	}
	if len(x.ranges) == 0 {
		x.ranges = append(x.ranges, ",")
	}
	if err := x.parseRanges(); err != nil {
		app.Fatal(err)
	}
	in := app.Lines(app.In())
	x.runFiles(x.lns, in)
	app.Exits(cerror(in))
}
