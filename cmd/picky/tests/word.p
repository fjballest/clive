/*
!../8.pc   word.p && echo '     		hola' | ../8.pi  p.out >[2=1]
 */
program Word;

consts:
	Maxword = 30;
types:
	Nat = int 0..Maxint;
	Tindchar = int 1..Maxword;
	Tchars = array[Tindchar] of char;
	Tword = record{
		chars: Tchars;
		nchars: Nat;
	};

function isblank(c: char): bool
{
	return c == ' ' or c == Tab or c == Eol;
}

procedure skipblanks(ref end: bool)
	c: char;
{
	do{
		peek(c);
		if(c == ' ' or c == Tab){
			read(c);
		}else if(c == Eol){
			readeol();
		}
	}while(not eof() and isblank(c));
	end = eof();

	if(eol()){
		writeln("at eol");
	}
	if(eof()){
		writeln("at eof");
	}
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
	i: Nat;
{
	write("'");
	for(i = 1, i <= w.nchars){
		write(w.chars[i]);
	}
	write("'");
}

procedure main()
	done: bool;
	w: Tword;
{
	skipblanks(done);
	w.nchars = 0;
	if(not done){
		readword(w);
	}else{
		writeln("done");
	}
	writeword(w);
	writeeol();
}
