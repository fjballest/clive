package main

import (
	"clive/cmd/wr/refs"
	"clive/dbg"
	"fmt"
	"strings"
)

type Kind int

const (
	Knone  Kind = iota
	Ktitle      // title or author info (first found is title)
	Khdr1       // heading
	Khdr2       // heading
	Khdr3       // heading
	Kitem       // bullet item
	Kenum       // numbered item
	Kname       // description list item
	Kfont       // font size change
	Kit         // italics
	Kbf         // bold face
	Ktt         // teletype
	Kitend      // end of it
	Kbfend      // end of bf
	Kttend      // end of tt
	Kverb       // verbatim
	Ksh         // verbatim shell output
	Kfig        // figure
	Kpic        // inlined pic figure
	Ktbl        // table
	Keqn        // equation
	Kcode       // code excerpts
	Ktext       // text

	Kindent      // relative indent
	Kitemize     // indented list of items
	Kenumeration // indented list of enums
	Kdescription // description list
	Kcite        // hand made cite,
	Ksref        // ref to a section
	Kfref        // to a fig
	Ktref        // to a tbl
	Keref        // to a eqn
	Kcref        // to a listing
	Kurl         // link
	Kbib         // wr/refs citation(s)
	Kpar         // forced end of paragraph
	Kbr          // forced line break
)

const (
	// these require a space after
	TitleMark = "_ "
	Hdr1Mark  = "* "
	Hdr2Mark  = "** "
	Hdr3Mark  = "*** "
	ItemMark  = "- "
	EnumMark  = "# "

	// these don't require a space after
	VerbMark = "[verb"
	ShMark   = "[sh"
	QlMark   = "[ql"
	RcMark   = "[rc"
	FigMark  = "[fig"
	PicMark  = "[pic"
	TblMark  = "[tbl"
	EqnMark  = "[eqn"
	CodeMark = "[code"
)

struct eKeys {
	el   *Elem
	keys map[string]bool
}

struct Text {
	*scan
	Elems   []*Elem
	bib     *refs.Bib
	biberr  error
	bibrefs []string
	refsdir string

	nhdr1, nhdr2, nhdr3 int

	itset, ttset, bfset bool

	refs map[Kind][]*eKeys

	pprintf, iprintf, sprintf dbg.PrintFunc
}

struct Elem {
	Kind      Kind
	Data      string  // in figs the file name, in pics the pic text
	Textchild []*Elem // child text for inlined formats
	Caption   *Elem   // in figs and pics and tables
	Tag       string  // in code, word after [code to use as the tag
	Child     []*Elem
	Tbl       [][]string // rows for tables; 1st rwo is just the fmt strings
	indent    int
	NameKind  Kind   // for Knames, the Kit, Kbf, or Ktt used in the label, if any.
	Inline    bool   // for Kit, Kbf, Ktt, if the font change is inline with the text.
	Nb        string // number of table, fig, ... A string so we can have 3.1 and so on.

	fname string
	lno   int
}

struct scan {
	lnc   chan string
	last  string
	saved bool
	eof   bool
	fname string
	nb    int
}

var marks = map[string]Kind{
	TitleMark: Ktitle,
	Hdr1Mark:  Khdr1,
	Hdr2Mark:  Khdr2,
	Hdr3Mark:  Khdr3,
	ItemMark:  Kitem,
	EnumMark:  Kenum,
	ShMark:    Ksh,
	QlMark:    Ksh,
	RcMark:    Ksh,
	VerbMark:  Kverb,
	FigMark:   Kfig,
	PicMark:   Kpic,
	TblMark:   Ktbl,
	EqnMark:   Keqn,
	CodeMark:  Kcode,
}

func (k Kind) String() string {
	switch k {
	case Knone:
		return "none"
	case Ktitle:
		return "title"
	case Khdr1:
		return "hdr1"
	case Khdr2:
		return "hdr2"
	case Khdr3:
		return "hdr3"
	case Kitem:
		return "item"
	case Kenum:
		return "enum"
	case Kname:
		return "name"
	case Kfont:
		return "font"
	case Kit:
		return "+it"
	case Kbf:
		return "+bf"
	case Ktt:
		return "+tt"
	case Kitend:
		return "-it"
	case Kbfend:
		return "-bf"
	case Kttend:
		return "-tt"
	case Kverb:
		return "verb"
	case Ksh:
		return "sh"
	case Kfig:
		return "fig"
	case Kpic:
		return "pic"
	case Ktbl:
		return "tbl"
	case Keqn:
		return "eqn"
	case Kcode:
		return "code"
	case Ktext:
		return "text"
	case Kindent:
		return "indent"
	case Kitemize:
		return "itemize"
	case Kenumeration:
		return "enumeration"
	case Kdescription:
		return "description"
	case Kcite:
		return "cite"
	case Ksref:
		return "sref"
	case Kfref:
		return "fref"
	case Ktref:
		return "tref"
	case Keref:
		return "eref"
	case Kcref:
		return "cref"
	case Kbib:
		return "bib"
	case Kurl:
		return "url"
	case Kpar:
		return "par"
	case Kbr:
		return "br"
	default:
		return "unknow"
	}
}

func (k Kind) HasData() bool {
	switch k {
	case Ktitle, Khdr1, Khdr2, Khdr3,
		Kcite, Kbib, Kurl, Ksref, Kfref, Ktref, Keref, Kcref,
		Kverb, Ksh, Kfig, Kpic, Ktbl, Keqn, Kcode, Ktext, Kfont, Kitem, Kenum, Kname:
		return true
	default:
		return false
	}
}

func (k Kind) HasChild() bool {
	switch k {
	case Kindent, Kitemize, Kenumeration, Kdescription, Kname,
		Ktext, Kenum, Kitem, Khdr1, Ktitle, Khdr2, Khdr3:
		return true
	default:
		return false
	}
}

func (e *Elem) sprint(lvl int) string {
	pref := strings.Repeat("   ", lvl)
	if e == nil {
		return pref + "nil elem"
	}
	s := fmt.Sprintf("%s%s.%d", pref, e.Kind, e.indent)
	if e.Tag != "" {
		s += "[#" + e.Tag + "]"
	}
	if e.Kind.HasData() {
		s += dbg.Str(e.Data, 20)
	} else if e.Data != "" || e.Caption != nil {
		s += "HASDATA"
	}
	if e.Caption != nil {
		s += "cap[\n"
		s += e.Caption.sprint(lvl + 1)
		s += pref + "]"
	}
	if len(e.Textchild) > 0 {
		s += "[\n"
		for _, c := range e.Textchild {
			s += c.sprint(lvl + 1)
			s += "\n"
		}
		s += pref + "]"
	}
	if e.Kind.HasChild() && len(e.Child) > 0 {
		s += "{\n"
		for _, c := range e.Child {
			s += c.sprint(lvl + 1)
			s += "\n"
		}
		s += pref + "}"
	} else if len(e.Child) > 0 {
		s += "HASCHILD"
	}
	return s
}

func (e *Elem) String() string {
	return e.sprint(0)
}
