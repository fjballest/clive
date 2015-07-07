/*
 *	Tetris	../pick/pick goodtetris.p
 *				../pam/pam -d out.pam
 */

program Tetris;

consts:
	Debug = False;
	SizeX = 12;	/* in squares */
	SizeY = 20;
	SqSide = 150;
	SizeText = 40;
	MsgStart = "Pulsa s para empezar";
	MsgExit = "Pulsa x para salir";
	Npieces = 6;
	Nkeys = 4;
	Margin = 2.0;
	Maxupd = 300;

types:
	TypePureCol =  color Red..Blue;
	TypeCol = array[TypePureCol] of strength;
	TypeSqPos = record {
		x: int;
		y: int;
	};
	TypePoint = record {
		x: float;
		y: float;
	};
	TypeSqs = array[0..3] of TypeSqPos;
	TypePiecePtr = ^TypePiece;
	TypePiece = record {
		col: TypeCol;
		upcorner: TypeSqPos;	/* relative to corner */
		squares: TypeSqs;
		speed: float;
		upcornerpt: TypePoint;
		next: TypePiecePtr;
		nsquares: int;
		id: int;
	};
	TypePieces = array[0..Npieces-1] of TypePiece;
	TypeKeys = array[0..Nkeys-1] of char;

	TypeLine = array[0..SizeX-1] of TypePiecePtr;
	TypeBoard = array[0..SizeY-1] of TypeLine;
consts:
	DefSpeed = 60.0;
	KeySpeed = 120.0;
	SqLine = TypeSqs(TypeSqPos(0, 0), TypeSqPos(0, 1), TypeSqPos(0, 2), TypeSqPos(0, 3));
	SqSquare = TypeSqs(TypeSqPos(0, 0), TypeSqPos(1, 0), TypeSqPos(0, 1), TypeSqPos(1, 1));
	SqTee = TypeSqs(TypeSqPos(0, 0), TypeSqPos(0, 1), TypeSqPos(1, 1), TypeSqPos(0, 2));
	SqLR = TypeSqs(TypeSqPos(1, 0), TypeSqPos(0, 0), TypeSqPos(0, 1), TypeSqPos(0, 2));
	SqLL = TypeSqs(TypeSqPos(0, 0), TypeSqPos(1, 0), TypeSqPos(1, 1), TypeSqPos(1, 2));
	SqZ = TypeSqs(TypeSqPos(0, 0), TypeSqPos(1, 1), TypeSqPos(0, 1), TypeSqPos(1, 2));
	ColLine = TypeCol(0 , 150, 200); 
	ColSquare = TypeCol(255 , 255, 0); 
	ColTee = TypeCol(128 , 0, 128); 
	ColLR = TypeCol(0 , 0, 255); 
	ColLL = TypeCol(255, 165, 0);
	ColZ = TypeCol(80, 202, 0);
	OriginSq = TypeSqPos(0, 0);
	OriginPt = TypePoint(float(SqSide*SizeX)/2.0, 0.0);
	PieceLine = TypePiece(ColLine, OriginSq, SqLine, DefSpeed, OriginPt, nil, 4, 0);
	PieceSquare = TypePiece(ColSquare, OriginSq, SqSquare, DefSpeed, OriginPt, nil, 4, 0);
	PieceTee = TypePiece(ColTee, OriginSq, SqTee, DefSpeed, OriginPt, nil, 4, 0);
	PieceLR = TypePiece(ColLR, OriginSq, SqLR, DefSpeed, OriginPt, nil, 4, 0);
	PieceLL = TypePiece(ColLL, OriginSq, SqLL, DefSpeed, OriginPt, nil, 4, 0);
	PieceZ = TypePiece(ColZ, OriginSq, SqZ, DefSpeed, OriginPt, nil, 4, 0);
	TQuantum = 50;
	Pieces = TypePieces(PieceLine, PieceSquare, PieceTee, PieceLR, PieceLL, PieceZ);

procedure drawbg(g: file)
{
	gclear(g);
	gfillcol(g, Black, Opaque);
	gpencol(g, Black, Opaque);
	gpenwidth(g, SizeX*SqSide);
	gline(g, (SizeX*SqSide)/2, 0, (SizeX*SqSide)/2, SizeY*SqSide);
}

