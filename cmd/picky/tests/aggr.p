/*
 !../8.pick  -g  aggr.p && ../8.pam    p.out >[2=1]
 */
program aggr;

types:
	Pt = record{
		x: int;
		y: int;
	};

	Pts = array[1..2] of Pt;

	Poly = record{
		pts: Pts;
		c: char;
	};

	Arry = array[0..1] of char;

	Word = record{
		chars: Arry;
		n: int;
	};

consts:
	ZP = Pt(0, 1);
	ZPS = Pts(ZP, ZP);
	ZPOL = Poly(Pts(ZP, ZP), 'X');

	Greet = Word("hi", 2);

vars:
	x: Pt;

procedure w(p: Pt)
{
	writeln(p.y);
}

procedure ww(w: Word)
{
	writeln(w.chars);
}

procedure main()
{
	if(ZPOL.c == 'X'){
		writeln("ok");
	}
	w(Pt(2, 3));
	if(ZPOL == Poly(Pts(ZP, ZP), 'X')){
		writeln("ok");
	}
	if(x == ZP){
		writeln("bad");
	}
	ww(Greet);
}
