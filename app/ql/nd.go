package ql

import (
	"bytes"
	"clive/app"
	"fmt"
	"io"
	"strings"
)

type NdKind int

const (
	Nnone NdKind = iota
	Nblk
	Ncmd
	Nredir
	Npipe
	Nteeblk
	Nfor
	Ncond
	Nwhile
	Nbg
	Nset
	Ninblk
	Nrawinblk
	Nsingleinblk
	Npipeblk
	Nnames
	Nexec
	Nname
	Nval
	Napp
	Njoin
	Nlen
	Nredirs
	Nfunc
	Nnop
)

const (
	tab = "    "
)

type Redir struct {
	From int    // for this fd
	Name string // to pipe ("|") or dup To ("=") or to this file
	To   int    // target fd for "="
	App  bool   // create or append.
}

type Redirs []*Redir

type Nd struct {
	Kind  NdKind
	Args  []string
	Child []*Nd
	Redirs
	IsGet bool // for gf pipes and to get names from input in for
	Addr
}

func (k NdKind) String() string {
	switch k {
	case Nnone:
		return "none"
	case Nnop:
		return "nop"
	case Nblk:
		return "blk"
	case Ncmd:
		return "cmd"
	case Nredir:
		return "redir"
	case Nredirs:
		return "redirs"
	case Npipe:
		return "pipe"
	case Nteeblk:
		return "teeblk"
	case Nfor:
		return "for"
	case Ncond:
		return "cond"
	case Nbg:
		return "bg"
	case Ninblk:
		return "inblk"
	case Nrawinblk:
		return "rawinblk"
	case Nsingleinblk:
		return "singleinblk"
	case Npipeblk:
		return "pipeblk"
	case Nnames:
		return "names"
	case Nexec:
		return "exec"
	case Nname:
		return "name"
	case Nset:
		return "set"
	case Nval:
		return "val"
	case Napp:
		return "app"
	case Njoin:
		return "join"
	case Nlen:
		return "len"
	case Nwhile:
		return "while"
	case Nfunc:
		return "func"
	default:
		return "unknown"
	}
}

func (x *xCmd) newNd(k NdKind, args ...string) *Nd {
	return &Nd{Kind: k, Args: args, Addr: x.Addr}
}

func (x *xCmd) newList(k NdKind, nds ...*Nd) *Nd {
	n := &Nd{Kind: k, Child: nds, Addr: x.Addr}
	if len(nds) > 0 && nds[0] != nil {
		n.Addr = nds[0].Addr
	}
	return n
}

func (r Redir) String() string {
	switch r.Name {
	case "|":
		return fmt.Sprintf("|[%d]", r.From)
	case "=":
		return fmt.Sprintf(">[%d=%d]", r.From, r.To)
	default:
		s := ">"
		if r.From == 0 {
			s = "<"
		}
		if r.App {
			s += s
		}
		return fmt.Sprintf("%s[%d] %s", s, r.From, r.Name)
	}
}

func (x *xCmd) noDups(rs Redirs) {
	m := map[int]bool{}
	for _, r := range rs {
		if r.From >= 0 && m[r.From] {
			x.Errs("dup redir %d", r.From)
			r.From = -1
		}
		m[r.From] = true
	}
}

func (x *xCmd) newDup(from, to string) Redirs {
	switch from {
	case "1", "2":
	default:
		x.Errs("unknown dup %s", from)
		return nil
	}
	switch to {
	case "1", "2":
	default:
		x.Errs("unknown dup %s", to)
		return nil
	}
	if from == to {
		x.Errs("stupid dup %s %s", from, to)
		return nil
	}
	return []*Redir{
		{
			From: int(from[0] - '0'),
			To:   int(to[0] - '0'),
			Name: "=",
		},
	}
}

func (x *xCmd) newRedir(from, name string, app bool) Redirs {
	if len(from) == 0 {
		return nil
	}

	if len(from) > 1 {
		rdr := x.newRedir(from[:1], name, app)
		for i := 1; i < len(from); i++ {
			rdr = append(rdr, x.newDup(from[i:i+1], from[:1])...)
		}
		return rdr
	}
	switch from {
	case "0", "1", "2":
	default:
		x.Errs("unknown redirect %c", from)
		return nil
	}
	return []*Redir{
		{
			From: int(from[0] - '0'),
			Name: name,
			App:  app,
		},
	}
}

