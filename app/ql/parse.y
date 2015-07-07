
%token <sval> NAME
%token NL  INBLK RAWINBLK SINGLEINBLK TEEBLK PIPEBLK LEN INPIPE
%token FOR APP WHILE FUNC
%token INTERRUPT ERROR OR AND GFPIPE

%{
package ql

import (
	"strings"
	"clive/app"
)

%}

%union {
	cval rune
	sval string
	nd *Nd
	bval bool
	rdr Redirs
}

%type <nd> xsubcmds cmd pipe simple var subcmds xpipe
%type <nd> maps map names name inblk simplerdr cond ors ands 
%type <sval> optbg optname optinname
%type <cval> sep
%type <rdr> redir inredir outredir optredirs redirs 
%type <bval> pipeop
%left '^'

%%

start
	: topcmds
	|
	;

topcmds
	: topcmds topcmd
	| topcmd
	;

// Like cmd but we run them when parsed.
topcmd
	: cond sep
	{
		x := yylex.(*xCmd)
		if $1 != nil && $1.Kind != Nnop && !x.interrupted {
			if x.nerrors > 0 {
				yprintf("ERR %s\n", $1.sprint())
			} else {
				yprintf("%s\n", $1.sprint())
				x.Run($1)
			}
		}
	}
	| func sep	/* procesed while parsing */
	| error NL
	{
		x := yylex.(*xCmd)
		x.nerrors = 0
		// Discard all errors only at top-level
		// so errors within commands discard full commands.
	}
	;

// conditional command
cond
	: ors
	{
		$$ = $1
		// If it's or(and(x)) then return just x
		if $$ != nil && len($$.Child) == 1 {
			c := $$.Child[0]
			if c != nil && len(c.Child) == 1 {
				$$ = c.Child[0]
			}
		}
		// XXX: TODO: must check out that we don't have weird
		// &s in the pipes used in the cond.
	}
	;

ors
	: ors OR ands
	{
		$$ = $1.Add($3)
	}
	| ands
	{
		x := yylex.(*xCmd)
		$$ = x.newList(Ncond, $1)
	}
	;

ands
	: ands AND cmd
	{
		$$ = $1.Add($3)
	}
	| cmd
	{
		x := yylex.(*xCmd)
		$$ = x.newList(Ncond, $1)
	}
	;

cmd
	: pipe optbg
	{
		$$ = $1
		if $2 != "" {
			$$.Args = append($$.Args, $2)
		}
	}
	| var
	{
		$$ = $1
	}
	| source	/* processed during parsing */
	{
		x := yylex.(*xCmd)
		$$ = x.newNd(Nnop)
	}
	| 
	{
		x := yylex.(*xCmd)
		$$ = x.newNd(Nnop)
	}
	;

/* these are commands within a block */
subcmds
	:
	{
		x := yylex.(*xCmd)
		x.lvl++
		x.plvl++
		x.promptLvl(1)
	}
	xsubcmds
	{
		x := yylex.(*xCmd)
		x.lvl--
		x.plvl--
		if x.lvl == 0 {
			x.promptLvl(0)
		}
		$$ = $2
	}
	;

xsubcmds
	: xsubcmds sep cond
	{
		$$ = $1.Add($3)
	}
	| cond
	{
		x := yylex.(*xCmd)
		$$ = x.newList(Nblk, $1)
	}
	;

func
	: FUNC NAME '{' subcmds '}'
	{
		x := yylex.(*xCmd)
		if x.nerrors == 0 {
			f := x.newNd(Nfunc, $2).Add($4)
			yprintf("%s\n", f.sprint())
			x.funcs[$2] = f
		}
	}
	;

source
	: '<' NAME
	{
		x := yylex.(*xCmd)
		if x.nerrors == 0 {
			yprintf("< %s\n", $2)
			x.source($2)
		}
	}
	;

opt_nl
	: NL
	|
	;

optname
	: '[' NAME ']'
	{
		$$ = $2
	}
	|
	{
		$$ = "1"
	}
	;

pipeop
	: '|'
	{
		$$ = false
	}
	| GFPIPE
	{
		$$ = true
	}
	;

pipe
	: xpipe
	{
		x := yylex.(*xCmd)
		$$ = x.pipeRewrite($1)
	}
	| '|' xpipe
	{
		$$ = $2
		x := yylex.(*xCmd)
		if len($2.Child) > 0 {
			c := $2.Child[0]
			c.Redirs = append(c.Redirs, x.newRedir("0", "/dev/null", false)...)
			x.noDups(c.Redirs)
		}
	}
	| INPIPE xpipe
	{
		$$ = $2
	}
	;

