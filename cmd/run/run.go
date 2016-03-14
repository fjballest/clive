/*
	run a command and use channels for I/O
*/
package run

import (
	"clive/ch"
	"clive/cmd"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// A running command.
// Out and Err correspond to the "out" and "err" channels.
// The status is reported by closing the Err channel using it.
// You can use ch.Merge() to merge Out and Err into a single stream.
struct Proc {
	Id    int
	Args  []string
	In    chan<- face{} // process input
	Out   <-chan face{} // process output
	Err   <-chan face{} // process errors
	in    <-chan face{}
	donec chan bool
	unix  bool
	x     *exec.Cmd
	ctx   *cmd.Ctx
}

func forkall(c *cmd.Ctx) {
	c.ForkEnv()
	c.ForkNS()
	c.ForkDot()
}

// Run args as a unix command with an open input channel.
// The command runs in a new clive cmd context with
//	"in" set to a new Proc.In chan
//	"out" set to a new Proc.Out chan
//	"err" set to a new Proc.Err chan
func PipeToUnix(args ...string) (*Proc, error) {
	in := make(chan face{})
	c, err := runCmd(forkall, true, in, args...)
	if err != nil {
		return nil, err
	}
	c.In = in
	return c, nil
}

// Run args as a clive command with an open input channel.
// The command runs in a new clive cmd context with
//	"in" set to a new Proc.In chan
//	"out" set to a new Proc.Out chan
//	"err" set to a new Proc.Err chan
func PipeTo(args ...string) (*Proc, error) {
	in := make(chan face{})
	c, err := runCmd(forkall, false, in, args...)
	if err != nil {
		return nil, err
	}
	c.In = in
	return c, nil
}

// Run args as a clive command with a context adjusted by the caller
// and an open input channel, and return it.
// The command runs in a new clive cmd context with:
//	"in" set to a new Proc.In chan
//	"out" set to a new Proc.Out chan
//	"err" set to a new Proc.Err chan
// Adjust is called in that context before actually starting
// the command, to let it adjust the context at will,
// but in, out, and err are set as said no matter what adjust does.
func PipeToCtx(adjust func(*cmd.Ctx), args ...string) (*Proc, error) {
	in := make(chan face{})
	c, err := runCmd(adjust, false, in, args...)
	if err != nil {
		return nil, err
	}
	c.In = in
	return c, nil
}

// Run args as a Unix command and return it.
// The command runs in a new clive cmd context with
//	"in" set to null
//	"out" set to a new Proc.Out chan
//	"err" set to a new Proc.Err chan
func UnixCmd(args ...string) (*Proc, error) {
	return runCmd(forkall, true, nil, args...)
}

// Run args as a Clive command and return it.
// The command runs in a new clive cmd context with:
//	"in" set to null
//	"out" set to a new Proc.Out chan
//	"err" set to a new Proc.Err chan
func Cmd(args ...string) (*Proc, error) {
	return runCmd(forkall, false, nil, args...)
}

// Run args as a clive command with a context adjusted by the caller, and return it.
// The command runs in a new clive cmd context with:
//	"in" set to null
//	"out" set to a new Proc.Out chan
//	"err" set to a new Proc.Err chan
// Adjust is called in that context before actually starting
// the command, to let it adjust the context at will,
// but in, out, and err are set as said no matter what adjust does.
func CtxCmd(adjust func(*cmd.Ctx), args ...string) (*Proc, error) {
	return runCmd(adjust, false, nil, args...)
}

func (p *Proc) input(c <-chan face{}, w io.WriteCloser) {
	if p.unix {
		ch.WriteBytes(w, c)
	} else {
		ch.WriteMsgs(w, 1, c)
	}
	w.Close()
}

// Wait for the command to terminate and return its status.
func (p *Proc) Wait() error {
	<-p.donec
	close(p.Out)
	close(p.Err)
	return cerror(p.donec)
}

func (p *Proc) output(r io.Reader, c chan<- face{}, iserr bool) {
	var err error
	if p.unix || iserr { // by now we use unix IO for stderr
		_, _, err = ch.ReadBytes(r, c)
	} else {
		_, _, err = ch.ReadMsgs(r, c)
	}
	close(c, err)
}

func cleanenv(env []string) []string {
	for i := 0; i < len(env); {
		if strings.HasPrefix(env[i], "dot=") || strings.HasPrefix(env[i], "cliveio#") ||
			strings.HasPrefix(env[i], "clivebg") {
			copy(env[i:], env[i+1:])
			env = env[:len(env)-1]
		} else {
			i++
		}
	}
	return env
}

func (p *Proc) addIn(name string) io.Closer {
	c := cmd.In(name)
	if c == nil {
		return nil
	}
	rfd, wfd, err := os.Pipe()
	if err != nil {
		return nil
	}
	xc := p.x
	no := 3 + len(xc.ExtraFiles)
	ev := fmt.Sprintf("cliveio#%s=<%d", name, no)
	xc.Env = append(xc.Env, ev)
	xc.ExtraFiles = append(xc.ExtraFiles, rfd)
	go func() {
		ch.WriteMsgs(wfd, 1, c)
		wfd.Close()
	}()
	return rfd
}

func (p *Proc) addOut(name string) io.Closer {
	c := cmd.Out(name)
	if c == nil {
		return nil
	}
	rfd, wfd, err := os.Pipe()
	if err != nil {
		return nil
	}
	xc := p.x
	no := 3 + len(xc.ExtraFiles)
	ev := fmt.Sprintf("cliveio#%s=>%d", name, no)
	xc.Env = append(xc.Env, ev)
	xc.ExtraFiles = append(xc.ExtraFiles, wfd)
	go func() {
		ch.ReadMsgs(rfd, c)
		rfd.Close()
	}()
	return wfd
}

func runCmd(adjust func(*cmd.Ctx), unix bool, in <-chan face{}, args ...string) (*Proc, error) {
	if len(args) == 0 || len(args[0]) == 0 {
		return nil, errors.New("no command name")
	}
	out := make(chan face{})
	ec := make(chan face{})
	p := &Proc{
		Args:  args,
		Out:   out,
		Err:   ec,
		in:    in,
		unix:  unix,
		donec: make(chan bool),
	}
	p.x = exec.Command(args[0], args[1:]...)
	startc := make(chan bool)
	p.ctx = cmd.New(func() {
		// adjust is forall by default and it forks the ns, dot, env
		// IO is always a dup.
		if !unix {
			p.x.Dir = cmd.Dot()
			p.x.Env = cleanenv(cmd.OSEnv())
			if path := cmd.LookPath(args[0]); path != "" {
				p.x.Path = path
			}
		}
		var closes, iocloses []io.Closer
		closeAll := func(closes []io.Closer) {
			for _, c := range closes {
				c.Close()
			}
		}
		rfd, wfd, err := os.Pipe()
		if err != nil {
			cmd.Exit(fmt.Errorf("run %s: pipe: %s", args[0], err))
		}
		defer wfd.Close()
		closes = append(closes, rfd)
		p.x.Stdout = wfd
		erfd, ewfd, err := os.Pipe()
		if err != nil {
			closeAll(closes)
			cmd.Exit(fmt.Errorf("run %s: pipe: %s", args[0], err))
		}
		defer ewfd.Close()
		closes = append(closes, erfd)
		p.x.Stderr = ewfd
		if in != nil {
			wfd, err := p.x.StdinPipe()
			if err != nil {
				closeAll(closes)
				cmd.Exit(fmt.Errorf("run %s: pipe: %s", args[0], err))
			}
			closes = append(closes, wfd)
			go p.input(in, wfd)
		}
		if !unix {
			ev := fmt.Sprintf("dot=%s", cmd.Dot())
			p.x.Env = append(p.x.Env, ev)
			i, o := cmd.Chans()
			for _, cn := range i {
				if cn == "in" || cn == "null" {
					continue
				}
				fd := p.addIn(cn)
				if fd != nil {
					closes = append(closes, fd)
					iocloses = append(iocloses, fd)
				}
			}
			for _, cn := range o {
				if cn == "out" || cn == "err" {
					continue
				}
				fd := p.addOut(cn)
				if fd != nil {
					closes = append(closes, fd)
					iocloses = append(iocloses, fd)
				}
			}
		}
		if err := p.x.Start(); err != nil {
			close(in, err)
			closeAll(closes)
			cmd.Exit(fmt.Errorf("run %s: start: %s", args[0], err))
		}
		p.Id = p.x.Process.Pid
		closeAll(iocloses)
		go p.output(rfd, out, false)
		go p.output(erfd, ec, true)
		close(p.donec, p.x.Wait())
	}, startc)
	adjust(p.ctx)
	close(startc)
	return p, nil

}
