package main

import (
	"strings"
)

func fields(s, sep string) []string {
	toks := strings.Split(s, sep)
	for i := 0; i < len(toks); i++ {
		toks[i] = strings.TrimSpace(toks[i])
	}
	return toks
}

// Args[0] is "<", ">", ">>"
// tag is Args[1] and can be "" or "in", "out", "in,out,foo,..."
func (nd *Nd) setRedir(set map[string]*Nd) {
	nd.chk(Nredir)
	if len(nd.Args) != 2 {
		panic("parseRedir: bad redir Args")
	}
	what, tag := nd.Args[0], nd.Args[1]
	bad := ":"
	if what == "<" {
		bad = ":,"
	}
	if strings.ContainsAny(tag, bad) {
		yylex.Errs("bad %s redirection '%s'", what, tag)
		panic(parseErr)
	}
	rdrs := fields(tag, ",")
	for _, r := range fields(tag, ",") {
		if set[r] != nil {
			yylex.Errs("double redirection for '%s'", r)
			panic(parseErr)
		}
		set[r] = nd
	}
}

func (nd *Nd) parseRedirs() {
	nd.chk(Nredirs)
	nd.Redirs = map[string]*Nd{}
	for _, c := range nd.Child {
		c.setRedir(nd.Redirs)
	}
}

// Add the pipe redir implied by tag to the child of a pipe
func (nd *Nd) addPipeRedir(tag string) {
	nc := len(nd.Child)
	if nc == 0 {
		panic("addPipeRedir: no redirs nd\n")
	}
	rnd := nd.Child[nc-1]
	rnd.chk(Nredirs)
	if len(rdrs) == 1 {
		rdrs = 
	}
}

// Called to add the redirs implied by a pipe
func (nd *Nd) addPipeRedirs(stdin bool) {
	nd.chk(Npipe)
	nc := len(nd.Child)
	if nc == 0 {
		panic("addPipeRedirs: no command 0\n")
	}
	if len(nd.Args) != nc {
		panic("addPipeRedirs: bad pipe Args")
	}
	c0 := nd.Child[0]
	if stdin {
		c0.addRedir("<", "in", newNd(Nname, "<"))
	}
	if nc == 1 {
		// single command, not really a pipe
		return
	}
	tags := nd.Args[1:]	// 1st arg is the bg tag
	for i, tag := range tags {
		rdrs := fields(tag, ":")
		if len(rdrs) == 1 {
			rdrs = append([]string{"in", rdrs)
		}
		if len(rdrs)%2
		nd.Child[i].addPipeRedir(">", tag)
		nd.Child[i+1].addPipeRedir("<", tag)
	}
}

// what is "<", ">", ">>"
// tag can be "" or "in", "out", "in,out,foo,..."
// nd is the target of the redir
func newRedir(what, tag string, name *Nd) *Nd {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		if what == "<" {
			tag = "in"
		} else {
			tag = "out"
		}
	}
	nd := newNd(Nredir, what, tag).Add(name)
	return nd
}
