/*
 *				../pick/pick mem.p
 *				killall pam && ../pam/pam out.pam
 *				killall pam && ../pam/pam -d out.pam
 *				killall pam && ../pam/pam -S -X out.pam
 */
program mem;

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

procedure main()
	pdead: TypePiecePtr;
	p: TypePiecePtr;
	lp: TypePiecePtr;
	prev: TypePiecePtr;
	i: int;
{
	lp = nil;
	for(i = 0, i < 100000){
		new(p);
		p^.id = i;
		p^.next = lp;
		lp = p;
	}
	i = 0;
	p = lp;
	prev = nil;
	while(p != nil){
		pdead = nil;
		if(i%3 == 0){
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
		i = i+1;
	}
	i = 0;
	p = lp;
	prev = nil;
	while(p != nil){
		pdead = nil;
		if(i%3 == 0){
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
		i = i+1;
	}
}