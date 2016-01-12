package main

import (
	"strings"
)

// A redir is a series of name: name,...,name that indicates that
// the left name corresponds to the names on the right
// A single name,...,name list means in: ... or out .... depending
// on the I/O nature of the redir
func (nd *Nd) parseRedir() map[string][]string {
	nd.chk(Nredir)
	if len(nd.Args) != 2 {
		panic("parseRedir: bad redir Args")
	}
	tag := strings.TrimSpace(nd.Args[1])
	if len(tag) == 0 {
		return nil
	}
	rdrs := strings.Split(tag, ":")
	for i := 0; i < len(rdrs); i++ {
		rdrs[i] = strings.TrimSpace(rdrs[i])
		if rdrs[i] == "" {
			yylex.Errs("empty redirection '%s'", tag)
			panic(parseErr)
		}
	}
	redir := map[string][]string {}
	if len(tag) == 1 {
		redir[tag] = strings.Fields(rdrs[0])
		return redir
	}
	if len(rdrs)%2 != 0 {
		yylex.Errs("bad redirection '%s'", tag)
		panic(parseErr)
	}
	for i := 0; i < len(rdrs); i += 2 {
		k := rdrs[i]
		if redir != nil {
			yylex.Errs("double redirection for '%s'", k)
			panic(parseErr)
		}
		dsts := strings.Split(rdrs[i+1], ",")
		for _, dst := range dsts {
			dst = strings.TrimSpace(dst)
			if dst == "" {
				yylex.Errs("empty redirection '%s'", tag)
				panic(parseErr)
			}
			redir[k] = append(redir[k], dst)
		}
	}
	return redir
}

func (nd *Nd) parseRedirs() {
return
	nd.chk(Nredirs)
	nd.Redirs = map[string]*Nd{}
	for _, c := range nd.Child {
		c.chk(Nredir)
		for r := range c.Redir {
			if nd.Redirs[r] != nil {
				yylex.Errs("double redirection '%s'", r)
				panic(parseErr)
			}
			nd.Redirs[r] = c
		}
	}
}

func (nd *Nd) addRedir(what, tag string, name *Nd) {
	nc := len(nd.Child)
	if nc == 0 {
		panic("addRedir: no redirs\n")
	}
	rc := nd.Child[nc-1]
	rc.chk(Nredirs)
	r := newRedir(what, tag, name)
	rc.Child = append(rc.Child, r)
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
		// XXX: parse the pipe redir tag
		_ = tag
		nd.Child[i].addRedir(">", "out", newNd(Nname, "|"))
		nd.Child[i+1].addRedir("<", "in", newNd(Nname, "|"))
	}
}

func newRedir(what, tag string, name *Nd) *Nd {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		if what == "<" {
			tag = "in"
		} else {
			tag = "out"
		}
	}
	if what == ">" || what == ">>" {
		if strings.Contains(tag, ":") {
			yylex.Errs("invalid redirection for >, >> '%s'", tag)
			panic(parseErr)
		}
		// XXX: the tag here may be a,b,c to redirect more than one chan to
		// the given file
	}
	if what == "<" {
		if strings.ContainsAny(tag, ":,") {
			yylex.Errs("invalid redirection for < '%s'", tag)
			panic(parseErr)
		}
	}
	nd := newNd(Nredir, what, tag).Add(name)
	return nd
}
