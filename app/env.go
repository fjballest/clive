package app

import (
	"os"
	"strings"
	"sync"
	"fmt"
)

type envSet {
	vars map[string]string
	lk sync.Mutex
}

func mkEnv() map[string]string {
	env := map[string]string{}
	for _, s := range os.Environ() {
		toks := strings.SplitN(s, "=", 2)
		if len(toks) == 2 {
			env[toks[0]] = toks[1]
		}
	}
	return env
}

// Initialize a new Env in the current context from that given, perhaps empty.
// If the given one is nil, the env is re-initialized from that in the underlying os.
func NewEnv(vars map[string]string) {
	if vars == nil {
		vars = mkEnv()
	} else {
		nvars := map[string]string{}
		for k, v := range vars {
			nvars[k] = v
		}
		vars = nvars
	}
	c := ctx()
	c.lk.Lock()
	defer c.lk.Unlock()
	c.env = &envSet{vars: vars}
}

// Start using a copy of the current environment
func DupEnv() {
	NewEnv(Env())
}

// Return the current environment
func Env() map[string]string {
	c := ctx()
	return c.Env()
}

// Return the environment of this context.
func (c *Ctx) Env() map[string] string {
	c.lk.Lock()
	e := c.env
	c.lk.Unlock()
	e.lk.Lock()
	defer e.lk.Unlock()
	vars := map[string]string{}
	for k, v := range c.env.vars {
		vars[k] = v
	}
	return vars
}

func (e *envSet) set(n, v string) {
	e.lk.Lock()
	defer e.lk.Unlock()
	if v == "" {
		delete(e.vars, n)
	} else {
		e.vars[n] = v
	}
}

func (e *envSet) get(n string) string {
	e.lk.Lock()
	defer e.lk.Unlock()
	return e.vars[n]
}

// Return the value of the environment variable in the current context.
func GetEnv(n string) string {
	c := ctx()
	c.lk.Lock()
	e := c.env
	c.lk.Unlock()
	return e.get(n)
}

// Set the value of a environment variable in the current context.
// If the value is empty the variable is undefined.
func SetEnv(n, v string) {
	c := ctx()
	c.lk.Lock()
	e := c.env
	c.lk.Unlock()
	e.set(n, v)
}

// Return a copy of the environment in the format expected by go os.
func OSEnv() []string {
	var env []string
	c := ctx()
	c.lk.Lock()
	e := c.env
	c.lk.Unlock()
	e.lk.Lock()
	defer e.lk.Unlock()
	for k, v := range e.vars {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}
