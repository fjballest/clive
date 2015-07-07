/*
!../8.pc   rec.p && ../8.pi   p.out < words.p >[2=1]
 */
program Word;

consts:
	Maxword = 2;

types:
	Tindchar = int 1..Maxword;
	Tchars = array[Tindchar] of char;
	Tword = record{
		chars: Tchars;
		nchars: int;
	};

procedure writeword(w: Tword)
	i: int;
{
	write("'");
	for(i = 1, i <= w.nchars){
		write(w.chars[i]);
	}
	writeln("'");
}

procedure initword(ref w: Tword)
	i: int;
{
	w.nchars = 0;
	for(i = 1, i <= Maxword){
		w.chars[i] = 'X';
	}
}

procedure main()
	done: bool;
	w: Tword;
	max: Tword;
{
	initword(w);
	writeword(w);
}
