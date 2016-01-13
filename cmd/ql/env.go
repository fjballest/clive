package main

import (
	fpath "path"
	"clive/cmd"
	"strconv"
	"strings"
	"sort"
	"os"
)

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
	n, err := strconv.ParseInt(idxs, 10, 32)
	i := int(n)
	if err != nil || i < 0 || i >= len(lst) {
		return ""
	}
	return lst[i]
}

func setListEl(lst []string, idxs, val string) []string {
	n, err := strconv.ParseInt(idxs, 10, 32)
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

func (x *xEnv) setPath() {
	var ps []string
	if p := cmd.GetEnv("path"); p == "" {
		p = cmd.GetEnv("PATH")
		if p == "" {
			p = "/bin:/usr/bin"
		}
		ps = strings.SplitN(p, ":", -1)
	} else {
		ps = envList(p)
	}
	x.path = ps
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

