%{
//
// Px grammar. /Users/paurea/src/picky/p.y
//

package comp

import (
	"fmt"
	"os"
)

type Env struct {
	id   uint
	tab  map[string]*Sym
	prev *Env
	prog *Sym
	rec  *Type
}

type List struct {
	kind   int
	item   []interface{}
}

type Builtin struct {
	name string
	id   uint32
	kind int
	args string
	r    rune
	fn   func(b *Builtin, args *List) *Sym
}

type Stmt struct {
	op      int
	sfname  string
	lineno  int

	//--one of:
	list    *List // '{'

	lval    *Sym  // =
	rval    *Sym

	cond    *Sym // IF
	thenarm *Stmt
	elsearm *Stmt

	fcall   *Sym // FCALL

	expr    *Sym // RETURN, DO, WHILE, FOR, CASE
	stmt    *Stmt
	incr    *Stmt // last statement in fors (i++|i--)
}

type Type struct {
	op     int
	sym    *Sym
	first  int
	last   int

	//--one of:
	lits   *List // Tenum

	ref    *Type // Tptr

	super  *Type // Trange

	idx    *Type // Tarry, Tstr
	elem   *Type

	fields *List // Trec

	parms  *List // Tproc, Tfunc
	rtype *Type

	//--
	// backend
	id    uint
	sz    uint
}

// pc/src table
type Pcent struct {
	next *Pcent
	st   *Stmt
	nd   *Sym
	pc   uint
}

// generated code
type Code struct {
	addr  uint32
	pcs   *Pcent
	pcstl *Pcent
	p     []uint32
	np    uint
	ap    uint
}

type Prog struct {
	psym   *Sym
	parms  *List
	rtype  *Type // ret type or nil if none
	consts *List
	types  *List
	vars   *List
	procs  *List
	stmt   *Stmt
	b      *Builtin
	nrets  int

	// backend
	code   Code
	parmsz uint
	varsz  uint
}

type Val struct {
	//--one of:
	ival int

	rval float64

	sval string

	vals *List
	//--
}

type Sym struct {
	id     uint
	name   string
	stype  int
	op     int
	fname  string
	lineno int
	ttype  *Type

	//--one of:
	tok    int

	Val

	used    int
	set     int

	left    *Sym
	right   *Sym // binary, unary

	fsym    *Sym // Sfcall
	fargs   *List

	rec     *Sym // "."
	field   *Sym

	swfield *Sym // switch field
	swval   *Sym // variant
	//--

	prog    *Prog
	// backend
	addr uint
	off  uint // fields
}

type Stats struct {
	nenvs  uint // # of envs used
	menvs  uint // # of envs allocated
	nsyms  uint // # of syms allocated
	nexpr  uint // # of syms for expressions
	nlists uint // # of lists allocated
	mlist  uint // # of items in longest list
	nstmts uint // # of stmts allocated
	nprogs uint // # of progs allocated
	ntypes uint // # of types allocated
	nstrs  uint // # of strings allocated
}

%}
%union {
	bval	int
	sval	string
	ival	int
	rval	float64
	sym	*Sym
	list	*List
	stmt	*Stmt
	ttype	*Type
	env	*Env
}

%left	','
%left	OR AND
%left	EQ NE LE GE '<' '>' BADOP
%left	'+' '-'
%left	'*' '/' '%'
%left	POW
%left	DOTDOT

//	|sed 's/%token//' | sed 's/[ 	]/\n/g' | sort -u |fmt -l 50|t+
//
%token
	ARRAY CASE  CONSTS DEFAULT DO  ELSE
	FOR FUNCTION IF SWITCH NIL NOT OF
	PROCEDURE PROGRAM RECORD REF RETURN
	TYPES VARS WHILE LEN
%token	<ival>	INT CHAR
%token	<sval>	STR
%token	<rval>	REAL
%token	<bval>	TRUE FALSE
%token	<sym>	ID TYPEID 

/* not a token; used as a Stmt op */
%token	FCALL

