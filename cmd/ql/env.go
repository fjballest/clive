package main

import (
	"clive/cmd"
	"strconv"
	"sort"
	"sync"
	"clive/u"
	"errors"
	"fmt"
	"strings"
)

var (
	fnslk sync.Mutex
	builtins = map[string]func(x*xEnv, args ...string)error{}
	fns = map[string]*Nd{}
	xpath []string

	errBreak = errors.New("break")
)

func newFunc(nd *Nd) {
	fnslk.Lock()
	fns[nd.Args[0]] = nd
	cmd.VWarn("func %s defined", nd.Args[0])
	fnslk.Unlock()
}

func getFunc(name string) *Nd {
	fnslk.Lock()
	defer fnslk.Unlock()
	return fns[name]
}

func init() {
	builtins["type"] = btype
	builtins["cd"] = bcd
	builtins["pwd"] = bpwd
	builtins["fork"] = bfork
	builtins["wait"] = bwait
	builtins["exit"] = bexit
	builtins["break"] = bbreak
	builtins["shift"] = bshift
}

func bshift(x *xEnv, args ...string) error {
	if len(args) > 2 {
		cmd.Warn("usage: shift [var]")
		return nil
	}
	v := "argv"
	if len(args) == 2 {
		v = args[1]
	}
	argv := cmd.GetEnvList(v)
	if len(argv) > 0 {
		cmd.SetEnvList(v, argv[1:])
	}
	return nil
}

func btype(x *xEnv, args ...string) error {
	for _, a := range args[1:] {
		found := false
		if getFunc(a) != nil {
			cmd.Printf("%s: func\n", a)
			found = true
		}
		if builtins[a] != nil {
			cmd.Printf("%s: builtin\n", a)
			found = true
		}
		if p := cmd.LookPath(a); p != "" {
			cmd.Printf("%s: %s\n", a, p)
		} else if !found {
			cmd.Printf("%s: unknown\n", a)
		}
	}
	return nil
}

func bcd(x *xEnv, args ...string) error {
	var err error
	switch len(args) {
	case 1:
		err = cmd.Cd(u.Home)
	case 2:
		d, derr := cmd.Dir(args[1])
		if derr != nil {
			err = derr
		} else {
			err = cmd.Cd(d["path"])
		}
	case 0:
		err = errors.New("missing argument");
	}
	if err != nil {
		cmd.Eprintf("cd: %s", err)
		cmd.SetEnv("sts", err.Error())
	} else {
		cmd.SetEnv("sts", "")
	}
	return nil
}

func bpwd(x *xEnv, args ...string) error {
	var err error
	if len(args) > 1 {
		err = errors.New("too many arguments");
		cmd.Eprintf("cd: %s", err)
		cmd.SetEnv("sts", err.Error())
	} else {
		cmd.SetEnv("sts", "")
	}
	cmd.Printf("%s\n", cmd.Dot())
	return nil
}

func isExit(err error) bool {
	return err != nil && strings.HasPrefix(err.Error(), "qlexit")
}

func isBreak(err error) bool {
	return err == errBreak || (err != nil && err.Error() == "break")
}

func bexit(x *xEnv, args ...string) error {
	if len(args) == 1 {
		return fmt.Errorf("qlexit")
	}
	return fmt.Errorf("qlexit%s", strings.Join(args, " "))
}

func bbreak(x *xEnv, args ...string) error {
	return errBreak
}

func bfork(x *xEnv, args ...string) error {
	var err error
	for _, a := range args[1:] {
		switch a {
		case "ns":
			cmd.ForkNS()
		case "env":
			cmd.ForkEnv()
		case "dot":
			cmd.ForkDot()
		case "io":
			cmd.ForkIO()
		default:
			err = fmt.Errorf("%s unknown (not ns, env, dot)", a)
			cmd.Eprintf("cd: %s", err)
			cmd.SetEnv("sts", err.Error())
			return nil
		}
	}
	cmd.SetEnv("sts", "")
	return nil
}

func bwait(x *xEnv, args ...string) error {
	if len(args) == 1 {
		args = append(args, "")
	}
	for _, a := range args[1:] {
		bgcmds.wait(a)
	}
	cmd.SetEnv("sts", "")
	return nil
}

// Vars are lists separated by \b
// Maps are lists with of key-value lists preceded by \a
// the first item in each list is the key.

func mapKeys(m map[string][]string) []string {
	s := []string{}
	for k := range m {
		s = append(s, k)
	}
	sort.Sort(sort.StringSlice(s))
	return s
}

func listEl(lst []string, idxs string) string {
	n, err := strconv.Atoi(idxs)
	i := int(n)
	if err != nil {
		return ""
	}
	if i < 0 || i >= len(lst) {
		return ""
	}
	return lst[i]
}

func setListEl(lst []string, idxs, val string) []string {
	n, err := strconv.Atoi(idxs)
	i := int(n)
	if err != nil || i < 0 || i > len(lst) {
		return lst
	}
	if i == len(lst) {
		return append(lst, val)
	}
	lst[i] = val
	return lst
}