procedure drawpiece(g: file, p: TypePiece)
	i: int;
	x: int;
	y: int;
{
	gfillrgb(g, p.col[Red], p.col[Green], p.col[Blue], Opaque);
	gpencol(g, Green, Opaque);
	gpenwidth(g, 2);
	for(i = 0, i < p.nsquares){
		x =  int(p.upcornerpt.x)+SqSide*p.squares[i].x+SqSide/2;
		y = int(p.upcornerpt.y)+SqSide*p.squares[i].y+SqSide/2;
		gpolygon(g, x, y, int(float(SqSide)*sqrt(2.0))/2, 4, 0.0);
		if(Debug){
			gfillcol(g, Red, Opaque);
			gpencol(g, Red, Opaque);
			gloc(g, x-SqSide/2, y, 0.0);
			gpenwidth(g, SqSide/5);
			fwrite(g, p.squares[i].x);
			fwrite(g, " ");
			fwrite(g, p.squares[i].y);
			gfillrgb(g, p.col[Red], p.col[Green], p.col[Blue], Opaque);
			gpencol(g, Green, Opaque);
			gpenwidth(g, 2);
		}
	}
}

function movepiece(p: TypePiece, x: float, y: float): TypePiece
{
	p.upcornerpt.y = p.upcornerpt.y + y;
	p.upcornerpt.x = p.upcornerpt.x + x; 
	return p;
}

function abs(a: float): float
{
	if(a > 0.0){
		return a;
	}else{
		return -a;
	}
}

function isoverlap(a: float, b: float): bool
{
	return abs(a - b) < float(SqSide)-Margin;
}

function isintersect(p1: TypePiece, p2: TypePiece): bool
	isinter: bool;
	i: int;
	j: int;
	a: float;
	b: float;
{
	isinter = False;
	i = 0;
	while(i < p1.nsquares and not isinter){
		j = 0;
		while(j < p2.nsquares and not isinter){
			a = p1.upcornerpt.x+float(SqSide*p1.squares[i].x);
			b = p2.upcornerpt.x+float(SqSide*p2.squares[j].x);
			isinter = isoverlap(a, b);
			a = p1.upcornerpt.y+float(SqSide*p1.squares[i].y);
			b = p2.upcornerpt.y+float(SqSide*p2.squares[j].y);
			isinter = isinter and isoverlap(a, b);
			j = j + 1;
		}
		i = i + 1;
	}
	return isinter;
}


function iscollide(pnext: TypePiece, p: TypePiecePtr, pp: TypePiecePtr): bool
	last: bool;
	iscol: bool;
{

	last = False;
	iscol = False;
	while(pp != nil and not iscol){
		if(p != pp){
			iscol = isintersect(pnext, pp^);
		}
		pp = pp^.next;
	}
	return iscol;
}



function isinside(p: TypePiece): bool
	isin: bool;
	i: int;
	x1: float;
	x2: float;
	y1: float;
	y2: float;
{
	isin = True;
	i = 0;
	while(i < p.nsquares and isin){
		x1 = p.upcornerpt.x+float(SqSide*p.squares[i].x);
		x2 = x1 + float(SqSide);
		y1 = p.upcornerpt.y+float(SqSide*p.squares[i].y);
		y2 = y1 + float(SqSide);
		if(x1 + Margin <= 0.0 or x2 - Margin >= float(SizeX*SqSide)){
			isin = False;
		}else if(y1 + Margin <= 0.0 or y2 - Margin >= float(SizeY*SqSide)){
			isin = False;
		}
		i = i + 1;
	}
	return isin;
}

function round(a: float, b: int): float
	n: float;
	ni: float;
	nr: float;
	bb: float;
{
	bb = float(b);
	n = a/bb;
	ni = float(int(n));
	nr = n - ni;
	if(abs(nr) > 0.5){
		return ni*bb + bb;
	}else{
		return ni*bb;
	}
}
procedure squarefy(ref p: TypePiece)
{
	p.upcornerpt.x = round(p.upcornerpt.x, SqSide);
	p.upcornerpt.y = round(p.upcornerpt.y, SqSide);
	p.upcorner.x = int(p.upcornerpt.x/float(SqSide));
	p.upcorner.y = int(p.upcornerpt.y/float(SqSide));
}

