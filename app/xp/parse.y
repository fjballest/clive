%token <ival>	INT
%token <uval>	UINT
%token <fval>	NUM
%token <sval>	FUNC NAME
%token <tval>	TIME
%{
package xp

import (
	"clive/dbg"
	"os"
	"math"
	"time"
)

var (
	debugYacc bool
	yprintf = dbg.FlagPrintf(os.Stderr, &debugYacc)
)

%}

%union {
	ival int64
	uval uint64
	fval float64
	sval string
	tval time.Time
	vval interface{}
}

%left OR
%left AND
%left '=' EQN NEQ
%left '<' '>' LE GE
%left '+' '-'
%left '*' '/' '%' '&' '|' SLEFT SRIGHT
%nonassoc UMINUS FUNC '!' '^'

%type <vval> expr

%%

toplvl:
	expr
	{
		x := yylex.(*lex)
		x.result = $1
	}

expr
	:  expr '+' expr
	{
		$$ = add($1, $3)
	}
	| expr '-' expr
	{
		$$ = sub($1, $3)
	}
	| expr '*' expr
	{
		$$ = mul($1, $3)
	}
	| '-' expr %prec UMINUS
	{
		$$ = minus($2)
	}
	| expr '/' expr
	{
		$$ = div($1, $3)
	}
	| expr '%' expr
	{
		$$ = mod($1, $3)
	}
	| expr SLEFT expr
	{
		$$ = shiftleft($1, $3)
	}
	| expr SRIGHT expr
	{
		$$ = shiftright($1, $3)
	}
	| '(' expr ')'
	{
		$$ = $2
	}
	| FUNC expr
	{
		if f, ok := funcs[$1]; ok {
			n := Nval($2)
			$$ = f(n)
		} else if v, err := fmtf($1, $2); err == nil {
			$$ = v
		} else if v, err := attr($1, $2); err == nil {
			$$ = v
		} else {
			panic("unknown function")
		}
	}
	| NUM
	{
		$$ = value($1)
	}
	| INT
	{
		$$ = value($1)
	}
	| UINT
	{
		$$ = value($1)
	}
	| NAME
	{
		$$ = value($1)
	}
	| TIME
	{
		$$ = value($1)
	}
	| expr '<' expr
	{
		$$ = value(cmp($1, $3) < 0)
	}
	| expr '>' expr
	{
		$$ = value(cmp($1, $3) > 0)
	}
	| expr LE expr
	{
		$$ = value(cmp($1, $3) <= 0)
	}
	| expr GE expr
	{
		$$ = value(cmp($1, $3) >= 0)
	}
	| expr '=' expr
	{
		$$ = value(cmp($1, $3) == 0)
	}
	| expr EQN expr
	{
		$$ = value(cmp($1, $3) == 0)
	}
	| expr NEQ expr
	{
		$$ = value(cmp($1, $3) != 0)
	}
	| expr AND expr
	{
		$$ = value(Bval($1) && Bval($3))
	}
	| expr OR expr
	{
		$$ = value(Bval($1) || Bval($3))
	}
	| expr '&' expr
	{
		$$ = value(Ival($1) & Ival($3))
	}
	| expr '|' expr
	{
		$$ = value(Ival($1) | Ival($3))
	}
	| '!' expr
	{
		$$ = value(! Bval($2))
	}
	| '^' expr
	{
		$$ = value(^ Ival($2))
	}
%%

var funcs = map[string]func(float64)float64{
	"abs": math.Abs,
	"acos": math.Acos,
	"acosh": math.Acosh,
	"asin": math.Asin,
	"asinh": math.Asinh,
	"atan": math.Atan,
	"atanh": math.Atanh,
	"cbrt": math.Cbrt,
	"cos": math.Cos,
	"cosh": math.Cosh,
	"exp": math.Exp,
	"exp2": math.Exp2,
	"floor": math.Floor,
	"Γ": math.Gamma,
	"log": math.Log,
	"log10": math.Log10,
	"log2": math.Log2,
	"sin": math.Sin,
	"sinh": math.Sinh,
	"sqrt": math.Sqrt,
	"tan": math.Tan,
	"tanh": math.Tanh,
	"trunc": math.Trunc,
}
