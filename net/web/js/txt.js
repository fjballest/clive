/*
	js code for the wax text frame
	which is similar to a Plan 9 text frame
	but implemented with the HTML5 canvas.

	HTML5 designers suggest that you don't do this, but on the other
	hand, they do NOT handle text correctly in dom and they do NOT
	provide the interfaces required to handle things like undo and
	redo correctly. 

 */

/*
 * Hack to make sure the fixed and var width fonts exist, and
 * global font names for those variants.
 */
var tffixed = "AndaleMono";
var tfvar = "LucidaGrande";
var fontscheckedout = false;

function checkoutfonts(ctx) {
	if(fontscheckedout)
		return;
	fontscheckedout = true;
	var old = ctx.font;
	ctx.font = "50px Arial";
	var sz = ctx.measureText("ABC").width;
	ctx.font = "50px " + tffixed;
	if(ctx.measureText("ABC").width == sz)
		tffixed = "Courier";
	ctx.font = "50px " + tfvar;
	if(ctx.measureText("ABC").width == sz)
		tffixed = "Arial";
	ctx.font = old;
}

function tfixfont() {
	var c = this;
	var ctx = c.getContext("2d");
	var mod = "";
	var style = "";
	switch(this.fontstyle){
	case 't':
		style = tffixed;
		break;
	case 'r':
	default:
		style = tfvar;
		break;
	case 'rb':
	case 'br':
	case 'b':
		style = tfvar;
		mod = "bold";
		break;
	case 'bt':
	case 'tb':
		style = tffixed;
		mod = "bold";
		break;
	case 'i':
	case 'ri':
	case 'ir':
		style = tfvar;
		mod = "italic";
		break;
	}
	var ht = this.fontht - 2;
	ctx.font = mod + " "  + ht+"px "+style;
	ctx.textBaseline="top";
}

function adjdel(pos, delp0, delp1) {
	if(pos <= delp0)
		return pos;
	if(pos <= delp1)
		return delp0;
	return pos - (delp1 - delp0);
}

function lnlen(ln){
	if(!ln)
		return 0;
	if(ln.eol)
		return ln.txt.length + 1;
	return ln.txt.length;
}

function tdump() {
	var off = 0;
	for(var i = 0; i < this.lines.length; i++){
		var n = lnlen(this.lines[i]);
		var o = this.lines[i].off
		if(!o && !(o === 0)){
			console.log("BAD off " + o + " in:");
			o = off;
		}
		if(o != off){
			console.log("BAD off " + o + " (!=" + off + ") in:");
			off = o;
		}
		off += n;
		if(this.lines[i].eol)
			console.log(""+i+": "+o+"["+n+"]=\t" + this.lines[i].txt + "\\n");
		else
			console.log(""+i+": "+o+"["+n+"]=\t" + this.lines[i].txt);
	}
	console.log("sel: " + this.p0 + " " + this.p1);
	console.log("vers: ", this.vers);
}

function notabs(t, pos0) {
	if(t.indexOf('\t') < 0)
		return t;
	var s = "";
	var pos = 0;
	if(pos0)
		pos = pos0;
	for(var i = 0; i < t.length; i++){
		var r = t.charAt(i);
		if(r == '\t') {
			do {
				s += " ";
				pos++;
			}while(pos%8);
		}else{
			pos++;
			s += r;
		}
	}
	return s;	
}

function tlinewrap(t) {
	var c = this;
	var ctx = c.getContext("2d");
	var marginsz = 3;
	var avail = c.width - 2*marginsz;
	var pos = 0;
	var s = "";
	for(var i = 0; i < t.length; i++){
		var r = t.charAt(i);
		if(r == '\t') {
			do {
				s += " ";
				pos++;
			}while(pos%8);
		}else{
			pos++;
			s += r;
		}
		if(ctx.measureText(s).width > avail){
			return i;
		}
	}
	return t.length;
}

function tposdx(ln, n) {
	var t = ln.txt;
	var c = this;
	var ctx = c.getContext("2d");
	var pos = 0;
	var dx = 0;
	for(var i = 0; i < t.length && i < n; i++){
		var r = t.charAt(i);
		if(r == '\t') {
			do {
				dx += ctx.measureText(" ").width;
				pos++;
			}while(pos%8);
		}else{
			pos++;
			dx += ctx.measureText(r).width;
		}
	}
	return dx;
}

function tptr2tpos(cx, cy) {
	var marginsz = 3;
	var x = cx;
	var y = cy;
	var ovf = 0;
	var nln = Math.floor(y/this.fontht);
	if(nln < 0 || this.lines.length == 0)
		return [0, 0, ovf];
	if(nln+this.ln0 >= this.lines.length){
		var ll = this.lines[this.lines.length-1];
		return [ll.txt.length, nln, 1];
	}
	if(nln > this.frlines){		// overflow
		if(this.frlines+this.ln0 >= this.lines.length){
			var ll = this.lines[this.lines.length-1];
			return [ll.txt.length, nln, 1];
		}else
			return [0, this.ln0+this.frlines, 1];
	}
	var pos = 0;
	var ll = this.lines[nln+this.ln0];
	for(; pos <= ll.txt.length; pos++){
		coff = this.posdx(ll, pos);
		if(coff+marginsz > x){
			if(pos > 0)
				pos--;
			break;
		}
	}
	if(pos > ll.txt.length){
		pos = ll.txt.length;
		return [pos, nln, 1];
	}
	return [pos, nln, 0];
}

