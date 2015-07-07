/*
!../8.pick  -P sqrts.p && ../8.pam  p.out >[2=1]
 */
program sqrts;

procedure main()
	x: float;
{
	write(sqrt(4.0));
	writeeol();
	x = 4.0;
	write(sqrt(x));
	writeeol();
	x = -2.0;
	write(sqrt(x));
	writeeol();
}
