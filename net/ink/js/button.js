"use strict";
/*
	js code for the clive buttons control.
 */

var bdebug = false;

// A Clive control with buttons.
function CliveButtons(d, cid, id) {
	this.d = d;
	this.c = d;
	this.cid = cid;
	this.id = id;
	this.vers = 0;

	this.apply = function(ev, fromserver) {
		if(!ev || !ev.Args || !ev.Args[0]){
			console.log("button: apply: nil ev");
			return;
		}
		var arg = ev.Args
		if(bdebug)console.log(this.id, "apply", ev.Id, ev.Args);
		switch(arg[0]){
		case "Set":
			if(arg.length < 4){
				console.log(this.divid, "apply: short set");
				break;
			}
			var on = (arg[3] == "on");
			var id = this.id + "_b" + arg[2];
			console.log("set ", id, on);
			var b = $("#"+id);
			if(b) {
				b.prop('checked', on);
				b.button("refresh");
			}
			break;
		case "show":
			this.showcontrol();
			break;
		default:
			console.log("button: unhandled", arg[0]);
		}
	};

	CliveCtlr.call(this);
}

document.mkbuttons = function(d, cid, id) {
	var c = new CliveButtons(d, cid, id);
	return c;
}