function ttpos2pos(x, nln) {
	if(nln < 0)
		return 0;
	if(nln+this.ln0 >= this.lines.length)
		return this.nrunes;
	var pos = this.lines[this.ln0].off;
	for(var i = 0; i < nln && i+this.ln0 < this.lines.length; i++)
		pos += lnlen(this.lines[this.ln0+i]);
	if(nln + this.ln0 < this.lines.length)
		pos += x;
	return pos;
}

function tuntick() {
	var c = this;
	var ctx = c.getContext("2d");
	if(!this.saved)
		return;
	ctx.putImageData(this.saved, this.tickx, this.ticky);
	this.saved = null;
}

function ttsetsel(p0, p1, refreshall) {
	var c = this;
	var ctx = c.getContext("2d");
	if(p0 > this.nrunes)
		p0 = this.nrunes;
	if(p1 < p0)
		p1 = p0;
	if(p1 > this.nrunes)
		p1 = this.nrunes;
	if(this.p0 != this.p1)
		refreshall = true;
	if(refreshall && (this.p1 <froff || this.p0 >froff+this.frsize))
		refreshall = false;
	var mp0 = p0;
	var mp1 = p1;
	if(refreshall){
		if(this.p0 < mp0)
			mp0 = this.p0;
		if(this.p1 > mp1)
			mp1 = this.p1;
	}
	this.p0 = p0;
	this.p1 = p1;
	this.untick();
	var froff = this.lines[this.ln0].off;
	if(mp1 <froff || mp0 >froff+this.frsize)
		return;
	var insel = false;
	for(var i = 0; i < this.frlines; i++){
		if(this.ln0+i >= this.lines.length)
			break;
		var xln = this.lines[this.ln0+i];
		if(mp1 >= xln.off && mp0 <= xln.off+xln.txt.length)
			insel=true;
		if(insel)
			this.drawline(xln, i);
		if(mp1 < xln.off)
			break;
	}
}

function tupdatescrl() {
	var c = this;
	var ctx = c.getContext("2d");
	var y0 = this.ln0 / this.lines.length * c.height;
	var dy = this.frlines / this.lines.length * c.height;
	ctx.clearRect(c.width-1, 0, 1, y0);
	ctx.fillRect(c.width-1, y0, 1, dy);
	ctx.clearRect(c.width-1, y0+dy, 1, c.height-(y0+dy));
}

function tdrawline(xln, i) {
	var c = this;
	var ctx = c.getContext("2d");
	var lnht = this.fontht;
	var marginsz = 3;
	var avail = c.width - 2*marginsz -1;
	var pos = i*lnht;
	if(pos >= c.height)
		return false;
	if(!xln){
		ctx.clearRect(0, pos, c.width-1, lnht);
		return true;
	}
	ln = notabs(xln.txt);
	/* draw selected line */
	if(this.p0 != this.p1){
		if(this.p0 > xln.off+xln.txt.length || this.p1 < xln.off){
			/* draw normal line */
			ctx.clearRect(0, pos, c.width-1, lnht);
			ctx.fillText(ln, marginsz, pos);
			return true;
		}
		/* up to p0 unselected */
		var dx = marginsz;
		var s0 = 0;
		var s0pos = 0;
		if(this.p0 > xln.off){
			s0 = this.p0 - xln.off;
			var s0ln = notabs(xln.txt.slice(0, s0));
			s0pos = s0ln.length;
			dx = marginsz + ctx.measureText(s0ln).width;
			ctx.clearRect(0, pos, dx, lnht);
			ctx.fillText(s0ln, marginsz, pos);
		}
		/* from p0 to p1 selected */
		var s1 = xln.txt.length - s0;
		if(this.p1 < xln.off+xln.txt.length)
			s1 = this.p1 - s0 - xln.off;
		var s1ln = notabs(xln.txt.slice(s0, s0+s1), s0pos);
		var s1pos = s0pos + s1ln.length;
		var sx = ctx.measureText(s1ln).width;
		var old = ctx.fillStyle;
		if(this.secondary == 2)
			ctx.fillStyle = "#FF7575";
		else if(this.secondary)
			ctx.fillStyle = "#7373FF";
		else
			ctx.fillStyle = "#D1A0A0";
		if(this.p1 > xln.off+xln.txt.length)
			ctx.fillRect(dx, pos, c.width-dx-1, lnht);
		else
			ctx.fillRect(dx, pos, sx, lnht);
		ctx.fillStyle = old;
		ctx.fillText(s1ln, dx, pos);
		if(this.p1 > xln.off+xln.txt.length)
			return true;
		/* from p1 unselected */
		ctx.clearRect(dx+sx, pos, c.width-(dx+sx)-1, lnht);
		if(s1 >= xln.txt.length)
			return true;
		var s2ln = notabs(xln.txt.slice(s0+s1, xln.txt.length), s1pos);
		ctx.fillText(s2ln, dx+sx, pos);
		return true;
	}

	/* draw unselected line */
	ctx.clearRect(0, pos, c.width-1, lnht);
	ctx.fillText(ln, marginsz, pos);

	/* draw tick if needed */
	if(this.p0 < xln.off || this.p0 > xln.off + xln.txt.length)
		return true;
	/*
	 * if p0 is at the end of a wrapped line don't draw it here,
	 * because the next line will draw it at the start of line.
	 */
	if(this.p0 == xln.off + xln.txt.length && !xln.eol &&
	   xln != this.lines[this.lines.length-1])
		return true;
	var dx = this.posdx(xln, this.p0 - xln.off);
	dx += marginsz - 2;
	this.saved = ctx.getImageData(dx, pos, 4, this.fontht);
	this.tickx = dx;
	this.ticky = pos;
	ctx.putImageData(this.tick, dx, pos);
	return true;
}

