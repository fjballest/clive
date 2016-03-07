"use strict";
/*
	js code for the clive radio buttons control
 */

var rdebug = false;

function CliveRadio(d, cid, id) {
	this.d = d;
	this.c = d;
	this.cid = cid;
	this.id = id;
	this.vers = 0;

	this.apply = function(ev, fromserver) {
		if(!ev || !ev.Args || !ev.Args[0]){
			console.log("apply: nil ev");
			return;
		}
		var arg = ev.Args
		if(rdebug)console.log(this.id, "apply", ev.Id, ev.Args);
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
			this.showcontrol(this);
			break;
		default:
			console.log("radio: unhandled", arg[0]);
		}
	};

	CliveCtlr.call(this);
}


document.mkradio = function(d, cid, id) {
	var c = new CliveRadio(d, cid, id);
	return c;
}
