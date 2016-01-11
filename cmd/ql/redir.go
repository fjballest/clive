package main

// A redir is a series of name: name,...,name that indicates that
// the left name corresponds to the names on the right
// A single name,...,name list means in: ... or out .... depending
// on the I/O nature of the redir
func (nd *Nd) parseRedir() {
	nd.chk(Nredir)
	if len(nd.Args) != 2 {
		panic("parseRedir: bad redir Args")
	}
	rdr = strings.TrimSpace(nd.Args[1])
	if len(rdr) == 0 {
		return nil
	}
	rdrs := strings.Split(rdr, ":")
	for i := 0; i < len(rdrs); i++ {
		rdrs[i] = strings.TrimSpace(rdrs[i])
	}
	nd.Redirs = map[string][]string {}
	if len(rdrs) == 1 {
		nd.Redirs[tag] = strings.Fields(rdrs[0])
		return
	}
	if len(rdrs)%2 != 0 {
		yylex.Errs("bad redirection '%s'", rdr)
		panic(parseErr)
	}
	for i := 0; i < len(rdrs); i += 2 {
		k := rdrs[i]
		if nd.Redirs[k] != "" {
			yylex.Errs("double redirection for '%s'", k)
			panic(parseErr)
		}
		nd.Redirs[k] = rdrs[i+1]
	}
}

func newRedir(what, tag string, name *Nd) *Nd {
	if tag == "" {
		if what == "<" {
			tag = "in"
		} else {
			tag = "out"
		}
	}
	nd := newNd(Nredir, what, tag).Add(name)
	nd.ParseRedir()
	return nd
}
