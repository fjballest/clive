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
	open(in, "rln.p", "r");
	freadln(in, c);
	if(c != Eol){
		writeln(c);
	}
	freadln(in, c);
	if(c != Eol){
		writeln(c);
	}
	freadln(in, c);
	if(c != Eol){
		writeln(c);
	}
	write(feol(in));
	write(feof(in));
	close(in);
}
