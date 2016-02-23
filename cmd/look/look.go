/*
	Return commands to run when the user looks for strings.

	Programs rely on this package to "plumb" user looks.
	By convention, $look, or $home/lib/look, or $home/.look
	are used to keep the rules.
	A rule is a pair of lines, the first line is a regular
	expression as provided by sre(2) and the
	second is the command to execute for the rule.
	Back-references may be used to build a command from parts
	of the matching text.
*/
package look

import (
	"sync"
	"clive/sre"
	"errors"
	"clive/cmd"
	"fmt"
	"bytes"
	"strings"
)

// If the user looks for something matching Rexp, then
// Cmd leads to a result string.
// Backquoting to refer to \0...\9 is ok in Cmd.
struct Rule {
	Rexp string
	Cmd string

	sync.Mutex
	re *sre.ReProg
}

type Rules []*Rule

var (
	debug bool
	dprintf = cmd.FlagPrintf(&debug)
	ErrNoMatch = errors.New("no match")
)

// Return a string that can be parsed later on by
// ParseRules to make a set of rules.
func (rs Rules) String() string {
	var buf bytes.Buffer
	for _, r := range rs {
		fmt.Fprintf(&buf, "%s\n\t%s\n", r.Rexp, r.Cmd)
	}
	return buf.String()
}

// Return the command to run if s matches the rule.
// ErrNoMatch is returned if there's no match.
func (r *Rule) CmdFor(s string) (string, error) {
	r.Lock()
	defer r.Unlock()
	if r.re == nil {
		re, err := sre.Compile([]rune(r.Rexp), sre.Fwd)
		if err != nil {
			return "", fmt.Errorf("look: rexp: %s", err)
		}
		r.re = re
	}
	outs := r.re.Match(s)
	dprintf("look: %s: %v\n", r.Rexp, outs)
	if len(outs) == 0 {
		return "", ErrNoMatch
	}
	return sre.Repl(outs, r.Cmd), nil
}

// Return the command for a user look, if any.
// ErrNoMatch is returned if no rule matches.
// If there's an error in any of the rules, no further
// rules are attempted.
func (rs Rules) CmdFor(s string) (string, error) {
	for _, r := range rs {
		c, err := r.CmdFor(s)
		if err == nil || err != ErrNoMatch {
			return c, err
		}
	}
	return "", ErrNoMatch
}


func ParseRules(txt string) (Rules, error) {
	var rs []*Rule
	lns := strings.Split(txt, "\n")
	for i := 0; i < len(lns); i++ {
		ln := strings.TrimSpace(lns[i])
		if len(ln) == 0 || ln[0] == '#' {
			continue
		}
		if i == len(lns)-1 {
			return rs, errors.New("ParseRules: missing command line")
		}
		r := &Rule{
			Rexp: ln,
			Cmd: strings.TrimSpace(lns[i+1]),
		}
		rs = append(rs, r)
		i++	// for the cmd line
	}
	return rs, nil
}