function treformat(l0) {
	this.fixfont();

	var lns = this.lines;
	for(var i = l0; i < lns.length;){
		while(!lns[i].eol && i < lns.length-1){
			lns[i].txt += lns[i+1].txt;
			lns[i].eol = lns[i+1].eol;
			lns.splice(i+1, 1);
		}
		/* remove empty lines (no \n) unless they are at the end
		 * because the last empty line is used to draw the tick
		 * past the last \n
		 */
		if(lns[i].txt.length == 0 && !lns[i].eol &&
		   lns.length>1 && i != lns.length-1)
			lns.splice(i, 1);
		else
			i++;
	}
	var off = 0;
	if(l0 > 0 && l0 < lns.length)
		off = lns[l0-1].off + lnlen(lns[l0-1]);
	for(var i = l0; i < lns.length; i++){
		lns[i].off = off;
		var nw = this.linewrap(lns[i].txt);
		if(nw < lns[i].txt.length){
			var old = lns[i].txt;
			var nl = {txt: old.slice(0, nw), off: lns[i].off};
			nl.eol = false;
			lns.splice(i, 0, nl);
			lns[i+1].txt = old.slice(nw, old.length);
			lns[i+1].off += nw;
		}
		off += lnlen(lns[i]);
	}
	if(l0 < this.ln0){
		if(this.ln0 >= lns.length)
			this.ln0 = lns.length-1;
	}
	if(lns.length == 0) {
		lns[0] = {txt:"", off: lnlen(last)};
	}
	var last = lns[lns.length-1];
	if(last.eol){
		lns[lns.length] = {txt:"", off: lnlen(last)};
	}
}

function tsetlinesoff(l0) {
	var lns = this.lines;
	for(var lns = this.lines; l0 < lns.length; l0++){
		if(l0 == 0)
			lns[l0].off = 0;
		else
			lns[l0].off = lns[l0-1].off + lnlen(lns[l0-1]);
	}
}

function tredrawtext() {
	var c = this;
	var ctx = c.getContext("2d");
	c.fixfont();
	var nlines = c.height / (this.fontht);

	if(!this.tick){
		var x = ctx.lineWidth;
		ctx.lineWidth = 1;
		ctx.fillRect(0, 0, 4, 2);
		ctx.fillRect(0, this.fontht-2, 4, 2);
		ctx.moveTo(2, 0);
		ctx.lineTo(2, this.fontht);
		ctx.stroke();
		ctx.lineWidth = x;
		this.tick = ctx.getImageData(0, 0, 4, this.fontht);
	}
	var off = this.lines[this.ln0].off;
	var froff = off;
	this.frsize = 0;
	this.frlines = 0;
	for(var i = 0; i <= nlines; i++){
		if(this.ln0+i < this.lines.length){
			this.frlines++;
			var xln = this.lines[this.ln0+i];
			off += lnlen(xln);
			if(!this.drawline(xln, i))
				break;
		}else
			if(!this.drawline(null, i))
				break;
	}
	this.frsize = off - froff;
	this.updatescrl();
}

function tscrolldown1() {
	if(this.ln0 >= this.lines.length-1)
		return false;
	this.froff += lnlen(this.lines[this.ln0]);
	this.ln0++;
	return true;
}

function tscrollup1() {
	if(this.ln0 == 0)
		return false;
	this.ln0--;
	return true;
}

function tscrolldown(n) {
	var some = false;
	for(; n > 0; n--){
		if(this.scrolldown1())
			some=true;
		else
			break;
	}
	return some;
}

function tscrollup(n) {
	var some = false;
	for(; n > 0; n--){
		if(this.scrollup1())
			some=true;
		else
			break;
	}
	return some;
}

/*
 * t may be arbitrary text (single line) or \n, but not both at once.
 * The text is always inserted at this.p0 and p0, p1 are updated.
 *
 */
function ttins(t, dontscroll) {
	this.untick();
	var nscrl = Math.floor(this.nlines/4);
	if(nscrl == 0)
		nscrl = 1;
	for(var i = 0; i < this.lines.length; i++){
		var xln = this.lines[i];
		var xlnlen = xln.txt.length;
		if(this.p0 >= xln.off && this.p0 <= xln.off+xlnlen){
			var pos = this.p0 - xln.off;
			if(t == '\n'){
				var nln = {
					txt: "",
					off: xln.off+pos+1,
					eol: xln.eol
				};
				if(pos < xlnlen){
					nln.txt = xln.txt.slice(pos, xlnlen);
					xln.txt = xln.txt.slice(0, pos);
				}
				this.lines.splice(i+1, 0, nln);
				xln.eol = true;
				this.p0++;
				this.p1 = this.p0;
				this.nrunes++;
				this.reformat(i);
				if(!dontscroll)
				if(i >= this.ln0+this.nlines-1 &&
				   i <= this.ln0+this.nlines+1)
					this.scrolldown(nscrl);
				this.redrawtext();
				return;
			}
			if(pos == xln.txt.length)
				xln.txt += t;
			else
				xln.txt = xln.txt.slice(0, pos) +
					t + xln.txt.slice(pos, xlnlen);
			this.p0 += t.length;
			this.p1 = this.p0;
			this.nrunes += t.length;
			var wr = this.linewrap(xln.txt);
			if(wr == xln.txt.length){
				this.setlinesoff(i+1);
				this.frsize += t.length;
				if(i >= this.ln0 && i <= this.ln0+this.frlines)
					this.drawline(xln, i-this.ln0);
			} else {
				this.reformat(i);
				if(!dontscroll)
				if(i >= this.ln0+this.nlines-1 &&
				   i <= this.ln0+this.nlines+1)
					this.scrolldown(nscrl);
				this.redrawtext();
			}
			return;
		}
	}
}

