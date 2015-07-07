program punteros;

types:
       TipoPera = int;
       Arry = array[1..10] of int;
       Iptr = ^int;
       Aptr = ^arry;            /* debería ser ^Arry */
       Pptr = ^tipopera;     /* debería ser ^TipoPera */

procedure main()
       a: TipoPera;
{
       read(a);
       write(a);
}


