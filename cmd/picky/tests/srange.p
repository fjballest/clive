/*
 	Los subrangos de tipos enumerados fallan.
		
 */

program dias;

types:
	TD = (Lun, Mar, Mie, Jue, Vie, Sab, Dom);
	TF = TD Vie..Dom;
	Natural = int 0..Maxint;

procedure main()
	d: TD;
	f: TF;
{

	d = Sab;
	f = d;
	writeln(d);
	writeln(f);		/* esto imprime <nil>  */

	d = Sab;
	f = Vie;
	writeln(d);
	writeln(f);		/* esto da el fallo de ejecucion run time violation: fault number 11 */	d = Jue;
	f = d;
}

