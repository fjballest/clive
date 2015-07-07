/*
 *	Graphx test	../pick/pick -D -L -E -P graphx.p
 *				../pick/pick graphx.p
 *				../pam/pam -D out.pam
 *				../pam/pam -D -S out.pam
 *				../pam/pam -S -X out.pam
 *				../pam/pam out.pam
 */

program G;


procedure main()
	g: graphx;
	x: int;
	y: int;
	nb: int;
{
	gopen(g, "mygraph");
	gclear(g);
	gfillcol(g, Black, 1.0);
		
	gellipse(g, 100, 100, 20, 20, 0.0);
	gfillcol(g, Green, 0.5);
	gpencol(g, Green, 1.0);
	gpolygon(g, 300, 300, 300, 7, 0.0);
	gflush(g);
	while(True){
		greadmouse(g, x, y, nb);
		if(nb != 0){
			gclear(g);
			gpencol(g, Red, 1.0);
			gfillcol(g, Black, 1.0);
		
			gellipse(g, x, y, 20, 20, 0.0);
			gfillcol(g, Green, 0.5);
			gpencol(g, Green, 1.0);
			gpolygon(g, 300, 300, 300, 7, 0.0);
			gflush(g);
		}
		sleep(100);
	}
}
