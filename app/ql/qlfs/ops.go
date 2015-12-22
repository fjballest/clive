package qlfs

import (
	"clive/app"
	"clive/app/ql"
	"clive/dbg"
	"fmt"
)

func newEnv(name string) *qEnv {
	e := &qEnv{
		name: name,
		vars: app.Env(),
		cmds: map[string]*qCmd{},
		runc: make(chan func()),
	}
	app.Go(e.run, "ctx")
	return e
}

func (e *qEnv) newCmd(name string) error {
	e.Lock()
	defer e.Unlock()
	if e.cmds[name] != nil {
		return fmt.Errorf("%s: %s: %s", e, name, dbg.ErrExists)
	}
	c := &qCmd{
		name: name,
		e:    e,
		in:   &qIO{},
		out:  &qIO{},
		err:  &qIO{},
	}
	e.cmds[name] = c
	return nil
}

func (e *qEnv) run() {
	e.Lock()
	app.NewEnv(e.vars)
	app.NewNS(nil)
	e.Unlock()
	for fn := range e.runc {
		if fn != nil {
			fn()
		}
	}
}

// called when e has been removed
func (e *qEnv) removed() {
	close(e.runc)
	for _, c := range e.cmds {
		c.removed()
	}
}

func (c *qCmd) start(txt string) error {
	c.Lock()
	if c.ctx != nil {
		c.Unlock()
		return fmt.Errorf("%s: one cmd is enough", c)
	}
	c.txt = txt
	var in chan interface{}
	if c.in.Len() > 0 {
		c.in.c = make(chan interface{}, len(c.in.msgs))
		for _, m := range c.in.msgs {
			c.in.c <- m
		}
		close(c.in.c)
		in = c.in.c
	}
	outfn := func(c *qCmd, io *qIO, outc chan interface{}) {
		for m := range outc {
			c.Lock()
			io.addOut(m)
			c.Unlock()
		}
		io.eof = true
		io.wakeup()
	}
	out := make(chan interface{})
	go outfn(c, c.out, out)
	err := make(chan interface{})
	go outfn(c, c.err, err)
	c.e.runc <- func() {
		c.ctx = app.Go(func() {
			c.Unlock()
			app.DupDot()
			app.DupEnv()
			app.NewIO(nil)
			app.SetIO(in, 0)
			app.SetIO(out, 1)
			app.SetIO(err, 2)
			ql.Run()
		}, "ql", "-c", c.txt)
	}
	return nil
}

func (c *qCmd) status() string {
	c.Lock()
	defer c.Unlock()
	if c.ctx == nil {
		return "not started\n"
	}
	select {
	case <-c.ctx.Wait:
	default:
		return "running\n"
	}
	sts := ""
	if c.ctx.Sts != nil {
		sts = c.ctx.Sts.Error() + "\n"
	}
	if sts == "" {
		return "success\n"
	}
	return sts
}

func (c *qCmd) post(sig string) {
	c.Lock()
	defer c.Unlock()
	if c.ctx != nil {
		app.Post(sig, c.ctx.Id)
	}
}

// called when c is removed
func (c *qCmd) removed() {
	c.post("kill")
}

func (c *qCmd) wait() {
	c.Lock()
	if c.ctx == nil {
		c.Unlock()
		return
	}
	c.Unlock()
	<-c.ctx.Wait
}

func (c *qCmd) clearIn() {
	c.Lock()
	defer c.Unlock()
	if c.ctx == nil {
		c.in.n = 0
		c.in.msgs = c.in.msgs[:0]
	}
}

func (c *qCmd) putIn(off int64, dc <-chan []byte) error {
	for x := range dc {
		c.Lock()
		c.in.msgs = append(c.in.msgs, x)
		c.in.n += len(x)
		c.Unlock()
	}
	return cerror(dc)
}

func (c *qCmd) get(in *qIO, off, count int64, dc chan<- []byte) (int64, error) {
	c.Lock()
	defer c.Unlock()
	tot := int64(0)
	for i := 0; i < len(in.msgs) && count != 0; i++ {
		m := in.msgs[i]
		b, ok := m.([]byte)
		if ok && len(b) > 0 {
			nb := int64(len(b))
			if off >= nb {
				off -= nb
				continue
			}
			if off > 0 {
				nb -= off
				b = b[int(off):]
				off = 0
			}
			n := nb
			if count > 0 && n > count {
				n = count
				count -= n
			}
			tot += n
			c.Unlock()
			ok := dc <- b[:n]
			c.Lock()
			if !ok {
				return tot, cerror(dc)
			}
		}
	}
	return tot, nil
}

func (c *qCmd) getIn(off, count int64, dc chan<- []byte) error {
	_, err := c.get(c.in, off, count, dc)
	return err
}

func (c *qCmd) getOut(off, count int64, dc chan<- []byte) error {
	_, err := c.get(c.out, off, count, dc)
	return err
}

func (c *qCmd) getErr(off, count int64, dc chan<- []byte) error {
	_, err := c.get(c.err, off, count, dc)
	return err
}

func (c *qCmd) pwait(out *qIO, off int64) {
	c.Lock()
	if int64(out.n) > off {
		c.Unlock()
		return
	}
	wc := make(chan bool)
	out.wc = append(out.wc, wc)
	c.Unlock()
	<-wc
}

func (out *qIO) wakeup() {
	for _, wc := range out.wc {
		close(wc)
	}
	out.wc = nil
}

func (out *qIO) addOut(m interface{}) {
	out.msgs = append(out.msgs, m)
	if b, ok := m.([]byte); ok {
		out.n += len(b)
	}
	out.wakeup()
}

func (c *qCmd) getp(out *qIO, off, count int64, dc chan<- []byte) error {
	c.Lock()
	if c.ctx == nil {
		c.Unlock()
		return fmt.Errorf("%s: not started", c)
	}
	c.Unlock()
	for count != 0 {
		n, err := c.get(out, off, count, dc)
		if err != nil {
			return err
		}
		if n == 0 {
			// wait for more or close if command is done
			select {
			case <-c.ctx.Wait:
				err := cerror(c.ctx.Wait)
				close(dc, err)
				return err
			default:
				if out.eof {
					close(dc)
					return nil
				}
				c.pwait(out, off)
			}
			continue
		}
		off += n
		if count > 0 {
			count -= n
			// ZX clients would ask for -1 to ask for all the data.
			// FUSE asks for 64k or so.
			// If we wait until we get 64K, FUSE may time out
			// and we don't stream output as we should.
			// So we break instead of honoring zx count semantics here.
			break
		}
	}
	close(dc)
	return nil
}

func (c *qCmd) getPout(off, count int64, dc chan<- []byte) error {
	return c.getp(c.out, off, count, dc)
}

func (c *qCmd) getPerr(off, count int64, dc chan<- []byte) error {
	return c.getp(c.err, off, count, dc)
}