/*
 * if the selection is empty, the previous rune is deleted, otherwise
 * the entire selection is removed. p0 and p1 are updated.
 */

function ttdel(dontscroll) {
	this.untick();
	if(this.p0 >= this.nrunes || this.p1 < this.p0)
		return;
	var nscrl = Math.floor(this.nlines/4);
	if(nscrl == 0)
		nscrl = 1;
	var mightscroll = (this.p1 >= this.lines[this.ln0].off);
	if(this.p0 > 0 && this.p0 == this.p1)
		this.p0--;
	var i;
	var xln;
	for(i = 0; i < this.lines.length; i++){
		xln = this.lines[i];
		var xlnlen = xln.txt.length;
		if(this.p0 >= xln.off && this.p0 <= xln.off+xlnlen)
			break;
	}
	if(i == this.lines.length){
		console.log("tdel: out of text");
		return;
	}
	var ln0 = i;
	var ndel = this.p1 - this.p0;
	var off = this.p0 - xln.off;
	var tot = 0;
	do{
		var xln = this.lines[i];
		var xlnlen = xln.txt.length;
		var nd = ndel-tot;
		if(off+nd > xlnlen)
			nd = xlnlen - off;
		xln.txt = xln.txt.slice(0, off) + xln.txt.slice(off+nd, xlnlen);
		tot += nd;
		if(i == ln0 && tot == ndel && xln.eol){
			/* del within a line; don't reformat; just adjust */
			this.nrunes -= tot;
			this.p1 -= tot;
			this.setlinesoff(i+1);
			if(ln0 >= this.ln0 && ln0 < this.ln0+this.frlines){
				this.frsize -= tot;
				this.drawline(xln, i-this.ln0);
			}
			return;
		}
		if(tot < ndel && xln.eol){
			xln.eol = false;
			tot++;
		}
		i++;
		off = 0;
	}while(tot < ndel);
	this.setlinesoff(ln0+1);
	this.nrunes -= tot;
	this.p1 -= tot;
	this.reformat(ln0);
	this.redrawtext();
	if(!dontscroll && mightscroll && this.p0 < this.lines[this.ln0].off){
		this.scrollup(nscrl);
		this.redrawtext();
	}
}

var wordre = null;
function iswordchar(c) {
	if(!wordre)
		wordre = /\w/;
	return wordre.test(c);
}

function islparen(c) {
	return "([{<'`\"/".indexOf(c) >= 0;
}

function isrparen(c) {
	return ")]}>'`\"/".indexOf(c) >= 0;
}

function rparen(c) {
	var i = "([{<".indexOf(c);
	if(i < 0)
		return c;
	return ")]}>".charAt(i);
}

function lparen(c) {
	var i = ")]}>".indexOf(c);
	if(i < 0)
		return c;
	return "([{<".charAt(i);
}

function ttgetword(pos) {
	if(pos >= this.nrunes)
		return ["", this.nrunes, this.nrunes];
	var i;
	var xln;
	for(i = 0; i < this.lines.length; i++){
		xln = this.lines[i];
		var xlnlen = xln.txt.length;
		if(pos < xln.off || pos > xln.off+xlnlen)
			continue;
		/* now select the word (won't loop again) */
		var epos = pos;
		var p0 = pos - xln.off;
		if(p0 == xlnlen){
			if(!xln.eol)
				return [xln.txt, xln.off, xln.off+lnlen(xln)];
			return [xln.txt+"\n", xln.off, xln.off+lnlen(xln)];
		}
		/*
		 * heuristic: if click at the right of lparen and not
		 * at rparen, use the lparen.
		 */
		if(p0 > 0 && !isrparen(xln.txt.charAt(p0)) &&
		   islparen(xln.txt.charAt(p0-1))){
			pos--;
			p0--;
		}
		var p1 = p0;
		var c = xln.txt.charAt(p0);
		if(islparen(c)){
			pos++;
			var n = 1;
			var rc = rparen(c);
			var txt = "";
			p1++;
			epos++;
			do{
				for(; p1 < xlnlen; p1++, epos++){
					x = xln.txt.charAt(p1);
					if(x == rc)
						n--;
					else if(x == c)
						n++;
					if(n != 0)
						txt += x;
					if(n == 0)
						return [txt, pos, epos-1];
				}
				if(xln.eol){
					epos++;
					txt += "\n";
				}
				i++;
				if(i < this.lines.length){
					xln = this.lines[i];
					xlnlen = xln.txt.length;
					p1 = 0;
				}
			}while(n > 0 && i < this.lines.length);
			return [txt, pos, epos];
		}
		if(isrparen(c)){
			var n = 1;
			var lc = lparen(c);
			var txt = "";
			do{
				for(p0--; p0 >= 0; p0--){
					x = xln.txt.charAt(p0);
					if(x == lc)
						n--;
					else if(x == c)
						n++;
					if(n != 0){
						pos--;
						txt = x + txt;
					}
					if(n == 0)
						return [txt, pos, epos];
				}
				if(i > 0){
					i--;
					xln = this.lines[i];
					xlnlen = xln.txt.length;
					p0 = xlnlen;
					if(xln.eol){
						pos--;
						txt = "\n" + txt;
					}
				}
			}while(i >= 0 && n > 0);
			return [txt, pos, epos];
		}
		if(!iswordchar(c))
			return "";
		while(p0 > 0 && iswordchar(xln.txt.charAt(p0-1))){
			pos--;
			p0--;
		}
		while(p1 < xlnlen && iswordchar(xln.txt.charAt(p1))){
			epos++;
			p1++;
		}
		return [xln.txt.slice(p0, p1), pos, epos];
	}
	return ["", pos, pos];
}

