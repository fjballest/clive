/*
 *				../pick/pick keys.p
 *				killall pam && ../pam/pam out.pam
 *				killall pam && ../pam/pam -d out.pam
 *				killall pam && ../pam/pam -S -X out.pam
 */
program keys;

consts:
	Nkeys =3;
	TQuantum = 1000; /* milliseconds */

types:
	TypeKeys = array[0..Nkeys-1] of char;

procedure main()
	k: TypeKeys;
	i: int;
	g: file;
{
	gopen(g, "keys");
	gclear(g);
	do{
		fflush(g);
		sleep(TQuantum);
		gclear(g);
		gkeypress(g, k);
		for(i = 0, i < Nkeys){
			if(k[i] != Nul){
				write(k[i]);
				write(" ");
			}else{
				write("Nul ");
			}
		}

		writeeol();
	}while(not feof(g));
	writeln("exited");
}