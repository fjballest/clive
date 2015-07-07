/*
 ../8.pick    arrys.p && ../8.pam  -DF  p.out 
 */
program arrys;

consts:
	Cs = "hola";
types:
	Str = array[0..3] of char;

procedure main()
	s: Str;
{
	s = Cs;
	writeln("hola");
	writeln(Cs);
	writeln(s); 
}
