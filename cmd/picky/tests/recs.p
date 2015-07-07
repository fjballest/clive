/*
!../8.pc  recs.p && ../8.pi  p.out >[2=1]
 */
program recs;

types:
	Tpt = record{
		x: int;
		y: int;
	};

procedure writept(pt: Tpt)
{
	write("[");
	write(pt.x);
	write(",");
	write(pt.y);
	write("]");
}

procedure incpt(ref pt: Tpt)
{
	pt.x = pt.x + 1;
	pt.y = pt.y + 1;
}

function addpt(p1: Tpt, p2: Tpt): Tpt
{
	p1.x = p1.x + p2.x;
	p1.y = p1.y + p2.y;
	return p1;
}

procedure main()
	p1: Tpt;
	p2: Tpt;
{
	p1.x = 3;
	p1.y = 3;
	p2.x = 4;
	p2.y = 4;
	writept(p1);
	writeeol();
	writept(p2);
	writeeol();
	incpt(p1);
	writept(p1);
	writeeol();
	p1 = addpt(p1, p2);
	writept(p1);
	writeeol();
}
