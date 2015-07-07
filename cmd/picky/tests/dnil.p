/*
!broke|rc ; ../8.pc   dnil.p && ../8.pi  -XDMS p.out < words.p >[2=1]
 */
program Word;

consts:
	Blocklen = 5;

types:
	Tblock = array[1..Blocklen] of char;
	Tword = ^Tnode;
	Tnode = record{
		block: Tblock;
		xlen: int;
		next: Tword;
	};


procedure main()
	w: Tword;
{
	w = nil;
	stack();
	if(w == nil){
		writeln("ok");
	}
}
