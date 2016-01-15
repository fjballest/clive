package main

import (
	"strings"
	"fmt"
)

func fields(s, sep string) []string {
	toks := strings.Split(s, sep)
	for i := 0; i < len(toks); i++ {
		toks[i] = strings.TrimSpace(toks[i])
	}
	return toks
}

// Args[0] is "<", ">", ">>"
// tag is Args[1] and can what's within [] in:
// >[out,err]
// <[in]
// |[in:out,err;in2:out2]
func (nd *Nd) addRedirTo(set []*Redir) []*Redir {
	nd.chk(Nredir)
	if len(nd.Args) != 2 {
		panic("addRedirTo: bad redir Args")
	}
	if len(nd.Child) >1 {
		panic("addRedirTo: bad redir children")
	}
	what, tag := nd.Args[0], nd.Args[1]
	bad := ":;,$"
	switch what {
	case ">:":
		bad = ";,$"
	case ">", ">>" :
		bad = ":;$"
	case  "<|", ">|":
		bad = "$"
	}
	if strings.ContainsAny(tag, bad) {
		yylex.Errs("bad %s redirection syntax '%s'", what, tag)
		panic(parseErr)
	}
	if what == ">:" {
		flds := fields(tag, ":")
		if len(flds) == 1 {
			flds = append(flds, "out")
			nd.Args[0] += ":out"
		}
		if len(flds) != 2 {
			yylex.Errs("bad > redirection syntax '%s'", tag)
			panic(parseErr)
		}
		set = append(set, &Redir{name: strings.Join(flds, ":")})
		return set
	}
	for _, r := range fields(tag, ",") {
		for _, rd := range set {
			if rd.name == r {
				yylex.Errs("double redirection for '%s' in '%s' ", r, tag)
				panic(parseErr)
			}
		}
		set = append(set, &Redir{name: r, nd: nd})
	}
	return set
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
		r := newRedir("<", "in", newNd(Nname, "<"))
		c0.Redirs = r.addRedirTo(c0.Redirs)
	}
	if nc == 1 {
		// single command, not really a pipe
		return
	}
	tags := nd.Args[1:]	// 1st arg is the bg tag
	for i, tag := range tags {
		tags := fields(tag, ";")
		for _, tag := range tags {
			rdrs := fields(tag, ":")
			if len(rdrs) == 1 {
				rdrs = append([]string{"in"}, rdrs[0])
			}
			if len(rdrs)%2 != 0 {
				yylex.Errs("syntax error in redir '%s'", tag)
				panic(parseErr)
			}
			for n := 0; n < len(rdrs); n += 2 {
				name := newNd(Nname, fmt.Sprintf("|%d", i))
				r := newRedir("<|", rdrs[n], name)
				nd.Child[i+1].Redirs = r.addRedirTo(nd.Child[i+1].Redirs)
				r = newRedir(">|", rdrs[n+1], name)
				nd.Child[i].Redirs = r.addRedirTo(nd.Child[i].Redirs)
			}
		}
	}
}

// what is "<", ">", ">>"
// tag can be "" or "in", "out", "in,out,foo,..."
// nd is the target of the redir
// name can be nil for >, in which case it's a dup.
func newRedir(what, tag string, name *Nd) *Nd {
	tag = strings.TrimSpace(tag)
	if what == ">" && name == nil {
		what = ">:"
	}
	if what != ">:" && name == nil {
		yylex.Errs("missing name for '%s' ", what)
		panic(parseErr)
	}
	if tag == "" {
		if what == "<" || what == "<|" {
			tag = "in"
		} else {
			tag = "out"
		}
	}
	nd := newNd(Nredir, what, tag);
	if name != nil {
		nd.Add(name)
	}
	return nd
}
