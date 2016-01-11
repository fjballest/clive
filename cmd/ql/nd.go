package main

/*
	The implementation using a generic tree of nodes is a lot shorter
	than it would be using interfaces and a different type per node type
*/

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"clive/cmd"
)

// We could have used a series of 
type NdType int

// we use (...) to mean args and {...} to mean children nodes
// In short:
//	toplevel -> pipe | src | func
//	pipe chilren are: cmd, blk, for, while, cond, set
//
// Nname[NAME]			x
// Nval[NAME]			$x
// Nval[NAME]{name}			$x[a]
// Nsingle[NAME]			$^a
// Nsingle[NAME]{name}		$^a[b]
// Nset[name]{names}		x = ...
// Nset[name]{name, names}		x[n] = ...
// Nsetmap[name]{names}		x = ([a b c] [d e])
// Nlen[NAME]			$#a
// Napp{names, names}		( .... ) ^ ( ...)
// Nioblk["<|>", NAME]{pipe,..., redirs}		<[x]{a b c} >[x]{a b c}
// Nioblk["<"]{pipe,..., redirs}			<{a b c}
// Nnames{name|app|len|single|val|ioblk| ....}	a b c
// Nredir["<|>|>>" NAME]{name}		<[a,x] b
// Nredir["<"]				| a ...
// Nredirs{redir...}
//
// Ncmd{names, redirs}		a b c <d >e ...
// Npipe[bg,pipe0,pipe1...]{cmd|set|cond|while|for|block,...}
//				a |[x] b | c &y -> [y,x,]{a, b, c}
// Nblock{pipe,..., redirs}		{ a ; b } > a
// Nfor{names, block, redirs}		for a b { ... } <a
// Nwhile{pipe, block, redirs}		while pipe { ... } <a
// Nfunc[NAME]{pipe...}		func a { ... }
// Ncond{or..., redirs}			cond { ... } or {... } ... or {...}
// Nor{pipe...}
// Nsrc{name}			source, < name
const (
	Nnone NdType = iota
	Nname
	Nval
	Nset
	Nsingle
	Nlen
	Napp
	Nnames
	Nsetmap
	Nredir
	Nredirs
	Ncmd
	Npipe
	Nblock
	Nfor
	Nwhile
	Nfunc
	Ncond
	Nor
	Nioblk
	Nsrc
)

struct NdAddr {
	File string
	Ln   int
}

struct Nd {
	typ   NdType
	Args  []string
	Child []*Nd
	NdAddr
	Redirs map[string][]string
}

func newNd(typ NdType, args ...string) *Nd {
	nd := &Nd{typ: typ, Args: args}
	nd.NdAddr = NdAddr{yylex.rdr.Name(), yylex.Line}
	return nd
}

func newList(typ NdType, child ...*Nd) *Nd {
	for i := range child {
		if child[i] == nil {
			child[i] = &Nd{} // safety
		}
	}
	nd := &Nd{typ: typ, Child: child}
	nd.NdAddr = NdAddr{yylex.rdr.Name(), yylex.Line}
	return nd
}

func (nd *Nd) Add(child ...*Nd) *Nd {
	nd.Child = append(nd.Child, child...)
	return nd
}

// Called to add a redir to stdin in | ... pipes
func (nd *Nd) addInRedir(stdin bool) {
	nd.chk(Npipe)
	if len(nd.Child) == 0 {
		cmd.Dprintf("addinrdr: no command 0\n")
		panic(parseErr)	// recovered at top-level
	}
	c := nd.Child[0]
	if len(c.Child) == 0 {
		cmd.Dprintf("addinrdr: child without children\n")
		panic(parseErr)	// recovered at top-level
	}
	rdr := c.Child[len(c.Child)-1]
	if rdr.typ != Nredirs {
		cmd.Dprintf("addinrdr: child without redirs\n")
		panic(parseErr)	// recovered at top-level
	}
	var in *Nd
	if stdin {
		in = newRedir("<", "in", nil)
	} else {
		in = newRedir("<", "in", newNd(Nname, "/dev/null"))
	}
	rdr.Child = append(rdr.Child, in)
}

