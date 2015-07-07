/*
	Px example
 */

program Xample;

consts:
	c1  = 11;	/* comment */
	c2 = 12;
	xx = 2 + 2 / 4;
	c3 = 4 * 2 + int(2.0) / 4 ** 1;
	r0 = 3.8;
	r1 = 3.8E2;
	greet = "hi";

types:
	month = (Ene, Feb, Mar);
	z = bool;
	digit = int 1..9;
	letter = char 'A'..'Z';
	mrange = month Ene..Feb;
	h = file;
	arry = array[mrange] of int;
	ptr =  ^int;
	zptr = ^arry;
	op = procedure( ref x: int, zzz: arry);

	r = record
	{
		f1: int;
		f2: arry;
		f3: op;
		f4: char;
	};

consts:
	mimes = Feb;

vars:
	e: arry;
	a: month;
	b: digit;
	c: bool;
	d: char;
	f: arry;
	g: r;
	n: int;
	xxx: ptr;

/* procedure p0();	/* forward */

procedure p0 ( )
	aa: month;
{
	aa = pred(a);
	aa = aa;
	do{
		c = True;
	}while(c != True);
	write(4);
}

procedure p1(a: int, ref b: float)
{
	p0();
	read(a);
	read(b);
}

function f0(x: arry): bool
	zz: int;
{
	stack();
	return 0 == 0;
}

procedure main()
	ff: h;
{
	a = Feb;
	a = pred(a);
	write(a);
	b = 8;
	write(b);
	b = succ(b);
	write("¡Hola mundo!");
	writeeol();
	if( f0(f) )
	{
		write('x'); writeeol();
	}
	if(3 == 4 and 5 == 3)
	{
		a = Ene;
		b = 3;
	}
	if(d < 'A' or n == 2)
	{
		c = False;
	}
	else
	{
		c= True;
	}
	for(n = 1, n < 2)
	{
		a = succ(a);
	}
	while(c == False)
	{
		c = True;
	}
	do{
		c = True;
	}while(c != True);
	fatal("problema");
	switch(4){
	case 3,4..8:
		c = True;
	case 1..4:
		c = True;
	case 5:
		c = True;
	default:
		;
	}
	a = pred(a);
	open(ff, "aaa", "r");
/*	fread(ff, a);
	readeol(); */
	write(a);
	peek(d);
	if( eol()){
		fread(ff, a);
	}
	if(eof()){;
	}
	
}
