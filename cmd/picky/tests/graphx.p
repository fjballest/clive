/*
 *	Graphx test	../pick/pick -D -L -E -P file.p
 *				../pick/pick file.p
 *				../pam/pam -D out.pam
 *				../pam/pam -D -S out.pam
 *				../pam/pam -S -X out.pam
 *			
 */

program G;


procedure main()
	g: file;
{
	gopen(g, "mygraph");
	gclear(g);
	gfillcol(g, Black, 1.0);
		
	gellipse(g, 100, 100, 20, 20, 0.0);
	gfillcol(g, 4, 1.0);
	gfillcol(g, Green, 0.0);
	gpolygon(3, 300, 300, 300, 7, 0.0);
	fflush(g);
	while(True){
		sleep(3200);
	}
}