// Called to add the redirs implied by a pipe
func (nd *Nd) addPipeRedirs() {
	nd.chk(Npipe)
	nc := len(nd.Child)-1	// last is a Nredirs
	if len(nd.Args) != nc + 1 {
		panic("addPipeRedirs: bad pipe Args")
	}
	if nc == 1 {
		// single command, not really a pipe
		return
	}
/*	rdrs := nd.Args[1:]	// 0 is the bg name
	for i, rdr := range rdrs {
		r := parseRedir(rdr, false)
		XXX: Now take the map and apply the
		keys are the inputs for nd.Child[i+1]
		and the values as the outputs for nd.Child[i]
		We should save the map to run the pipe later on.

		Also, think when to parse the Nredir's, perhaps
		when they are created, chk can 

		The map can be saved a NRedir for each redir
		when parsed and Nredirs may have a map created
		by chkRedirs that is the union of the redirs and
		checks that there are no dups

		Once redirs are checked and parsed, we'll just
		look at the map and never at the nodes.
		_ = r
		_ = i
	}*/
}

// Redirs are the last child at Ncmd, Nblock, Nfor, Nwhile, Ncond,
// empty Redirs are the the last child of Nioblk, for/while child Nblocks
func (nd *Nd) chkRedirs() {
	nd.chk(Ncmd, Nblock, Nfor, Nwhile, Ncond, Nioblk)
	nc := len(nd.Child)
	if nc == 0 {
		panic("chkredirs: no children")
	}
	rdr := nd.Child[nc-1]
	if rdr.typ != Nredirs {
		panic("chkredirs: not a redirs node")
	}
	if len(rdr.Child) == 0 {
		return
	}
}

func (t NdType) String() string {
	switch t {
	case Nnone:
		return "none"
	case Nname:
		return "name"
	case Nval:
		return "val"
	case Nset:
		return "set"
	case Nsingle:
		return "single"
	case Nlen:
		return "len"
	case Napp:
		return "app"
	case Nnames:
		return "names"
	case Nsetmap:
		return "setmap"
	case Nredir:
		return "redir"
	case Nredirs:
		return "redirs"
	case Ncmd:
		return "cmd"
	case Npipe:
		return "pipe"
	case Nblock:
		return "block"
	case Nfor:
		return "for"
	case Nwhile:
		return "while"
	case Nfunc:
		return "func"
	case Ncond:
		return "cond"
	case Nor:
		return "or"
	case Nioblk:
		return "ioblk"
	case Nsrc:
		return "source"
	default:
		return fmt.Sprintf("BADTYPE<%d>", t)
	}
}

// debug only
func (n *Nd) String() string {
	if n == nil {
		return "<nil nd>"
	}
	return fmt.Sprintf("%s", n.typ)
}

// debug
func (n *Nd) writeTo(w io.Writer, lvl int) {
	pref := strings.Repeat("    ", lvl)
	fmt.Fprintf(w, "%s%s", pref, n)
	if n == nil {
		fmt.Fprintf(w, "\n")
		return
	}
	if len(n.Args) > 0 {
		fmt.Fprintf(w, "(")
		sep := ""
		for _, a := range n.Args {
			fmt.Fprintf(w, "%s%s", sep, a)
			sep = ", "
		}
		fmt.Fprintf(w, ")")
	}
	if len(n.Child) == 0 {
		fmt.Fprintf(w, "\n")
		return
	}
	fmt.Fprintf(w, " {\n")
	for _, c := range n.Child {
		c.writeTo(w, lvl+1)
	}

	fmt.Fprintf(w, "%s}\n", pref)
}

// debug
struct dnd {
	*Nd
}

func (d dnd) String() string {
	var buf bytes.Buffer
	d.writeTo(&buf, 0)
	return buf.String()
}

type dnames []string

func (n dnames) String() string {
	return strings.Join(n, " ")
}
