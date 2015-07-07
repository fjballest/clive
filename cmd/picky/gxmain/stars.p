/*
 *	Gully Foyle is my name
 *	And Terra is my nation
 *	Deep space is my dwelling place
 *	And death's my destination.
 *
 *				../pick/pick stars.p
 *				killall pam && ../pam/pam out.pam
 *				killall pam && ../pam/pam -d out.pam
 *				killall pam && ../pam/pam -S -X out.pam
 */

/*
 *	Pseudo 3D without homogeneous coordinates
 *	Background with random stars moving away from center and lateraly with speed
 *	Stars appear at center (vector of spaceship) and move.
 *	Speed of Spaceship z + lateral speed (for stars)
 *	up down left, right, accelerate, deccelerate
 */

program starsmydest;

consts:
	Debug = True;
	Nstars = 40;
	TQuantum = 50; /* milliseconds */
	RadStar = 1;
	ZMinSpeed = 0.5;
	Pi = 3.1415;
	Floatlen = 8;
	Eps = 100.0;

types:
	TypeCoor = record{
		x: float;
		y: float;
	};

	TypeObject = record {
		col: color;
		radius: int;
		latvelocity: TypeCoor;
		accel: TypeCoor;
		zspeed: float;
		pos: TypeCoor;
	};
	TypeStars = array[0..Nstars-1] of TypeObject;
	TypeBackG = record{
		stars: TypeStars;
	};
	TypeNotes = array['a'..'g'] of sound;

consts:
	HalfStrength = 128;
	DarkStrength = 100;
	OffsetText = 20;
	SizeText = 20;
	BigPrime = 179426549;
	SmallPrime = 7;
	BigPrime2 = 17077;
	SizeX = 3000;
	SizeY = 2500;
	VeryTransp = 0.18;
	Center = TypeCoor(float(SizeX)/2.0, float(SizeY)/2.0);
	Origin = TypeCoor(0.0, 0.0);
	EmptyObj = TypeObject(Black, 0, Origin, Origin, 0.0, Origin);
	Vectlen = 200.0;
	Notes = TypeNotes(ANote, BNote, CNote, DNote, ENote, FNote, GNote);


function sumcoor(c1: TypeCoor, c2: TypeCoor): TypeCoor
	s: TypeCoor;
{
	s.x = c1.x+c2.x;
	s.y = c1.y+c2.y;
	return s;
}

function rotatecoor(c: TypeCoor, angle: float): TypeCoor
	s: TypeCoor;
{
	s.x = cos(angle)*c.x-sin(angle)*c.y;
	s.y = sin(angle)*c.x+cos(angle)*c.y;
	return s;
}

function negcoor(c: TypeCoor): TypeCoor
	s: TypeCoor;
{
	s.x = -c.x;
	s.y = -c.y;
	return s;
}

function scalcoor(l: float, c: TypeCoor): TypeCoor
	s: TypeCoor;
{
	s.x = l*c.x;
	s.y = l*c.y;
	return s;
}

procedure genrandstarcol(ref col: color)
	cgen: int;
{
	rand(20, cgen);
	if(cgen >= 2){
		col = White;
	}else if(cgen == 0){
		col = Yellow;
	}else if(cgen == 1){
		col = Orange;
	}
}

procedure randstar(ref star: TypeObject)
	col:int;
	x: float;
	sgen: int;
{
	star = EmptyObj;
	star.radius = RadStar;
	rand(SizeX, sgen);
	star.pos.x = float(sgen);
	rand(SizeY, sgen);
	star.pos.y = float(sgen);
	genrandstarcol(star.col);
}

procedure initstars(ref stars: TypeStars)
	i: int;
{
	for(i= 0, i < Nstars){
		randstar(stars[i]);
	}
}

procedure probj(obj: TypeObject)
{
	write("[");
	write(obj.pos.x);
	write(",");
	write(obj.pos.y);
	writeln("]");
}

