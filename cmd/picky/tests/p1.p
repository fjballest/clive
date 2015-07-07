/*
	Programa en Picky
	Autor: aturing
 */


/*
	Nombre del programa
 */

program hello;

/*
	Tipos de datos
 */
types:
	/* Dias de la semana */
	TDiaSem = (Lun, Mar, Mier, Jue, Vie, Sab, Dom);
	
/*
	Constantes
 */
	consts:
		Pi = 3.1415926;
		RadioPrueba = 2.0;

/*
	Funciones y procedimientos
 */

function LongitudCirculo(r: float): float
{
	return 2.0 * Pi * r;
}

procedure main()
{
	writeln(LongitudCirculo(RadioPrueba));
	writeln("Hola que tal.");
	
}