%type	<stmt>	stmt body assignstmt casestmt forstmt ifstmt procstmt
%type	<stmt>	repeatstmt retstmt whilestmt case ifhdr nullstmt

%type	<ttype>	type newtype ordtype ptrtype rangetype funcret
%type	<ttype>	proctype functype recordtype arraytype recordfields
%type	<list>	args ids fields parms optparms stmts optargs cases swfields swcases swcase
%type	<sym>	field prochdr funchdr expr primary lvalue forcond cexpr parm lenarg
%type	<ival>	forop

%%
prog:
	proghdr decls
	{
		if env.prev != nil { panic("env stack not empty; missing popenv()") }
		if debug['P'] != 0 { dumpprog(os.Stderr, env.prog) }
	}


proghdr:
|
	ID ID ';'
	{
		env.prog = newprog($2)
		s := oksym($1)
		errmsg := fmt.Sprintf("'program' expected, found %s", s.name)
		panic(errmsg)
	}
|
	PROGRAM ID ';'
	{
		env.prog = newprog($2)
	}
|
	PROGRAM ID error
	{
		env.prog = newprog($2)
		Yylex.Error("';' missing after program name")
	}
|	error ';'
	{
		panic("not a picky program")
	}

decls:
	decls decl
|
	decl

decl:
	CONSTS ':' consts
	{
		if env.prog == nil { panic("missing program declaration") }
	}
|
	TYPES ':' types
	{
		if env.prog == nil { panic("missing program declaration") }
	}
|
	VARS ':' vars
	{
		if env.prog == nil { panic("missing program declaration") }
		if !globalsok { diag("global variables are not allowed")}
	}
|
	procdef
|
	funcdef

consts:
	consts constdef
|
	constdef

types:
	types typedef
|
	typedef

vars:
	vars vardef
|
	vardef

constdef:
	ID '=' expr ';'
	{
		declconst($1, $3)
	}
|
	error ';'
	{
		diag("constant declaration expected")
		Errflag = 0
	}

typedef:
	ID '=' type ';'
	{
		decltype($1, $3)
	}
|
	TYPEID '=' type ';'
	{
		decltype($1, $3)
	}
|
	error ';'
	{
		diag("type declaration expected")
		Errflag = 0
	}


type:
	TYPEID
	{
		$$ = $1.ttype
	}
|
	newtype

vardef:
	ID ':' TYPEID ';'
	{
		declvar($1, $3.ttype)
	}
|
	TYPEID ':' type ';'
	{
		diag("'%s' is a type name", $1.name)
	}
|
	ID ':' ID ';'
	{
		diag("type name expected; found '%s'", $3.name)
	}
|
	error ';'
	{
		diag("var declaration expected")
		Errflag = 0
	}

procdef:
	prochdr ';'
	{
		declprogdone(env.prog)
		popenv()
	}
|
	prochdr optvars body
	{
		if env.prog == nil { panic("missing program declaration") }
		env.prog.prog.stmt = $3
		declprogdone(env.prog)
		popenv()
	}

optvars:
	vars
|
	/* empty */

prochdr:
	PROCEDURE ID
	{
		declproc($2)
	}
	procparms
	{
		$$ = env.prog
	}

procparms:
	'(' optparms ')'
	{
		if env.prog == nil { panic("missing program declaration") }
		env.prog.prog.parms = $2
	}
|	/* empty */
	{
		diag("missing '()'")
	}

funcdef:
	funchdr ';'
	{
		declprogdone(env.prog)
		popenv()
	}
|
	funchdr optvars body
	{
		if env.prog == nil { panic("missing program declaration") }
		env.prog.prog.stmt = $3
		declprogdone(env.prog)
		popenv()
	}

funchdr:
	FUNCTION ID
	{
		declfunc($2)
	}
	'(' optparms ')' ':' funcret
	{
		if env.prog == nil { panic("missing program declaration") }
		$$ = env.prog
		$$.prog.parms = $5
		$$.prog.rtype = $8
	}

