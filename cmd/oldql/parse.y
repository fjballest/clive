
%token <sval> NAME
%token NL  INBLK HEREBLK FORBLK PIPEBLK LEN
%token IF ELSE ELSIF FOR APP WHILE FUNC
%token INTERRUPT ERROR

%{
package main

import (
	"clive/dbg"
	"os"
	"strings"
)

var (
	debugYacc bool
	yprintf = dbg.FlagPrintf(os.Stderr, &debugYacc)
	lvl int
)

%}

%union {
	sval string
	nd *Nd
	rdr Redirs
}

%type <nd> cmds cmd pipe simple elsif  var subcmds func
%type <nd> maps map names name inblk simplerdr toplvl xcmd source
%type <sval> optbg opt_name
%type <rdr> redir inredir outredir optredirs redirs 
%left '^'

%%

toplvl
	: cmds
	;
cmds
	: cmds xcmd
	{
		if lvl == 0 {
			$$ = nil
		} else {
			$$ = $1.Add($2)
		}
	}
	| xcmd
	{
		if lvl == 0 {
			$$ = nil
		} else {
			$$ = NewList(Ncmds, $1)
		}
	}
	;

xcmd
	: cmd
	{
		if lvl == 0 && $1 != nil && !Interrupted {
			if nerrors > 0 {
				yprintf("ERR %s\n", $1.sprint())
			} else {
				yprintf("%s\n", $1.sprint())
				$1.Exec()
			}
			nerrors = 0
		}
		$$ = $1
	}
	| func
	{
		$$ = $1
	}
	| source
	{
		$$ = $1
	}
	;

cmd
	: pipe optbg sep
	{
		$$ = $1
		if $2 != "" {
			$$.Args = append($$.Args, $2)
		}
	}
	| var sep
	{
		$$ = $1
	}
	| sep
	{
		$$ = NewNd(Nnop)
	}
	| error
	{
		$$ = nil
	}
	;

subcmds
	:
	{
		lvl++
		Prompter.SetPrompt(prompt2)
	}
	cmds
	{
		lvl--
		if lvl == 0 {
			Prompter.SetPrompt(prompt)
		}
		$$ = $2
	}
	;

func
	: FUNC NAME '{' subcmds '}'
	{
		f := NewNd(Nfunc, $2).Add($4)
		yprintf("%s\n", f.sprint())
		funcs[$2] = f
		$$ = nil
	}
	;

source
	: '<' NAME
	{
		yprintf("< %s\n", $2)
		lexer.source($2)
		$$ = nil
	}
	;

opt_nl
	: NL
	|
	;

opt_name
	: '[' NAME ']'
	{
		$$ = $2
	}
	|
	{
		$$ = "1"
	}
	;

pipe
	: pipe  '|' opt_name
	{
		Prompter.SetPrompt(prompt2)
	}
	opt_nl simplerdr
	{
		Prompter.SetPrompt(prompt)
		last := $1.Last()
		if strings.Contains($3, "0") {
			dbg.Warn("bad redirect for pipe")
			nerrors++
		}
		last.Redirs = append(last.Redirs, NewRedir($3, "|", false)...)
		last.Redirs.NoDups()
		$6.Redirs = append($6.Redirs, NewRedir("0", "|", false)...)
		$6.Redirs.NoDups()
		$$ = $1.Add($6)
	}
	| simplerdr
	{
		$$ = NewList(Npipe, $1)
	}
	;

simplerdr
	: simple optredirs
	{
		$$ = $1
		$2.NoDups()
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
	| FORBLK subcmds '}'
	{
		$2.Kind = Nforblk
		$$ = $2
	}
	| elsif
	| elsif ELSE '{' subcmds '}'
	{
		$$ = $1.Add(nil, $4)
	}
	| FOR names '{' subcmds '}'
	{
		$$ = NewList(Nfor, $2, $4)
	}
	| WHILE pipe '{' subcmds '}'
	{
		$$ = NewList(Nwhile, $2, $4)
	}
	;

elsif
	: IF pipe '{' subcmds '}'
	{
		$$ = NewList(Nif, $2, $4)
	}
	| elsif ELSIF pipe '{' subcmds '}'
	{
		$$ = $1.Add($3, $5)
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
	:  '<' NAME
	{
		$$ = NewRedir("0", $2, false)
	}
	;

outredir
	: '>' opt_name NAME
	{
		if strings.Contains($2, "0") {
			dbg.Warn("bad redirect for '>'")
			nerrors++
		}
		$$ = NewRedir($2, $3, false)
	}
	| APP opt_name NAME
	{
		if strings.Contains($2, "0") {
			dbg.Warn("bad redirect for '>'")
			nerrors++
		}
		$$ = NewRedir($2, $3, true)
	}
	|  '>' '[' NAME '=' NAME ']' 
	{
		$$ = NewDup($3, $5)
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
		$$ = NewNd(Nset, $1).Add($3)
	}
	| NAME '=' inblk
	{
		$$ = NewNd(Nset, $1).Add($3)
	}
	| NAME '=' '{' names '}'
	{
		$$ = NewNd(Nset, $1).Add($4.Child...)
	}
	| NAME '[' NAME ']'  '=' name
	{
		$$ = NewNd(Nset, $1, $3).Add($6)
	}
	| NAME '[' NAME ']'  '=' '{' names '}'
	{
		$$ = NewNd(Nset, $1, $3).Add($7.Child...)
	}
	| NAME '=' '{' maps '}'
	{
		$4.Args = append($4.Args, $1)
		$$ = $4
	}
	;

inblk
	: INBLK pipe '}'
	{
		$$ = NewList(Ninblk, $2)
	}
	| HEREBLK pipe '}'
	{
		$$ = NewList(Nhereblk, $2)
	}
	| PIPEBLK pipe '}'
	{
		$$ = NewList(Npipeblk, $2)
	}
	;

maps
	: maps map
	{
		$$ = $1.Add($2)
	}
	| map
	{
		$$ = NewList(Nset, $1)
	}
	;
map
	: '[' NAME ']' NAME
	{
		$$ = NewNd(Nset, $2, $4)
	}
	;

sep
	: NL
	| ';'
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
		$$ = NewList(Nnames, $1)
	}
	| inblk 
	{
		$$ = NewList(Nnames, $1)
	}
	;

name
	: NAME
	{
		$$ = NewNd(Nname, $1)
	}
	| '$' NAME
	{
		$$ = NewNd(Nval, $2)
	}
	| '$' '^' NAME
	{
		$$ = NewNd(Njoin, $3)
	}
	| '$' NAME '[' NAME ']'
	{
		$$ = NewNd(Nval, $2, $4)
	}
	| LEN NAME
	{
		$$ = NewNd(Nlen, $2)
	}
	| name '^' name
	{
		$$ = NewList(Napp, $1, $3)
	}
	;
%%
