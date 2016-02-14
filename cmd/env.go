package cmd

import (
	"os"
	"strings"
	"sync"
)

struct envSet {
	vars map[string]string
	sync.Mutex
}

// Initialize a new env from the os
func osenv() map[string]string {
	env := map[string]string{}
	for _, s := range os.Environ() {
		toks := strings.SplitN(s, "=", 2)
		if len(toks) == 2 {
			env[toks[0]] = toks[1]
		}
	}
	return env
}

func (e *envSet) set(n, v string) {
	e.Lock()
	defer e.Unlock()
	if v == "" {
		delete(e.vars, n)
	} else {
		e.vars[n] = v
	}
	os.Setenv(n, v) // in case someone execs...
}

func (e *envSet) get(n string) string {
	e.Lock()
	defer e.Unlock()
	return e.vars[n]
}

func mkEnv() *envSet {
	ne := &envSet{
		vars: map[string]string{},
	}
	for k, v := range osenv() {
		ne.vars[k] = v
	}
	return ne
}

func (e *envSet) dup() *envSet {
	e.Lock()
	defer e.Unlock()
	ne := &envSet{
		vars: map[string]string{},
	}
	for k, v := range e.vars {
		ne.vars[k] = v
	}
	return ne
}
