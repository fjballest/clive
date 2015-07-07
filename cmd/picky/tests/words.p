/*
!../8.pc   words.p  && ../8.pi  p.out < words.p >[2=1]
 */
program Word;

consts:
	Maxword = 30;

types:
	Tindchar = int 1..Maxword;
	Tchars = array[Tindchar] of char;
	Tword = record{
		chars: Tchars;
		nchars: int;
	};

function isblank(c: char): bool
{
	return c == ' ' or c == Tab or c == Eol;
}

function nc(w: Tword): int
{
	return w.nchars;
}

procedure skipblanks(ref end: bool)
	c: char;
{
	do{
		peek(c);
		if(c == ' ' or c == '	'){
			read(c);
		}else if(c == Eol){
			readeol();
		}
	}while(not eof() and isblank(c));
	end = eof();
}

procedure readword(ref w: Tword)
	c: char;
{
	w.nchars = 0;
	do{
		read(c);
		w.nchars = w.nchars + 1;
		w.chars[w.nchars] = c;
		peek(c);
	}while(not eof() and not isblank(c));
		
}

procedure writeword(w: Tword)
	i: int;
{
	write("'");
	for(i = 1, i <= w.nchars){
		write(w.chars[i]);
	}
	write("'");
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
	initword(max);
	do{
		skipblanks(done);
		if(not done){
			readword(w);
			if(nc(w) > nc(max)){
				max = w;
			}
		}
	}while(not eof());
	writeword(max);
	write(" with len ");
	writeln(nc(max));
}
