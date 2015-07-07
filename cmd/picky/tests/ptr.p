/*
!../8.pc  -P ptr.p && ../8.pi  -XDMS p.out >[2=1]
 */
program ptr;

types:
	Pint =  ^int;

procedure main()
	p: Pint;
	q: Pint;
{
	new(p);
	new(q);
	stack();
	data();
	p^ = 3;
	q = p;
	writeln(q^);
	stack();
	data();
}
