/*
	Structural Regular Expressions on rune based text.

	Besides those understood by Sam, these ones
	have \w and \s to match unicode alpha and space runes
	(can be also used within character classes).
	Matching does not wrap if no further matches are found.

*/
package sre

// REFERENCE(x): Rob Pike's 1987 Structural Regular Expressions paper.

/*
	The expression is compiled to a NFA following
	Ken Thompson's algorithm.
	The NFA proceeds by taking the input and advacing
	a state set on each step, computing the next state
	set.

	The expression source is compiled using a stack and
	operator precedence directly to the NFA (with an
	implicit postfix operator notation.

	The excelent description made by Russ Cox at
	http://swtch.com/~rsc/regexp/regexp[12].html
	is probably the best thing to read before reading the code
	if what's been said so far does not help you.
*/

import (
	"fmt"
	"runtime"
	"unicode/utf8"
)

/*
	Interface for a rune provider to match a regexp against it.
*/
type Text interface {
	Len() int
	Getc(at int) rune
}

/*
	The operators are runes or special rune values defined here,
	they double as instruction codes for the NFA virtual machine.
	The operator value is its precedence.
*/
const (
	tOPERATOR = 0x1000000
	tSTART    = tOPERATOR + iota
	tRPAREN
	tLPAREN
	tOR
	tCAT
	tSTAR
	tPLUS
	tQUEST

	tANY = 0x2000000 + iota
	tWORD
	tBLANK
	tNOP
	tBOL
	tEOL
	tCCLASS
	tNCCLASS
	tEND = tANY + 0x77

	tISAND = tANY

	tQUOTE = 0x4000000 // used to escape runes
)

// A selection in the string implied by a regexp.
type Range  {
	P0, P1 int
}

// program counters.
type pinst int

/*
	NFA instruction.
	It might continue through left, right, or both.
*/
type inst  {
	op    rune   // op code
	class []rune // opt. char class
	subid int    // expr. subid used (\0, \1, ...)
	left  pinst  // left pc (also used as next if there's just one)
	right pinst  // right pc
}

// parsing node
type node  {
	first, last pinst
}

// entry in operator stack
type opRec  {
	op    rune // operator
	subid int  // expr. subid for it
}

/*
	A compiled regexp
*/
type ReProg  {
	// for the compiler
	code       []*inst // program
	opstk      []opRec // operator stack
	cursubid   int     // expr. subid in use as of now
	ndstk      []*node // node stack
	nparen     int     // nb. of open parens
	expr       []rune  // what's left to be compiled
	err        error   // during parsing
	lastwasand bool
	entry      pinst // entry point to execute the program
	back       bool  // compiled to search backward
}

/*
	emit a instruction for the compiled program and
	eturn the new pc and the new instruction.
*/
func (prg *ReProg) emit(op rune) (pinst, *inst) {
	i := &inst{op: op}
	n := pinst(len(prg.code))
	if n == 0 {
		/* make 0 invalid as a pc */
		prg.code = append(prg.code, &inst{})
		n++
	}
	prg.code = append(prg.code, i)
	return n, i
}

/*
	push the t operator on the stack
*/
func (prg *ReProg) pushOp(t rune) {
	nrec := opRec{op: t, subid: prg.cursubid}
	prg.opstk = append(prg.opstk, nrec)
}

/*
	pop an operator from the stack and return the operator and its subid
*/
func (prg *ReProg) popOp() (rune, int) {
	n := len(prg.opstk)
	if n == 0 {
		panic("operator stack underflow")
	}
	top := prg.opstk[n-1]
	prg.opstk = prg.opstk[:n-1]
	return top.op, top.subid
}

/*
	push a parse node given the first and last instructions
*/
func (prg *ReProg) pushNd(first, last pinst) {
	nd := &node{first: first, last: last}
	prg.ndstk = append(prg.ndstk, nd)
}

/*
	pop a parse node for the operator op.
*/
func (prg *ReProg) popNd(op rune) *node {
	n := len(prg.ndstk)
	if n == 0 {
		panic(fmt.Sprintf("missing operand for '%s'", tokStr(op)))
	}
	top := prg.ndstk[n-1]
	prg.ndstk = prg.ndstk[:n-1]
	return top
}

