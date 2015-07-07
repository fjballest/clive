program hello;
/* /Users/paurea/src/gox/src/clive/cmd/picky/pick sw.p */
function bla(a: int): int
	x: int;
{
	x = a;
	switch(3){
	case 2..3:
		return 3;
	case 4, 5:
		;
	}
	return 5;

}

procedure main()
{
	writeln(bla(3));
	
}