/* for debug */
procedure drawcoor(g: file, c: TypeCoor)
	x: int;
	y: int;
{
	x = int(c.x) + OffsetText;
	y = int(c.y) + OffsetText;
	
	gloc(g, x, y, 0.5);
	gpenwidth(g, SizeText);
	gfillcol(g, Green, Opaque);
	fwrite(g, "[");
	fwrite(g, c.x);
	fwrite(g, ",");
	fwrite(g, c.y);
	fwriteln(g, "]");
	fwrite(g, "spaceship");

}

procedure drawstar(g: file, star: TypeObject)
	x: int;
	y: int;
{
/*	if(Debug){
		drawcoor(g, star.pos);
	}
*/
	gfillcol(g, star.col, Opaque);
	gpencol(g, star.col, Opaque);
	gpenwidth(g, 1);
	x = int(star.pos.x);
	y = int(star.pos.y);
	
	gellipse(g, x, y, star.radius, star.radius, 0.0);
}

procedure drawstars(g: file, stars: TypeStars)
	i: int;
{
	for(i= 0, i < Nstars){
		drawstar(g, stars[i]);
	}
}

procedure drawspace(g: file)
{
	gfillcol(g, Black, Opaque);
	gpenwidth(g, SizeY);
	gpencol(g, Black, Opaque);
	gline(g, 0, SizeY/2, SizeX, SizeY/2);
}

procedure drawbground(g: file, stars: TypeStars)
{
	drawspace(g);
	drawstars(g, stars);
}

procedure initspship(ref spship:TypeObject)
{
	spship = EmptyObj;
	spship.zspeed = ZMinSpeed;
	spship.pos = Center;
	spship.radius = 40;
}

procedure updateobj(ref obj: TypeObject)
{
	obj.pos.x = obj.pos.x + (obj.latvelocity.x/100.0)*float(TQuantum);
	obj.pos.y = obj.pos.y + (obj.latvelocity.y/100.0)*float(TQuantum);

}

procedure boxcoor(ref c: TypeObject)
{
	if(c.pos.x <= 0.0){
		c.pos.x = 0.0;
	}
	if(c.pos.x >= float(SizeX)){
		c.pos.x = float(SizeX);
	}
	if(c.pos.y <= 0.0){
		c.pos.y = 0.0;
	}
	if(c.pos.y >= float(SizeY)){
		c.pos.y = float(SizeY);
	}
}

procedure boxspship(ref c: TypeObject)
{
	if(c.pos.x <= 2.0*float(SizeX)/5.0){
		c.pos.x = 2.0*float(SizeX)/5.0;
	}
	if(c.pos.x >= 3.0*float(SizeX)/5.0){
		c.pos.x = 3.0*float(SizeX)/5.0;
	}
	c.pos.y = 3.0*float(SizeY)/5.0;
}

function abs(x: float): float{
	if (x < 0.0){
		return -x;
	}else{
		return x;
	}
}

function modcoor(c: TypeCoor): float
{
	return sqrt(c.x*c.x + c.y*c.y);
}

procedure recyclestar(spship: TypeObject, ref star: TypeObject)
	fugue: TypeCoor;
{
	if(spship.zspeed > 0.0 and (star.pos.x <= 0.0 or star.pos.y <= 0.0 or star.pos.x >= float(SizeX) or star.pos.y >= float(SizeY))){
		randstar(star);
		fugue = sumcoor(negcoor(star.pos), spship.pos);
		fugue = scalcoor(0.8, fugue);
		star.pos = sumcoor(star.pos, fugue);
	}
}

procedure updatestar(spship: TypeObject, ref star: TypeObject)
	fugue: TypeCoor;
	p: TypeCoor;
	fm: float;
{
	star.latvelocity = negcoor(spship.latvelocity);

	fugue = sumcoor(star.pos, negcoor(spship.pos));
	star.radius = int(0.99 + modcoor(fugue)/150.0);
	fugue = scalcoor((100.0*spship.zspeed)/(30.0*float(TQuantum)), fugue);
	star.latvelocity = fugue;
	star.latvelocity = scalcoor(modcoor(fugue)/2.0, fugue);
	updateobj(star);
	recyclestar(spship, star);
}

procedure updatestars(spship: TypeObject, ref stars: TypeStars)
	i: int;
{
	for(i= 0, i < Nstars){
		updatestar(spship, stars[i]);
	}
}

