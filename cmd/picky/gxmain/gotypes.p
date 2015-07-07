/*
 *				../pick/pick gotypes.p
 *				killall pam && ../pam/pam -d out.pam
 *				killall pam && ../pam/pam -S -X out.pam
 */
program keys;

consts:
	GenC = 23;

types:
	TypeInt = int;

procedure main()
	ti: TypeInt;
	i: int;
	s: strength;
	o: opacity;
	c: color;
{
	ti = 3;
	i = 5;
	ti = i;
	ti = int(i);
	ti = 5;
	ti = ti + 4;
	if(ti == 5){
		writeln(ti);
	}
	s = 45;
	if(s == 5){
		writeln(s);
	}
	writeln(s);
	s = succ(s);
	s = strength(s);
	s = i;
	s = ti;
	o = 4.5;
	if(o == 5.3){
		writeln(o);
	}
	writeln(o);
	s = succ(o);
	s = opacity(o);
	s = opacity(s);
	o = opacity(s);
	c = color(i);
}
