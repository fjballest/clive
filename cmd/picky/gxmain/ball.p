program ball;

/*
 *				../pick/pick ball.p
 *				killall pam && ../pam/pam out.pam
 *				killall pam && ../pam/pam -d out.pam
 *				killall pam && ../pam/pam -S -X out.pam
 */

types:
	TypeVect = record {
		x: int;
		y: int;
	};

	TypeBall = record {
		pos: TypeVect;
		speed: TypeVect;
	};

consts:
	TQuantum = 50; /* milliseconds */
	SpeedScale = 50; /* divisor for milliseconds */
	SizeX = 5000;
	SizeY = 5000;
	SpeedX = -20;
	SpeedY = 43;
	BallRad = 100;
	Ball = TypeBall(TypeVect(BallRad, BallRad), TypeVect(SpeedX, SpeedY));

function sumvect(v1: TypeVect, v2: TypeVect): TypeVect
	s: TypeVect;
{
	s.x = v1.x+v2.x;
	s.y = v1.y+v2.y;
	return s;
}

function scalevect(v: TypeVect, l: int): TypeVect
	s: TypeVect;
{
	s.x = v.x*l;
	s.y = v.y*l;
	return s;
}

procedure reflect(ref b: TypeBall)
{
	if(b.pos.x < 0){
		b.pos.x = 0;
		b.speed.x = -b.speed.x;
	}else	if(b.pos.x > SizeX){
		b.pos.x = SizeX;
		b.speed.x = -b.speed.x;
	}
	if(b.pos.y < 0){
		b.pos.y = 0;
		b.speed.y = -b.speed.y;
	}else	if(b.pos.y > SizeY){
		b.pos.y = SizeY;
		b.speed.y = -b.speed.y;
	}
}

procedure prball(b: TypeBall)
{
	writeln(b.pos.x);
	writeln(b.pos.y);
}

procedure update(ref b: TypeBall)
{
	b.pos = sumvect(b.pos, scalevect(b.speed, TQuantum/SpeedScale));
	reflect(b);
}

procedure drawball(g: file, ref b: TypeBall)
	x: int;
	y: int;
{
	gfillcol(g, Green, Opaque);
	gpencol(g, Red, Opaque);
	gpenwidth(g, 1);
	x = b.pos.x;
	y = b.pos.y;
	
	gellipse(g, x, y, BallRad, BallRad, 0.0);
}

procedure main()
	b: TypeBall;
	g: file;
	k: char;
	debug: bool;
{
	b = Ball;
	gopen(g, "ball");
	debug = False;
	do{
		update(b);
		gclear(g);
		drawball(g, b);
		fflush(g);
		gkeypress(g, k);
		if(k == 'd'){
			debug = not debug;
		}
		if(debug){
			prball(b);
		}
		sleep(TQuantum);
	}while(not feof(g) and k != 'q');
	gclose(g);
}