function canmovey(p: TypePiecePtr, pp: TypePiecePtr): bool
	pnext: TypePiece;
{
	pnext = movepiece(p^, 0.0, float(SqSide)/4.0);
	return isinside(pnext) and not iscollide(pnext, p, pp);
}

procedure updatepiece(p: TypePiecePtr, pp: TypePiecePtr, ref wasupd: bool)
	porig: TypePiece;
	pnext: TypePiece;
	iscol: bool;
	isin: bool;
{
	wasupd = False;
	porig = p^;
	pnext = movepiece(p^, 0.0, p^.speed*float(TQuantum)/50.0);
	iscol = iscollide(pnext, p, pp);
	isin = isinside(pnext);

	if(not  iscol and isin){
		p^ = pnext;
		wasupd = True;
	}else {
		if(canmovey(p, pp)){
			wasupd = True;
		}else{
			squarefy(p^);
			wasupd = canmovey(p, pp);
		}
	}
}

function isbreak(p: TypePiecePtr, lineidx: int): bool
	i: int;
	over: bool;
	under: bool;
	y: int;
{
	over = False;
	under = False;
	for(i = 0, i < p^.nsquares){
		y = p^.upcorner.y + p^.squares[i].y;
		if(y > lineidx){
			over = True;
		}else if(y < lineidx){
			under = True;
		}
	}
	return over and under;
}

procedure elimsquares(p: TypePiecePtr, lineidx: int)
	i: int;
	squares: TypeSqs;
	nsquares: int;
	y: int;
	pgreat: TypePiecePtr;
	isb: bool;
{
	squares = p^.squares;
	nsquares = p^.nsquares;
	p^.nsquares = 0;
	
	isb = isbreak(p, lineidx);
	if(isb){
		new(pgreat);
		pgreat^ = p^;
		pgreat^.next = p^.next;
		p^.next = pgreat;
		pgreat^.speed = float(SqSide)/4.0;
	}else{
		pgreat = p;
	}

	for(i = 0, i < nsquares){
		y = p^.upcorner.y + squares[i].y;
		if(y < lineidx){
			p^.squares[p^.nsquares] = squares[i];
			p^.nsquares = p^.nsquares + 1;
		}else if (y > lineidx){
			pgreat^.squares[pgreat^.nsquares] = squares[i];
			pgreat^.squares[p^.nsquares].y = pgreat^.squares[p^.nsquares].y - 1;
			pgreat^.nsquares = pgreat^.nsquares + 1;
		}
	}
	p^.speed = float(SqSide)/4.0;	/*make sure it falls later */
}

function newline(): TypeLine
	l: TypeLine;
	i: int;
{
	for(i = 0, i < len l){
		l[i] = nil;
	}
	return l;
}

procedure drawline(g: file, lineidx: int)
	x1: int;
	y1: int;
	x2: int;
	y2: int;
{
	gfillcol(g, Yellow, Tlucid);
	gpencol(g, Yellow, Tlucid);
	gpenwidth(g, SqSide);
	x1 =  0;
	y1 = SqSide*lineidx+SqSide/2;
	x2 =  SqSide*SizeX;
	y2 = SqSide*lineidx+SqSide/2;
	gline(g, x1, y1, x2, y2);
	fflush(g);
}

function isletter(k: char): bool
{
	return k > 'a' and k < 'z';
}

procedure waitpress(g: file, k: char)
	rk: char;
{
	do{
		fpeek(g, rk);
		if(rk == Eol){
			freadeol(g);
		}else if (rk != Eof) {
			fread(g, rk);
		}
		if(Debug and isletter(rk)){
			write("pressed ");
			writeln(rk);
		}
	}while(k != rk and not feof(g));
	if(Debug){
		writeln("pressed key, end wait");
	}
}

procedure waitstart(g: file)
	k: char;
	l: int;
{
	gfillcol(g, White, Opaque);
	gpencol(g, White, Opaque);
	l = len MsgStart;
	gloc(g, (SqSide*SizeX)/2-(l*SizeText)/2, SqSide*SizeY/2, 0.0);
	gpenwidth(g, SizeText);
	fwrite(g, MsgStart);
	fflush(g);
	if(not feof(g)){
		waitpress(g, 's');
	}
}

