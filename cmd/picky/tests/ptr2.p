/*
!../8.pc   ptr2.p && ../8.pi  p.out >[2=1]
 */
program ptr2;

types:
	Pint =  ^int;

procedure main()
	p: Pint;
	q: Pint;
{
	new(p);
	p^ = 3;
	new(q);
	dispose(q);
	q = p;
	stack();
	data();
	writeln(q^);
	dispose(p);
	stack();
	data();
}
