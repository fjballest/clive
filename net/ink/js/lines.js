"use strict";
/*
	text frame support
*/

/*
 * Hack to make sure the fixed and var width fonts exist, and
 * global font names for those variants.
 */
var tffixed = "monospace";
var tfvar = "Lucida Grande";	// or Verdana
var fontscheckedout = false;
var tdebug = false;

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

var wordre = null;
function iswordchar(c) {
	if(!wordre)
		wordre = /\w/;
	return wordre.test(c);
}

function islongwordchar(c) {
	if(!wordre)
		wordre = /\w/;
	return c == '-' || c == '(' || c == ')' || c == '/' || c == '.' || c == ':' || c == '#' || c == ',' || wordre.test(c);
}

function islparen(c) {
	return "([{<'`\"".indexOf(c) >= 0;
}

function isrparen(c) {
	return ")]}>'`\"".indexOf(c) >= 0;
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


function Line(lni, off, txt, eol) {
	this.lni = lni;
	this.off = off;
	this.txt = txt;
	this.eol = eol;
	this.next = null;
	this.prev = null;

	// not toString(), by intention.
	this.str = function() {
		if(this.eol) {
			return ""+this.off+"["+this.lni+"]"+" =\t[" + this.txt + "\\n]";
		} else {
			return ""+this.off+"["+this.lni+"]"+" =\t[" + this.txt + "]";
		}
	};

	// len counts the \n, this.txt.length does not.
	this.len = function() {
		if(this.eol) {
			return this.txt.length+1;
		}
		return this.txt.length;
	};

	this.split = function(lnoff, addnl) {
		var nln = new Line(this.lni+1, this.off+lnoff+1, "", this.eol);
		var lnlen = this.txt.length;
		if(lnoff < lnlen) {
			nln.txt = this.txt.slice(lnoff, lnlen);
			this.txt = this.txt.slice(0, lnoff);
		}
		this.eol = addnl;
		nln.next = this.next;
		if(nln.next) {
			nln.next.prev = nln;
		}
		nln.prev = this;
		this.next = nln;
	};

	this.join = function() {
		if(!this.next) {
			return;
		}
		this.txt += this.next.txt;
		this.eol = this.next.eol;
		this.next = this.next.next;
		if(this.next) {
			this.next.prev = this;
		}
	};

	this.ins = function(t, lnoff) {
		if(lnoff == this.txt.length) {
			this.txt += t;
		} else {
			this.txt = this.txt.slice(0, lnoff) +
				t + this.txt.slice(lnoff, this.txt.length);
		}
	};

	// does not del eol
	this.del = function(lnoff, n) {
		var lnlen = this.txt.length;
		if(lnoff+n > lnlen) {
			n = lnlen - lnoff;
		}
		if(n > 0) {
			this.txt = this.txt.slice(0,lnoff) + this.txt.slice(lnoff+n, lnlen);
		}
		return n;
	};

	this.delline = function() {
		if(this.prev) {
			this.prev.next = this.next;
		}
		if(this.next) {
			this.next.prev = this.prev;
		}
	};

	this.renumber = function() {
		for(var ln = this; ln != null; ln = ln.next) {
			if(ln.prev == null) {
				ln.off = 0;
				ln.lni = 0;
			} else {
				ln.off = ln.prev.off + ln.prev.len();
				ln.lni = ln.prev.lni+1;
			}
		}
	};
}

function Lines(els) {
	this.clear = function() {
		this.lns = new Line(0, 0, "", false);
		this.ln0 = this.lns;	// first line shown
		this.lne = this.lns;	// last line
		this.nrunes = 0;
		this.p0 = 0;
		this.p1 = 0;
		this.marks = [];	// of {name: mark, pos: p}
	};
	this.clear();
	this.tabstop = 4;

	// these must be redefined to draw the lines.
	this.untick = function(){};
	this.mayscrollins = function(ln){};
	this.mayscrolldel = function(ln){};
	this.scrolldown = function(n){ return 0;};
	this.scrollup = function(n){ return 0;};
	this.redrawtext = function(){};
	this.wrapoff = function(t){ return t.length; };
	this.frlninsdel = function(ln, ninsdel){};

	// pos0 is optional (0 by default).
	this.tabtxt = function(t, pos0) {
		if(t.indexOf('\t') < 0)
			return t;
		var s = "";
		var pos = 0;
		if(pos0) {
			pos = pos0;
		}
		for(var i = 0; i < t.length; i++){
			var r = t.charAt(i);
			if(r == '\t') {
				do {
					s += " ";
					pos++;
				}while(pos%this.tabstop);
			}else{
				pos++;
				s += r;
			}
		}
		return s;	
	};

	this.markins = function(p0, n) {
		for(var i = 0; i < this.marks.length; i++){
			var m = this.marks[i];
			if(m.pos > p0) {
				m.pos += n;
			}
		}
	};

	this.markdel = function(p0, p1) {
		for(var i = 0; i < this.marks.length; i++){
			var m = this.marks[i];
			if(m.pos <= p0) {
				continue;
			}
			var mp1 = p1;
			if(mp1 > m.pos) {
				mp1 = m.pos;
			}
			m.pos -= (mp1-p0);
		}
	};

	this.setmark = function(mark, p) {
		for(var i = 0; i < this.marks.length; i++){
			var m = this.marks[i];
			if(m.name == mark) {
				m.pos = p;
				return;
			}
		}
		this.marks.push({name: mark, pos: p});
	};

	this.getmark = function(mark) {
		for(var i = 0; i < this.marks.length; i++){
			var m = this.marks[i];
			if(m.name == mark) {
				return m;
			}
		}
		return null;
	};

	this.delmark = function(mark) {
		for(var i = 0; i < this.marks.length; i++){
			var m = this.marks[i];
			if(m.name == mark) {
				this.marks.splice(i, 1);
				break;
			}
		}
	}

	this.addln = function(ln) {
		ln.prev = this.lne;
		this.lne = ln;
		if(ln.prev) {
			ln.lni = ln.prev.lni+1;
			ln.off = ln.prev.off + ln.prev.len();
			ln.prev.next = ln;
		} else {
			ln.lni = 0;
			ln.off = 0;
			this.lns = ln;
			this.ln0 = ln;
		}
		this.nrunes += ln.len();
	};

	// seek a line (first is 0).
	this.seekln = function(pos) {
		var ln = this.lns;
		for(var ln = this.lns; ln; ln = ln.next) {
			if(pos-- <= 0) {
				return ln;
			}
		}
		return this.lns;
	};

	// return [line, off at line] or [null, 0]
	// if pos is at the end of a line, that line is returned,
	// and not the next line at 0.
	this.seek = function(pos) {
		for(var ln = this.lns; ln; ln = ln.next) {
			if(pos >= ln.off && pos <= ln.off + ln.txt.length) {
				return [ln, pos-ln.off];
			}
		}
		return [null, 0];
	};

	// return the pos for a seek
	this.seekpos = function(ln, lnoff) {
		if(ln == null) {
			return 0;
		}
		if(lnoff > ln.txt.length) {
			return ln.off + ln.len();
		}
		return ln.off + lnoff;
	};

	this.reformat = function(ln0) {
		var ctx = this.ctx;
		this.fixfont();
		if(tdebug) {
			var avail = this.c.width - this.marginsz;
			var ln0i = ln0?ln0.lni:-1;
			console.log("reformat ln " + ln0i + " wid " + avail + ":" );
			console.trace();
		}
		// TODO: should get an indication regarding at which
		// point it's safe to assume that no further reformat
		// work is needed and stop there.
		for(var ln = ln0; ln != null; ) {
			// merge text on the same line
			while(!ln.eol && ln.next != null) {
				if(ln.next == this.lne) {
					this.lne = ln;
				}
				if(ln.next == this.ln0) {
					this.ln0 = ln;
				}
				ln.join();
			}
			// remove empty lines but keep an empty line at the end.
			var next = ln.next;
			if(ln.len() == 0 && next) {
				if(this.lne == ln) {
					console.log("lines: reformat join bug?");
				}
				if(ln0 == ln) {
					ln0 = next;
				}
				if(this.ln0 == ln) {
					this.ln0 = next;
				}
				if(this.lns == ln) {
					this.lns = next;
				}
				ln.delline();
			}
			ln = next;
		}
		// recompute wraps, offsets, and numbers.
		for(var ln = ln0; ln != null; ln = ln.next) {
			if(!ln.prev) {
				ln.off = 0;
				ln.lni = 0;
			} else {
				ln.off = ln.prev.off + ln.prev.len();
				ln.lni = ln.prev.lni + 1;
			}
			var woff = this.wrapoff(ln.txt);
			if(woff < ln.txt.length) {
				if(tdebug) {
					console.log("wrap  off " + woff + " ln" + ln.str());
				}
				ln.split(woff, false);
				if(this.lne == ln) {
					this.lne = ln.next;
				}
			} else if(tdebug) {
				console.log("no wrap ln " + ln.str());
			}
		}
		// keep the empty line at the end
		if(this.lne.eol) {
			this.addln(new Line(0, 0, "", false));
		}
		// if ln0 moved to the end marker, backup if we can.
		if(!ln0.next && ln0.prev) {
			ln0 = ln0.prev;
		}
		if(tdebug) {
			console.log("after reformat:");
			this.dump();
		}
		return ln0;
	};

	// add a single line or a \n.
	this.ins1 = function(t, dontscroll) {
		this.untick();
		this.markins(this.p0, t.length);
		var xln, lnoff;
		[xln, lnoff] = this.seek(this.p0);
		if(!xln) {
			console.log("Lines.ins: no line for p0");
			return;
		}
		if(t == '\n') {
			xln.split(lnoff, true);
			if(this.lne === xln) {
				this.lne = xln.next;
			}
		} else {
			xln.ins(t, lnoff);
		}
		this.p0 += t.length;
		this.p1 = this.p0;
		this.nrunes += t.length;
		if(t != '\n') {
			var woff = this.wrapoff(xln.txt);
			if(woff == xln.txt.length) {
				// ins within a line, don't reformat; just redraw it.
				xln.renumber();
				this.frlninsdel(xln, +t.length);
				return;
			}
		}
		xln = this.reformat(xln);
		if(!dontscroll) {
			this.mayscrollins(xln);
		}
		this.redrawtext();
	};

	// add arbitrary text at p0
	this.ins = function(s, dontscroll) {
		var lns = s.split('\n');
		for(var i = 0; i < lns.length; i++) {
			if(lns[i].length > 0) {
				this.ins1(lns[i], dontscroll);
			}
			if(i < lns.length-1) {
				this.ins1('\n', dontscroll);
			}
		}
	};

	// del p0:p1 or last char if p0 == p1
	this.del = function(dontscroll) {
		if(this.p0 >= this.nrunes || this.p1 <= this.p0) {
			return;
		}
		this.untick();
		if(this.p0 > 0 && this.p0 == this.p1) {
			this.p0--;
		}
		this.markdel(this.p0, this.p1);
		var xln, lnoff;
		[xln, lnoff] = this.seek(this.p0);
		if(!xln) {
			console.log("lines: del: no line");
			return;
		}
		var ndel = this.p1 - this.p0;
		var tot = 0;
		var xln0 = xln;
		for(; tot < ndel && xln != null; xln = xln.next) {
			if(tdebug && 0) {
				console.log("lines del " + ndel + " loff " + lnoff + " " + xln.str());
			}
			var nd = xln.del(lnoff, ndel-tot);
			if(tot+nd < ndel && xln.eol) {
				xln.eol = false;
				nd++;
			}
			if(tot == 0 && nd == ndel && xln.eol) {
				// del within a line; don't reformat; redraw it.
				if(tdebug) {
					console.log("single line del");
				}
				this.nrunes -= nd;
				this.p1 -= nd;
				xln.renumber();
				this.frlninsdel(xln, -nd);
				return;
			}
			tot += nd;
			lnoff = 0;
		}
		var mightscroll = (this.p1 >= xln0.off);
		this.nrunes -= tot;
		this.p1 -= tot;
		if(xln0.prev) {
			xln0 = xln0.prev;
		}
		this.reformat(xln0);
		this.redrawtext();
		if(!dontscroll && mightscroll) {
			this.mayscrolldel(xln0);
		}
	};

	this.get = function(p0, p1) {
		if(p0 == p1 || p0 >= this.nrunes || p1 < p0 || p1 <= 0) {
			return "";
		}
		var ln0, lnoff;
		[ln0, lnoff] = this.seek(p0);
		if(ln0 == null) {
			return "";
		}
		var ln = ln0;
		var nget = p1 - p0;
		var off = p0 - ln.off;
		var tot = 0;
		var txt = "";
		do{
			var ng = nget-tot;
			if(off+ng > ln.txt.length) {
				ng = ln.txt.length - off;
			}
			txt += ln.txt.slice(off, off+ng);
			tot += ng;
			if(tot < nget && ln.eol){
				txt += "\n";
				tot++;
			}
			ln = ln.next;
			off = 0;
		}while(tot < nget && ln != null);
		return txt;
	};

	// returns [word, wp0, wp1]
	this.getword = function(pos, long) {
		if(pos < 0) {
			return ["", 0, 0];
		}
		if(pos >= this.nrunes) {
			return ["", this.nrunes, this.nrunes];
		}
		var ischar = iswordchar;
		if(long) {
			ischar = islongwordchar;
		}
		var ln, lnoff;
		[ln, lnoff] = this.seek(pos);
		if(ln == null) {
			ln = this.lne;
		}
		var epos = pos;
		var p0 = pos - ln.off;
		if(p0 == ln.txt.length){
			if(!ln.eol) {
				return [ln.txt, ln.off, ln.off+ln.txt.length];
			}
			return [ln.txt+"\n", ln.off, ln.off+ln.txt.length+1];
		}
		// heuristic: if click at the right of lparen and not
		// at rparen, use the lparen.
		if(p0 > 0 && !isrparen(ln.txt.charAt(p0)) &&
		   islparen(ln.txt.charAt(p0-1))){
			pos--;
			p0--;
		}
		var p1 = p0;
		var c = ln.txt.charAt(p0);
		if(islparen(c)){
			pos++;
			var n = 1;
			var rc = rparen(c);
			var txt = "";
			p1++;
			epos++;
			do {
				for(; p1 < ln.txt.length; p1++, epos++) {
					var x = ln.txt.charAt(p1);
					if(x == rc)
						n--;
					else if(x == c)
						n++;
					if(n != 0)
						txt += x;
					if(n == 0)
						return [txt, pos, epos-1];
				}
				if(ln.eol){
					epos++;
					txt += "\n";
				}
				ln = ln.next;
				p1 = 0;
			} while(n > 0 && ln != null);
			return [txt, pos, epos];
		}
		if(isrparen(c)){
			var n = 1;
			var lc = lparen(c);
			var txt = "";
			do{
				for(p0--; p0 >= 0; p0--){
					x = ln.txt.charAt(p0);
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
				ln = ln.prev;
				if(ln != null){
					p0 = ln.txt.length;
					if(ln.eol){
						pos--;
						txt = "\n" + txt;
					}
				}
			}while(n > 0 && ln != null);
			return [txt, pos, epos];
		}
		if(!islongwordchar(c))
			return [ln.txt.slice(p0, p1), pos, epos];
		while(p0 > 0 && ischar(ln.txt.charAt(p0-1))){
			pos--;
			p0--;
		}
		while(p1 < ln.txt.length && ischar(ln.txt.charAt(p1))){
			epos++;
			p1++;
		}
		return [ln.txt.slice(p0, p1), pos, epos];
	};

	this.dump = function() {
		var off = 0;
		var i = 0;
		for(var ln = this.lns; ln; ln = ln.next){
			var n = ln.len();
			var o = ln.off;
			if(!o && !(o === 0)){
				console.log("BAD off " + o + " in:");
				o = off;
			}
			if(o != off){
				console.log("BAD off " + o + " (!=" + off + ") in:");
				off = o;
			}
			off += n;
			console.log(""+ ln.str());
			i++;
		}
	};
}

// Lines that know how to draw using a canvas
function DrawLines(c) {
	Lines.apply(this, arguments);
	this.nlines = 0;	// lines in window
	this.frlines = 0;	// lines with text
	this.frsize = 0;	// nb. of runes in frame
	this.c = c;			// canvas, perhaps it's this.
	this.fontstyle = 'r';
	this.tabstop = 4;
	this.marginsz = 6;
	this.tscale = 4;	// scale must be even; we /2 without Math.floor
	this.secondary = 0;	// button for selection


	this.tickimg = undefined;	// tick image
	this.tickx = 0;
	this.ticky = 0;
	this.saved = undefined;	// saved image under tick

	var ctx = c.getContext("2d", {alpha: false});
	this.ctx = ctx;

	checkoutfonts(ctx);
	ctx.fillStyle = "#FFFFEA";
	var tabtext = Array(this.tabstop+1).join("X");

	this.tabwid = ctx.measureText(tabtext).width;
	// 14 pixels = 12pt font + 2pts of separation at the bottom,
	// but we scale the canvas *tscale.
	this.fontht = 14*this.tscale;

	this.fixfont = function() {
		var ctx = this.ctx;
		var mod = "";
		var style = "";
		style = tfvar;
		if(this.fontstyle.indexOf('r') === -1) {
			style = tffixed;
		}
		if(this.fontstyle.indexOf('b') > -1) {
			mod = "bold " + mod;
		}
		if(this.fontstyle.indexOf('i') > -1) {
			mod = "italic " + mod;
		}
		// at scale 1, we keep two empty pts at the bottom.
		var ht = this.fontht - 2*this.tscale;
		ctx.font = mod + " "  + ht+"px "+ style;
		ctx.textBaseline="top";
	};

	var oldclear = this.clear;
	this.clear = function() {
		oldclear.call(this);
		this.nlines = 0;
		this.frlines = 0;
		this.frsize = 0;
		this.saved = undefined;
		this.tickx = this.ticky = 0;
	};

	this.clearline = function(i) {
		var ctx = this.ctx;
		var pos = i*this.fontht;
		if(pos >= this.c.height) {
			return false;
		}
		ctx.clearRect(1, pos, this.c.width-1, this.fontht);
		return true;
	};

	this.mktick = function() {
		var ctx = this.ctx;
		var x = ctx.lineWidth;
		ctx.lineWidth = 1;
		var d = 3*this.tscale;
		ctx.fillRect(0, 0, d, d);
		ctx.fillRect(0, this.fontht-d, d, d);
		ctx.moveTo(d/2, 0);
		ctx.lineTo(d/2, this.fontht);
		ctx.stroke();
		ctx.lineWidth = x;
		this.tickimg = ctx.getImageData(0, 0, d, this.fontht);
	};

	this.untick = function() {
		if(!this.saved) {
			return;
		}
		var ctx = this.ctx;
		ctx.putImageData(this.saved, this.tickx, this.ticky);
		this.saved = undefined;
	};

	this.tick = function(x, y) {
		var ctx = this.ctx;
		if(0)console.log("tick", x, y);
		this.saved = ctx.getImageData(x, y, 3*this.tscale, this.fontht);
		this.tickx = x;
		this.ticky = y;
		ctx.putImageData(this.tickimg, x, y);
	};

	// draw a line and return false if it's out of the draw space.
	this.drawline = function(ln) {
		var ctx = this.ctx;
		var lnht = this.fontht;
		var avail = this.c.width - 2*this.marginsz - 1;
		var y = (ln.lni-this.ln0.lni)*lnht;
		if(y > this.c.height) {
			return false;
		}

		// non-empty selection.
		if(this.p0 != this.p1) {
			if(this.p0 > ln.off+ln.txt.length || this.p1 < ln.off){
				// unselected line
				ctx.clearRect(1, y, this.c.width-this.marginsz-1, lnht);
				var t = this.tabtxt(ln.txt);
				ctx.fillText(t, this.marginsz, y);
				return true;
			}
			// up to p0 unselected
			var dx = this.marginsz;
			var s0 = 0;
			var s0pos = 0;
			if(this.p0 > ln.off){
				s0 = this.p0 - ln.off;
				var s0t = this.tabtxt(ln.txt.slice(0, s0));
				s0pos = s0t.length;
				dx += ctx.measureText(s0t).width;
				ctx.clearRect(1, y, dx, lnht);
				ctx.fillText(s0t, this.marginsz, y);
			}
			// from p0 to p1 selected
			var s1 = ln.txt.length - s0;
			if(this.p1 < ln.off+ln.txt.length)
				s1 = this.p1 - s0 - ln.off;
			var s1t = this.tabtxt(ln.txt.slice(s0, s0+s1), s0pos);
			var s1pos = s0pos + s1t.length;
			var sx = ctx.measureText(s1t).width;
			var old = ctx.fillStyle;
			if(this.secondary >= 2) {
				ctx.fillStyle = "#FF7575";
			} else if(this.secondary) {
				ctx.fillStyle = "#7373FF";
			} else {
				ctx.fillStyle = "#D1A0A0";
			}
			if(this.p1 > ln.off+ln.txt.length) {
				ctx.fillRect(dx, y, this.c.width-dx-this.marginsz-1, lnht);
			} else {
				ctx.fillRect(dx, y, sx, lnht);
			}
			ctx.fillStyle = old;
			ctx.fillText(s1t, dx, y);
			if(this.p1 > ln.off+ln.txt.length) {
				return true;
			}
			// from p1 unselected
			ctx.clearRect(dx+sx, y, this.c.width-(dx+sx)-this.marginsz-1, lnht);
			if(s1 >= ln.txt.length) {
				return true;
			}
			var s2t = this.tabtxt(ln.txt.slice(s0+s1, ln.txt.length), s1pos);
			ctx.fillText(s2t, dx+sx, y);
			return true;
		}

		// unselected line
		ctx.clearRect(1, y, this.c.width-this.marginsz-1, lnht);
		var t = this.tabtxt(ln.txt);
		ctx.fillText(t, this.marginsz, y);

		if(this.p0 < ln.off || this.p0 > ln.off + ln.txt.length) {
			return true;
		}

		// line with tick
		var x = this.posdx(ln.txt, this.p0 - ln.off);
		x += this.marginsz - 3*this.tscale/2;	// a bit to the left
		this.tick(x, y);
		return true;
	};

	this.updatescrl = function() {
		var ctx = this.ctx;
		var y0 = this.ln0.lni / this.lne.lni * this.c.height;
		var dy = this.frlines / this.lne.lni * this.c.height;
	
		ctx.clearRect(this.c.width-this.marginsz, 0, this.marginsz, y0);
		var old = ctx.fillStyle;
		ctx.fillStyle = "#7373FF";
		ctx.fillRect(this.c.width-this.marginsz, y0, this.marginsz, dy);
		ctx.fillStyle = old;
		ctx.clearRect(this.c.width-this.marginsz, y0+dy,
			this.marginsz, this.c.height-(y0+dy));
	};

	this.redrawtext = function() {
		this.fixfont();
		this.nlines = Math.floor(this.c.height/this.fontht);
		if(!this.tickimg) {
			this.mktick();
		}
		if(!this.ln0) {
			console.log("redrawtext: no ln0");
			return;
		}
		var froff = this.ln0.off;
		this.frsize = 0;
		this.frlines = 0;
		var ln = this.ln0;
		for(var i = 0; i <= this.nlines; i++){
			if(ln != null){
				if(!this.drawline(ln))
					break;
				this.frlines++;
				this.frsize += ln.len();
				ln = ln.next;
			}else if(!this.clearline(i)) {
					break;
			}
		}
		if(tdebug)console.log("redraw " + i + " " + this.nlines);
		this.updatescrl();
	};

	// requires a redraw if returns true.
	this.scrolldown = function(n) {
		var old = this.ln0;
		for(; n > 0; n--) {
			if(!this.ln0.prev) {
				break;
			}
			this.ln0 = this.ln0.prev;
		}
		return this.ln0 != old;
	};

	// requires a redraw if returns true.
	this.scrollup = function(n) {
		var old = this.ln0;
		for(; n > 0; n--) {
			if(!this.ln0.next || !this.ln0.next.next) {
				break;
			}
			this.ln0 = this.ln0.next;
		}
		return old != this.ln0;
	};

	this.nscrl = function() {
		var nscrl = Math.floor(this.nlines/4);
		if(nscrl > 0) {
			return nscrl;
		}
		return 1;
	};

	this.mayscrollins = function(ln) {
		if(ln.lni >= this.ln0.lni+this.nlines-1 &&
		   ln.lni <= this.ln0.lni+this.nlines+1 && this.nlines > 1) {
			this.scrolldown(this.nscrl());
		}
	};

	this.mayscrolldel = function(ln) {
		if(this.p0 < this.ln0.off) {
			this.scrollup(this.nscrl());
			this.redrawtext();
		}
	};

	this.wrapoff = function(t) {
		var ctx = this.ctx;
		var avail = this.c.width - this.marginsz;
		var pos = 0;
		var s = "";
		if(tdebug) {
			console.log("wrapoff: X wid: " + ctx.measureText("X").width);
		}
		for(var i = 0; i < t.length; i++){
			var r = t.charAt(i);
			if(r == '\t') {
				do {
					s += " ";
					pos++;
				}while(pos%this.tabstop);
			}else{
				pos++;
				s += r;
			}
			if(ctx.measureText(s).width > avail){
				if(tdebug) {
					console.log('wrapoff: ' + s + ': wrap: ' + ctx.measureText(s).width + " " + avail);
				}
				return i;
			}
		}
		if(tdebug) {
			console.log('wrapoff: ' + s + ': no wrap: ' + ctx.measureText(s).width + " " + avail);
		}
		return t.length;
	};

	this.posdx = function(t, n) {
		var ctx = this.ctx;
		var pos = 0;
		var dx = 0;
		var spcwid = ctx.measureText(" ").width;
		for(var i = 0; i < t.length && i < n; i++){
			var r = t.charAt(i);
			if(r == '\t') {
				do {
					dx += spcwid;
					pos++;
				}while(pos%this.tabstop);
			}else{
				pos++;
				dx += ctx.measureText(r).width;
			}
		}
		return dx;
	};

	// returns [line, off at line, click past text?]
	// later you can use seekpos(line, lnoff) to get a valid pos.
	this.ptr2seek = function(cx, cy) {
		var marginsz = Math.floor(this.marginsz/2);
		var x = cx;
		var y = cy;
		var ovf = 0;
		x *= this.tscale;
		y *= this.tscale;
		var nln = Math.floor(y/this.fontht);
		if(nln < 0) {
			return [this.ln0, 0, false];
		}
		if(nln >= this.frlines) {		// overflow
			return [this.lne, this.lne.txt.length, true];
		}
		var ln = this.ln0;
		while(nln-- > 0 && ln.next) {
			ln = ln.next;
		}
		var pos = 0;
		for(; pos <= ln.txt.length; pos++){
			var coff = this.posdx(ln.txt, pos);
			if(coff+marginsz > x){
				if(pos > 0)
					pos--;
				break;
			}
		}
		if(pos > ln.txt.length){
			pos = ln.txt.length;
			return [ln, pos, true];
		}
		return [ln, pos, false];
	};

	this.viewsel = function() {
		if(this.p0 >= this.ln0.off && this.p0 <= this.ln0.off+this.frsize) {
			return;
		}
		for(var ln = this.lns; ln != null; ln = ln.next) {
			if(this.p0 >= ln.off && this.p0 <= ln.off+ln.txt.length) {
				for(var n = Math.floor(this.frlines/3); n > 0 && ln.prev; n--) {
					ln = ln.prev;
				}
				this.ln0 = ln;
				this.redrawtext();
				break;
			}
		}
	};

	this.setsel = function(p0, p1, refreshall) {
		var ctx = this.ctx;
		if(p0 > this.nrunes) {
			p0 = this.nrunes;
		}
		if(p1 < p0) {
			p1 = p0;
		}
		if(p1 > this.nrunes) {
			p1 = this.nrunes;
		}
		if(this.p0 != this.p1) {
			refreshall = true;
		}
		var froff = this.ln0.off;
		if(refreshall && (this.p1 <froff || this.p0 >froff+this.frsize))
			refreshall = false;
		var mp0 = p0;
		var mp1 = p1;
		if(refreshall){
			if(this.p0 < mp0) {
				mp0 = this.p0;
			}
			if(this.p1 > mp1) {
				mp1 = this.p1;
			}
		}
		this.p0 = p0;
		this.p1 = p1;
		this.untick();
		if(mp1 <froff || mp0 >froff+this.frsize) {
			return;
		}
		var insel = false;
		var ln = this.ln0;
		for(var i = 0; i < this.frlines && ln != null; i++){
			if(mp1 >= ln.off && mp0 <= ln.off+ln.txt.length) {
				insel=true;
			}
			if(insel) {
				this.drawline(ln);
			}
			if(mp1 < ln.off) {
				break;
			}
			ln = ln.next;
		}
	};

	this.frlninsdel = function(ln, ninsdel){
		if(ln.lni >= this.ln0.lni && ln.lni < this.ln0.lni+this.frlines) {
			this.frsize += ninsdel;
			this.drawline(ln);
		}
	};

	this.fixfont();
}