func (nd *Nd) hasInOutRedirs() bool {
	for _, r := range nd.Redirs {
		if r.From == 0 {
			return true
		}
		if r.From == 1 && r.Name != "|" {
			return true
		}
	}
	return false
}

// Add lf or gf to the 1st child of a pipe if it's a std pipe and not |a b c or -|a b c
// Add |pf to std pipes that have a single child
// No rewrite happens in inner pipes within subcmds, because they inherit the
// input from the outer command.
// builtins that do run here do not use a rewrite.
func (x *xCmd) pipeRewrite(nd *Nd) *Nd {
	if x.plvl > 0 || nd == nil {
		return nd
	}
	if nd.Kind != Npipe {
		app.Fatal("pipeRewrite bug")
	}
	if len(nd.Child) == 0 {
		return nd
	}
	c := nd.Child[0]
	if c.Kind != Nexec || c.hasInOutRedirs() {
		return nd
	}
	if c.isHereCmd() || c.isCmd(noRewrites...) {
		return nd
	}
	c.Child = append(c.Child, nil)
	copy(c.Child[1:], c.Child[0:])
	lf := "lf"
	if nd.IsGet {
		lf = "gf"
	}
	c.Child[0] = &Nd{Kind: Nname, Args: []string{lf}}
	if len(nd.Child) > 1 {
		f := nd.Child[1]
		if nd.IsGet || f.Kind != Nfor || len(f.Child) == 0 || len(f.Child[0].Child) != 1 {
			return nd
		}
		// insert pf between lf and for
		nd.Child = append(nd.Child, nil)
		copy(nd.Child[2:], nd.Child[1:])
		pfnames := &Nd{Kind: Nname, Args: []string{"pf"}}
		pf := &Nd{Kind: Nexec, Child: []*Nd{pfnames}}
		pf.Redirs = append(pf.Redirs, x.newRedir("0", "|", false)...)
		pf.Redirs = append(pf.Redirs, x.newRedir("1", "|", false)...)
		nd.Child[1] = pf
		return nd
	}
	c.Redirs = append(c.Redirs, x.newRedir("1", "|", false)...)
	x.noDups(c.Redirs)
	pfnames := &Nd{Kind: Nname, Args: []string{"pf"}}
	pf := &Nd{Kind: Nexec, Child: []*Nd{pfnames}}
	pf.Redirs = append(pf.Redirs, x.newRedir("0", "|", false)...)
	nd.Child = append(nd.Child, pf)
	return nd
}

func (n *Nd) Add(nd ...*Nd) *Nd {
	if len(nd) == 0 {
		return n
	}
	if len(n.Child) > 0 {
		if len(nd) == 1 && nd[0].Kind == Nnop {
			return n
		}
		if n.Child[len(n.Child)-1].Kind == Nnop {
			n.Child = n.Child[:len(n.Child)-1]
		}
	}
	n.Child = append(n.Child, nd...)
	return n
}

func (n *Nd) Last() *Nd {
	if n == nil || len(n.Child) == 0 {
		return nil
	}
	return n.Child[len(n.Child)-1]
}

// debug only
func (n *Nd) String() string {
	var b bytes.Buffer
	n.fprint(&b, 0)
	words := strings.Fields(b.String())
	s := strings.Join(words, " ")
	if len(s) > 50 {
		s = s[:50] + "..."
	}
	return s
}

func (n *Nd) sprint() string {
	var b bytes.Buffer
	if n != nil {
		fmt.Fprintf(&b, "%s: ", n.Addr)
	}
	n.fprint(&b, 0)
	return b.String()
}

func (n *Nd) fprint(w io.Writer, lvl int) {
	pref := strings.Repeat(tab, lvl)
	fmt.Fprintf(w, "%s", pref)
	if n == nil {
		fmt.Fprintf(w, "<nil nd>\n")
		return
	}
	fmt.Fprintf(w, "%s", n.Kind)
	if n.IsGet {
		fmt.Fprintf(w, ":gf")
	}
	for _, a := range n.Args {
		fmt.Fprintf(w, "(%s)", a)
	}
	if len(n.Child) > 0 || len(n.Redirs) > 0 {
		fmt.Fprintf(w, " {\n")
	}
	for _, c := range n.Child {
		c.fprint(w, lvl+1)
	}
	for _, v := range n.Redirs {
		fmt.Fprintf(w, "%s%s%s\n", pref, tab, v)
	}
	if len(n.Child) > 0 || len(n.Redirs) > 0 {
		fmt.Fprintf(w, "%s}", pref)
	}
	fmt.Fprintf(w, "\n")
}