optparms:
	parms
|
	/*empty*/
	{
		$$ = newlist(Lsym)
	}

parms:
	parms ',' parm
	{
		$$ = $1
		if $1 != nil { addsym($1, $3) }
		Errflag = 0
	}
|
	parms ';' parm
	{
		diag("',' expected; not ';'")
		$$ = $1
		if $1 != nil { addsym($1, $3) }
	}
|
	parm
	{
		$$ = newlist(Lsym)
		addsym($$, $1)
	}
|
	parms error
	{
		$$ = newlist(Lsym)
	}
|
	parms error parm
	{
	$$ = newlist(Lsym)
		Errflag = 0
	}
|
	parms ',' error
	{
		$$ = newlist(Lsym)
	}
|
	error
	{
		$$ = nil
	}

parm:
	ID ':' TYPEID
	{
		$$ = newparm($1, $3.ttype, 0)
	}
|
	ID ':' ID
	{
		diag("'%s' is not a type name", $3.name)
		$$ = newparm($1, tundef, 0)
	}
|
	REF ID ':' TYPEID
	{
		$$ = newparm($2, $4.ttype, 1)
	}
|
	REF ID ':' ID
	{
		diag("'%s' is not a type name", $4.name)
		$$ = newparm($2, tundef, 0)
	}
|
	TYPEID ':' TYPEID
	{
		diag("type name '%s' is an invalid parameter name", $1.name)
		$$ = newparm($1, $3.ttype, 0)
	}
|
	REF TYPEID ':' TYPEID
	{
		diag("type name '%s' is an invalid parameter name", $2.name)
		$$ = newparm($2, $4.ttype, 1)
	}

newtype:
	ordtype
|
	rangetype
|
	ptrtype
|
	proctype
|
	functype
|
	arraytype
|
	recordtype

ptrtype:
	'^' TYPEID
	{
		$$ = newtype(Tptr)
		$$.ref = $2.ttype
	}
|
	'^' ID
	{
		ft := decltype($2, nil)
		$$ = newtype(Tptr)
		$$.ref = ft.ttype
	}

ids:
	ids ',' ID
	{
		$$ = $1
		if $$ != nil { addsym($$, $3) }
		Errflag = 0
	}
|
	ID
	{
		$$ = newlist(Lsym)
		if $$ != nil { addsym($$, $1) }
	}
|
	error
	{
		diag("identifier expected")
		$$ = nil
	}
|
	ids error
	{
		$$ = nil
	}
|
	ids error ID
	{
		$$ = $1
		Errflag = 0
	}
|
	ids ',' error
	{
		$$ = nil
	}


ordtype:
	'('  ids ')'
	{
		$$ = newordtype($2)
	}

rangetype:
	TYPEID expr DOTDOT expr
	{
		$$ = newrangetype($1.ttype, $2, $4)
	}

arraytype:
	ARRAY '[' TYPEID ']' OF TYPEID
	{
		$$ = newarrytype($3.ttype, $6.ttype)
	}
	|
	ARRAY '[' expr DOTDOT expr ']' OF TYPEID
	{
		if env.prog == nil { panic("missing program declaration") }
		$$ = newarrytype(newrangetype(nil, $3, $5), $8.ttype)
	}

recordtype:
	RECORD
	{
		t := newtype(Trec)
		Pushenv()
		env.rec = t
	}
	recordfields
	{
		$$ = $3
	}

recordfields:
	'{' fields '}'
	{
		$$ = env.rec
		$$.fields = $2
		popenv()
		initrectype($$)
	}
|
	error '}'
	{
		$$ = env.rec
		$$.fields = newlist(Lsym)
		popenv()
		initrectype($$)
	}


fields:
	fields field
	{
		$$ = $1
		if $$ != nil { addsym($$, $2) }
	}
|
	fields swfields
	{
		$$ = $1
		if $$ != nil { appsyms($$, $2) }
	}
