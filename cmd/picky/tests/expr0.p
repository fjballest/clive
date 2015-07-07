/*
 *  El compilador casca así:
 *  
 *  pick 172270: suicide: sys: trap: divide error pc=0x00007084
 *  
 *  Debería dar un error de compilación si precalcula expresiones
 *  constantes, no cascar.
 */

program prueba;

consts:
       A =  2;
       B =  0;

procedure main()
{
       if(B != 0){
               writeln(A / B);
       }else{
               writeln("No puedo. B = 0");
       }
}