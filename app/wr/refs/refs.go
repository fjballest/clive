/*
	References for wr using the refer format

		A	Author's name
		B	Title of book containing item
		C	City of publication
		D	Date
		E	Editor(s) of book containing item
		F	Caption
		G	Government (NTIS) ordering number
		I	Issuer (publisher)
		J	Journal name
		K	Keys for searching
		N	Issue number
		O	Other information
		P	Page(s) of article
		R	Technical report number
		S	Series title
		T	Title
		V	Volume number
		W	Where the item can be found locally
		X	Annotations (not in all macro styles)
		
		Books
		%A	R. E. Griswold
		%A	J. F. Poage
		%A	I. P. Polonsky
		%T	The SNOBOL4 Programming Language
		%I	PRHALL
		%D	second edition 1971
		
		Journal article
		%A	M. A. Harrison
		%A	W. L. Ruzzo
		%A	J. D. Ullman
		%T	Protection in Operating Systems
		%J	CACM
		%V	19
		%N	8
		%P	461-471
		%D	AUG 1976
		%K	hru
		
		Article in conference proceedings
		%A	M. Bishop
		%A	L. Snyder
		%T	The Transfer of Information and Authority
		in	a Protection System
		%J	Proceedings of the 7th SOSP
		%P	45-54
		%D	1979
		
		Article in book
		%A	John B. Goodenough
		%T	A Survey of Program Testing Issues
		%B	Research Directions in Software Technology
		%E	Peter Wegner
		%I	MIT Press
		%P	316-340
		%D	1979
		
		Technical Reports
		%A	T. A. Budd
		%T	An APL Complier
		%R	University of Arizona Techical Report 81-17
		%C	Tucson, Arizona
		%D	1981
		
		PhD Thesis
		%A	Martin Brooks
		%T	Automatic Generation of Test Data for
		Recursive	Programs Having Simple Errors
		%I	PhD Thesis, Stanford University
		%D	1980
		
		Miscellaneous
		%F	BHS--
		%A	Timothy A. Budd
		%A	Robert Hess
		%A	Frederick G. Sayward
		%T	User's Guide for the EXPER Mutation Analysis system
		%O	(Yale university, memo)
*/
package refs

import (
	"io"
	"fmt"
	"bytes"
	"strings"
	"unicode"
	"clive/app"
	"clive/app/nsutil"
)

const (
	Dir = "/zx/lib/bib"	// default bib dir
	Keys = "ATBSJPRVNEFGICDOWX"

)

// When true, Load() reads .bib files containing "bib2ref ok"
// in the first line (with this line being discarded).
// Parsing of the bibtex entries is naive and assumes that each
// field is described in a single line. 
var BibTexOk = true

// A reference maps from the key (eg. 'A') to values (eg. authors)
type Ref {
	Keys map[rune][]string
}

// A bib maps from words found in references to references
type Bib {
	refs map[string] map[*Ref]bool
	All []*Ref	// once loaded, can be used to iterate over the references.
}

// Load the files at the given dir into a Bib set. 
func Load(dir string) (*Bib, error) {
	ds, err := nsutil.GetDir(dir)
	if err != nil {
		return nil, err
	}
	b := &Bib{
		refs: make(map[string]map[*Ref]bool),
	}
	for _, d := range ds {
		nm := d["name"]
		if strings.HasSuffix(nm, ".ref") {
			if xerr := b.load(d["path"]); xerr != nil && err == nil {
				err = xerr
			}
		} else if BibTexOk && strings.HasSuffix(nm, ".bib") {
			b.loadBib(d["path"])	// errors ignored here.
		}
	}
	return b, err
}

func (b *Bib) load(fn string) error {
	app.Dprintf("add file %s\n", fn)
	lnc := nsutil.GetLines(fn)
	return b.loadLines(lnc)
}

func (b *Bib) loadLines(lnc <-chan string) error {
	r := &Ref{Keys: make(map[rune][]string)}
	for t := range lnc {
		t = strings.TrimSpace(t)
		if t == "" {
			if len(r.Keys) > 0 {
				b.add(r)
			}
			r = &Ref{Keys: make(map[rune][]string)}
		}
		if len(t) < 4 || t[0] != '%' || (t[2] != ' ' && t[2] != '	') {
			continue
		}
		k := rune(t[1])
		r.Keys[k] = append(r.Keys[k], t[3:])
	}
	if len(r.Keys) > 0 {
		b.add(r)
	}
	return cerror(lnc)
}

func (b *Bib) add(r *Ref) {
	app.Dprintf("add %v\n", r.Keys['T'])
	b.All = append(b.All, r)
	for _, v := range r.Keys {
		for _, k := range v {
			for _, tok := range strings.Fields(k) {
				tok = strings.ToLower(tok)
				tok = strings.TrimFunc(tok, unicode.IsPunct)
				if b.refs[tok] == nil {
					b.refs[tok] = map[*Ref]bool{}
				}
				b.refs[tok][r] = true
			}
		}
	}
}

func (b *Bib) WriteTo(w io.Writer) {
	for _, r := range b.All {
		fmt.Fprintf(w, "%s\n", r)
	}
}

func (r *Ref) String() string {
	var buf bytes.Buffer
	for _, k := range Keys {
		vs := r.Keys[k]
		for _, v := range vs {
			fmt.Fprintf(&buf, "%c\t%s\n", k, v)
		}
	}
	return buf.String()
}

// return authors separated by "," and terminated with a "."
func (r *Ref) Authors() string {
	a := r.Keys['A']
	if len(a) == 0 {
		return "Anonymous"
	}
	return strings.Join(a, ", ") + "."
}

// return title terminated in "."
func (r *Ref) Title() string {
	ts := r.Keys['T']
	return strings.Join(ts, ". ") + "."
}

// Return text lines or sentences for a reference.
// The strings might be written in a single paragraph.
// The first two strings are usually the title and author list, but
// that depends on the existence of such keys.
func (r *Ref) Reference() []string {
	lines := []string{}
	for _, k := range "TFABEJVNPOIRCD" {
		var buf bytes.Buffer
		vs := r.Keys[k]
		if len(vs) == 0 {
			continue
		}
		v := vs[0]
		switch k {
		case 'T', 'A':
			v = strings.Join(vs, ",")
		case 'V':
			v = "Vol. " + v
		case 'N':
			v = "Nb. " + v
		case 'P':
			v = "Pgs." + v
		}
		v = strings.TrimSpace(v)
		fmt.Fprintf(&buf, "%s", v)
		if v[len(v)-1] != '.' {
			fmt.Fprintf(&buf, ".")
		}
		lines = append(lines, buf.String())
	}
	return lines
}

// Search bib for keys and return all matching references
func (b *Bib) Cites(keys ...string) []*Ref {
	if b == nil {
		return nil
	}
	refs := map[*Ref]bool{}

	for _, k := range keys {
		set := b.refs[strings.ToLower(k)]
		if len(set) == 0 {
			return nil
		}
		if len(refs) == 0 {
			for nk := range set {
				refs[nk] = true
			}
			continue
		}
		for old := range refs {
			if !set[old] {
				delete(refs, old)
			}
		}
		if len(refs) == 0 {
			return nil
		}
	}
	out := []*Ref{}
	for k := range refs {
		out = append(out, k)
	}
	return out
}

// Search bib for keys and return at most one reference
func (b *Bib) Cite(keys ...string) *Ref {
	refs := b.Cites(keys...)
	if len(refs) == 0 {
		return nil
	}
	return refs[0]
}