procedure waitend(g: file)
	k: char;
	l: int;
{
	gfillcol(g, White, Opaque);
	gpencol(g, White, Opaque);
	l = len MsgExit;
	gloc(g, (SqSide*SizeX)/2-(l*SizeText)/2, SqSide*SizeY/2, 0.0);
	gpenwidth(g, SizeText);
	fwrite(g, MsgExit);
	fflush(g);
	if(not feof(g)){
		waitpress(g, 'x');
	}
}


procedure randpiece(ref p: TypePiece)
	n: int;
{
	rand(Npieces, n);
	p = Pieces[n];
	rand(Maxint, n);
	p.id = n;
}

procedure drawpieces(g: file, p: TypePiecePtr)
	last: bool;
{
	while(p != nil){
		drawpiece(g, p^);
		p = p^.next;
	}
}

procedure freepieces(p: TypePiecePtr)
	nextp: TypePiecePtr;
{
	while(p != nil){
		nextp = p^.next;
		dispose(p);
		p = nextp;
	}
}

procedure freezepiece(p: TypePiecePtr, ref b: TypeBoard)
	x: int;
	y: int;
	i: int;
{
	for(i = 0, i < p^.nsquares){
		x = p^.upcorner.x + p^.squares[i].x;
		y = p^.upcorner.y + p^.squares[i].y;
		b[y][x] = p;
	}
}

procedure unfreezepiece(p: TypePiecePtr, ref b: TypeBoard)
	x: int;
	y: int;
	i: int;
{
	for(i = 0, i < p^.nsquares){
		x = p^.upcorner.x + p^.squares[i].x;
		y = p^.upcorner.y + p^.squares[i].y;
		b[y][x] = nil;
	}
}


function rotate(p: TypePiece): TypePiece
	i: int;
	pr: TypePiece;
	a: int;
{
	pr = p;
	for(i = 0, i < p.nsquares){
		a = pr.squares[i].x;
		pr.squares[i].x = -pr.squares[i].y;
		pr.squares[i].y = a;
	}
	return pr;
}


procedure keyeffect(g: file, keys: TypeKeys, p: TypePiecePtr, pp: TypePiecePtr, ref wasupd: bool, ref lastrot: bool)
	pnext: TypePiece;
	x: float;
	y: float;
	i: int;
	iscol: bool;
	isin: bool;
	isrot: bool;
{
	x = 0.0;
	y = 0.0;
	isrot = False;
	for(i = 0, i < Nkeys){
		if(keys[i] == Left){
			wasupd = True;
			x = -KeySpeed;
		}else if(keys[i] == Right) {
			wasupd = True;
			x = KeySpeed;
		}else if(keys[i] == Up) {
			wasupd = True;
			isrot = True;
		}else if(keys[i] == Down) {
			wasupd = True;
			y = DefSpeed * 2.0;
		}else if(keys[i] == 'q'){
			fatal("quit now");
		}
	}
	if(p != nil){
		pnext = movepiece(p^, x, y);
		if(not isinside(pnext) and x != 0.0){
			/* make speed smaller to be able to reach corners */
			x = x/5.0;
		}
		pnext = p^;
		if(isrot and not lastrot){
			pnext = rotate(pnext);
		}
		pnext = movepiece(pnext, x, y);
		iscol = iscollide(pnext, p, pp);
		isin = isinside(pnext);
		if(not  iscol and isin){
			p^ = pnext;
		}
	}
	lastrot = isrot;
}

procedure redraw(g: file, lp: TypePiecePtr)
{
	gclear(g);
	drawbg(g);
	drawpieces(g, lp);
	fflush(g);
}

function islinepresent(b: TypeBoard, lineidx: int): bool
	i: int;
	present: bool;
{
	i = 0;
	present = True;
	do{
		if(b[lineidx][i] == nil){
			present = False;
		}
		i = i + 1;
	}while(i < SizeX and present);
	return present;
}