|
	field
	{
		$$ = newlist(Lsym)
		if $$ != nil { addsym($$, $1) }
	}

field:
	TYPEID ':' TYPEID ';'
	{
		diag("'%s' is a type name", $1.name)
	}
|
	ID ':' ID ';'
	{
		diag("type name expected; found '%s'", $3.name)
	}
|
	ID ':' TYPEID ';'
	{
		$$ = declfield($1, $3.ttype)
	}

swfields:
	SWITCH '(' ID ')' '{' swcases '}'
	{
		setswfield($6, $3)
		$$ = $6
	}

swcases:
	swcases swcase
	{
		$$ = $1
		if $$ != nil { appsyms($$, $2) }
	}
|
	swcase
	{
		$$ = $1
	}

swcase:
	CASE cexpr ':' fields
	{
		setswval($4, $2)
		$$ = $4
	}

proctype:
	PROCEDURE
	{
		Pushenv()
	}
	'(' parms ')'
	{
		$$ = newtype(Tproc)
		$$.parms = $4
		popenv()
	}

functype:
	FUNCTION
	{
		Pushenv()
	}
	'(' parms ')' ':' funcret
	{
		$$ = newtype(Tfunc)
		$$.parms = $4
		$$.rtype = $7
		popenv()
	}

funcret:
	TYPEID
	{
		$$ = $1.ttype
	}
|
	ID
	{
		diag("type name expected; found '%s'", $1.name)
		$$ = tundef
	}
body:
	'{' stmts '}'
	{
		$$ = newbody($2)
	}
|
	'{' '}'
	{
		diag("empty block")
		$$ = newbody(newlist(Lstmt))
	}

stmts:
	stmts stmt
	{
		$$ = $1
		addstmt($$, $2)
	}
|
	stmt
	{
		$$ = newlist(Lstmt)
		addstmt($$, $1)
	}

stmt:
	assignstmt 
|
	procstmt 
|
	body 
|
	casestmt 
|
	repeatstmt 
|
	ifstmt 
|
	whilestmt 
|
	forstmt
|
	retstmt
|
	nullstmt
|
	error ';'
	{
		$$ = newstmt(0)
		diag("statement expected")
	}


nullstmt:
	';'
	{
		$$ = newstmt(';')
	}

retstmt:
	RETURN expr ';'
	{
		$$ = newstmt(RETURN)
		$$.expr = $2
		if env.prog == nil { panic("missing program declaration") }
		env.prog.prog.nrets++
	}

repeatstmt:
	DO body WHILE '(' expr ')' ';'
	{
		$$ = newstmt(DO)
		$$.expr = $5
		$$.stmt = $2
		cpsrc($$, $2)
		checkcond($$, $$.expr)
	}

whilestmt:
	WHILE '(' expr ')' body
	{
		$$ = newstmt(WHILE)
		$$.expr = $3
		$$.stmt = $5
		$$.sfname = $3.fname
		$$.lineno = $3.lineno
		checkcond($$, $$.expr)
	}

forstmt:
	FOR '(' lvalue '=' expr ',' forcond ')' body
	{
		$$ = newfor($3, $5, $7, $9)
		$$.sfname = $3.fname
		$$.lineno = $3.lineno
	}

forcond:
	ID forop expr
	{
		$$ = newexpr(Sbinary, $2, newvarnode($1), $3)
	}

forop:
	'<'
	{
		$$ = '<'
	}
|
	'>'
	{
		$$ = '>'
	}
|
	LE
	{
		$$ = Ole
	}
|
	GE
	{
		$$ = Oge
	}

ifstmt:
	ifhdr body
	{
		$$ = $1
		$$.thenarm = $2
	}
|
	ifhdr body ELSE body
	{
		$$ = $1
		$$.thenarm = $2
		$$.elsearm = $4
		if $4.op == '{' { $4.op = ELSE }
	}
|
	ifhdr body ELSE ifstmt
	{
		$$ = $1
		$$.thenarm = $2
		$$.elsearm = $4
	}