function newcoor(x: float, y: float): TypeCoor
	c: TypeCoor;
{
	c.x = x;
	c.y = y;
	return c;
}

function rotatecoororig(c: TypeCoor, angle: float): TypeCoor
	rc: TypeCoor;
	o: TypeCoor;
{
	o = Center;
	rc = sumcoor(c, negcoor(o));
	rc = rotatecoor(rc, angle);
	rc = sumcoor(rc, o);
	return rc;
}

function prodcoor(c1: TypeCoor, c2: TypeCoor): float
{
	return c1.x *c2.x + c1.y *c2.y;
}

procedure prvector(g: file, angle: float, origin: TypeCoor)
{
	gpencol(g, White, Opaque);
	gfillcol(g, White, Opaque);
	gpenwidth(g, SizeText);
	gloc(g, int(origin.x)+20, int(origin.y)+20, 0.0);
	fwrite(g, angle);
	gpenwidth(g, 3);
	gpolygon(g, int(origin.x+cos(angle)*Vectlen), int(origin.y+sin(angle)*Vectlen), 5, 3, angle+Pi/2.0);
	gline(g, int(origin.x), int(origin.y), int(origin.x+cos(angle)*Vectlen), int(origin.y+sin(angle)*Vectlen));
}

procedure drawship(g:file, spship: TypeObject, origin: TypeCoor)
	fugue: TypeCoor;
	c: TypeCoor;
	d: TypeCoor;
	fm: float;
	angle: float;
	pitch: float;
	x: float;
	y: float;
	ep: float;
	i: int;
	s: strength;
{

	if(Debug){
		gfillcol(g, Green, 0.3);
		gpencol(g, Green, 0.3);
		gpenwidth(g, 1);
		gpolygon(g, int(spship.pos.x), int(spship.pos.y), 10, 5, 0.0);
	}


	gfillrgb(g, HalfStrength, HalfStrength, HalfStrength, Opaque);
	gpenwidth(g, 1);
	gpenrgb(g, HalfStrength, HalfStrength, HalfStrength, Opaque);
	fugue = sumcoor(negcoor(spship.pos), origin);
	fm = modcoor(fugue);
	pitch = fm/20.0;
	angle = (acos(prodcoor(fugue, TypeCoor(1.0, 0.0))/(fm + 0.1e-10))-3.0*Pi/4.0) * 2.0 + 2.0*Pi;
	gpencol(g, Blue, Opaque);

	x = spship.pos.x + 2.0*fugue.x;
	y = spship.pos.y - fugue.y + 50.0;
	if(Debug){
		prvector(g, angle, origin); 
	}
	angle = angle + Pi/2.0;
	gpenwidth(g, 1);

	/* turbines */
	gpencol(g, Yellow, Transp);
	gfillcol(g, Yellow, VeryTransp);
	c = newcoor(x, y+60.0-0.15*pitch);
	c = rotatecoororig(c, angle);
	gellipse(g, int(c.x), int(c.y), spship.radius*2, int(pitch/2.5)*spship.radius/2, angle);

	gpencol(g, Red, 0.4);
	gfillcol(g, Red, 0.4);
	c = newcoor(x-10.0, y+50.0-0.15*pitch);
	c = rotatecoororig(c, angle);
	gellipse(g, int(c.x), int(c.y), spship.radius/2, int(pitch/3.0)*spship.radius/2, angle);

	gpencol(g, Red, 0.4);
	gfillcol(g, Red, 0.4);
	c = newcoor(x+10.0, y+50.0-0.15*pitch);
	c = rotatecoororig(c, angle);
	gellipse(g, int(c.x), int(c.y), spship.radius/2, int(pitch/3.0)*spship.radius/2, angle);

	gfillrgb(g, HalfStrength, HalfStrength, HalfStrength, Opaque);
	gpenrgb(g, HalfStrength, HalfStrength, HalfStrength, Opaque);

	/* central wing */
	c = newcoor(x, y+10.0);
	c = rotatecoororig(c, angle);
	gpolygon(g, int(c.x), int(c.y), 3*(spship.radius-int(pitch)), 3, angle);

	/* wings */
	c = newcoor(x+20.0+pitch/5.0, y+12.0-pitch);
	c = rotatecoororig(c, angle);
	gpolygon(g, int(c.x), int(c.y), 2*spship.radius, 3, angle);

	c = newcoor(x-20.0-pitch/5.0, y+12.0-pitch);
	c = rotatecoororig(c, angle);
	gpolygon(g, int(c.x), int(c.y), 2*spship.radius, 3, angle);

	/* extra wings */
	c = newcoor(x+60.0, y+10.0);
	c = rotatecoororig(c, angle);
	gpolygon(g, int(c.x), int(c.y), spship.radius, 3, angle);

	c = newcoor(x-60.0, y+10.0);
	c = rotatecoororig(c, angle);
	gpolygon(g, int(c.x), int(c.y), spship.radius, 3, angle);


	/* beak of the ship */
	c = newcoor(x, y-10.0-2.0*pitch);
	c = rotatecoororig(c, angle);
	gellipse(g, int(c.x), int(c.y), (12*spship.radius)/15, (3*int(pitch)*spship.radius)/15, angle); 

	gfillrgb(g, DarkStrength, DarkStrength, DarkStrength, Opaque);
	gpenwidth(g, 1);
	gpenrgb(g, DarkStrength, DarkStrength, DarkStrength, Opaque);

	/* tail */
	s = DarkStrength;
	for(i = 0, i < 50){
		c = newcoor(x, y+float(i));
		c = rotatecoororig(c, angle);
		d = newcoor(x, y);
		d = rotatecoororig(c, (angle-6.14)/23.0);
		gline(g, int(c.x), int(c.y), int(d.x), int(d.y));
		gpenrgb(g, s, s, s, Opaque);
		s = DarkStrength + strength(2 * i);
	}

	/* windows */
	gfillcol(g, Black, Opaque);
	gpencol(g, Black, Opaque);
	c = newcoor(x, y-2.0*pitch);
	c = rotatecoororig(c, angle);
	gpolygon(g, int(c.x), int(c.y), spship.radius/2, 4, angle);
	c = newcoor(x, y-17.0-2.0*pitch);
	c = rotatecoororig(c, angle);
	gpolygon(g, int(c.x), int(c.y), (2*spship.radius)/5, 3, angle);
}

