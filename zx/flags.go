package zx

import (
	"clive/dbg"
	"errors"
	"fmt"
	"bytes"
	"strings"
	"strconv"
)

// Useful to embed into a Fs
type Flags {
	Dbg   bool // set to true to print zx requests to stderr
	NoPermCheck bool // set to avoid perm checks in trees that support this
	WstatAll bool	// enable wstat of everything if ai is nil.
	IOstats *IOstats // set to &IOstats{} to account
	usr map[string] interface{} // user defined flags
	ro map[string] bool // read only flags
}

// Return a pointer to the debug flag
func (t *Flags) Debug() *bool {
	return &t.Dbg
}

// Add a user defined flag to the flag set.
// vp must be a pointer type.
// Known flag types are *bool, *int, *string, and func(...string)error
func (t *Flags) Add(name string, vp interface{}) {
	t.add(name, vp, false)
}

// Add a read-only user defined flag to the flag set.
// vp must be a pointer type.
// Known flag types are *bool, *int, and *string
func (t *Flags) AddRO(name string, vp interface{}) {
	t.add(name, vp, true)
}

func (t *Flags) add(name string, vp interface{}, ro bool) {
	if t.usr == nil {
		t.usr = make(map[string]interface{})
		t.ro = make(map[string]bool)
	}
	if vp == nil {
		dbg.Fatal("flag %s: nil value", name)
	}
	switch t := vp.(type) {
	case *bool:
	case *int:
	case *string:
	case func(...string)error:
	default:
		dbg.Fatal("unknown flag type %T", t)
	}
	if t.usr == nil {
		t.usr = make(map[string]interface{})
	}
	t.usr[name] = vp
	t.ro[name] = ro
}

// Set the named flag to the given value
func (t *Flags) Set(name string, v interface{}) error {
	if t.usr == nil {
		t.usr = make(map[string]interface{})
		t.ro = make(map[string]bool)
	}
	vp, ok := t.usr[name]
	if !ok {
		return errors.New("no such flag")
	}
	if t.ro[name] {
		return errors.New("read-only flag")
	}
	switch pt := vp.(type) {
	case *bool:
		switch t := v.(type) {
		case bool:
			*pt = t
		default:
			return errors.New("wrong flag type")
		}
	case *int:
		switch t := v.(type) {
		case int:
			*pt = t
		default:
			return errors.New("wrong flag type")
		}
	case *string:
		switch t := v.(type) {
		case string:
			*pt = t
		default:
			return errors.New("wrong flag type")
		}
	}
	return errors.New("unknown flag type")
}

// Return a string describing the flag values
// Only for user-defined flags.
// Add t.Dbg and t.NoPermCheck if you want them here.
func (t *Flags) String() string {
	var buf bytes.Buffer
	for k, v := range t.usr {
		switch t := v.(type) {
		case *bool:
			s := "off"
			if *t {
				s = "on"
			}
			fmt.Fprintf(&buf, "%s %v\n", k, s)
		case *int:
			fmt.Fprintf(&buf, "%s %d\n", k, *t)
		case *string:
			fmt.Fprintf(&buf, "%s %s\n", k, *t)
		}
	}
	return buf.String()
}

// Parse a string of the form <flagname flagvalue> and
// set the flag or return an error.
// Only for user-defined flags.
// Add t.Dbg and t.NoPermCheck if you want them here.
// For each boolean flag with name <name>, the clts
//	<name>
//	<name>  [1|on|y|yes]
// set the flag, and the ctls
//	no<name>
//	<name> [none of 1|on|y|yes]
// clear the flag
func (t *Flags) Ctl(cmd string) error {
	toks := strings.Fields(cmd)
	if len(toks) < 1 {
		return dbg.ErrBadCtl
	}
	if len(toks) == 1 {
		if strings.HasPrefix(toks[0], "no") {
			toks[0] = toks[0][2:]
			toks = append(toks, "off")
		} else {
			toks = append(toks, "on")
		}
	}
	vp, ok := t.usr[toks[0]]
	if !ok {
		return fmt.Errorf("%s: %s", toks[0], dbg.ErrBadCtl)
	}
	if t.ro[toks[0]] {
		return fmt.Errorf("%s: read only flag", toks[0])
	}
	switch t := vp.(type) {
	case *bool:
		if len(toks) > 2 {
			return fmt.Errorf("usage: '%s' on|off", toks[0])
		}
		*t = toks[1] == "on" || toks[1] == "1" || toks[1] == "y" ||
			toks[1] == "yes"
	case *int:
		if len(toks) != 2 {
			return fmt.Errorf("usage: '%s' number", toks[0])
		}
		var err error
		*t, err = strconv.Atoi(toks[1])
		if err != nil {
			return fmt.Errorf("usage: '%s' number", toks[0])
		}
	case *string:
		if len(toks) == 1 {
			*t = ""
		} else {
			*t = strings.Join(toks[1:], " ")
		}
	case func(...string)error:
		return t(toks...)
	default:
		return fmt.Errorf("unknown flag type %T", t)
	}
	return nil
}

