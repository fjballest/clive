package main

import (
	"clive/cmd"
	"strconv"
	"strings"
	"sort"
	"sync"
	"os/exec"
)

var (
	fnslk sync.Mutex
	builtins = map[string]func(x*xEnv, args ...string)error{}
	fns = map[string]*Nd{}
	xpath []string

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
}

func btype(x *xEnv, args ...string) error {
	for _, a := range args[1:] {
		if getFunc(a) != nil {
			cmd.Printf("func %s\n", a)
		}
		if builtins[a] != nil {
			cmd.Printf("builtin %s\n", a)
		}
		if p, err := exec.LookPath(a); err == nil {
			cmd.Printf("%s\n", p)
		} else {
			cmd.Printf("unknown %s\n", a)
		}
	}
	return nil
}


// Vars are lists separated by \b
// Maps are lists with of key-value lists preceded by \a
// the first item in each list is the key.

func isMap(env string) bool {
	return strings.ContainsRune(env, '\a')
}

func envMap(env string) map[string][]string {
	toks := strings.Split(env, "\a")
	if len(toks) > 0 {
		toks = toks[1:]
	}
	m := map[string][]string{}
	for _, t := range toks {
		lst := envList(t)
		if len(lst) == 0 {
			continue
		}
		m[lst[0]] = lst[1:]
	}
	return m
}

func mapKeys(m map[string][]string) []string {
	s := []string{}
	for k := range m {
		s = append(s, k)
	}
	sort.Sort(sort.StringSlice(s))
	return s
}

func mapEnv(m map[string][]string) string {
	s := ""
	for k, v := range m {
		lst := append([]string{k}, v...)
		s += "\a" + listEnv(lst)
	}
	return s
}

func envList(env string) []string {
	return strings.Split(env, "\b")
}

func listEnv(lst []string) string {
	return strings.Join(lst, "\b")
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


