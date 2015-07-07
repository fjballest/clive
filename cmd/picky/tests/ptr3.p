/*
!../8.pc   ptr2.p && ../8.pi  p.out >[2=1]
 */
program ptr2;

types:
	Pint =  ^int;

procedure mkp(ref p: Pint)
{
	new(p);
}

procedure writep(p: Pint)
{
	writeln(p^);
}

procedure main()
	p: Pint;
	q: Pint;
{
	mkp(p);
	p^ = 3;
	mkp(q);
	dispose(q);
	q = p;
	stack();
	data();
	writep(q);
	dispose(p);
	stack();
	data();
}
