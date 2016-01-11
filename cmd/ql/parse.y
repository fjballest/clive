/* 
	Lgo tool yacc parse.y ; Lgo install
	other toks: = { } ; [ ] ^ = $ ( )
*/

%token FOR WHILE FUNC NL OR AND LEN SINGLE ERROR COND OR

%token <sval> PIPE IREDIR OREDIR BG APP NAME INBLK OUTBLK

%type <nd> name names cmd optnames list nameel mapels
%type <nd> bgpipe pipe cmd redir redirs optredirs spipe
%type <nd> blkcmds func cond setvar
%type <sval> optbg
%type <bval> optin 
%{
package main

%}

%union {
	sval string
	nd *Nd
	bval bool
}

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


topcmd
	: bgpipe sep
	{
		$1.run()
	}
	| func sep
	{
		$1.run()
	}
	| sep
	| error NL
	{
		// scripts won't continue upon errors
		yylex.(*lex).nerrors++
		if !yylex.(*lex).interactive {
			panic(parseErr)
		}
	}
	;

func
	: FUNC NAME '{' optsep blkcmds optsep '}'
	{
		$$ = newNd(Nfunc, $2).Add($5)
	}
	;

bgpipe
	: pipe optbg
	{
		$$ = $1
		$$.Args[0] = $2
	}
	| IREDIR name
	{
		$$ = newList(Nsrc, $2)
	}
	;

optbg
	: BG
	|
	{
		$$ = ""
	}
	;

pipe
	: optin spipe
	{
		$$ = $2
		$$.Args = append([]string{""}, $$.Args...)
		$2.addInRedir($1)
		$2.addPipeRedirs()
	}
	;

optin
	: PIPE
	{
		$$ = true
	}
	|
	{
		$$ = false
	}
	;

spipe
	: spipe PIPE cmd
	{
		$$ = $1.Add($3)
		$$.Args = append($$.Args, $2)
	}
	| cmd
	{
		$$ = newList(Npipe, $1)
	}
	;

cmd
	: names optredirs
	{
		$$ = newList(Ncmd, $1, $2)
	}
	| '{' optsep blkcmds optsep '}' optredirs
	{
		$$ = $3.Add($6)
	}
	| FOR names '{' optsep blkcmds optsep '}' optredirs
	{
		$5.Add(newList(Nredirs))
		$$ = newList(Nfor, $2, $5, $8)
	}
	| WHILE pipe '{' optsep blkcmds optsep '}' optredirs
	{
		$5.Add(newList(Nredirs))
		$$ = newList(Nwhile, $2, $5, $8)
	}
	| cond optredirs
	{
		$$ = $1.Add($2)
	}
	| setvar
	;

setvar
	: NAME as names
	{
		$$ = newNd(Nset, $1).Add($3)
	}
	| NAME as '(' mapels ')'
	{
		$$ = $4
		$$.Args = []string{$1}
	}
	| NAME '[' name ']' as names
	{
		$$ = newNd(Nset, $1).Add($3).Add($6)
	}
	;
as
	: '='
	| '←'
	;

cond
	: COND '{' optsep blkcmds optsep '}'
	{
		nd := $4
		nd.typ = Nor
		$$ = newList(Ncond, nd)
	}
	| cond OR '{' optsep blkcmds optsep '}'
	{
		nd := $5
		nd.typ = Nor
		$$ = $1.Add(nd)
	}
	;
blkcmds
	: blkcmds sep bgpipe
	{
		$$ = $1.Add($3)
	}
	| bgpipe
	{
		$$ = newList(Nblock, $1)
	}
	;

optredirs
	: redirs
	|
	{
		$$ = newList(Nredirs)
	}
	;

redirs
	: redirs redir
	{
		$$ = $1.Add($2)
	}
	| redir
	{
		$$ = newList(Nredirs, $1)
	}
	;

redir
	: IREDIR name
	{
		$$ = newRedir("<", $1, $2)
	}
	| OREDIR name
	{
		$$ = newRedir(">", $1, $2)
	}
	| APP name {
		$$ = newRedir(">>", $1, $2)
	}
	;

sep
	: NL
	| ';'
	;

optsep
	: sep
	|
	;

names
	: names nameel
	{
		$$ = $1.Add($2)
	}
	| nameel
	{
		$$ = newList(Nnames, $1)
	}
	;

nameel
	: name
	| list
	;
list
	: '(' optnames ')'
	{
		$$ = $2
	}
	| name '^' list
	{
		nd := newList(Nnames, $1)
		$$ = newList(Napp, nd, $3)
	}
	| list '^' name
	{
		nd := newList(Nnames, $3)
		$$ = newList(Napp, $1, nd)
	}
	| list '^' list
	{
		$$ = newList(Napp, $1, $3)
	}
	| INBLK optsep blkcmds optsep '}' 
	{
		$$ = $3
		$3.Args = []string{"<"}
		if $1 != "" {
			$3.Args = append($3.Args, $1)
		}
		$3.Add(newList(Nredirs))
		$3.typ = Nioblk
	}
	| OUTBLK optsep blkcmds optsep '}' 
	{
		$$ = $3
		if $1 == "" {
			$1 = "out"
		}
		$3.Args = []string{">", $1}
		$3.typ = Nioblk
		$3.Add(newList(Nredirs))
	}
	;

mapels
	:  mapels '[' names ']'
	{
		$$ = $1.Add($3)
	}
	| '[' names ']' 
	{
		// the parent adds Args with the var name
		$$ = newList(Nsetmap, $2)
	}
	;

optnames
	: names
	|
	{
		$$ = newList(Nnames)
	}
	;
name
	: NAME
	{
		$$ = newNd(Nname, $1)
	}
	| '$' NAME
	{
		$$ = newNd(Nval, $2)
	}
	| SINGLE NAME
	{
		$$ = newNd(Nsingle, $2)
	}
	| '$' NAME '[' name ']'
	{
		$$ = newNd(Nval, $2).Add($4)
	}
	| SINGLE NAME '[' name ']'
	{
		$$ = newNd(Nsingle, $2).Add($4)
	}
	| LEN NAME
	{
		$$ = newNd(Nlen, $2)
	}
	;
%%
