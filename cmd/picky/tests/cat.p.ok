/*
!../8.pc    cat.p >[2=1] && ../8.pi  p.out >[2=1]

 */
program io;

/*
 *	We can use c == Eol and c == Eof instead of
 *	relying on feol() and feof().
 */

procedure main()
	in: file;
	c: char;
	i: int;
{
	open(in, "cat.p", "r");
	do{
		fread(in, c);
		if(feol(in)){
			freadeol(in);
			writeeol();
		}else if(not feof(in)){
			write(c);
		}
	}while(not feof(in));
	close(in);
}