function ttget(p0, p1) {
	if(p0 == p1 || p0 >= this.nrunes || p1 < p0)
		return "";
	var i;
	var xln;
	for(i = 0; i < this.lines.length; i++){
		xln = this.lines[i];
		var xlnlen = xln.txt.length;
		if(p0 >= xln.off && p0 <= xln.off+xlnlen)
			break;
	}
	if(i == this.lines.length){
		console.log("tget: out of text");
		return "";
	}
	var ln0 = i;
	var nget = p1 - p0;
	var off = p0 - xln.off;
	var tot = 0;
	var txt = "";
	do{
		var xln = this.lines[i];
		var xlnlen = xln.txt.length;
		var ng = nget-tot;
		if(off+ng > xlnlen)
			ng = xlnlen - off;
		txt += xln.txt.slice(off, off+ng);
		tot += ng;
		if(tot < nget && xln.eol){
			txt += "\n";
			tot++;
		}
		i++;
		off = 0;
	}while(tot < nget);
	return txt;
}

/*
 * call tins for arbitrary text.
 */
function ttinslines(x) {
	try{
		var ln = x.split('\n');
		for(var i = 0; i < ln.length; i++) {
			if(ln[i].length > 0)
				this.tins(ln[i]);
			if(i < ln.length-1)
				this.tins('\n');
		}
	}catch(ex){
		console.log("tinslines: " + ex);
	}
}

function tmayresize(dx, dy) {
	var p = $(this).parent();
	var dx = p.width();
	var dy = p.height();
	console.log('text resized', dx, dy);
	var c = this;
	var ctx = this.getContext("2d");
	c.width = dx;
	c.height = dy;
	this.nlines = Math.floor(c.height/this.fontht);
	this.saved = null;
	this.reformat(0);
	this.redrawtext();
}

function tmrlse(e) {
	var b = 1<<(e.which-1);
	if(b == 1 && this.malt){
		b = 2;
		this.buttons &= ~1;
		this.malt = false;
	}
	this.buttons &= ~b;
	return b;
}

function tmpress(e) {
	var b = 1<<(e.which-1);
	if(b == 1 && e.altKey){
		b = 2;
		this.malt = true;
	}
	this.buttons |= b;
	return b;
}

// set lastx, lasty to ev coords relative to canvas
function tevxy(e) {
	if(e.fakex != undefined) {
		this.lastx = e.fakex;
		this.lasty = e.fakey;
		return
	}
	var poff = $(this).offset();
	this.lastx = e.pageX - poff.left;
	this.lasty = e.pageY - poff.top;
}

function tmwait() {
	this.onmousemove = evxy;
	this.onmousedown = function(e){
		try{
			this.evxy(e);
			this.tmpress(e);
		}catch(ex){
			console.log("tmwait: down: " + ex);
		}
	};
	this.onmouseup = function(e){
		try{
			this.evxy(e);
			this.tmrlse(e);
			if(this.buttons == 0){
				this.onmousedown = this.tmdown;
				this.onmouseup = this.tmup;
				this.onmousemove = this.evxy;
			}
		}catch(ex){
			console.log("tmwait: up: " + ex);
		}
	};
}

/*
 * XXX: while selecting we should post a hold event to defer
 * updates until we are done with the mouse, that means we
 * select the right text and only after we are done the server would
 * update our text and selection to whatever is current.
 */
function tm1(pos) {
	var now = new Date().getTime();
	if(!this.clicktime || now-this.clicktime>500){
		this.dblclick = false;
		this.clicktime = now;
	}else{
		this.dblclick = true;
		this.clicktime = now;
	}

	if(this.dblclick){
		var x = this.tgetword(pos);
		this.post(["click1", x[0], ""+x[1], ""+x[2]]);
		this.tsetsel(x[1], x[2]);
		this.post(["tick", ""+this.p0, ""+this.p1]);
		this.clicktime = null;
	} else
		this.post(["tick", ""+pos, ""+pos]);

	this.onmousemove = function(e){
		try{
			this.evxy(e);
			if(!this.buttons)
				return;
			var tpos = this.ptr2tpos(this.lastx, this.lasty);
			var npos = this.tpos2pos(tpos[0], tpos[1]);
			if(npos > pos){
				if(this.p0 != pos || this.p1 != npos)
					this.tsetsel(pos, npos, true);
			}else{
				if(this.p0 != npos || this.p1 != pos)
					this.tsetsel(npos, pos, true);
			}
			if(tpos[2] < 0 && this.scrollup1())
				this.redrawtext();
			if(tpos[1] >= this.frlines && this.scrolldown1())
				this.redrawtext();
			return false;
		}catch(ex){
			console.log("tm1: move: " + ex);
		}
	};
	this.onmousedown = function(e){
		try{
			this.evxy(e);
			this.tmpress(e);
			if(this.buttons == 3){
				this.post(["tick", ""+this.p0, ""+this.p1]);
				this.Post(["ecut", ""+this.p0, ""+this.p1]);
			}
			if(this.buttons == 5){
				if(this.p0 != this.p1){
					this.Post(["edel", ""+this.p0, ""+this.p1]);
				}
				this.post(["epaste", ""+this.p0, ""+this.p1]);
			}
		}catch(ex){
			console.log("tm1: down: " + ex);
		}
	};

	this.onmouseup = function(e){
		try{
			this.evxy(e);
			this.tmrlse(e);
			if(this.buttons == 0){
				this.onmousedown = this.tmdown;
				this.onmouseup = this.tmup;
				this.onmousemove = this.evxy;
			}
		}catch(ex){
			console.log("tm1: up: " + ex);
		}
	}
}

