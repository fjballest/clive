"use strict";
/*
	Clive js code for text frames.
	similar to Plan 9 text frames using the HTML5 canvas.

	HTML5 designers suggest that you don't do this, but on the other
	hand, they do NOT handle text correctly in dom and they do NOT
	provide the interfaces required to handle things like undo and
	redo correctly. 

	This requires also lines.js.
	The code interfacing with pg.js needs a rewrite, as does pg.js itself.
*/

var selecting = false;
var tdebug=false;

// This is to prevent the event from being propagated to the parent
// container.
// Despite this, it seems that if we return true in safari for a keydown
// then it's too late and the space bubbles and we scroll when we shouldnt.
// So, locknkeydown returns false and calls, by hand, the down/key/up handlers.
function dontbubble(e) {
	if (e) {
		e.bubbles = false;
		if(e.stopPropagation) {
			e.stopPropagation();
		}
		e.cancelBubble = true;
	}
}

// A frame of lines using the Clive ink framework.
// d is the div, c is the canvas, cid and id are the ink ids.
// This will have to be rewritten when we rewrite ink js code.
function CliveText(d, c, cid, id) {
	DrawLines.call(this, c);
	this.d = d;
	this.c = c;
	this.cid = cid;
	this.id = id;

	this.vers = 0;
	this.noedits = false;

	this.islocked = false;
	this.locking = false;
	this.mustunlock = false;
	this.whenlocked = [];

	this.buttons = 0;
	this.nclicks = {1: 0, 2: 0, 4: 0};
	this.lastx = 0;
	this.lasty = 0;
	this.dblclick = 0; // 1 for double, 2 for triple, ...
	this.secondary = 0;	// button for selection (also defined by DrawLines)
	this.secondaryabort = false;
	this.malt = false;
	this.userresized = false;
	this.selecting = false;
	this.oldp0 = -1;
	this.oldp1 = -1;
	this.clicktime = new Date().getTime();

	this.markinsdata = undefined;	// will be defined during markins
	this.einsdata = undefined;	// will be defined during eins
	this.reloadln0 = 0;

	this.composing = false;
	this.latin = "";

	var self = this;	// we rewrite handlers later, and use self.

	this.mrlse = function(e) {
		var b = 1<<(e.which-1);
		if(b == 1 && this.malt){
			b = 2;
			this.buttons &= ~1;
			this.malt = false;
		}
		this.buttons &= ~b;
		return b;
	};

	this.mpress = function(e) {
		var b = 1<<(e.which-1);
		if(b == 1 && e.altKey){
			b = 2;
			this.malt = true;
		}
		this.buttons |= b;
		return b;
	};

	// set lastx, lasty to ev coords relative to canvas
	this.evxy = function(e) {
		var x = 0;
		var y = 0;
		if(e.fakex != undefined) {
			x = e.fakex;
			y = e.fakey;
		} else {
			var poff = $(this.c).offset();
			x = e.pageX - poff.left;
			y = e.pageY - poff.top;
		}
		this.lastx = x;
		this.lasty = y;
	};

	this.mayresize = function(user) {
		var c = $(this.c);
		var p = c.parent();
		var dx = p.width();
		var dy = p.height() - 5;	// -5: leave a bit of room
		if(tdebug)console.log('mayresize: text resized dx ' + dx + " dy " + dy + " " + user?"user":"win");
		// TODO: use helper when we rewrite ink js.
		var tag = $("#"+this.id+"t")
		if(tag) {
			dy -= tag.height();
		}
		// Using a width scaled and making the style use the width
		// makes the text better.
		c.width(dx);
		c.height(dy);
		this.c.width = this.tscale*dx;
		this.c.height = this.tscale*dy;
		this.nlines = Math.floor(this.c.height/this.fontht);
		this.saved = null;
		this.reformat(this.lns);
		this.redrawtext();
		
	};

	// this is just a bunch of heuristics to make it feel ok.
	this.autoresize = function(addsize, moreless) {
		var p = $(this.c);
		var oldht = p.height();
		var ht = oldht;
		var fontht = this.fontht/this.tscale;
		if(addsize) {
			this.userresized = true;
			if(moreless > 1){
				var wtop = $(window).scrollTop();
				var etop = p.offset().top;
				var eoff = etop-wtop;
				if(tdebug)console.log("resize ", wtop, etop, eoff);
				ht = window.innerHeight - 10 - eoff; // -10: leave some room
			} else if(moreless >= 0) {
				ht += fontht*6;
			} else {
				ht -= fontht*6;
				if(ht < 5*fontht) {
					ht = 5*fontht;
				}
			}
		}else{
			var nln = this.frlines;
			if(nln < 3) {
				nln = 3;
			}
			ht = (nln+2) * fontht;
			if (ht >= 400) {	// some initial arbitrary space.
				ht = 400;
			}
		}
		if(tdebug)console.log("auto rsz", nln, ht, oldht);
		if (oldht < ht - fontht || oldht > ht + fontht) {
			if(tdebug)console.log("auto resizing");
			var delta = ht - oldht;
			p = p.parent();
			var nht = p.height() + delta;
			p.height(nht);
			this.mayresize(false);
		}
	};

	this.selectstart = function() {
		if(!this.selecting) {
			if(tdebug)console.log("selecting...");
		}
		this.selecting = true;
		selecting = true;
		this.oldp0 = this.p0;
		this.oldp1 = this.p1;
	};

	this.selectend = function() {
		if(this.mustunlock) {
			this.unlocked();
		}
		if(!this.selecting) {
			return;
		}
		if(tdebug)console.log("select end");
		if(this.oldp0 != this.p0 || this.oldp1 != this.p1) {
			this.post(["tick", ""+this.p0, ""+this.p1]);
			this.oldp0 = this.p0;
			this.oldp1 = this.p1;
		}
		this.selecting = false;
		selecting = false;
	};

	this.adjdel = function(pos, delp0, delp1) {
		if(pos <= delp0)
			return pos;
		if(pos <= delp1)
			return delp0;
		return pos - (delp1 - delp0);
	};
	
	this.apply = function(ev, fromserver) {
		if(!ev || !ev.Args || !ev.Args[0]){
			console.log("apply: nil ev");
			return;
		}
		var arg = ev.Args
		if(tdebug && arg[0] != "reloading") {
			console.log(this.id, "apply", ev.Args, "v", ev.Vers, this.vers);
		}
		switch(arg[0]){
		case "held":
			this.locked();
			break;
		case "rlse":
			if(this.selecting) {
				this.mustunlock = true;
				break;
			}
			this.unlocked();
			break;
		case "noedits":
			this.noedits = true;
			break;
		case "edits":
			this.noedits = false;
			break;
		case "clean":
			if(document.setclean) {
				document.setclean(this);
			}
			break;
		case "dirty":
			if(document.setdirty) {
				document.setdirty(this);
			}
			break;
		case "show":
			if(document.showcontrol) {
				document.showcontrol(this);
			}
			break;
		case "tag":
			if(arg.length < 2){
				console.log(this.id, "apply: short tag");
				break;
			}
			if(document.settag) {
				document.settag(this, arg[1]);
			}
			break;
		case "font":
			if(arg.length < 2){
				console.log(this.id, "apply: short font");
				break;
			}
			console.log(this.id, "font", arg[1]);
			this.fontstyle = arg[1];
			this.fixfont();
			this.reformat(this.lns);
			this.redrawtext();
			break;
		case "markinsing":
			if(arg.length < 3){
				console.log(this.id, "apply: short markinsing");
				break;
			}
			if (!this.markinsdata) {
				console.log("markins evs...");
				this.markinsdata = [];
			}
			this.markinsdata.push(arg[2]);
			break;
		case "markinsdone":
			if(tdebug)console.log("markins run...");
			if(arg.length < 2){
				console.log(this.id, "apply: short markinsdone");
				break;
			}
			var m = this.getmark(arg[1]);
			if(!m) {
				console.log(this.id, "apply: no mark", arg[1]);
				break;
			}
			var op0 = this.p0;
			var op1 = this.p1;
			if(op0 != op1) {
				this.setsel(op0, op0, false);
			}
			for(var i = 0; i < this.markinsdata.length; i++) {
				var data = this.markinsdata[i];
				var nlen = data.length;
				var npos = m.pos + nlen;
				var opos = m.pos;
				op0 = this.p0;
				op1 = this.p1;
				this.p0 = m.pos;
				this.p1 = m.pos;
				this.ins(data, true);
				m.pos = npos;
				if(op0 > opos)
					op0 += nlen;
				if(op1 > opos)
					op1 += nlen;
				this.p0 = op0;
				this.p1 = op1;
				if(ev.Vers) {
					this.vers = ev.Vers;
				}
			}
			this.setsel(op0, op1, false);
			delete this.markinsdata;
			if(!this.userresized) {
				this.autoresize();
			} 
			if(tdebug)console.log(this.id, "markins done");
			break;
		case "einsing":
			if(arg.length < 2){
				console.log(this.divid, "apply: short einsing");
				break;
			}
			if (!this.einsdata) {
				console.log("eins evs...");
				this.einsdata = [];
			}
			this.einsdata.push(arg[1]);
			break;
		case "einsdone":
			if(tdebug)console.log(this.id, "eins run...");
			if(arg.length < 2){
				console.log(this.id, "apply: short ins");
				break;
			}
			if(ev.Vers && fromserver && ev.Vers != this.vers+1){
				console.log("OUT OF SYNC", ev.Args, "v", ev.Vers, this.vers);
				this.post(["needreload"]);
				delete this.einsdata;
				break;
			}
			var p0 = parseInt(arg[1]);
			var op0 = this.p0;
			var op1 = this.p1;
			if(op0 != op1) {
				this.setsel(op0, op0, false);
			}
			this.p0 = p0;
			this.p1 = p0;
			for(var i = 0; i < this.einsdata.length; i++) {
				var data = this.einsdata[i];
				this.ins(data, false);
				if(op0 > p0)
					op0 += data.length;
				if(op1 > p0)
					op1 += data.length;
			}
			delete this.einsdata;
			this.setsel(op0, op1, false);
			if(ev.Vers) {
				this.vers = ev.Vers;
			}
			if(!this.userresized) {
				this.autoresize();
			} 
			if(tdebug)console.log(this.id, "eins done");
			break;
		case "eins":
			if(arg.length < 3){
				console.log(this.id, "apply: short ins");
				break;
			}
			if(ev.Vers && fromserver && ev.Vers != this.vers+1){
				console.log("OUT OF SYNC", ev.Args, "v", ev.Vers, this.vers);
				this.post(["needreload"]);
				break;
			}
			var p0 = parseInt(arg[2]);
			var op0 = this.p0;
			var op1 = this.p1;
			if(op0 != op1) {
				this.setsel(op0, op0);
			}
			this.p0 = p0;
			this.p1 = p0;
			this.ins(arg[1], false);
			if(op0 > p0)
				op0 += arg[1].length;
			if(op1 > p0)
				op1 += arg[1].length;
			if(fromserver) {
				this.setsel(op0, op1, false);
			}
			if(ev.Vers) {
				this.vers = ev.Vers;
			}
			if(!this.userresized && arg[1].indexOf('\n') >= 0) {
				this.autoresize();
			} 
			break;
		case "edel":
			if(arg.length < 3){
				console.log(this.id, "apply: short del");
				break;
			}
			if(ev.Vers && fromserver && ev.Vers != this.vers+1){
				console.log("OUT OF SYNC", ev.Args, "v", ev.Vers, this.vers);
				this.post(["needreload"]);
			}
			var p0 = parseInt(arg[1]);
			var p1 = parseInt(arg[2]);
			var op0 = this.p0;
			var op1 = this.p1;
			this.p0 = p0;
			this.p1 = p1;
			try{
				this.del(false);
			}catch(ex){
				console.log(this.divid, "apply: del: " + ex);
			}
			op0 = this.adjdel(op0, p0, p1);
			op1 = this.adjdel(op1, p0, p1);
			if(fromserver) {
				this.setsel(op0, op1, false);
			}
			if(ev.Vers) {
				this.vers = ev.Vers;
			}
			break;
		case "ecut":
			try{
				this.del(false);
			}catch(ex){
				console.log(this.id, "apply: cut: " + ex);
			}
			if(ev.Vers)
				this.vers = ev.Vers;
			break;
		case "reload":
			this.reloadln0 = this.ln0.lni;
			this.clear();
			if(tdebug) {
				console.log("cleared", this);
				this.dump();
			}
			break;
		case "reloading":
			if(arg.length < 2){
				console.log(this.id, "apply: short reloading");
				break;
			}
			var nln = new Line(0, 0, arg[1], true);
			var logit = (tdebug && (!this.lns || !this.lns.next))
			this.addln(nln);
			if(logit) {
				console.log("reloading", this);
				this.dump();
			}
			break
		case "reloaded":
			if(arg.length < 2){
				console.log(this.id, "apply: short reloaded");
				break;
			}
			this.vers = parseInt(arg[1]);
			if(this.reloadln0) {
				this.ln0 = this.seekln(this.reloadln0);
				this.reloadln0 = 0;
				if(!this.ln0) {
					this.ln0 = this.lns;
				}
			}
			this.reformat(this.lns);
			this.redrawtext();
			if(!this.userresized) {
				this.autoresize();
			}
			break;
		case "mark":
			if(arg.length < 3){
				console.log(this.id, "apply: short mark");
				break;
			}
			var pos = parseInt(arg[2]);
			this.setmark(arg[1], pos);
			break;
		case "sel":
			if(arg.length < 3){
				console.log(this.id, "apply: short sel");
				break;
			}
			var pos0 = parseInt(arg[1]);
			var pos1 = parseInt(arg[2]);
			this.setmark("p0", pos0);
			this.setmark("p1", pos1);
			this.setsel(pos0, pos1, true);
			this.viewsel();
			if(tdebug)console.log("setsel", pos0, pos1);
			break;
		case "delmark":
			if(arg.length < 2){
				console.log(this.divid, "apply: short delmark");
				break;
			}
			this.delmark(arg[1]);
			break;
		case "close":
			this.ws.close();
			$("#"+this.id).remove();
			break;
		default:
			console.log("text: unhandled", arg[0]);
		}
	};

	this.Post = function(e) {
		var ev = this.post(e);
		if(ev){
			try {
				this.apply(ev);
			}catch(ex){
				console.log("txt apply: " + ex);
			}
		}
	};

	// Only the frame with the lock may change the text,
	// we replace the handlers to gain the lock before actually
	// doing anything.

	this.tkeydown = function(e, deferred) {
		var key = e.keyCode;
		if(!e.keyCode)
			key = e.which;
		var rune = String.fromCharCode(e.keyCode);
		e.stopPropagation();
		if(tdebug) {
			console.log("keydown which " + e.which + " key " + e.keyCode +
				" '" + rune + "'" +
				" " + e.ctrlKey + " " + e.metaKey);
		}
		switch(key){
		case 27:	/* escape */
			if(deferred) {
				break;
			}
			this.post(["intr", "esc"]);
			this.dump();
			console.log("sel = ["+this.p0+","+this.p1+"] = '" +
				this.get(this.p0, this.p1) + "'");
			break;
		case 8:		/* backspace */
			if(this.noedits) {
				return;
			}
			if(deferred) {
				break;
			}
			if(this.p0 != this.p1){
				this.Post(["edel", ""+this.p0, ""+this.p1]);
			}else if(this.p0 > 0){
				var p0 = this.p0-1;
				this.Post(["edel", ""+p0, ""+this.p1]);
			}
			break;
		case 9:		/* tab */
			if(this.noedits) {
				return;
			}
			if(deferred) {
				break;
			}
			if(this.p0 != this.p1){
				this.Post(["edel", ""+this.p0, ""+this.p1]);
			}
			this.Post(["eins", "\t", ""+this.p0]);
			break;
		case 32:	/* space */
			if(deferred) {
				break;
			}
			this.Post(["eins", " ", ""+this.p0]);
			break;
		case 37:	/* left */
			if(this.noedits) {
				return;
			}
			if(deferred) {
				break;
			}
			this.post(["eundo"]);
			break;
		case 38:	/* up */
			if(deferred) {
				break;
			}
			var n = Math.floor(this.frlines/4);
			if(n < 1) {
				n = 1;
			}
			if(this.scrollup(n)){
				this.untick();
				this.redrawtext();
			}
			break;
		case 39:	/* right */
			if(this.noedits) {
				return;
			}
			if(deferred) {
				break;
			}
			this.post(["eredo"]);
			break;
		case 40:	/* down */
			if(deferred) {
				break;
			}
			this.untick();
			var n = Math.floor(this.frlines/4);
			if(n < 1) {
				n = 1;
			}
			if(this.scrolldown(n)){
				this.untick();
				this.redrawtext();
			}
			break;
		case 46:	/* delete */
			if(deferred) {
				break;
			}
			this.post(["intr", "del"]);
			break;
		case 112:	/* F1 */
		case 113:	/* F2 */
		case 114:	/* F3 */
		case 115:	/* F4 */
			if(deferred) {
				break;
			}
			var mev = {
				fakex: this.lastx,
				fakey: this.lasty,
				which: key-112+1,
			};
			mev.preventDefault = function(){}
			this.c.onmousedown(mev);
			break;
		case 123:	/* F12 */
			tdebug = !tdebug;
			break;
		default:
			return true;
		}
		return false;
	};

	this.tlocknkeydown = function(e) {
		dontbubble(e);
		if(this.islocked) {
			return this.tkeydown(e);
		}
		if(!this.locking) {
			this.locking = true;
			this.post(["hold"]);
			console.log("holding...");
		}
		var self = this;
		var xe = jQuery.Event("keydown");
		xe.which = e.which;
		xe.keyCode = e.keyCode;
		xe.ctrlKey = e.ctrlKey;
		xe.metaKey = e.metaKey;
		xe.preventDefault = function(){};
		this.whenlocked.push(function() {
			console.log("held keydown");
			$(self.c).trigger(xe);
			return false;
		});
		return this.tkeydown(e, true);
	};

	this.tkeypress = function(e, deferred) {
		var key = e.keyCode;
		if(!e.keyCode)
			key = e.which;
		var rune = String.fromCharCode(e.keyCode);
		if(tdebug) {
			console.log("key: which " + e.which + " key " + e.keyCode +
				" '" + rune + "'");
		}
		switch(key) {
		case 9:
			rune = "\t";
			break;
		case 13:
			rune = "\n";
			break;
		}
		switch(rune) {
		case 'c':
		case 'C':
			if(deferred)
				break;
			if(e.ctrlKey || e.metaKey) {
				e.preventDefault();
				this.post(["ecopy", ""+this.p0, ""+this.p1]);
				return false;
			}
			break;
		case 'v':
		case 'V':
			if(deferred || this.noedits) {
				break;
			}
			if(e.ctrlKey || e.metaKey) {
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
			if(deferred || this.noedits) {
				break;
			}
			if(e.ctrlKey || e.metaKey) {
				e.preventDefault();
				this.Post(["ecut", ""+this.p0, ""+this.p1]);
				return false;
			}
			break;
		}
		if(deferred || e.metaKey || e.ctrlKey || this.noedits) {
			return;
		}
		if(this.p0 != this.p1){
			this.Post(["edel", ""+this.p0, ""+this.p1]);
		}
		if(this.composing) {
			if(!this.latin) {
				this.latin = "" + rune;
			} else {
				this.latin += rune;
			}
			if(!kmap.islatin(this.latin)) {
				this.composing = false;
				rune = this.latin;
				this.latin = "";
			} else {
				var r = kmap.latin(this.latin);
				if (!r) {
					return;
				}
				this.composing = false;
				rune = r;
				this.latin = "";
			}
		}
		this.Post(["eins", rune, ""+this.p0]);
	};

	this.tlocknkeypress = function() {
		dontbubble(e);
		if(this.islocked) {
			return this.tkeypress(e);
		}
		if(!this.locking) {
			this.locking = true;
			this.post(["hold"]);
			console.log("holding...");
		}
		var self = this;
		var xe = jQuery.Event("keypress");
		xe.which = e.which;
		xe.keyCode = e.keyCode;
		xe.ctrlKey = e.ctrlKey;
		xe.metaKey = e.metaKey;
		xe.preventDefault = function(){};
		this.whenlocked.push(function() {
			console.log("held keypress");
			$(self.c).trigger(xe);
			return false;
		});
		return this.tkeypress(e, true);
	};

	this.tkeyup = function(e, deferred) {
		var key = e.keyCode;
		if(!e.keyCode)
			key = e.which;
		var rune = String.fromCharCode(e.keyCode);
		var isdeadkey = e && e.originalEvent &&
				e.originalEvent.keyIdentifier == "Unidentified";
		if(tdebug) {
			var ds = (isdeadkey ? " dead" : "");
			console.log("keyup which " + e.which + " key " + e.keyCode +
				" '" + rune + "'" + ds +
				" " + e.ctrlKey + " " + e.metaKey, e);
		}
		switch(key){
		case 112:	/* F1 */
		case 113:	/* F2 */
		case 114:	/* F3 */
		case 115:	/* F4 */
			if(deferred) {
				break;
			}
			var mev = {
				fakex: this.lastx,
				fakey: this.lasty,
				which: key-112+1,
			};
			mev.preventDefault = function(){}
			this.c.onmouseup(mev);
			break;
		case 18: /* Alt */
			this.composing = true;
			return true;
		default:
			return true;
		}
		return false;
	};

	this.tlocknkeyup = function() {
		dontbubble(e);
		if(this.islocked) {
			return this.tkeyup(e);
		}
		if(!this.locking) {
			this.locking = true;
			this.post(["hold"]);
			console.log("holding...");
		}
		var self = this;
		var xe = jQuery.Event("keyup");
		xe.which = e.which;
		xe.keyCode = e.keyCode;
		xe.ctrlKey = e.ctrlKey;
		xe.metaKey = e.metaKey;
		xe.preventDefault = function(){};
		this.whenlocked.push(function() {
			console.log("held keyup");
			$(self.c).trigger(xe);
			return false;
		});
		return this.tkeyup(e, true);
	};

	this.tmdown = function(e) {
		if(tdebug)console.log("tmdown ", this.id, e);
		this.selectstart();
		e.preventDefault();
		this.secondary = 0;		/* paranoia: see tm234 */
		this.secondaryabort = false;
		this.mpress(e);
		this.evxy(e);
		var b = this.buttons;
		switch(b){
		case 1:
			var ln, lnoff, past;
			[ln, lnoff, past] = this.ptr2seek(this.lastx, this.lasty);
			var pos = this.seekpos(ln, lnoff);
			this.setsel(pos, pos);
			this.m1(pos);
			break;
		case 2:
		case 4:
		case 8:
			var ln, lnoff, past;
			[ln, lnoff, past] = this.ptr2seek(this.lastx, this.lasty);
			var pos = this.seekpos(ln, lnoff);
			this.oldp0 = this.p0;
			this.oldp1 = this.p1;
			this.setsel(pos, pos);
			this.m234(pos, b);
			break;
		default:
			this.mwait();
		}
		e.returnValue = false;
	};

	this.tlocknmdown = function(e) {
		if(this.islocked) {
			return this.tmdown(e);
		}
		if(!this.locking) {
			this.locking = true;
			this.post(["hold"]);
			console.log("holding...");
		}
		var self = this;
		var xe = jQuery.Event("mousedown");
		xe.which = e.which;
		xe.pageX = e.pageX;
		xe.pageY = e.pageY;
		xe.preventDefault = function(){};
		this.whenlocked.push(function() {
			console.log("held mousedown");
			$(self.c).trigger(xe);
			return false;
		});
		return false;
	};

	this.tmup = function(e) {
		e.preventDefault();
		this.mrlse(e);
		this.evxy(e);
		if(this.buttons == 0) {
			this.selectend();
		}
	};

	this.tlocknmup = function(e) {
		if(this.islocked) {
			return this.tmup(e);
		}
		if(!this.locking) {
			this.locking = true;
			this.post(["hold"]);
			console.log("holding...");
		}
		var self = this;
		var xe = jQuery.Event("mouseup");
		xe.which = e.which;
		xe.pageX = e.pageX;
		xe.pageY = e.pageY;
		xe.preventDefault = function(){};
		this.whenlocked.push(function() {
			console.log("held mouseup");
			$(self.c).trigger(xe);
			return false;
		});
		return false;
	};

	this.locked = function() {
		if(this.islocked)
			return;
		if(this.locking) {
			this.locking = false;
			this.islocked = true;
			this.keydown = this.tkeydown;
			this.keypress = this.tkeypress;
			this.keyup = this.tkeyup;
			this.mdown = this.tmdown;
			this.mup = this.tmup;
			for(var i = 0; i < this.whenlocked.length; i++) {
				this.whenlocked[i]();
			}
			this.whenlocked = [];
		}
	};

	this.unlocked = function() {
		this.islocked = false;
		this.locking = false;
		this.mustunlock = false;
		this.whenlocked = [];
		this.keydown = this.tlocknkeydown;
		this.keypress = this.tlocknkeypress;
		this.keyup = this.tlocknkeyup;
		this.mdown = this.locknmdown;
		this.mup = this.tlocknmup;
		this.post(["tick", ""+this.p0, ""+this.p1]);
		this.post(["rlsed"]);
		// collapse the selection or other's might insert in the middle.
		if(this.p0 != this.p1) {
			this.setsel(this.p0, this.p1, true);
		}
	};

	this.keydown = this.tlocknkeydown;
	this.keypress = this.tlocknkeypress;
	this.keyup = this.tlocknkeyup;
	this.mdown = this.locknmdown;
	this.mup = this.tlocknmup;

	this.menter = function(e) {
		if(selecting) {
			return;
		}
		var x = window.scrollX;
		var y = window.scrollY;
		$("#" + this.id ).focus();
		window.scrollTo(x, y);
		if(this.islocked || this.locking) {
			return;
		}
		this.locking = true;
		this.post(["hold"]);
		console.log("holding...");
	};

	this.mwheel = function(e) {
		e.stopPropagation();
		if(!this.islocked && !this.locking) {
			this.locking = true;
			this.post(["hold"]);
			console.log("holding...");
			return false;
		}
		try {
			e.preventDefault();
			var d = e.wheelDelta * -1;
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
	};

	this.mmove = function(e) {
		if(this.islocked || this.locking) {
			return this.evxy(e);
		}
		this.locking = true;
		this.post(["hold"]);
		console.log("holding...");
		return false;
	};

	// holding down button-1, change handlers to speak
	// a different mouse language.
	this.m1 = function(pos) {
		var now = new Date().getTime();
		if(!this.clicktime || now-this.clicktime>500) {
			this.dblclick = 0;
			this.clicktime = now;
		}else{
			this.dblclick++;
			this.clicktime = now;
		}
		var wassel = true;
		if(this.dblclick) {
			var x = this.getword(pos, this.dblclick>1);
			this.post(["click1", x[0], ""+x[1], ""+x[2]]);
			this.setsel(x[1], x[2]);
			wassel = false;
		}

		this.c.onmousemove = function(e) {
			self.evxy(e);
			if(!self.buttons)
				return;
			var ln, lnoff, past;
			[ln, lnoff, past] = self.ptr2seek(self.lastx, self.lasty);
			var npos = self.seekpos(ln, lnoff);
			if(npos > pos) {
				if(self.p0 != pos || self.p1 != npos)
					self.setsel(pos, npos, true);
			}else {
				if(self.p0 != npos || self.p1 != pos)
					self.setsel(npos, pos, true);
			}
			return false;
		};

		this.c.onmousedown = function(e){
			self.evxy(e);
			self.mpress(e);
			if(self.noedits) {
				return;
			}
			if(self.buttons == 1+2){
				wassel = false;
				self.Post(["ecut", ""+self.p0, ""+self.p1]);
			}
			if(self.buttons == 1+4){
				wassel = false;
				if(self.p0 != self.p1){
					self.Post(["edel", ""+self.p0, ""+self.p1]);
				}
				self.post(["epaste", ""+self.p0, ""+self.p1]);
			}
			if(self.buttons == 1+8){
				wassel = false;
				self.post(["ecopy", ""+self.p0, ""+self.p1]);
			}
		};

		this.c.onmouseup = function(e){
			self.evxy(e);
			self.mrlse(e);
			if(self.buttons == 0){
				self.c.onmousemove = self.c.mmove;
				self.c.onmousedown = self.c.mdown;
				self.c.onmouseup = self.c.mup;
				self.post(["focus"]);
				self.selectend();
				if(wassel && self.p0 != self.p1) {
					var x = self.get(self.p0, self.p1);
					self.post(["click1", x, ""+self.p0, ""+self.p1]);
				}
				if(document.setfocus) {
					document.setfocus(self);
				}
			}
		};
	};

	// holding down button-[234], change handlers to speak
	// a different mouse language.
	this.m234 = function(pos, b) {
		this.secondary = b;
		this.c.onmousemove = function(e){
			self.evxy(e);
			if(!self.buttons)
				return;
			var ln, lnoff, past;
			[ln, lnoff, past] = self.ptr2seek(self.lastx, self.lasty);
			var npos = self.seekpos(ln, lnoff);
			if(npos > pos){
				if(self.p0 != pos || self.p1 != npos) {
					self.setsel(pos, npos, true);
				}
			}else {
				if(self.p0 != npos || self.p1 != pos) {
					self.setsel(npos, pos, true);
				}
			}
			return false;
		};

		this.c.onmousedown = function(e) {
			self.evxy(e);
			self.mpress(e);
			self.secondaryabort = true;
		};

		this.c.onmouseup = function(e) {
			self.evxy(e);
			self.mrlse(e);
			if(self.buttons == 0){
				var sp0 = self.p0;
				var sp1 = self.p1;
				var ln = self.lne;
				var tsize = 0;
				if(ln && ln.txt.length == 0) {
					tsize = ln.off;
				}
				self.secondary = 0;
				self.setsel(self.oldp0, self.oldp1);
				if(!self.secondaryabort)
				if(sp0 != sp1) {
					var txt = self.get(sp0, sp1);
					self.post(["click"+b, "", ""+sp0, ""+sp1, txt]);
				} else if(self.p0 != self.p1 &&
						 sp0 >= self.p0 && sp0 <= self.p1) {
					var txt = self.get(self.p0, self.p1);
					self.post(["click"+b, txt, ""+self.p0, ""+self.p1]);
				} else if(b != 1 && sp0 == sp1 && tsize &&
					sp0 == tsize && sp0>0) {
					// a click at a final empty line selects the previous
					// line (which is the last one shown).
					var x = self.getword(sp0-1, b != 8 || self.dblclick>1);
					self.post(["click"+b, x[0], ""+x[1], ""+x[2]]);
				} else {
					var x = self.getword(sp0, b != 8 || self.dblclick>1);
					self.post(["click"+b, x[0], ""+x[1], ""+x[2]]);
				}
				self.c.onmousemove = self.c.mmove;
				self.c.onmousedown = self.c.mdown;
				self.c.onmouseup = self.c.mup;
			}
			if(self.buttons == 0){
				self.p0 = self.oldp0;
				self.p1 = self.oldp1;
				self.secondary = 0;
				self.secondaryabort = false;
				self.selectend();
			}
		}
	};

	this.mwait = function(e) {
		this.c.onmousemove = function(e) {
			return self.evxy(e);
		};
		this.c.onmousedown = function(e) {
			self.evxy(e);
			self.mpress(e);
		};
		this.c.onmouseup = function(e) {
			self.evxy(e);
			self.mrlse(e);
			if(self.buttons == 0) {
				self.c.onmousemove = self.c.mmove;
				self.c.onmousedown = self.c.mdown;
				self.c.onmouseup = self.c.mup;
			}
		};
	};

	var self = this;
	this.c.onmousedown = function(e) {
		return self.mdown(e);
	};
	this.c.onmouseup = function(e) {
		return self.mup(e);
	};
	this.c.onmousemove = function(e) {
		return self.mmove(e);
	};
	this.c.mdown = this.c.onmousedown;
	this.c.mup = this.c.onmouseup;
	this.c.mmove = this.c.onmousemove;

	this.c.onmousewheel = function(e) {
		return self.mwheel(e);
	};
	this.c.onmouseenter = function(e) {
		return self.menter(e);
	};

	this.c.onpaste = function(){return false;};
	this.c.oncontextmenu = function(){return false;};
	this.c.onclick = null;
	this.c.ondblclick = null;

	this.d.keypress(function(e){
		dontbubble(e);
		return self.tkeypress(e);
	})
	.keyup(function(e){
		dontbubble(e);
		return self.tkeyup(e);
	})
	.keydown(function(e){
		dontbubble(e);
		return self.tkeydown(e);
	});

	this.mayresize(false);
	this.redrawtext();

	// Now that we have everything defined, make it a clive ctlr
	// with post and everything.
	CliveCtlr.call(this);

}

document.mktxt = function(d, e, cid, id, font) {
	var c = new CliveText(d, e, cid, id);
	if(!font) {
		font = "r";
	}
	c.fontstyle = font;
	c.fixfont();
	return c;
};

