/*
	xcmd is a command launcher

	Dir structure:
		/xcmd
			name/
				args: command line
				in: stdin
				out: stdout
				err: stderr
				sig: signal to be sent
				xx: set $xx to its contents
				restart: restart if it dies
				at: run at this time spec in cron format
				bin: restart if bin changes
				exit: status after the command has exited
			/name
				...
*/
package main

/*
	dirs are ignored until there is a non-empty args file.

	known signals are kill, quit, intr, and stack.
	stack is implemented with SIGUSR1 and all clive commands intercept
	it to dump all stacks and continue execution.

	restart sends a term and then kill and then restarts the command.

	args is an rc command line, use exec to run a Go command so that
	quit and stack signals work as expected.
	otherwise it's rc the one getting the signals.

*/

import (
	"clive/dbg"
	"clive/app/opt"
	"os"
	"os/exec"
	"bytes"
	"io/ioutil"
	"errors"
	"path"
	"fmt"
	"strings"
	"time"
	"strconv"
	"syscall"
)

// A command to be run under xcmd control.
type Cmd {
	Ln string	// command line
	Restart bool	// restart if the command dies
	At string	// run at time times (cron format: min hour day month wday)
	Bin string	// restart if the binary changes
	Env map[string]string
	x *exec.Cmd
	name string	// Base(dir)
	dir string	// path of the dir for the cmd
	runt time.Time	// start time

	donec chan bool
	sigrestart bool
	bint time.Time
}

var (
	opts = opt.New("")
	debug, verb bool
	dprintf = dbg.FlagPrintf(os.Stderr, &debug)
	vprintf = dbg.FlagPrintf(os.Stderr, &verb)
	shell = "rc"
	dir = "/tmp/xcmd"
	cmd1 string
	ErrNotYet = errors.New("not ready to start")
	ErrExited = errors.New("command has exited")
	ErrRestart = errors.New("restart required")
)


func walk(ds []os.FileInfo, name string) os.FileInfo {
	for _, d := range ds {
		if d.Name() == name {
			return d
		}
	}
	return nil
}

func (c *Cmd) String() string {
	var b bytes.Buffer
	fmt.Fprintf(&b, "cmd %s:\n", c.dir)
	fmt.Fprintf(&b, "args: %s\n", c.Ln)
	if c.Restart {
		fmt.Fprintf(&b, "restart\n")
	}
	if c.At != "" {
		fmt.Fprintf(&b, "run at %s\n", c.At)
	}
	if c.Bin != "" {
		fmt.Fprintf(&b, "watch bin %s\n", c.Bin)
	}
	for k, v := range c.Env {
		fmt.Fprintf(&b, "env %s=%s\n", k, v)
	}
	return b.String()
}

func (c *Cmd) fileStr(name string) (string, error) {
	dat, err := ioutil.ReadFile(path.Join(c.dir, name))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(dat)), nil
}

func (c *Cmd) exit(s string) {
	dprintf("%s: exit '%s'\n", c.name, s)
	fn := path.Join(c.dir, "exit")
	ioutil.WriteFile(fn, []byte(s+"\n"), 0640)
	if s != "" {
		flag := os.O_WRONLY|os.O_APPEND|os.O_CREATE
		fd, err := os.OpenFile(path.Join(c.dir, "err"), flag, 0644)
		if err != nil {
			return
		}
		defer fd.Close()
		fmt.Fprintf(fd, "exited: %s\n", s)
	}
}

func setEnv(env []string, name, val string) []string {
	pref := name + "=";
	nv := fmt.Sprintf("%s=%s", name, val)
	for i, e := range env {
		if strings.HasPrefix(e, pref) {
			env[i] = nv
			return env
		}
	}
	return append(env, nv)
}