/*
 * XXX: while selecting we should post a hold event to defer
 * updates until we are done with the mouse, that means we
 * select the right text and only after we are done the server would
 * update our text and selection to whatever is current.
 */
function tm23(pos, b) {
	this.secondary = b;
	this.onmousemove = function(e){
		try{
			this.evxy(e);
			if(!this.buttons)
				return;
			var tpos = this.ptr2tpos(this.lastx, this.lasty);
			var npos = this.tpos2pos(tpos[0], tpos[1]);
			if(npos > pos){
				if(this.p0 != pos || this.p1 != npos)
					this.tsetsel(pos, npos, true);
			}else
				if(this.p0 != npos || this.p1 != pos)
					this.tsetsel(npos, pos, true);
			if(tpos[2] < 0 && this.scrollup1())
				this.redrawtext();
			if(tpos[1] >= this.frlines && this.scrolldown1())
				this.redrawtext();
			return false;
		}catch(ex){
			console.log("tm1: move: " + ex);
		}
	};
	this.onmousedown = function(e){
		try{
			this.evxy(e);
			this.tmpress(e);
			this.secondaryabort = true;
		}catch(ex){
			console.log("tm2: down: " + ex);
		}
	};
	this.onmouseup = function(e){
		try{
			this.evxy(e);
			this.tmrlse(e);
			if(this.buttons == 0){
				var sp0 = this.p0;
				var sp1 = this.p1;
				this.secondary = 0;
				this.tsetsel(this.oldp0, this.oldp1);
				if(!this.secondaryabort)
				if(sp0 != sp1){
					var txt = this.tget(sp0, sp1);
					this.post(["click"+b, "", ""+sp0, ""+sp1, txt]);
				}else if(this.p0 != this.p1 &&
					 sp0 >= this.p0 && sp0 <= this.p1){
					var txt = this.tget(this.p0, this.p1);
					this.post(["click"+b, txt,
						  ""+this.p0, ""+this.p1]);
				}else{
					var x = this.tgetword(sp0);
					this.post(["click"+b, x[0],
						  ""+x[1], ""+x[2]]);
				}
				this.onmousedown = this.tmdown;
				this.onmouseup = this.tmup;
				this.onmousemove = this.evxy;
			}
		}catch(ex){
			console.log("tm2: up: " + ex);
		}
		if(this.buttons == 0){
			this.p0 = this.oldp0;
			this.p1 = this.oldp1;
			this.secondary = 0;
			this.secondaryabort = false;
		}
	}
}

function tmup(e) {
	e.preventDefault();
	try {
		this.tmrlse(e);
		this.evxy(e);
		if(this.buttons == 0){
			this.post(["tick", ""+this.p0, ""+this.p1]);
		}
	}catch(ex){
		console.log("tevmup: " + ex);
	}
}

function tmwheel(e) {
	try {
		e.preventDefault();
		var d = e.wheelDelta;
		var s = 1;
		if(d < 0){
			d = -d;
			d = 1 + Math.floor(d/10);
			if(this.scrolldown(d)){
				this.untick();
				this.redrawtext();
			}
		}else{
			d = 1 + Math.floor(d/10);
			if(this.scrollup(d)){
				this.untick();
				this.redrawtext();
			}
		}
	}catch(ex){
		console.log("tmwheel: " + ex);
	}
}

function tmdown(e) {
	e.preventDefault();
	$("#" + this.divid ).focus();
	// console.log("tmdown ", this.divid);
	this.secondary = 0;		/* paranoia: see tm23 */
	this.secondaryabort = false;
	try {
		this.tmpress(e);
		this.evxy(e);
		var b = this.buttons;
		switch(b){
		case 1:
			var tpos = this.ptr2tpos(this.lastx, this.lasty);
			var pos = this.tpos2pos(tpos[0], tpos[1]);
			this.tsetsel(pos, pos);
			this.tm1(pos);
			break;
		case 2:
		case 4:
			var tpos = this.ptr2tpos(this.lastx, this.lasty);
			var pos = this.tpos2pos(tpos[0], tpos[1]);
			this.oldp0 = this.p0;
			this.oldp1 = this.p1;
			this.tsetsel(pos, pos);
			this.tm23(pos, b);
			break;
		default:
			this.tmwait();
		}
	}catch(ex){
		console.log("evmdown: " + ex);
	}
	e.returnValue = false;
}

