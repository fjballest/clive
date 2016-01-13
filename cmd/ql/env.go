package main

import (
	fpath "path"
	"clive/cmd"
	"fmt"
	"strconv"
	"strings"
	"sort"
	"os"
)

func mapNames(m map[string]string) []string {
	s := []string{}
	for k := range m {
		s = append(s, k)
	}
	sort.Sort(sort.StringSlice(s))
	return s
}

func envMap(v string) map[string]string {
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
	cmd.SetEnv(n, s)
}

func GetEnvMap(n string) map[string]string {
	return envMap(cmd.GetEnv(n))
}

func idx(toks []string, idxs string) ([]string, error) {
	if idxs == "" {
		return toks, nil
	}
	n, err := strconv.ParseInt(idxs, 10, 32)
	i := int(n)
	if err != nil || i < 0 || i >= len(toks) {
		return nil, fmt.Errorf("bad index '%s'", idxs)
	}
	return toks[i : i+1], nil
}

func SetEnvList(name string, toks ...string) {
	cmd.SetEnv(name, strings.Join(toks, "\b"))
}

func GetEnvAt(name, idxs string) ([]string, error) {
	s := cmd.GetEnv(name)
	if len(s) == 0 {
		return nil, nil
	}
	if strings.Contains(s, "\a") {
		m := envMap(s)
		if idxs == "" {
			return mapNames(m), nil
		}
		s := m[idxs]
		return []string{s}, nil
	}
	toks := strings.SplitN(s, "\b", -1)
	return idx(toks, idxs)
}

func SetEnvAt(name, idx string, val string) error {
	s := cmd.GetEnv(name)
	ismap := strings.Contains(s, "\a")
	n, err := strconv.ParseInt(idx, 10, 32)
	i := int(n)
	if err == nil && !ismap {
		toks := strings.SplitN(s, "\b", -1)
		if i == len(toks) {
			toks = append(toks, val)
		} else if i < 0 || i > len(toks) {
			return fmt.Errorf("bad index '%s' for '%s'", idx, name)
		} else {
			toks[i] = val
		}
		SetEnvList(name, toks...)
		return nil
	}
	m := envMap(name)
	m[idx] = val
	SetEnvMap(name, m)
	return nil
}

func (x *xEnv) setPath() {
	var ps []string
	if p := cmd.GetEnv("path"); p == "" {
		p = cmd.GetEnv("PATH")
		if p == "" {
			p = "/bin:/usr/bin"
		}
		ps = strings.SplitN(p, ":", -1)
	} else {
		ps = strings.Fields(p)
	}
	x.path = nil
	for _, d := range ps {
		x.path = append(x.path, strings.SplitN(d, "\b", -1)...)
	}
}

func (x *xEnv) lookCmd(name string) string {
	if strings.HasPrefix(name, "./") || strings.HasPrefix(name, "../") ||
		strings.HasPrefix(name, "/") {
		return name
	}
	for _, pd := range x.path {
		nm := fpath.Join(pd, name)
		if d, err := os.Stat(nm); err == nil {
			if !d.IsDir() && d.Mode()&0111 != 0 {
				return nm
			}
		}
	}
	return ""
}