ifhdr:
	IF '(' expr ')'
	{
		$$ = newstmt(IF)
		$$.cond = $3
		checkcond($$, $$.cond)
	}

assignstmt:
	lvalue '=' expr ';'
	{
		$$ = newassign($1, $3)
	}
|
	lvalue ':' '='
	{
		diag("unexpected ':'")
	}
	expr ';'
	{
		$$ = newstmt(';')
	}

procstmt:
	ID '(' optargs ')' ';'
	{
		$$ = newstmt(FCALL)
		$$.fcall = newfcall($1, $3, Tproc)
	}

optargs:
	args
|
	/*empty*/
	{
		$$ = newlist(Lsym)
	}

args:
	args ',' expr
	{
		$$ = $1
		if $$ != nil { addsym($$, $3) }
	}
|
	expr
	{
		$$ = newlist(Lsym)
		addsym($$, $1)
	}

casestmt:
	SWITCH '(' expr ')' '{' cases '}'
	{
		$$ = newswitch($3, $6)
	}
cases:
	cases case
	{
		$$ = $1
		addstmt($$, $2)
	}
|
	case
	{
		$$ = newlist(Lstmt)
		addstmt($$, $1)
	}

case:
	CASE cexpr ':' stmts
	{
		$$ = newstmt(CASE)
		$$.expr = $2
		$$.stmt = newbody($4)
		cpsrc($$, $$.stmt)
	}
|
	DEFAULT ':' stmts
	{
		$$ = newstmt(CASE)
		$$.stmt = newbody($3)
	}

cexpr:
	cexpr ',' cexpr
	{
		$$ = newexpr(Sbinary, ',', $1, $3)
	}
|
	primary
	{
		if !evaluated($1) { diag("case expression must be a constant") }
		$$ = $1
	}
|
	primary DOTDOT primary
	{
		$$ = newexpr(Sbinary, Odotdot, $1, $3)
		if !evaluated($1) { diag("case expression must be a constant") }
		if !evaluated($3) { diag("case expression must be a constant") }
	}
expr:
	primary
|
	expr '+' expr
	{
		$$ = newexpr(Sbinary, '+', $1, $3)
	}
|
	expr '-' expr
	{
		$$ = newexpr(Sbinary, '-', $1, $3)
	}
|
	expr '*' expr
	{
		$$ = newexpr(Sbinary, '*', $1, $3)
	}
|
	expr '/' expr
	{
		$$ = newexpr(Sbinary, '/', $1, $3)
	}
|
	expr AND expr
	{
		$$ = newexpr(Sbinary, Oand, $1, $3)
	}
|
	expr OR expr
	{
		$$ = newexpr(Sbinary, Oor, $1, $3)
	}
|
	expr EQ expr
	{
		$$ = newexpr(Sbinary, Oeq, $1, $3)
	}
|
	expr NE expr
	{
		$$ = newexpr(Sbinary, One, $1, $3)
	}
|
	expr '%' expr
	{
		$$ = newexpr(Sbinary, '%', $1, $3)
	}
|
	expr '<' expr
	{
		$$ = newexpr(Sbinary, '<', $1, $3)
	}
|
	expr '>' expr
	{
		$$ = newexpr(Sbinary, '>', $1, $3)
	}
|
	expr GE expr
	{
		$$ = newexpr(Sbinary, Oge, $1, $3)
	}
|
	expr LE expr
	{
		$$ = newexpr(Sbinary, Ole, $1, $3)
	}
|
	expr POW expr
	{
		$$ = newexpr(Sbinary, Opow, $1, $3)
	}
|
	expr BADOP expr
	{
		$$ = nil
	}

primary:
	lvalue
|
	'+' primary
	{
		$$ = $2
	}
|
	'-' primary
	{
		$$ = newexpr(Sunary, Ouminus, $2, nil)
	}
|
	INT
	{
		$$ = newint($1, Oint, nil)
	}
