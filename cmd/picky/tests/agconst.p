program romano;
consts: 
	NumCifras = 6;
types:
	TipoIndice = int 0.. NumCifras -1;
	TipoRomano = (I, V, X, L, C, D, M);
	TipoNumRom = array [TipoIndice] of TipoRomano;
	
function valorde (romano: TipoRomano): int
{
	switch (romano){
	case I:
		return 1;
	case V:
		return 5;
	case X:
		return 10;
	case L:
		return 50;
	case C:
		return 100;
	case D:
		return 500;
	default:
		return 1000;
	}
}
consts:
	Prueba = TipoNumRom (I, V, X, L, C, D);

procedure main ()
{
	writeln (valorde (Prueba[0]));
	writeln("Ok");
	
}