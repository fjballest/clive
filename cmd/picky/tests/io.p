/*
!../8.pc    io.p >[2=1] && ../8.pi  p.out >[2=1]

 */
program io;

types:
	Enum = (Aaa, Bbb, Ccc);
	Strrange = int 0..7;
	Str = array[Strrange] of char;

procedure main()
	out: file;
	in: file;
	i: int;
	s: Str;
	r: float;
	b: bool;
	e: Enum;
{
	open(out, "afile", "w");
	fwrite(out, "a string");
	fwriteeol(out);
	fwrite(out, 43);
	fwriteeol(out);
	fwrite(out, 32.1235);
	fwriteeol(out);
	fwrite(out, True);
	fwriteeol(out);
	fwrite(out, False);
	fwriteeol(out);
	fwrite(out, Aaa);
	fwriteeol(out);
	fwrite(out, Ccc);
	fwriteeol(out);
	close(out);

	open(in, "afile", "r");
	for(i = 0, i < 8){
		fread(in, s[i]);
	}
	if(feol(in)){
		write("eol ok");
		writeeol();
		freadeol(in);
	}
	for(i = 0, i < 8){
		write(s[i]);
	}
	writeeol();
	if(feol(in)){
		write("bad eol");
		writeeol();
	}
	fread(in, i);
	write(i);
	writeeol();
	fread(in, r);
	write(r);
	writeeol();
	fread(in, b);
	write(b);
	writeeol();
	fread(in, b);
	write(b);
	writeeol();
	fread(in, e);
	write(e);
	writeeol();
	frewind(in);
	fread(in, s);
	writeln(s);
	writeln(s);
	close(in);
}
