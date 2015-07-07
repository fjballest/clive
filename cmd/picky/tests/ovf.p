/*
!../8.pc  ovf.p && ../8.pi  p.out
 */
program overflow;

function ovf(n: int): int
{
	return n * ovf(n-1);
}

procedure main()
{
	write(ovf(-1));
}