procedure drawspship(g: file, spship: TypeObject)
	i: int;
	angle: float;
	pitch: float;
	c: TypeCoor;
{
	if(Debug){
		drawcoor(g, spship.pos);
	}
	drawship(g, spship, Center);
}

procedure main()
	stars: TypeStars;
	spship: TypeObject;
	g: file;
	x: int;
	y: int;
	nb: button;
	i: int;
	c: char;
	lastc: char;
	k: char;
	nticks: int;
{
	nticks = 0;
	lastc = Nul;
	initstars(stars);
	initspship(spship);
	gopen(g, "stars");
	gclear(g);
	for(i = 0, i < 10){
		updatestars(spship, stars);
	}
	while(not feof(g)){
		nticks = nticks + 1;
		updatestars(spship, stars);
		drawbground(g, stars);
		drawspship(g, spship);
		fflush(g);
		sleep(TQuantum);
		gclear(g);
		greadmouse(g, x, y, nb);
		spship.zspeed = ZMinSpeed + (float(y)-Center.x)/1500.0;
		if(spship.zspeed <= 0.0){
			spship.zspeed = 0.1;
		}
		if(nb != NoBut){
			if(nb == 1){
				gstop(g);
				gplay(g, Phaser);
				gshowcursor(g, True);
			}
			if(nb == 2){
				gshowcursor(g, False);
			}
		}
		gkeypress(g, c);
		if(c == ' '){
			gstop(g);
		}else if(c== 's'){
			gstop(g);
			gplay(g, Sheep);
		}else if(c>= 'a' and c <= 'g'){
			if(lastc != c and nticks > 2){
				gstop(g);
				gplay(g, Notes[c]);
			}
			nticks = 0;
		}else if(c != Eof){
			lastc = c;
			spship.pos.x = float(x);
			spship.pos.y = float(y);
			boxspship(spship);
			gkeypress(g, k);
		}
	}
	writeln("exited");
}
