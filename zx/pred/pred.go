/*
	Predicates on files as used by the zx.Finder interface.

	A predicate is a string with a specific format that may be compiled to a Pred
	and used to implement zx.Find.

	Example predicates are:

		``	true
		`true`	true (`t` is also true)
		`false`	false (`f` is also false)
		`attr="value"`	true if dir[attr] == "value"
		`="value"`	idem for attrs path (val == /...) or name (val != /...)
		`attr==value`	true if dir.Int64("attr") == value
		`attr≡value`	true if dir.Int64("attr") == value
		`attr!="value"	true if dir[attr] != "value"
		`attr≠"value"	true if dir[attr] != "value"
		`attr~"expr"	true if dir[attr] matches "expr" (globbing)
		`~expr`		idem for attrs path (val == /...) or name (val != /...)
		`attr~~"regexp"	true if dir[attr] matches "regexp" (not globbing)
		`~~regexp`	idem for attrs path (val == /...) or name (val != /...)
		`attr≈"regexp"	true if dir[attr] matches "regexp" (not globbing)
		`≈regexp`	idem for attrs path (val == /...) or name (val != /...)
		`attr>value`	true if dir.Int64(attr) >= value
				// you can use also >=, <, and <=, ≤, ≥
		`prune`		false, and indicates that the tree can be pruned
	The predefined attribute "depth" may be used to indicate the depth of the
	Dir evaluted.

		n	(where n is an int) is understood as `depth<=n`
		d	is understood as `type=d`
		-	is understood as `type=-`
		c	is understood as `type=c`

	When checking that "name" or "path" do not match or differ from
	a given expression, prune is implied if the name or path match.
	That is, name!=value is actually name!=value|prune, and so on.

	Also, when checking that path equals or matches something, if that something does
	not start with /, then name is used instead of path. That is, the path attribute
	can be used to check for both paths and names, as a convenience.

	These may be combined using

		pred&pred	similar to && in C.
		pred,pred		similar to && in C.
		pred|pred		similar to || in C.
		pred:pred		similar to || in C.
		!pred		similar to ! in C.
		(pred)		like pred, to group operations.

	For example, these predicates select...

		``	everything
		`type=d`	directories
		`depth==1`	directory contents
*/
package pred

import (
	"clive/zx"
	"errors"
	"fmt"
	"os"
	"clive/sre"
	"strconv"
	"path/filepath"
)

// REFERENCE(x): clive/nspace, name spaces and Finder interface.

// REFERENCE(x): clive/zx, ZX implementations and finders.

/*
	predicate operators
*/
type op int

const (
	oNop   op = 0
	oLe    op = '≤'
	oEq    op = '≡'
	oNeq   op = 'd'
	oNeqs  op = '≠'
	oGe    op = '≥'
	oLt    op = '<'
	oGt    op = '>'
	oEqs   op = '='
	oMin   op = 'm'
	oMax   op = 'M'
	oMatch op = '~'
	oRexp op = '≈'
	oNot   op = '!'
	oOr    op = ':'
	oAnd   op = ','
	oPrune op = 'p'
	oTrue  op = 't'
	oFalse op = 'f'
)

var (
	debug bool
)

/*
	A compiled predicate.
*/
type Pred  {
	op    op // operation
	name  string
	value string
	args  []*Pred // for Or and And
	re *sre.ReProg
}

// Compile a predicate from a string representation.
func New(s string) (*Pred, error) {
	if s == "" {
		return nil, nil
	}
	l := newLex(s)
	return l.parse()
}

// Create a new predicate with the logical OR of the arguments.
func Or(args ...*Pred) *Pred {
	return &Pred{
		op:   oOr,
		args: args,
	}
}

// Create a new predicate with the logical AND of the arguments.
func And(args ...*Pred) *Pred {
	return &Pred{
		op:   oAnd,
		args: args,
	}
}

// Create a new predicate with the logical NOT of the argument.
func Not(arg *Pred) *Pred {
	return &Pred{
		op:   oNot,
		args: []*Pred{arg},
	}
}

//	exp	p	->
//	/a/b	/x	-> false, true
//	/a/b	/a	-> false, false
//	/a/b	/a/b/c	-> false, true
func pathMatch(exp, p string) (value, pruned bool, err error) {
	if len(exp) > 0 && exp[0] != '/' {
		m, err := filepath.Match(exp, p)
		return m, false, err
	}
	els := zx.Elems(exp)
	pels := zx.Elems(p)
	if len(pels) > len(els) {
		return false, true, nil
	}
	for i := 0; i < len(pels); i++ {
		m, err := filepath.Match(els[i], pels[i])
		if !m {
			return false, true, err
		}
	}
	return len(pels) == len(els), false, nil
}

/*
	Like Pred.EvalAt, but useful when you want to specify the predicate
	by using a string.
*/
func EvalStr(e zx.Dir, p string, depth int) (value, pruned bool, err error) {
	x, err := New(p)
	if err != nil {
		return false, false, err
	}
	return x.EvalAt(e, depth)
}