function tevkey(e) {
	try {
		// console.log("key");
		var key = e.keyCode;
		if(!e.keyCode)
			key = e.which;
		var rune = String.fromCharCode(e.keyCode);
		if(0)console.log("key: which " + e.which + " key " + e.keyCode +
			" '" + rune + "'");
		switch(key){
		case 9:
			rune = "\t";
			break;
		case 13:
			rune = "\n";
			break;
		}
		switch(rune){
		case 'c':
		case 'C':
			if(e.ctrlKey || e.metaKey){
				e.preventDefault();
				this.post(["ecopy", ""+this.p0, ""+this.p1]);
				return false;
			}
			break;
		case 'v':
		case 'V':
			if(e.ctrlKey || e.metaKey){
				e.preventDefault();
				if(this.p0 != this.p1){
					this.Post(["edel", ""+this.p0, ""+this.p1]);
				}
				this.post(["epaste", ""+this.p0, ""+this.p1]);
				return false;
			}
			break;
		case 'x':
		case 'X':
			if(e.ctrlKey || e.metaKey){
				e.preventDefault();
				this.Post(["ecut", ""+this.p0, ""+this.p1]);
				return false;
			}
			break;
		}
		if(e.metaKey || e.ctrlKey)
			return;
		if(this.p0 != this.p1){
			this.Post(["edel", ""+this.p0, ""+this.p1]);
		}
		this.Post(["eins", rune, ""+this.p0]);
	}catch(ex){
		console.log("text: fixtab: " + ex);
	}
}


function tkeyup(e) {
	try{
		var key = e.keyCode;
		if(!e.keyCode)
			key = e.which;
		var rune = String.fromCharCode(e.keyCode);
		if(0)
		console.log("keyup which " + e.which + " key " + e.keyCode +
			" '" + rune + "'" +
			" " + e.ctrlKey + " " + e.metaKey);
		switch(key){
		case 112:	/* F1 */
		case 113:	/* F2 */
		case 114:	/* F3 */
			var mev = {
				fakex: this.lastx,
				fakey: this.lasty,
				which: key-112+1,
			};
			mev.preventDefault = function(){}
			this.onmouseup(mev);
			break;
		default:
			return true;
		}
		return false;
	}catch(ex){
		console.log("keydown: " + ex);
	}
}

function tkeydown(e) {
	try{
		var key = e.keyCode;
		if(!e.keyCode)
			key = e.which;
		var rune = String.fromCharCode(e.keyCode);
		if(0)console.log("keydown which " + e.which + " key " + e.keyCode +
			" '" + rune + "'" +
			" " + e.ctrlKey + " " + e.metaKey);
	
		switch(key){
		case 8:		/* backspace */
			if(this.p0 != this.p1){
				this.Post(["edel", ""+this.p0, ""+this.p1]);
			}else if(this.p0 > 0){
				var p0 = this.p0-1;
				this.Post(["edel", ""+p0, ""+this.p1]);
			}
			break;
		case 9:		/* tab */
			if(this.p0 != this.p1){
				this.Post(["edel", ""+this.p0, ""+this.p1]);
			}
			this.Post(["eins", "\t", ""+this.p0]);
			break;
		case 27:	/* escape */
			this.post(["intr", "esc"]);
			this.dump();
			console.log("["+this.p0+","+this.p1+"]='" +
				this.tget(this.p0, this.p1) + "'");
			break;
		case 37:	/* left */
			this.post(["eundo"]);
			break;
		case 38:	/* up */
			if(this.scrollup(3)){
				this.untick();
				this.redrawtext();
			}
			break;
		case 39:	/* right */
			this.post(["eredo"]);
			break;
		case 40:	/* down */
			this.untick();
			if(this.scrolldown(3)){
				this.untick();
				this.redrawtext();
			}
			break;
		case 46:	/* delete */
			this.post(["intr", "del"]);
			break;
		case 112:	/* F1 */
		case 113:	/* F2 */
		case 114:	/* F3 */
			var mev = {
				fakex: this.lastx,
				fakey: this.lasty,
				which: key-112+1,
			};
			mev.preventDefault = function(){}
			this.onmousedown(mev);
			break;
		default:
			return true;
		}
		return false;
	}catch(ex){
		console.log("keydown: " + ex);
	}
}

function tapply(ev) {
	if(!ev || !ev.Args || !ev.Args[0]){
		console.log("apply: nil ev");
		return;
	}
	var arg = ev.Args
	if(0)console.log(this.divid, "apply", ev.Args, "v", ev.Vers);
	switch(arg[0]){
	case "eins":
		if(arg.length < 3){
			console.log(this.divid, "apply: short ins");
			break;
		}
		var p0 = parseInt(arg[2]);
		var op0 = this.p0;
		var op1 = this.p1;
		this.p0 = p0;
		this.p1 = p0;
		this.tinslines(arg[1]);
		if(op0 > p0)
			op0 += arg[1].length;
		if(op1 > p0)
			op1 += arg[1].length;
		if(ev.Vers)
			this.vers = ev.Vers;
		break;
	case "edel":
		if(arg.length < 3){
			console.log(this.divid, "apply: short del");
			break;
		}
		var p0 = parseInt(arg[1]);
		var p1 = parseInt(arg[2]);
		var op0 = this.p0;
		var op1 = this.p1;
		this.p0 = p0;
		this.p1 = p1;
		try{
			this.tdel();
		}catch(ex){
			console.log(this.divid, "apply: del: " + ex);
		}
		op0 = adjdel(op0, p0, p1);
		op1 = adjdel(op1, p0, p1);
		if(ev.Vers)
			this.vers = ev.Vers;
		break;
	case "ecut":
		try{
			this.tdel();
		}catch(ex){
			console.log(this.divid, "apply: cut: " + ex);
		}
		if(ev.Vers)
			this.vers = ev.Vers;
		break;
	case "reload":
		this.reloadoff = 0;
		this.tclear()
		break
	case "reloading":
		if(arg.length < 2){
			console.log(this.divid, "apply: short reloading");
			break;
		}
		this.lines.push({txt: arg[1], off: this.reloadoff, eol: true});
		this.reloadoff += arg[1].length;
		break
	case "reloaded":
		if(arg.length < 2){
			console.log(this.divid, "apply: short reloaded");
			break;
		}
		this.vers = parseInt(arg[1])
		this.mayresize();
		this.redrawtext();
		this.dump()
		break
	default:
		console.log("text: unhandled", arg[0]);
	}
}

