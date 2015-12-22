package ocfs

import (
	"bytes"
	"clive/dbg"
	"clive/zx"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (z *Cfs) setFlag(name string, flg *bool, args []string) {
	if len(args) == 1 {
		*flg = !*flg
	} else {
		*flg = args[1] == "on"
	}
	dbg.Warn("%s: %s %v", z.Name(), name, *flg)
}

func (z *Cfs) putctl(datc <-chan []byte, dc chan<- zx.Dir) error {
	var req bytes.Buffer

	for d := range datc {
		req.Write(d)
	}
	if err := cerror(datc); err != nil {
		close(dc, err)
		return err
	}
	str := req.String()
	args := strings.Fields(str)
	if len(args) == 0 {
		dc <- zx.Dir{"size": "0"}
		close(dc)
		return nil
	}
	var err error
	switch args[0] {
	case "refresh":
		if z.pollc != nil {
			go func() {
				dbg.Warn("%s: refresh...", z.Name())
				z.pollc <- 1
				z.pollc <- -1
				dbg.Warn("%s: refresh... done", z.Name())
			}()
		}
		wt, ok := z.fs.(zx.RWTree)
		if !ok {
			break
		}
		msg := req.Bytes()
		ndatc := make(chan []byte, 1)
		ndatc <- msg
		close(ndatc)
		wt.Put("/Ctl", nil, 0, ndatc, "")
	case "pass":
		wt, ok := z.fs.(zx.RWTree)
		if !ok {
			err = errors.New("can't pass to a read-only tree")
			break
		}
		msg := req.Bytes()[4:] // remove pass
		ndatc := make(chan []byte, 1)
		ndatc <- msg
		close(ndatc)
		ndc := wt.Put("/Ctl", nil, 0, ndatc, "")
		if d := <-ndc; d == nil {
			err = cerror(ndc)
		}
	case "debug":
		z.setFlag("debug", &z.Debug, args)
	case "cdebug":
		z.setFlag("cdebug", &z.Cdebug, args)
	case "zdebug":
		z.setFlag("zdebug", &z.Zdebug, args)
	default:
		err = fmt.Errorf("%s: %s", args[0], dbg.ErrBadCtl)
	}
	if err == nil {
		dc <- zx.Dir{"size": "0"}
	}
	close(dc, err)
	close(datc, err)
	return err
}

func (z *Cfs) getctl(off, count int64, c chan []byte, cs *zx.CallStat) (int, int64, error) {
	var buf bytes.Buffer
	if off > 0 {
		return 0, 0, nil
	}
	fmt.Fprintf(&buf, "%s:\n", z.Name())
	if z.who != nil {
		z.wholk.Lock()
		for k := range z.who {
			fmt.Fprintf(&buf, "user\t%s\n", k)
		}
		z.wholk.Unlock()
	}
	fmt.Fprintf(&buf, "debug  %v\n", z.Debug)
	fmt.Fprintf(&buf, "cdebug %v\n", z.Cdebug)
	fmt.Fprintf(&buf, "zdebug %v\n", z.Zdebug)
	fmt.Fprintf(&buf, "rdonly  %v\n", z.ronly)
	z.IOstats.Averages()
	fmt.Fprintf(&buf, "%s\n", z.IOstats.String())
	next, _ := zx.GetAll(z.fs, "/Ctl")
	if len(next) > 0 {
		buf.Write(next)
	}
	sts := buf.String()
	n := int(count)
	resp := []byte(sts)
	if n > len(resp) || n < 0 {
		n = len(resp)
	}
	c <- resp[:n]
	cs.End(false)
	return 1, int64(n), nil
}

// report a change made by the client served by z.
// zd must be locked by the caller.
func (z *Cfs) changed(zd *Dir) {
	if zd != nil {
		if z.ci == nil {
			z.ci = &zx.ClientInfo{Tag: "none"}
		}
		d := zd.d.Dup()
		if zd.ghost {
			d["rm"] = "y"
		}
		z.chgc <- chg{d: d, ci: z.ci}
	}
}

/*
	process in charge of notifying changes.
	We must send updates (for metadata, which are invals actually)
	to everyone, including those causing the changes.
	Otherwise two clients might perform concurrent updates and they would just
	swap their metadata and get out of sync.
*/
func (z *Cfs) chgProc() {
	defer z.dprintf("chproc", "done")
	chans := []getchgreq{}
Loop:
	for {
		var ch chg
		var ok bool
		select {
		case ch, ok = <-z.chgc:
			if !ok {
				break Loop
			}
			if ch.d == nil {
				dbg.Warn("chgproc: nil d")
				continue Loop
			}
		case getreq := <-z.getchgc:
			chans = append(chans, getreq)
			// ack the get with a null change
			getreq.getc <- chg{}
			continue Loop
		}
		sm := ch.d["Sum"]
		if len(sm) > 6 {
			sm = sm[:6] + "..."
		}
		z.dprintf(ch.d["path"], "change",
			"by %d %s meta %s %s sz %s v%s sum %s\n",
			ch.ci.Id, ch.ci.Tag,
			ch.d["type"], ch.d["mode"], ch.d["size"], ch.d["vers"], sm)
		for i := 0; i < len(chans); {
			x := chans[i]
			snail := cerror(x.getc) != nil
			if !snail {
				select {
				case x.getc <- ch:
				default:
					snail = true
				}
			}
			if snail {
				z.dprintf("/Chg", "notify", "slow reader %d %s\n",
					x.ci.Id, x.ci.Tag)
				n := len(chans)
				if i < n-1 {
					chans[i] = chans[n-1]
				}
				chans = chans[:n-1]
				close(x.getc, "too slow")
				continue
			}
			i++
		}
	}
}

/*
	If we rely on a server not providing invalidations (no /Chg, ie, epoch == 0),
	and we have clients that ask for invalidations, we must scan from time to time
	the underlying tree.
	Otherwise, the clients trust us and won't check out anything unless we
	sent them an update, yet our underlying fs won't send any updates for
	external changes, and our clients will have old data.

	We must only poll for external changes to anything we served, i.e., to
	anything we have in memory.

	And we can stop when the last /Chg reader goes.
*/
func (z *Cfs) pollProc() {
	nreaders := 0
	// want fixed interfvals, ticking with nbsend.
	go func() {
		for {
			time.Sleep(CachePollIval)
			select {
			case z.pollc <- 0:
			default:
			}
		}
	}()
	for n := range z.pollc {
		nreaders += n
		z.dprintf("/Chg", "pollproc", "%d readers epoch %d\n", nreaders, *z.epoch)
		z.root.poll(true)
	}
}

func (z *Cfs) getchg(off, count int64, c chan []byte, cs *zx.CallStat) (int, int64, error) {
	z.pollc <- 1
	defer func() {
		z.pollc <- -1
	}()
	getc := make(chan chg, 100)
	z.cprintf("/Chg", "changes", "asking for changes at %x\n", getc)
	z.getchgc <- getchgreq{getc: getc, ci: z.ci}
	var tot int64
	var nm int
	defer z.cprintf("/Chg", "changes", "changes closed at %x\n", getc)
	for ch := range getc {
		if ch.d == nil {
			// post null change to client to ack
			d := zx.Dir{}
			msg := d.Pack()
			c <- msg
			nm++
			tot += int64(len(msg))
			continue
		}
		d := ch.d
		z.cprintf(ch.d["path"], "notify",
			"to %d %s: change meta %s %s %s\n",
			z.ci.Id, z.ci.Tag,
			ch.d["type"], ch.d["mode"], ch.d["size"])
		msg := d.Pack()
		if ok := c <- msg; !ok {
			err := cerror(c)
			z.cprintf(d["path"], "post", err)
			close(getc, err)
			return nm, tot, err
		}
		nm++
		tot += int64(len(msg))
	}
	return nm, tot, nil
}

func (z *Cfs) ibufProc(dirc <-chan zx.Dir) {
	defer z.dprintf("ibufproc", "done")
Loop:
	for d := range dirc {
		gone := d["rm"] != ""
		dpath := d["path"]
		s := ""
		if gone {
			s = "gone"
		}
		z.cprintf(dpath, "update",
			"meta %s %s sz %s v %s %s\n",
			d["type"], d["mode"], d["size"], d["vers"], s)
		zd := z.root
		elems := zx.Elems(dpath)
		for len(elems) > 0 {
			el0 := elems[0]
			elems = elems[1:]
			zd.RLock()
			if zd.d["type"] != "d" {
				zd.cprintf("update", "invalidating")
				zd.RUnlock()
				zd.Lock()
				zd.invalAll()
				zd.data = nil
				zd.Unlock()
				continue Loop
			}
			if len(zd.child) == 0 {
				// not used, ignore the update
				z.cprintf(dpath, "update", "ignored: no here")
				zd.RUnlock()
				continue Loop
			}
			zcd, ok := zd.child[el0]
			if !ok || zcd.ghost {
				zd.RUnlock()
				if !gone {
					zd.Lock()
					// could just add the entry
					zd.cprintf("update", "old dir data")
					zd.invalData()
					zd.Unlock()
				}
				continue Loop
			}
			zd.RUnlock()
			zd = zcd
		}
		zd.Lock()
		zd.cprintf("oldmode", zd.d["mode"])
		mchg, dchg, err := zd.stated("update", d, nil)
		if mchg || dchg || err != nil {
			z.changed(zd)
		}
		zd.Unlock()
	}
}

func (z *Cfs) invalProc(getc <-chan []byte) {
	z.dprintf("invalproc", "started")
	defer z.dprintf("invalproc", "done")
	idirc := make(chan zx.Dir, 100)
	go z.ibufProc(idirc)
	for {
		for msg := range getc {
			d, left, err := zx.UnpackDir(msg)
			if err == nil && len(left) > 0 {
				err = errors.New("bytes left in dir msg")
			}
			if err != nil {
				z.dprintf("invalproc", "read", err)
				close(getc, err)
				break
			}
			idirc <- d
		}
		oldepoch := *z.epoch
		*z.epoch = 0
		err := cerror(getc)
		if err == nil {
			dbg.Warn("%s: invalidation chan closed\n", z.fs.Name())
			break
		}
		dbg.Warn("%s: invalidation chan closed: %s\n", z.fs.Name(), err)

		if !strings.Contains(err.Error(), "slow") || true {
			// TODO: to reopen the inval. chan we have to
			// make sure that
			break
		}
		getc := z.fs.Get("/Chg", 0, zx.All, "")
		if len(<-getc) == 0 {
			z.cprintf("/Chg", "get", cerror(getc))
			break
		}
		oldepoch++
		*z.epoch = oldepoch
		z.cprintf("invalproc", "Epoch", "%d\n", *z.epoch)
	}
	close(idirc)
}