procedure elimline(ref b: TypeBoard, lineidx: int)
	i: int;
	p: TypePiecePtr;
	pn: TypePiecePtr;
	lastp: TypePiecePtr;
{
	p = nil;
	lastp = nil;
	for(i = 0, i < SizeX){
		p = b[lineidx][i];
		/* NB there are no U or O pieces */
		if(p != nil and p != lastp){
			unfreezepiece(p, b);
			pn = p^.next;
			elimsquares(p, lineidx);
			if(pn != p^.next){
				freezepiece(pn, b);
			}
			freezepiece(p, b);
		}
		lastp = p;
	}
}

procedure updatepieces(ref lp: TypePiecePtr, ref b: TypeBoard, lineidx: int, ref wasupd: bool)
	i: int;
	p: TypePiecePtr;
	isupd: bool;
{
	p = nil;
	wasupd = False;
	for(i = 0, i < SizeX){
		p = b[lineidx][i];
		/* NB there are no U or O pieces */
		if(p != nil){
			unfreezepiece(p, b);
			isupd = False;
			do{
				if(isupd){
					wasupd = True;
				}
				updatepiece(p, lp, isupd);
			}while(isupd);
			freezepiece(p, b);
		}
	}
}


procedure elimlines(g: file, lp: TypePiecePtr, ref b: TypeBoard)
	i: int;
	j: int;
	wasupd: bool;
{
	/* from the bottom, up */
	for(i = SizeY-1, i >= 0){
		if(islinepresent(b, i)){
			drawline(g, i);
			sleep(500);
			elimline(b, i);
			gplay(g, Woosh);
			sleep(700);
			gstop(g);
			updatepieces(lp, b, i, wasupd);
			if(wasupd){
				redraw(g, lp);
			}
		}
	}
	for(i = SizeY-1, i >= 0){
		updatepieces(lp, b, i, wasupd);
	}
	if(wasupd){
		redraw(g, lp);
	}
}

procedure prlist(lp: TypePiecePtr)
	p: TypePiecePtr;
{
	writeln("pieces list:");
	p = lp;
	while(p != nil){
		write(p^.id);
		write("-> ");
		p = p^.next;
	}
	writeeol();
}

procedure freedead(ref lp: TypePiecePtr)
	p: TypePiecePtr;
	prev: TypePiecePtr;
	pdead: TypePiecePtr;
{
	p = lp;
	prev = nil;
	while(p != nil){
		pdead = nil;
		if(p^.nsquares == 0){
			if(prev == nil){
				lp = p^.next;
			}else{
				prev^.next = p^.next;
			}
			pdead = p;
		}else{
			prev = p;
		}
		p = p^.next;
		if(pdead != nil){
			pdead^.next = nil;
			dispose(pdead);
		}
	}
}

procedure initboard(ref b: TypeBoard)
	i: int;
	j: int;
{
	for(i = 0, i < SizeY){
		for(j = 0, j < SizeX){
			b[i][j] = nil;
		}
	}
}

procedure main()
	g: file;
	k: char;
	lp: TypePiecePtr;
	p: TypePiecePtr;
	wasupd: bool;
	lost: bool;
	lastrot: bool;
	keys: TypeKeys;
	b: TypeBoard;
{
	wasupd = False;
	lp = nil;
	gopen(g, "tetris");
	drawbg(g);
	waitstart(g);
	lost = False;
	p = nil;
	lastrot = False;
	initboard(b);
	while(not feof(g) and not lost){
		redraw(g, lp);
		gkeypress(g, keys);
		keyeffect(g, keys, p, lp, wasupd, lastrot);
		if(not wasupd){
			if(p != nil){
				freezepiece(p, b);
			}
			elimlines(g, lp, b);
			freedead(lp);
			new(p);
			randpiece(p^);
			if(iscollide(p^, p, lp)){
				lost = True;
			}
			p^.next = lp;
			lp = p;
			wasupd = True;
			gstop(g);
			gplay(g, Beep);
		}
		if(not lost){
			sleep(TQuantum);
			updatepiece(p, lp, wasupd);
		}
	}
	gstop(g);
	gplay(g, Sheep);
	sleep(2000);
	gstop(g);
	waitend(g);
	freepieces(lp);
	if(Debug){
		writeln("exited");
	}
}
