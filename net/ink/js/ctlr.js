"use strict";
/*
	Clive js code for ink controls.
*/

// A clive ctlr.
// Callers must have defined .d, .c, .cid, and .id before calling this.
// If this.d has wsaddr defined, that base address is used.
// This provides the post method and calls (if defined) to apply, autoresize, and mayresize methods.
function CliveCtlr() {
	this.d.clivectlr = this;
	this.c.clivectlr = this;
	this.userresized = false;
	this.wsurl = "wss://" + window.location.host + "/ws/" + this.cid;
	if(this.d.wsaddr) {
		this.wsurl = this.d.wsaddr + "/ws/" + this.cid;
	}

	$(this.d).addClass("clivectl");
	var self = this;
	// use self here, because post will be bound also to this.d
	this.post = function(args) {
		var ws = self.ws;
		if(!ws){
			console.log("post: no ws");
			return nil;
		}
		if(!args || !args[0]){
			console.log("post: no args");
			return nil;
		}
		if(!self.vers) {
			self.vers = 0;
		}
		// cut advances the vers (might del nothing)
		// this is for text and shouldn't be here.
		if(args[0] == "eins" || args[0] == "edel") {
			self.vers++;
		}
		var ev = {Id: self.cid, Src: self.id, Vers: self.vers, Args: args};
		var msg = JSON.stringify(ev);
		try {
			self.ws.send(msg);
			// console.log("posting ", msg);
		}catch(ex){
			console.log("post: " + ex);
		}
		// if this is a cut, it implies a del and we
		// must advance our vers, the event didn't
		// advance the vers.
		// Same for paste.
		// this is for text and shouldn't be here.
		if(args[0] == "ecut" || args[0] == "epaste") {
			ev.Vers++;
		}
		return ev;
	};

	this.settag = function(t) {
		if(document.settag) {
			document.settag(this, t);
		}
	};

	this.setdirty = function() {
		if(document.setdirty) {
			document.setdirty(this);
		}
	};

	this.setclean = function() {
		if(document.setclean) {
			document.setclean(this);
		}
	};

	this.showcontrol = function() {
		if(document.showcontrol) {
			document.showcontrol(this);
		}
	};

	this.ws = new WebSocket(this.wsurl);
	this.ws.onopen = function() {
		self.post(["id"]);
	};
	this.ws.onerror = function(ev) {
		console.log("ws err", ev);
	};
	this.ws.onmessage = function(ev) {
		var o = JSON.parse(ev.data);
		if(!o || !o.Id) {
			console.log("update: no objet id");
			return;
		}
		if(tdebug)console.log("update to", o.Id, o.Args);
		if(self.apply) {
			self.apply(o, true);
		}
	};
	this.ws.onclose = function() {
		console.log("text socket " + self.wsurl+ " closed\n");
		self.d.replaceWith("<h3>disconnected</h3>");
	};

	// this is for pg.js, will go.
	var d0 = this.d.get(0);
	d0.ws = this.ws;
	d0.post = this.post;
	this.d.post = this.post;

	d0.addsize = function(moreless) {
		if(self.autoresize) {
			self.autoresize(true, moreless);
		}
	};
	this.d.resizable({
		handles: 's'
	}).on('resize', function() {
		console.log("user resized");
		self.userresized = true;
		if(self.mayresize) {
			self.mayresize(true);
		}
	});
	$(window).resize(function() {
		console.log("window resized");
		if(self.mayresize) {
			self.mayresize(false);
		}
	});


}
