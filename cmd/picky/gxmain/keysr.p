/*
 *				../pick/pick keysr.p
 *				killall pam && ../pam/pam out.pam
 *				killall pam && ../pam/pam -d out.pam
 *				killall pam && ../pam/pam -S -X out.pam
 */
program keys;

consts:
	Nkeys =3;
	TQuantum = 50; /* milliseconds */

types:
	TypeKeys = array[0..Nkeys-1] of char;

procedure drawcoor(g: file)
{
	gloc(g, 0, 0, 0.5);
	gpenwidth(g, 20);
	gfillcol(g, Green, Opaque);
	fwriteln(g, "hola");
	fwrite(g, "perola");
	fflush(g);
}

procedure main()
	k: char;
	i: int;
	g: file;
{
	gopen(g, "keys");
	gclear(g);
	k = 'U';
	while(not feof(g)){
		sleep(TQuantum);
		drawcoor(g);
		fread(g, k);
		if(k != Nul){
			write(k);
			write(" ");
		}else{
			write("Nul ");
		}
		writeeol();
	}
	writeln("exited");
}