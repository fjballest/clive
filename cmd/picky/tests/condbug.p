program pp;


function bla(x: int): int
{
	return 3;
}


procedure main()
{
	if(bla(3)){
		writeln("na na");
	}

	while(bla(3)){
		writeln("na na");
	}
	do{
		writeln("na na");
	}while(bla(3));
}

