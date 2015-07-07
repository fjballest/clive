/*
!../8.pc  cast.p && ../8.pi   p.out >[2=1]
 */
program cast;

types:
	Tmon = (Jan, Feb, Mar);
	E = int;
	R = float;
consts:
	C1 = 3.4;
	C2 = int(C1);
	C3 = int('X');
	C4 = char(C3);

procedure main()
	c: char;
	m: Tmon;
	e: E;
	r: float;
{
	writeln(C2);
	writeln(C3);
	writeln(C4);
	c = char(int('A') + 1);
	writeln(c);
	writeln(1.2 + float(3));
	writeln(Tmon(int(Jan) + 1));
	m = Feb;
	m = Tmon(int(m) + 1);
	writeln(m);
	e = 3;
	e = E(3.4);
	r = 4.3;
	writeln(r);
}