xpipe
	: xpipe  pipeop optname
	{
		x := yylex.(*xCmd)
		x.promptLvl(1)
	}
	opt_nl simplerdr
	{
		x := yylex.(*xCmd)
		x.promptLvl(0)
		last := $1.Last()
		if strings.Contains($3, "0") {
			x.Errs("bad redirect for pipe")
		}
		if $2 {
			if $1.IsGet {
				x.Errs("'||' valid only in the first component of a pipe.")
			}
			$1.IsGet = true
		}
		last.Redirs = append(last.Redirs, x.newRedir($3, "|", false)...)
		x.noDups(last.Redirs)
		$6.Redirs = append($6.Redirs, x.newRedir("0", "|", false)...)
		x.noDups($6.Redirs)
		$$ = $1.Add($6)
	}
	| simplerdr
	{
		x := yylex.(*xCmd)
		$$ = x.newList(Npipe, $1)
	}
	;

simplerdr
	: simple optredirs
	{
		x := yylex.(*xCmd)
		$$ = $1
		x.noDups($2)
		$$.Redirs = $2
	}
	;

simple
	: names
	{
		$1.Kind = Nexec
		$$ = $1
	}
	| '{' subcmds '}'
	{
		$$ = $2
	}
	| TEEBLK subcmds '}'
	{
		$2.Kind = Nteeblk
		$$ = $2
	}
	| FOR names '{' 
	{
		x := yylex.(*xCmd)
		x.lvl++
		x.plvl++
	}
	subcmds '}'
	{
		x := yylex.(*xCmd)
		x.lvl--
		x.plvl--
		$$ = x.newList(Nfor, $2, $5)
		if $2.Kind == Nnames && len($2.Child) == 1 {
			$$.IsGet = true
		}
	}
	| WHILE pipe '{'
	{
		x := yylex.(*xCmd)
		x.lvl++
		x.plvl++
	}
	subcmds '}'
	{
		x := yylex.(*xCmd)
		x.lvl--
		x.plvl--
		$$ = x.newList(Nwhile, $2, $5)
	}
	;

optredirs
	: redirs
	{
		$$ = $1
	}
	|
	{
		$$ = nil
	}
	;

redirs
	: redirs redir
	{
		$$ = append($1, $2...)
	}
	| redir
	;

redir
	: inredir
	| outredir
	;


inredir
	:  '<' optinname
	{
		x := yylex.(*xCmd)
		$$ = x.newRedir("0", $2, false)
	}
	;

optinname
	: NAME
	| /* empty */
	{
		$$ = "/dev/null"
	}

outredir
	: '>' optname NAME
	{
		x := yylex.(*xCmd)
		if strings.Contains($2, "0") {
			x.Errs("bad redirect for '>'")
		}
		$$ = x.newRedir($2, $3, false)
	}
	| APP optname NAME
	{
		x := yylex.(*xCmd)
		if strings.Contains($2, "0") {
			x.Errs("bad redirect for '>'")
		}
		$$ = x.newRedir($2, $3, true)
	}
	|  '>' '[' NAME '=' NAME ']' 
	{
		x := yylex.(*xCmd)
		$$ = x.newDup($3, $5)
	}
	| '>' '[' NAME ']'	/* 3=2 is NAME, 3 =2 is NAME = NAME */
	{
		x := yylex.(*xCmd)
		toks := strings.SplitN($3, "=", -1)
		if len(toks) != 2 {
			x.Errs("bad [] redirection")
			toks = append(toks, "???")
		}
		$$ = x.newDup(toks[0], toks[1])
	}
	;

optbg
	: '&'
	{
		$$ = "&"
	}
	| '&' NAME
	{
		$$ = $2
	}
	|
	{
		$$ = ""
	}
	;