|
	CHAR
	{
		$$ = newint($1, Ochar, nil)
	}
|
	REAL
	{
		$$ = newreal($1, nil)
	}
|
	STR
	{
		$$ = newstr($1)
	}
|
	NIL
	{
		$$ = newexpr(Sconst, Onil, nil, nil)
	}
|
	TRUE
	{
		$$ = newint(1, Otrue, nil)
	}
|
	FALSE
	{
		$$ = newint(0, Ofalse, nil)
	}
|
	ID '(' optargs ')'
	{
		$$ = newfcall($1, $3, Tfunc)
	}
|
	'(' expr ')'
	{
		$$ = $2
	}
|
	NOT primary
	{
		$$ = newexpr(Sunary, Onot, $2, nil)
	}
|
	TYPEID '(' args ')'
	{
		$$ = newaggr($1.ttype, $3)
	}
|
	LEN lenarg
	{
		$2.ttype = tderef($2.ttype)
		if $2.ttype == tundef { diag("argument '%s' of len is undefined", $2.name) }
		if tisatom($2.ttype) {
			$$ = newint(1, Oint, nil)
		}else{
			$$ = newint(tlen($2.ttype), Oint, nil)
		}
	}

lenarg:
	TYPEID
|
	ID
|
	ID '[' expr ']'
	{
		$$ = newexpr(Sbinary, '[', $1, $3)
	}
|
	ID '.' ID
	{
		$$ = fieldaccess($1, $3.name)
	}
|
	ID '^'
	{
		$$ = newexpr(Sunary, '^', $1, nil)
	}
	

lvalue:
	ID
	{
		$$ = newvarnode($1)
	}
|
	lvalue '[' expr ']'
	{
		$$ = newexpr(Sbinary, '[', $1, $3)
	}
|
	lvalue '.' ID
	{
		$$ = fieldaccess($1, $3.name)
	}
|
	lvalue '.' TYPEID
	{
		diag("'%s' is a type name", $3.name)
		$$ = $1
	}
|
	lvalue '^'
	{
		$$ = newexpr(Sunary, '^', $1, nil)
	}
|
	TYPEID
	{
		diag("'%s' is a type name", $1.name)
		$$ = newexpr(Snone, 0, nil, nil)
	}

%%

func puterror(fn string, ln int, name string, sfmt string, arg ...interface{}) {
	var s1, s2 string

	if name != "" {
		s1 = fmt.Sprintf("%s:%d: at '%s'", fn, ln, name)
	} else {
		s1 = fmt.Sprintf("%s:%d", fn, ln)
	}
	s2 = fmt.Sprintf(sfmt, arg...)
	if debug['S'] != 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s\n", s1, s2)
	if Nerrors > 10 {
		fmt.Fprintf(os.Stderr, "too many errors\n")
		os.Exit(1)
	}
}

func diag(sfmt string, arg ...interface{}) {
	Nerrors++
	puterror(Scanner.fname, Scanner.lineno, "", sfmt, arg...)
}

func (sc*Scan) Errorf(ffmt string, arg ...interface{}) {
	Nerrors++
	if ffmt == "syntax error" && len(Scanner.sval) > 0 {

		puterror(sc.fname, sc.lineno, string(sc.sval[:]), "syntax error", arg...)
	} else {
		puterror(sc.fname, sc.lineno, "", ffmt, arg...)
	}
}

func (pl PickyLex) Error(sfmt string) {
	Scanner.Errorf(sfmt)
}

//TODO Stmt and Sym should probably have a fname, lineno, etc interface

func (s *Sym) Error(sfmt string, arg ...interface{}) {
	Nerrors++
	puterror(s.fname, s.lineno, s.name, sfmt, arg...)
}

func (s *Stmt) Error(sfmt string, arg ...interface{}) {
	Nerrors++
	puterror(s.sfname, s.lineno, "", sfmt, arg...)
}

func SetYYDebug(){
	YyDebug = debug['Y']
}