func (c *Cmd) Start() error {
	dprintf("%s: start\n", c.name)
	os.Remove(path.Join(c.dir, "sig"))
	c.sigrestart = false
	c.runt = time.Now()
	x := exec.Command(shell, "-c", c.Ln)
	c.x = x
	fd, err := os.Open(path.Join(c.dir, "in"))
	if err != nil {
		return fmt.Errorf("%s: in: %s", c.name, err)
	}
	x.Stdin = fd
	fd, err = os.Create(path.Join(c.dir, "out"))
	if err != nil {
		return fmt.Errorf("%s: out: %s", c.name, err)
	}
	x.Stdout = fd
	flag := os.O_WRONLY|os.O_APPEND|os.O_CREATE
	fd, err = os.OpenFile(path.Join(c.dir, "err"), flag, 0644)
	if err != nil {
		return fmt.Errorf("%s: err: %s", c.name, err)
	}
	if len(c.Env) > 0 {
		env := os.Environ()
		for k, v := range c.Env {
			env = setEnv(env, k , v)
		}
		x.Env = env
	}
	x.Stderr = fd
	if err = x.Start(); err != nil {
		c.exit(err.Error())
	}
	go c.ctlproc()
	return err
}

func (c *Cmd) ctlproc() {
	dprintf("%s: ctlproc started\n", c.name)
	defer dprintf("%s: ctlproc terminated\n", c.name)
	doselect {
	case <-c.donec:
		return
	case <-time.After(5*time.Second):
		sig := ""
		if c.Bin != "" {
			if d, err := os.Stat(c.Bin); err == nil {
				if d.ModTime().After(c.bint) {
					dprintf("%s: kill\n", c.name)
					sig = "restart"
				}
			}
		}
		if sig == "" {
			sig, _ = c.fileStr("sig")
			if sig != "" {
				os.Remove(path.Join(c.dir, "sig"))
			}
		}
		if sig == "" {
			continue
		}
		p := c.x.Process
		switch sig {
		case "kill":
			dprintf("%s: sig: kill\n", c.name)
			p.Kill()
		case "stack":
			dprintf("%s: sig: stack\n", c.name)
			p.Signal(os.Signal(syscall.SIGUSR1))
			continue
		case "quit":
			dprintf("%s: sig: quit\n", c.name)
			p.Signal(syscall.SIGQUIT)
		case "restart":
			c.sigrestart = true
			dprintf("%s: sig: restart\n", c.name)
			p.Signal(syscall.SIGTERM)
			time.Sleep(time.Second)
			p.Kill()
			
		default:
			dprintf("%s: sig: intr\n", c.name)
			p.Signal(os.Interrupt)
		}
		break
	}
}

