/*
 !../8.pick  -g  len.p && ../8.pam    p.out >[2=1]
 */
program alen;

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
	writeln(len p);
}

procedure ww(w: Word)
{
	writeln(len Word );
	writeln(len w.chars);
	writeln(len patata);
}

procedure main()
{
	w(Pt(2, 3));
	ww(Greet);
}