/*
	Evaluate the predicate at the given directory
	entry (considering that its depth is the given one).
	Returns true or false as the value of the predicate, a prune indication
	that is true if we can prune the tree at this directory entry (e.g., the depth
	indicated is beyond that asked by the predicate), and any error indication.
*/
func (p *Pred) EvalAt(e zx.Dir, lvl int) (value, pruned bool, err error) {
	if p == nil {
		return true, false, nil
	}
	if debug {
		fmt.Fprintf(os.Stderr, "[eval:\n\t%s NAME %sVAL %s\n\t%s\n", p, p.name, p.value, e)
		defer fmt.Fprintf(os.Stderr, "\t%v %v %v\n]\n", value, pruned, err)
	}
	switch p.op {
	case oTrue, oFalse:
		return p.op == oTrue, false, nil
	case oPrune:
		return false, true, nil
	case oNot:
		m, prune, err := p.args[0].EvalAt(e, lvl)
		if o := p.args[0].op; (o == oMatch || o == oEqs || o == oRexp) &&
			(p.args[0].name == "name" || p.args[0].name == "path") {
			prune = m
		}
		return !m, prune, err
	case oAnd:
		for i := range p.args {
			v, pruned, err := p.args[i].EvalAt(e, lvl)
			if err != nil || !v || pruned {
				return v, pruned, err
			}
		}
		return true, false, nil
	case oOr:
		for i := range p.args {
			v, pruned, err := p.args[i].EvalAt(e, lvl)
			if err != nil || v || pruned {
				return v, pruned, err
			}
		}
		return false, false, nil
	case oLt, oLe, oNeq, oEq, oGe, oGt:
		var n1 int64
		var err error
		isdepth := p.name == "depth"
		if isdepth {
			n1 = int64(lvl)
		} else {
			v, ok := e[p.name]
			if !ok {
				return false, false, nil
			}
			n1, err = strconv.ParseInt(v, 0, 64)
			if err != nil {
				err = errors.New("not a number")
				return false, false, err
			}
		}
		n2, err := strconv.ParseInt(p.value, 0, 64)
		if err != nil {
			err = errors.New("not a number")
			return false, false, err
		}
		toodeep := false
		var v bool
		switch p.op {
		case oLt:
			v = n1 < n2
			toodeep = isdepth && n1>=n2-1
		case oLe:
			v = n1 <= n2
			toodeep = isdepth && n1>=n2
		case oEq:
			v = n1 == n2
			toodeep = isdepth && n1>=n2
		case oGe:
			v = n1 >= n2
		default:
			v = n1 > n2
		}
		return v, toodeep, nil
	case oMatch:
		nm := p.name
		if nm == "path" && len(p.value) > 0 && p.value[0] != '/' {
			nm = "name"
		}
		n, ok := e[nm]
		if !ok {
			return false, false, nil
		}
		return pathMatch(p.value, n)
	case oRexp:
		nm := p.name
		if nm == "path" && len(p.value) > 0 && p.value[0] != '/' {
			nm = "name"
		}
		n, ok := e[nm]
		if !ok {
			return false, false, nil
		}
		if p.re == nil {
			x, err := sre.CompileStr(p.value, sre.Fwd)
			if err != nil {
				return false, false, err
			}
			p.re = x
		}
		x := p.re.ExecStr(n, 0, len(n))
		return len(x) > 0 , false, err
	case oEqs:
		nm := p.name
		if nm == "path" && len(p.value) > 0 && p.value[0] != '/' {
			nm = "name"
		}
		v, ok := e[nm]
		if !ok {
			return false, false, nil
		}
		if !ok {
			return false, false, nil
		}
		return v == p.value, false, nil
	case oNeqs:
		nm := p.name
		if nm == "path" && len(p.value) > 0 && p.value[0] != '/' {
			nm = "name"
		}
		v, ok := e[nm]
		if !ok {
			return true, false, nil
		}
		prune := false
		if p.name == "name" || p.name == "path" {
			prune = v == p.value
		}
		return v != p.value,  prune, nil
	}
	return false, false, nil
}

/*
	Execute the part of the ns.Find operation that evaluates p
	at the tree rooted at d (considering that its level is the one
	indicated). Found entries are sent through the given channel,
	which is closed only upon errors.

	This is useful to implement ns.Find when writting services.
*/
func (p *Pred) FindAt(fs zx.Sender, d zx.Dir, c chan<- zx.Dir, lvl int) {
	match, pruned, err := p.EvalAt(d, lvl)
	if err != nil {
		close(c, err)
		return
	}
	if pruned {
		nd := d.Dup()
		nd["err"] = "pruned"
		c <- nd
		return
	}
	if d["rm"] != "" {
		return
	}
	var ds []zx.Dir
	if d["type"] == "d" {
		ds, err = zx.GetDir(fs, d["path"])
	}
	if err != nil {
		nd := d.Dup()
		nd["err"] = err.Error()
		c <- nd
		return
	}
	if match {
		if ok := c <- d; !ok {
			return
		}
	}
	for i := 0; i < len(ds); i++ {
		cd := ds[i]
		if cd["rm"] != "" {
			continue
		}
		p.FindAt(fs, cd, c, lvl+1)
	}
}

func Find(fs zx.Tree, path, pred string) <-chan zx.Dir {
	c := make(chan zx.Dir)
	go func() {
		d, err := zx.Stat(fs, path)
		if d == nil {
			close(c, err)
			return
		}
		x, err := New(pred)
		if err != nil {
			close(c, err)
			return
		}
		x.FindAt(fs, d, c, 0)
		close(c)
	}()
	return c
}
