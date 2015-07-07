package main

import (
	"bytes"
	"clive/dbg"
	"fmt"
	"io"
	"strings"
)

type NdKind int

const (
	Nnone NdKind = iota
	Ncmds
	Ncmd
	Nredir
	Npipe
	Nforblk
	Nfor
	Nif
	Nwhile
	Nbg
	Nset
	Ninblk
	Nhereblk
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

type Addr  {
	File string
	Line int
}

type Redir  {
	From int    // for this fd
	Name string // to pipe ("|") or to To fd ">" or to this name
	To   int
	App  bool // create or append.
}

type Redirs []*Redir

type Nd  {
	Kind  NdKind
	Args  []string
	Child []*Nd
	Redirs

	Addr

	*NdExec
}

func (a Addr) String() string {
	return fmt.Sprintf("%s:%d", a.File, a.Line)
}

func (k NdKind) String() string {
	switch k {
	case Nnone:
		return "none"
	case Nnop:
		return "nop"
	case Ncmds:
		return "cmds"
	case Ncmd:
		return "cmd"
	case Nredir:
		return "redir"
	case Nredirs:
		return "redirs"
	case Npipe:
		return "pipe"
	case Nforblk:
		return "forblk"
	case Nfor:
		return "for"
	case Nif:
		return "if"
	case Nbg:
		return "bg"
	case Ninblk:
		return "inblk"
	case Nhereblk:
		return "hereblk"
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

func NewNd(k NdKind, args ...string) *Nd {
	return &Nd{Kind: k, Args: args, Addr: addr}
}

func NewList(k NdKind, nds ...*Nd) *Nd {
	n := &Nd{Kind: k, Child: nds, Addr: addr}
	if len(nds)>0 && nds[0]!=nil {
		n.Addr = nds[0].Addr
	}
	return n
}

func (r Redir) String() string {
	switch r.Name {
	case "|":
		return fmt.Sprintf("|[%d]", r.From)
	case "|=":
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

func (rs Redirs) NoDups() {
	m := map[int]bool{}
	for _, r := range rs {
		if r.From>=0 && m[r.From] {
			dbg.Warn("%s: dup redir %d", addr, r.From)
			nerrors++
			r.From = -1
		}
		m[r.From] = true
	}
}

func NewDup(from, to string) Redirs {
	switch from {
	case "1", "2":
	default:
		dbg.Warn("%s: unknown dup %s", addr, from)
		nerrors++
		return nil
	}
	switch to {
	case "1", "2":
	default:
		dbg.Warn("%s: unknown dup %s", addr, to)
		nerrors++
		return nil
	}
	if from == to {
		dbg.Warn("%s: stupid dup %s %s", addr, from, to)
		nerrors++
		return nil
	}
	return []*Redir{
		{
			From: int(from[0] - '0'),
			To:   int(to[0] - '0'),
			Name: "|=",
		},
	}
}

func NewRedir(from, name string, app bool) Redirs {
	if len(from) == 0 {
		return nil
	}

	if len(from) > 1 {
		rdr := NewRedir(from[:1], name, app)
		for i := 1; i < len(from); i++ {
			rdr = append(rdr, NewDup(from[i:i+1], from[:1])...)
		}
		return rdr
	}
	switch from {
	case "0", "1", "2":
	default:
		dbg.Warn("%s: unknown redirect %c", addr, from)
		nerrors++
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

func (n *Nd) Add(nd ...*Nd) *Nd {
	if len(nd) == 0 {
		return n
	}
	n.Child = append(n.Child, nd...)
	return n
}

func (n *Nd) Last() *Nd {
	if n==nil || len(n.Child)==0 {
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
	for _, a := range n.Args {
		fmt.Fprintf(w, "(%s)", a)
	}
	if len(n.Child)>0 || len(n.Redirs)>0 {
		fmt.Fprintf(w, " {\n")
	}
	for _, c := range n.Child {
		c.fprint(w, lvl+1)
	}
	for _, v := range n.Redirs {
		fmt.Fprintf(w, "%s%s%s\n", pref, tab, v)
	}
	if len(n.Child)>0 || len(n.Redirs)>0 {
		fmt.Fprintf(w, "%s}", pref)
	}
	fmt.Fprintf(w, "\n")
}
