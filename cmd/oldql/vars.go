package main

import (
	"clive/dbg"
	"errors"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

var (
	funcs    = map[string]*Nd{}
	pathDirs []string
	pathlk   sync.Mutex
	Argv     []string
)

func setPath() {
	pathlk.Lock()
	defer pathlk.Unlock()
	s := os.Getenv("path")
	if s == "" {
		s = os.Getenv("PATH")
	}
	if s == "" {
		s = "/bin:/usr/bin"
	}
	pathDirs = []string{}
	for _, d := range strings.SplitN(s, ":", -1) {
		pathDirs = append(pathDirs, strings.SplitN(d, "\b", -1)...)
	}
}

func init() {
	setPath()
}

func LookCmd(name string) string {
	if strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") ||
		strings.HasPrefix(name, "/") {
		return name
	}
	for _, d := range pathDirs {
		nm := path.Join(d, name)
		if st, err := os.Stat(nm); err == nil {
			if !st.IsDir() && st.Mode()&0111!=0 {
				return nm
			}
		}
	}
	return name
}

func ArgVal(toks []string, idx string) []string {
	if idx == "" {
		return toks
	}
	n, err := strconv.ParseInt(idx, 10, 32)
	i := int(n)
	if err!=nil || i<0 || i>=len(toks) {
		dbg.Warn("%s: bad index in $%s", addr, "argv")
		return nil
	}
	return toks[i : i+1]
}

func EnvVal(name, idx string) []string {
	s := os.Getenv(name)
	if len(s) == 0 {
		return nil
	}
	if strings.Contains(s, "\a") {
		m := EnvMap(s)
		if idx == "" {
			return MapNames(m)
		}
		s := m[idx]
		return []string{s}
	}
	toks := strings.SplitN(s, "\b", -1)
	return ArgVal(toks, idx)
}

func SetEnvList(name string, toks ...string) {
	os.Setenv(name, strings.Join(toks, "\b"))
}

func SetEnvVal(name, idx string, val string) {
	pathlk.Lock()
	defer pathlk.Unlock()
	s := os.Getenv(name)
	ismap := strings.Contains(s, "\a")
	n, err := strconv.ParseInt(idx, 10, 32)
	i := int(n)
	if err==nil && !ismap {
		toks := strings.SplitN(s, "\b", -1)
		if i == len(toks) {
			toks = append(toks, val)
		} else if i<0 || i>len(toks) {
			dbg.Warn("%s: bad index in $%s", addr, name)
			return
		} else {
			toks[i] = val
		}
		SetEnvList(name, toks...)
		return
	}
	m := EnvMap(name)
	m[idx] = val
	SetEnvMap(name, m)
}

func EnvMap(v string) map[string]string {
	toks := strings.SplitN(v, "\a", -1)
	if len(toks)%2 == 1 {
		toks = toks[:len(toks)-1]
	}
	if len(toks) == 0 {
		return map[string]string{}
	}
	m := make(map[string]string)
	for i := 0; i < len(toks)-1; i += 2 {
		m[toks[i]] = toks[i+1]
	}
	return m
}

func SetEnvMap(n string, m map[string]string) {
	s := ""
	for k, v := range m {
		s += k + "\a" + v + "\a"
	}
	os.Setenv(n, s)
}

func MapNames(m map[string]string) []string {
	s := []string{}
	for k := range m {
		s = append(s, k)
	}
	return s
}

// See exec.go for conventions that this function must obey
func (nd *Nd) xSet() error {
	xprintf("x %s\n", nd)
	var err error
	var args []string
	if len(nd.Args)==0 || len(nd.Args[0])==0 {
		err = errors.New("empty var name")
		goto Done
	}
	if err = nd.redirs(); err != nil {
		goto Done
	}
	if len(nd.Child)>0 && nd.Child[0].Kind==Nset {
		m := map[string]string{}
		for _, c := range nd.Child {
			if len(c.Args) < 2 {
				continue
			}
			m[c.Args[0]] = c.Args[1]
		}
		SetEnvMap(nd.Args[0], m)
		goto Done
	}
	args = nd.names()
	if len(nd.Args)>1 && len(nd.Args[1])>0 {
		SetEnvVal(nd.Args[0], nd.Args[1], strings.Join(args, " "))
		goto Done
	}
	if nd.Args[0]=="prompt" && len(args)>0 && len(args[0])>0 {
		Prompter.SetPrompt(args[0])
	}
	SetEnvList(nd.Args[0], args...)
Done:
	if len(nd.Args)>0 && (nd.Args[0]=="path" || nd.Args[0]=="PATH") {
		setPath()
	}
	xprintf("x %s done\n", nd)
	nd.closeAll()
	nd.waitc <- err
	return err
}