func (c *Cmd) Wait() error {
	err := c.x.Wait()
	close(c.donec)
	if c.sigrestart {
		err = ErrRestart
	}
	if err == nil {
		c.exit("")
		return nil
	}
	c.exit(err.Error())
	return err
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func timeHasCome(toks []string) bool {
	m, h, dm, mo, dw := toks[0], toks[1], toks[2], toks[3], toks[4]
	now := time.Now()
	if m != "*" && atoi(m) != now.Minute() {
		return false
	}
	if h != "*" && atoi(h) != now.Hour() {
		return false
	}
	if dm != "*" && atoi(dm) != now.Day() {
		return false
	}
	if mo != "*" && atoi(mo) != int(now.Month()) {
		return false
	}
	if dw != "*" && atoi(dw) != int(now.Weekday()) {
		return false
	}
	return true
}

func (c *Cmd) waitForStart() error {
	dprintf("%s: waiting for '%s'...\n", c.name, c.At)
	for {
		toks := strings.Fields(c.At)
		if len(toks) != 5 {
			err := errors.New("'at' must have 5 fields")
			dbg.Warn("%s: %s", c.name, err)
			return err
		}
		time.Sleep(time.Minute)
		if timeHasCome(toks) {
			return nil
		}
		// reload at in case it was edited
		nat, _ := c.fileStr("at")
		if nat != "" {
			c.At = nat
		}
	}
}

func (c *Cmd) Go() error {
	for {
		if c.At != "" {
			if err := c.waitForStart(); err != nil {
				vprintf("%s: waitForStart: %s\n", c.name, err)
				return err
			}
		}
		if err := c.Start(); err != nil {
			vprintf("%s: start: %s\n", c.name, err)
			if c.At != "" {
				continue
			}
			return err
		}
		err := c.Wait()
		if err != ErrRestart && err != nil && !c.Restart && c.At == "" {
			vprintf("%s: wait: %s\n", c.name, err)
			return err
		}
		vprintf("%s: exit sts '%v'\n", c.name, err)
		if err != ErrRestart && c.Restart && time.Since(c.runt) < 5*time.Second {
			time.Sleep(5*time.Second)
		}
		if err := c.load(); err != nil {
			vprintf("%s: load: %s\n", c.name, err)
			return err
		}
	}
}

func NewCmd(dir string) (*Cmd, error) {
	c := &Cmd{
		dir: dir,
		name: path.Base(dir),
		Env: make(map[string]string),
	}
	return c, c.load()
}

func (c *Cmd) load() error {
	var err error
	c.donec = make(chan bool, 1)
	c.Ln, err = c.fileStr("args")
	if err != nil {
		return ErrNotYet
	}
	ds, err := ioutil.ReadDir(c.dir)
	if err != nil {
		return err
	}
	c.At, _ = c.fileStr("at")
	if c.At == "" {
		c.Bin, _ = c.fileStr("bin")
		c.Restart = c.At == "" && walk(ds, "restart") != nil
	}
	if c.Bin != "" {
		st, err := os.Stat(c.Bin)
		if err == nil {
			c.bint = st.ModTime()
		}
	}
	if walk(ds, "exit") != nil && !c.Restart && c.At == "" && !c.sigrestart {
		return ErrExited
	}
	for _, d := range ds {
		name := d.Name()
		switch name {
		case "args", "at", "bin", "src", "restart", "in", "out", "err", "exit", "sig":
			continue
		}
		val, err := c.fileStr(name)
		if err == nil {
			c.Env[name] = val
		}
	}
	if walk(ds, "in") == nil {
		fn := path.Join(c.dir, "in")
		if err := ioutil.WriteFile(fn, []byte{}, 0640); err != nil {
			return fmt.Errorf("%s: in: %s", c.name, err)
		}
	}	
	return nil
}

func xcmd() error {
	cmds := map[string]*Cmd{}
	for {
		ds, err := ioutil.ReadDir(dir)
		if err != nil {
			dbg.Warn("%s: %s", dir, err)
			return err
		}
		ncmds := map[string]bool{}
		for _, d := range ds {
			nm := d.Name()
			ncmds[nm] = true
			c := cmds[nm]
			if c != nil {
				continue	// old
			}
			c, err = NewCmd(path.Join(dir, nm))
			if err == ErrNotYet {
				dprintf("%s: %s\n", nm, err)
				continue
			}
			if err != nil {
				dbg.Warn("%s: %s", nm, err)
				continue
			}
			vprintf("go %s\n", nm)
			dprintf("new cmd:\n%s\n", c)
			cmds[nm] = c
			go c.Go()
		}
		for nm, c := range cmds {
			if !ncmds[nm] {
				// dir is gone
				// kill in case it's still running
				vprintf("gone %s\n", nm)
				if c.x != nil {
					p := c.x.Process
					if p != nil {
						p.Kill()
					}
				}
				delete(cmds, nm)
			}
		}
		time.Sleep(5*time.Second)
	}
}

func main() {
	defer dbg.Exits("")
	os.Args[0] = "xcmd"
	dir = fmt.Sprintf("/x/%s", dbg.Sys)
	opts.NewFlag("D", "debug", &debug)
	opts.NewFlag("s", "shell: shell used to run commands", &shell)
	opts.NewFlag("c", "cmd: run just cmd dir and exit", &cmd1)
	opts.NewFlag("d", "dir: use dir to find commands (default /x/<sys>)", &dir)
	opts.NewFlag("v", "verbose (report commands started)", &verb)
	args, err := opts.Parse(os.Args)
	if err != nil {
		dbg.Warn("%s", err)
		opts.Usage()
		dbg.Exits(err)
	}
	if len(args) != 0 {
		dbg.Warn("too many arguments")
		opts.Usage()
		dbg.Exits("usage")
	}
	if cmd1 != "" {
		c, err := NewCmd(cmd1)
		if err != nil {
			dbg.Fatal("new: %s", err)
		}
		vprintf("cmd %s\n", c.name)
		dprintf("%s\n", c)
		if err = c.Go(); err != nil {
			dbg.Fatal("wait: %s", err)
		}
		dbg.Exits(err)
	}
	d, err := os.Stat(dir)
	if err != nil {
		dbg.Fatal("%s: %s", dir, err)
	}
	if !d.IsDir() {
		dbg.Fatal("%s: %s", dir, dbg.ErrNotDir)
	}
	dbg.Warn("running commands at %s", dir)
	dbg.Exits(xcmd())
}
