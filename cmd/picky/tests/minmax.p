/*
!../8.pc  rzero.p && ../8.pi  out.pam
 */
program mm;

procedure main()
	c: char;
	i: int;
{
	for(c = Minchar, c <= Maxchar){
		if(c == 'a'){
			c = 'z';
		}
		write("char pos is ");
		writeln(int(c));
	}
}
