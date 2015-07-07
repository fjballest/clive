/*
	Programa para calcular la longitud de
	la circunferencia de un círculo de radio r
	ROTO
 */

program calculoarea;

consts:
	Pi = 3.1415926;
	Radio1 = 3.2;
	Radio2 = 4.0;

function areacirculo(r: float): float
{
	return Pi * r ** 2.0;
}

/*
	Cuerpo del programa Principal.
 */

procedure main()
{
	writeln(areacirculo(Radio1));
	writeln(areacirculo(Radio2));
	writeln(2 ** 3);
	writeln(3 ** 2);
}
