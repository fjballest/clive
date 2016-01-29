/*
	run a command and use channels for I/O
*/
package run

import (
	"clive/ch"
	"clive/cmd"
	"os"
	"os/exec"
	"io"
	"errors"
	"strings"
	"fmt"
)

// A running command.
// Out and Err correspond to the "out" and "err" channels.
// The status is reported by closing the Err channel using it.
// You can use ch.Merge() to merge Out and Err into a single stream.
struct Proc {
	Args []string
	Out <-chan interface{}
	Err <-chan interface{}
	in <-chan interface{}
	donec chan bool
	unix bool
	x *exec.Cmd
}

// Run args as a unix command and return a channel to send input to it.
func PipeToUnix(args ...string) (chan<- interface{}, *Proc, error) {
	in := make(chan interface{})
	c, err := runCmd(true, in, args...)
	if err != nil {
		return nil, nil, err
	}
	return in, c, nil
}

// Run args as a Clive command and return a channel to send input to it.
func PipeTo(args ...string) (chan<- interface{}, *Proc, error) {
	in := make(chan interface{})
	c, err := runCmd(false, in, args...)
	if err != nil {
		return nil, nil, err
	}
	return in, c, nil
}

// Run args as a Unix command and return it.
func UnixCmd(args ...string) (*Proc, error) {
	return runCmd(true, nil, args...)
}

// Run args as a Clive command and return it.
func Cmd(args ...string) (*Proc, error) {
	return runCmd(false, nil, args...)
}

func (p *Proc) input(c <-chan interface{}, w io.WriteCloser) {
	if p.unix {
		ch.WriteBytes(w, c)
	} else {
		ch.WriteMsgs(w, 1, c)
	}
	w.Close()
}

// If wait we close donec when done; else we wait for it and wait for the cmd.
func (p *Proc) output(r io.Reader, c chan<-interface{}, donec chan bool, wait bool) {
	var err error
	if p.unix {
		_, _, err = ch.ReadBytes(r, c)
	} else {
		_, _, err = ch.ReadMsgs(r, c)
	}
	if !wait {
		close(donec)
	} else {
		<-donec
		werr := p.x.Wait()
		if err == nil {
			err = werr
		}
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

func runCmd(unix bool, in <-chan interface{}, args ...string) (*Proc, error) {
	if len(args) == 0 || len(args[0]) == 0 {
		return nil, errors.New("no command name")
	}
	out := make(chan interface{})
	ec := make(chan interface{})
	p := &Proc{
		Args: args,
		Out: out,
		Err: ec,
		in: in,
		unix: unix,
	}
	p.x = exec.Command(args[0], args[1:]...)
	if !unix {
		p.x.Dir = cmd.Dot()
		p.x.Env = cleanenv(cmd.OSEnv())
		if path := cmd.LookPath(args[0]); path != "" {
			p.x.Path = path
		}
	}
	var closes []io.Closer
	closeAll := func() {
		for _, c := range closes {
			c.Close()
		}
	}
	rfd, wfd, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("run %s: pipe: %s", args[0], err)
	}
	defer wfd.Close()
	closes = append(closes, rfd)
	p.x.Stdout = wfd
	erfd, ewfd, err := os.Pipe()
	if err != nil {
		closeAll()
		return nil, fmt.Errorf("run %s: pipe: %s", args[0], err)
	}
	defer ewfd.Close()
	closes = append(closes, erfd)
	p.x.Stderr = ewfd
	if in != nil {
		wfd, err := p.x.StdinPipe()
		if err != nil {
			closeAll()
			return nil, fmt.Errorf("run %s: pipe: %s", args[0], err)
		}
		closes = append(closes, wfd)
		go p.input(in, wfd)
	}
	if !unix {
		ev := fmt.Sprintf("dot=%s", cmd.Dot())
		p.x.Env = append(p.x.Env, ev)
	}
	if err := p.x.Start(); err != nil {
		close(in, err)
		closeAll()
		return nil, fmt.Errorf("run %s: start: %s", args[0], err)
	}
	donec := make(chan bool, 1)
	go p.output(rfd, out, donec, false)
	go p.output(erfd, ec, donec, true)
	return p, nil
	
}