function tclear() {
	this.vers = 0;
	this.nlines = 0;
	this.ln0 = 0;
	this.p0 = 0;
	this.p1 = 0;
	this.frsize = 0;
	this.frlines = 0;
	this.lines = [];
}

/*
	e is a canvas element with id and class already set,
	and .lines[] with initial text.
	d is is the (jquery) parent that will supply kbd events.
	cid is the class id for e.
 */
function mktext(d, e, cid, id) {
	var ctx=e.getContext("2d");
	e.vers = 0;
	e.nlines = 0;
	e.ln0 = 0;
	e.frsize = 0;
	e.frlines = 0;
	e.p0 = e.p1 = 0;
	e.fontstyle = 'r';
	checkoutfonts(ctx);
	e.tabwid = ctx.measureText("XXXX").width;
	e.fontht = 14; // TODO: use font height from fixfont
	e.buttons = 0;
	e.nclicks = {1: 0, 2: 0, 4: 0};

	e.drawline = tdrawline;
	e.dump = tdump;
	e.evxy = tevxy;
	e.fixfont = tfixfont;
	e.linewrap = tlinewrap;
	e.mayresize = tmayresize;
	e.posdx = tposdx;
	e.ptr2tpos = tptr2tpos;
	e.redrawtext = tredrawtext;
	e.reformat = treformat;
	e.scrolldown = tscrolldown;
	e.scrolldown1 = tscrolldown1;
	e.scrollup = tscrollup;
	e.scrollup1 = tscrollup1;
	e.setlinesoff = tsetlinesoff;
	e.tdel = ttdel;
	e.tget = ttget;
	e.tgetword = ttgetword;
	e.tins = ttins;
	e.tinslines = ttinslines;
	e.tpos2pos = ttpos2pos;
	e.tsetsel = ttsetsel;
	e.untick = tuntick;
	e.updatescrl = tupdatescrl;

	e.tkeydown = tkeydown;
	e.tkeypress = tevkey;
	e.tkeyup = tkeyup;

	e.onmousedown = tmdown;
	e.onmousemove = tevxy;
	e.onmouseup = tmup;
	e.onmousewheel = tmwheel;
	e.onpaste = function(){return false;}
	e.oncontextmenu = function(){return false;}
	e.onclick = null;
	e.onddblclick = null;
	e.tm1 = tm1;
	e.tm23 = tm23;
	e.tmdown = tmdown;
	e.tmpress = tmpress;
	e.tmrlse = tmrlse;
	e.tmup = tmup;
	e.tmwait = tmwait;

	e.fixfont();
	e.mayresize();
	e.redrawtext();

	e.tclear = tclear;

	d.keypress(function(ev){
		return e.tkeypress(ev);
	})
	.keyup(function(ev){
		return e.tkeyup(ev);
	})
	.keydown(function(ev){
		return e.tkeydown(ev);
	}).get(0).update = function(ev) {
		if(ev.Src == id){
			console.log("txt ignd update", ev.Args);
			return;
		}
		console.log("txt update", ev.Args);
		e.apply(ev);
	};
	var preht = d.height();
	e.divcid = cid;
	e.divid = id;
	e.apply = tapply;
	e.Post = function(args) {
		var ev = this.post(args);
		if(ev){
			try {
				this.apply(ev);
			}catch(ex){
				console.log("txt apply: " + ex);
			}
		}
	};
	e.post = function(args) {
		if(!e.ws){
			console.log("no ws");
			return nil;
		}
		var ws = e.ws;
		if(!args || !args[0]){
			console.log("post: no args");
			return nil;
		}
		var ev = {}
		ev.Id = cid;
		ev.Src = id;
		if(!this.vers)
			this.vers = 0;
		if(args[0] == "eins" || args[0] == "edel" || args[0] == "ecut" || args[0] == "epaste"){
			this.vers++;
		}
		ev.Vers = this.vers;
		ev.Args = args;
		var msg = JSON.stringify(ev);
		try {
			ws.send(msg);
			// console.log("posting ", msg);
		}catch(ex){
			console.log("post: " + ex);
		}
		return ev;
	};

	var wsurl = "wss://" + window.location.host + "/ws/" + cid;
	e.ws = new WebSocket(wsurl);
	e.ws.onopen = function() {
		e.post(["id"]);
	};
	e.ws.onmessage = function(ev) {
		// console.log("got msg", e.data);
		var o = JSON.parse(ev.data);
		if(!o || !o.Id) {
			console.log("update: no objet id");
			return;
		}
		//console.log("update to", o.Id);
		e.apply(o)
	};
	e.ws.onclose = function() {
		console.log("text socket " + wsurl+ " closed\n");
		var nd = document.open("text/html", "replace")
		nd.write("<b>disconnected</b>")
		nd.close();
	};
	d.resizable().on('resize', function() {
		console.log("resized");
		e.mayresize();
	});
}

document.mktext = mktext