var
	: NAME '=' name
	{
		x := yylex.(*xCmd)
		$$ = x.newNd(Nset, $1).Add($3)
	}
	| NAME '=' inblk
	{
		x := yylex.(*xCmd)
		$$ = x.newNd(Nset, $1).Add($3)
	}
	| NAME '=' '{' names '}'
	{
		x := yylex.(*xCmd)
		$$ = x.newNd(Nset, $1).Add($4.Child...)
	}
	| NAME '[' NAME ']'  '=' name
	{
		x := yylex.(*xCmd)
		$$ = x.newNd(Nset, $1, $3).Add($6)
	}
	| NAME '[' NAME ']'  '=' inblk
	{
		x := yylex.(*xCmd)
		$$ = x.newNd(Nset, $1, $3).Add($6)
	}
	| NAME '[' NAME ']'  '=' '{' names '}'
	{
		x := yylex.(*xCmd)
		$$ = x.newNd(Nset, $1, $3).Add($7.Child...)
	}
	| NAME '=' '{' maps '}'
	{
		$4.Args = append($4.Args, $1)
		$$ = $4
	}
	;

inblk
	: INBLK 
	{
		x := yylex.(*xCmd)
		x.lvl++
	}
	pipe '}'
	{
		x := yylex.(*xCmd)
		x.lvl--
		$$ = x.newList(Ninblk, $3)
		// The last child of the pipe must have its output redirected to the pipe.
		if last := $3.Last(); last != nil {
			last.Redirs = append(last.Redirs, x.newRedir("1", "|", false)...)
			x.noDups(last.Redirs)
		}
	}
	| RAWINBLK
	{
		x := yylex.(*xCmd)
		x.lvl++
	}
	pipe '}'
	{
		x := yylex.(*xCmd)
		x.lvl--
		$$ = x.newList(Nrawinblk, $3)
		// The last child of the pipe must have its output redirected to the pipe.
		if last := $3.Last(); last != nil {
			last.Redirs = append(last.Redirs, x.newRedir("1", "|", false)...)
			x.noDups(last.Redirs)
		}
	}
	| SINGLEINBLK
	{
		x := yylex.(*xCmd)
		x.lvl++
	}
	pipe '}'
	{
		x := yylex.(*xCmd)
		x.lvl--
		$$ = x.newList(Nsingleinblk, $3)
		// The last child of the pipe must have its output redirected to the pipe.
		if last := $3.Last(); last != nil {
			last.Redirs = append(last.Redirs, x.newRedir("1", "|", false)...)
			x.noDups(last.Redirs)
		}
	}
	| PIPEBLK
	{
		x := yylex.(*xCmd)
		x.lvl++
	}
	pipe '}'
	{
		x := yylex.(*xCmd)
		x.lvl--
		$$ = x.newList(Npipeblk, $3)
		// The last child of the pipe must have its output redirected to the pipe.
		if last := $3.Last(); last != nil {
			last.Redirs = append(last.Redirs, x.newRedir("1", "|", false)...)
			x.noDups(last.Redirs)
		}
	}
	;

maps
	: maps map
	{
		$$ = $1.Add($2)
	}
	| map
	{
		x := yylex.(*xCmd)
		$$ = x.newList(Nset, $1)
	}
	;
map
	: '[' NAME ']' NAME
	{
		x := yylex.(*xCmd)
		$$ = x.newNd(Nset, $2, $4)
	}
	;

sep
	: NL
	{
		$$ = '\n'
	}
	| ';'
	{
		$$ = ';'
	}
	;

names
	: names name
	{
		$$ = $1.Add($2)
	}
	| names inblk
	{
		$$ = $1.Add($2)
	}
	| name
	{
		x := yylex.(*xCmd)
		$$ = x.newList(Nnames, $1)
	}
	| inblk 
	{
		x := yylex.(*xCmd)
		$$ = x.newList(Nnames, $1)
	}
	;

name
	: NAME
	{
		x := yylex.(*xCmd)
		$$ = x.newNd(Nname, $1)
	}
	| '$' NAME
	{
		x := yylex.(*xCmd)
		$$ = x.newNd(Nval, $2)
	}
	| '$' '^' NAME
	{
		x := yylex.(*xCmd)
		$$ = x.newNd(Njoin, $3)
	}
	| '$' NAME '[' NAME ']'
	{
		x := yylex.(*xCmd)
		$$ = x.newNd(Nval, $2, $4)
	}
	| LEN NAME
	{
		x := yylex.(*xCmd)
		$$ = x.newNd(Nlen, $2)
	}
	| name '^' name
	{
		x := yylex.(*xCmd)
		$$ = x.newList(Napp, $1, $3)
	}
	;
%%

func yprintf(l interface{}, fmts string, args ...interface{}) {
	app.Dprintf(fmts, args...)
}
