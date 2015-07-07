/*
!../8.pc  rfns.p && ../8.pi  p.out >[2=1]

1.159279
1.159279
0.411517
0.411517
0.380506
0.380506
0.921061
0.921061
1.491825
1.491825
-0.916291
-0.916291
-0.397940
-0.397940
0.389418
0.389418
0.632456
0.632456
0.422793
0.422793
0.033699
0.033699

 */
program rfns;

procedure main()
	x: float;
	y: float;
{
	x = 0.4;
	y = 3.7;
	write(acos(0.4));
	writeeol();
	write(acos(x));
	writeeol();

	write(asin(0.4));
	writeeol();
	write(asin(x));
	writeeol();

	write(atan(0.4));
	writeeol();
	write(atan(x));
	writeeol();

	write(cos(0.4));
	writeeol();
	write(cos(x));
	writeeol();

	write(exp(0.4));
	writeeol();
	write(exp(x));
	writeeol();

	write(log(0.4));
	writeeol();
	write(log(x));
	writeeol();

	write(log10(0.4));
	writeeol();
	write(log10(x));
	writeeol();

	write(sin(0.4));
	writeeol();
	write(sin(x));
	writeeol();

	write(sqrt(0.4));
	writeeol();
	write(sqrt(x));
	writeeol();

	write(tan(0.4));
	writeeol();
	write(tan(x));
	writeeol();

	write(pow(0.4, 3.7));
	writeeol();
	write(pow(x, y));
	writeeol();

}