/*
	Evaluate on the operator stack until reaching the given priority.
	This is compiling, not regexp execution.
*/
func (prg *ReProg) evalUntil(pri rune) {
	if len(prg.opstk) == 0 {
		panic("operator stack underflow")
	}
	for pri==tRPAREN || prg.opstk[len(prg.opstk)-1].op>=pri {
		switch op, subid := prg.popOp(); op {
		case tLPAREN:
			op1 := prg.popNd('(')
			i2, x2 := prg.emit(tRPAREN)
			x2.subid = subid
			prg.code[op1.last].left = i2
			i1, x1 := prg.emit(tLPAREN)
			x1.subid = subid
			x1.left = op1.first
			prg.pushNd(i1, i2)
			return // it's tRPAREN
		case tOR:
			op2 := prg.popNd('|')
			op1 := prg.popNd('|')
			i2, _ := prg.emit(tNOP)
			prg.code[op2.last].left = i2
			prg.code[op1.last].left = i2
			i1, x1 := prg.emit(tOR)
			x1.right = op1.first
			x1.left = op2.first
			prg.pushNd(i1, i2)
		case tCAT:
			op2 := prg.popNd('.')
			op1 := prg.popNd('.')
			if prg.back && prg.code[op2.first].op!=tEND {
				op1, op2 = op2, op1
			}
			prg.code[op1.last].left = op2.first
			prg.pushNd(op1.first, op2.last)
		case tSTAR:
			op2 := prg.popNd('*')
			i1, x1 := prg.emit(tOR)
			prg.code[op2.last].left = i1
			x1.right = op2.first
			prg.pushNd(i1, i1)
		case tPLUS:
			op2 := prg.popNd('+')
			i1, x1 := prg.emit(tOR)
			prg.code[op2.last].left = i1
			x1.right = op2.first
			prg.pushNd(op2.first, i1)
		case tQUEST:
			op2 := prg.popNd('?')
			i1, x1 := prg.emit(tOR)
			i2, _ := prg.emit(tNOP)
			x1.left = i2
			x1.right = op2.first
			prg.code[op2.last].left = i2
			prg.pushNd(i1, i2)
		default:
			panic(fmt.Sprint("unknown regexp op ", op))
		}
	}
}

/*
	Compile an operator
*/
func (prg *ReProg) operator(op rune, val []rune) {
	switch op {
	case tRPAREN:
		if prg.nparen == 0 {
			panic("unmatched ')'")
		}
		prg.nparen--
		prg.evalUntil(op)
	case tLPAREN:
		prg.nparen++
		prg.cursubid++
		if prg.lastwasand {
			prg.operator(tCAT, nil)
		}
		prg.pushOp(op)
	default:
		prg.evalUntil(op)
		prg.pushOp(op)
	}
	prg.lastwasand =
		op==tSTAR || op==tQUEST || op==tPLUS || op==tRPAREN
}

/*
	Compile an operand (val is the class for '[]' tokens)
*/
func (prg *ReProg) operand(op rune, val []rune) {
	if prg.lastwasand {
		prg.operator(tCAT, nil) // implicit cat
	}
	i, x := prg.emit(op)
	if op==tCCLASS || op==tNCCLASS {
		x.class = val
	}
	prg.pushNd(i, i)
	prg.lastwasand = true
}

/*
	Optimize generated code by jumping directly to the target of nops.
*/
func (prg *ReProg) eatNops() {
	for pc := range prg.code {
		i := prg.code[pc]
		t := i.left
		for prg.code[t].op == tNOP {
			t = prg.code[t].left
		}
		i.left = t
	}
}

// Argument to Compile.
type Dir int

// Argument to Compile.
const (
	Fwd = iota // compile for forward search in text
	Bck        // compile for backward search in text
)

/*
	Compile re as a regexp to match in text, forward if
	dir is Fwd, and backward otherwise.
*/
func CompileStr(re string, dir Dir) (prg *ReProg, err error) {
	return Compile([]rune(re), dir)
}

