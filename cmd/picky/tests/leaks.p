/*
!broke|rc ; ../8.pc   leaks.p && ../8.pi   p.out < words.p >[2=1]
 */
program Word;

consts:
	Blocknc = 2;

types:
	Tblock = array[1..Blocknc] of char;
	Tword = ^Tnode;
	Tnode = record{
		block: Tblock;
		nc: int;
		next: Tword;
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
		if(c == ' ' or c == '	'){
			read(c);
		}else if(c == Eol){
			readeol();
		}
	}while(not eof() and isblank(c));
	end = eof();
}

procedure initword(ref w: Tword)
{
	w = nil;
}

function wordnc(w: Tword): int
	tot: int;
{
	tot = 0;
	while(w != nil){
		tot = tot + w^.nc;
		w = w^.next;
	}
	return tot;
}

procedure writeword(w: Tword)
	i: int;
{
	write("'");
	while(w != nil){
		for(i = 1, i <= w^.nc){
			write(w^.block[i]);
		}
		w = w^.next;
	}
	write("'");
}

procedure mkblock(ref w: Tword)
{
	new(w);
	w^.nc = 0;
	w^.next = nil;
}

procedure addtoword(ref w: Tword, c: char)
	p: Tword;
{
	if(w == nil){
		mkblock(w);
	}
	p = w;
	while(p^.next != nil){
		p = p^.next;
	}
	if(p^.nc == Blocknc){
		mkblock(p^.next);
		p = p^.next;
	}
	p^.nc = p^.nc + 1;
	p^.block[p^.nc] = c;
}

procedure delword(ref w: Tword)
{
	if(w != nil){
		delword(w^.next);
		dispose(w);
		initword(w);
	}
}

procedure readword(ref w: Tword)
	c: char;
{
	do{
		read(c);
		addtoword(w, c);
		peek(c);
	}while(not eof() and not isblank(c));
		
}

function wordchar(w: Tword, n: int): char
	c: char;
{
	c = Nul;
	while(n > 0 and w != nil){
		if(n <= Blocknc){
			c = w^.block[n];
			n = 0;
		}else{
			n = n - Blocknc;
			w = w^.next;
		}
	}
	return c;
}

procedure cpword(ref dw: Tword, sw: Tword)
	i: int;
{
	for(i = 1, i <= wordnc(sw)){
		addtoword(dw, wordchar(sw, i));
	}
}

procedure main()
	done: bool;
	w: Tword;
	max: Tword;
{
	initword(max);
	do{
		skipblanks(done);
		if(not done){
			initword(w);
			readword(w);
			if(wordnc(w) > wordnc(max)){
				cpword(max, w);
			}
		}
	}while(not eof());
	writeword(max);
	write(" with len ");
	writeln(wordnc(max));
}
