"use strict";
/*
	js code for the buttons control
 */


function bapply(ev, fromserver) {
	if(!ev || !ev.Args || !ev.Args[0]){
		console.log("apply: nil ev");
		return;
	}
	var arg = ev.Args
	if(1)console.log(this.divid, "apply", ev.Id, ev.Args);
	switch(arg[0]){
	case "Set":
		if(arg.length < 4){
			console.log(this.divid, "apply: short set");
			break;
		}
		var on = (arg[3] == "on");
		var id = this.divid + "_b" + arg[2];
		console.log("set ", id, on);
		var b = $("#"+id);
		if(b) {
			b.prop('checked', on);
			b.button("refresh");
		}
		break;
	default:
		console.log("text: unhandled", arg[0]);
	}
}

/*
	d is is the (jquery) parent that will supply events.
	cid is the class id d.
 */
function mkbuttons(d, cid, id) {
	var wsurl = "wss://" + window.location.host + "/ws/" + cid;
	d.post = function(args) {
		if(!d.ws){
			console.log("no ws");
			return nil;
		}
		var ws = d.ws;
		if(!args || !args[0]){
			console.log("post: no args");
			return nil;
		}
		var ev = {}
		ev.Id = cid;
		ev.Src = id;
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
	d.apply = bapply;
	d.divid = id;
	d.divcid = cid;
	d.ws = new WebSocket(wsurl);
	d.ws.onopen = function() {
		d.post(["id"]);
	};
	d.ws.onmessage = function(ev) {
		// console.log("got msg", e.data);
		var o = JSON.parse(ev.data);
		if(!o || !o.Id) {
			console.log("update: no object id");
			return;
		}
		console.log("update to", o.Id);
		d.apply(o, true)
	};
	d.ws.onclose = function() {
		console.log("text socket " + wsurl+ " closed\n");
		d.replaceWith("<h3>disconnected</h3>")
	};
}

document.mkbuttons = mkbuttons;

