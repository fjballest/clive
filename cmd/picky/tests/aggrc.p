/*
 *  Parece que está fallando algo con los agregados cuando
 *  hay campos que son enumerados definidos por el usuario.
 * 
 *  /usr/prof/pickybook/src/p.p:32: assigned value out of range
 *  pc=0x40, sp=0x14c, fp=0x14c
 * 
 *  Si en lugar de asignar el valor a c con el agregado llamo
 *  a getcarta, casca a la hora de imprimir el campo palo de
 *  HuevoFrito en putcarta, "value out of range".
 * 
 *  Si no llamo a putcarta, ejecuta la rama del else (c != HuevoFrito).
 * 
 *  Con estos tipos parece funcionar, por lo que parece que es
 *  cosa de los enumerado definidos en el programa:
 *
 *  types:
 *        TipoPalo = char;
 *        TipoValor = int 1..10;
 *        TipoCarta = record
 *        {
 *                valor: TipoValor;
 *                palo: TipoPalo;
 *        HuevoFrito = TipoCarta(1, 'o');
 * 
 */

program cartas;

types:
       TipoPalo = (Oros, Copas, Espadas, Bastos);
       TipoValor = int 1..10;
       TipoCarta = record
       {
               valor: TipoValor;
               palo: TipoPalo;
       };

consts:
       HuevoFrito = TipoCarta(1, Oros);

procedure getcarta(ref carta: TipoCarta)
{
       read(carta.valor);
       read(carta.palo);
}

procedure putcarta(carta: TipoCarta)
{
       writeln(carta.valor);
       writeln(carta.palo);
}

procedure main()
       c: TipoCarta;
{

       c = TipoCarta(1, Oros);    /* casca */
       writeln("carta: ");
       putcarta(c);
       writeln("huevo frito: ");
       putcarta(HuevoFrito);
       if(c == HuevoFrito){
               writeln("Es el huevo frito");
       }else{
               writeln("NO es el huevo frito");
       }
}