/*
	Compile re as a regexp to search forward or backward in text
*/
func Compile(re []rune, dir Dir) (prg *ReProg, err error) {
	prg = &ReProg{back: dir == Bck}
	prg.expr = re
	defer func() {
		if s := recover(); s != nil {
			if x, ok := s.(runtime.Error); ok {
				panic(x)
			}
			err = fmt.Errorf("%s", s)
		}
	}()
	prg.pushOp(tSTART - 1) // start with lo pri
	for {
		tok, val := prg.lex()
		if tok == tEND {
			break
		}
		if tok&tOPERATOR != 0 {
			prg.operator(tok, val)
		} else {
			prg.operand(tok, val)
		}
	}
	prg.evalUntil(tSTART)  // close with low pri
	prg.operand(tEND, nil) // force end
	prg.evalUntil(tSTART)
	if prg.nparen != 0 {
		panic("unmatched '('")
	}
	nd := prg.ndstk[len(prg.ndstk)-1]
	prg.entry = nd.first
	prg.eatNops()
	return prg, nil
}

func safe(i, n int) int {
	if i < 0 {
		return 0
	}
	if i > n {
		return n
	}
	return i
}

// Match (forward) the given sre against the given string, return the (sub)strings matching
// (nil if none) and any error.
func Match(sre, text string) ([]string, error) {
	p, err := CompileStr(sre, Fwd)
	if err != nil {
		return nil, err
	}
	rtext := []rune(text)
	n := len(rtext)
	rg := p.Exec(runestr(rtext), 0, n)
	var rs []string
	for _, r := range rg {
		rs = append(rs, string(rtext[safe(r.P0, n):safe(r.P1, n)]))
	}
	return rs, nil
}

func (prg *ReProg) peek() rune {
	if len(prg.expr) == 0 {
		return tEND
	}
	return prg.expr[0]
}

func (prg *ReProg) getc() rune {
	if len(prg.expr) == 0 {
		return tEND
	}
	r := prg.expr[0]
	prg.expr = prg.expr[1:]
	return r
}

const (
	// used to flag a range in a character class, not a valid rune
	cRange = utf8.MaxRune
)

/*
	scan the next element within a character class
*/
func (prg *ReProg) scanEl() rune {
	c := prg.getc()
	switch c {
	case tEND:
		panic("malformed '[]'")
	case '\\':
		switch c = prg.getc(); c {
		case tEND:
			panic("malformed '[]'")
		case 'n':
			return '\n'
		case 't':
			return '\t'
		case 'w':
			return tWORD
		case 's':
			return tBLANK
		default:
			return c | tQUOTE
		}
	}
	return c
}

/*
	Aafter '[' has been seen, scan the entire char (rune) class
	and return both the class and whether it's a negated class or not.
*/
func (prg *ReProg) scanClass() (class []rune, neg bool) {
	class = make([]rune, 0, 16)
	c := prg.peek()
	if c == tEND {
		panic("malformed []")
	}
	neg = c == '^'
	if neg {
		class = append(class, '\n') // exclude also \n
		neg = true
		prg.getc()
	}
	if prg.peek() == tEND {
		panic("malformed []")
	}
	for c1 := prg.scanEl(); c1 != ']'; c1 = prg.scanEl() {
		if c1==tEND || c1=='-' {
			panic("malformed []")
		}
		if prg.peek() == '-' {
			/* a-b: remove '-' and use [maxrune,a,b] */
			prg.getc()
			c2 := prg.scanEl()
			if c2==']' || c2==tEND {
				panic("malformed range in '[]'")
			}
			class = append(class, cRange, c1, c2)
		} else {
			class = append(class, c1&^tQUOTE)
		}
	}
	return
}

/*
	return the next token and the class value for the token (if any),
	or tEND if none.
*/
func (prg *ReProg) lex() (rune, []rune) {
	if len(prg.expr) == 0 {
		return tEND, nil
	}
	c := prg.getc()
	switch c {
	case '\\':
		switch n := prg.getc(); n {
		case 'n':
			c = '\n'
		case 't':
			c = '\t'
		case 'w':
			c = tWORD
		case 's':
			c = tBLANK
		default:
			c = n
		}
	case '*':
		c = tSTAR
	case '?':
		c = tQUEST
	case '+':
		c = tPLUS
	case '|':
		c = tOR
	case '.':
		c = tANY
	case '(':
		c = tLPAREN
	case ')':
		c = tRPAREN
	case '^':
		c = tBOL
	case '$':
		c = tEOL
	case '[':
		c = tCCLASS
		cls, neg := prg.scanClass()
		if neg {
			c = tNCCLASS
		}
		return c, cls
	}
	return c, nil
}

type runestr []rune

func (t runestr) Len() int {
	return len(t)
}

func (t runestr) Getc(n int) rune {
	return t[n]
}
