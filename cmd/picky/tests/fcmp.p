/*
!../8.pick  -P fcmp.p && ../8.pam  out.pam >[2=1]
 */
program fcmp;
consts:
	A = 1.2;
	B = 1.2;

procedure main()
{
	if(A == B){
		writeln("ok");
		writeln(A==B);
	}
}
