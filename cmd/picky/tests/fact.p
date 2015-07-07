/*
!../8.pc  fact.p && ../8.pi  p.out
 */
program factorial;

function fact(n: int): int
{
	if(n <= 0){
		return 1;
	}else{
		return n * fact(n-1);
	}
}

consts:
	N = 8;	/* yields 40320 */
/* 	N = 18;	/* yields -898433024 */

procedure main()
{
	write("el factorial de ");
	write(N);
	write(" es ");
	write(fact(N));
	writeeol();
}
