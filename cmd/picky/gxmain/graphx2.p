/*
 *	Graphx test	../pick/pick -D -L -E -P graphx2.p
 *				../pick/pick graphx2.p
 *				../pam/pam -D out.pam
 *				../pam/pam -D -S out.pam
 *				../pam/pam -S -X out.pam
 *				../pam/pam -d out.pam
 *				../pam/pam out.pam
 */

program G;


procedure main()
	g: file;
	k: char;
{
	gopen(g, "mygraph");
	gclear(g);
	gfillcol(g, Black, 1.0);
	gline(g, 100, 100, 200, 300);
	do{
		sleep(100);
		fread(g, k);
		if (k == Eol){
			freadeol(g);
			writeeol();
		} else if (k != Eof) {
			writeln(k);
		}else{
			writeln("got eol");
		}
		gkeypress(g, k);
	}while(not feof(g));
	writeln("exited");
}
