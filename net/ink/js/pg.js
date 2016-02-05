"use strict";
/*
 * clive ink pg tools
*/

// controls may call this to set the icon for dirty (and get saves on clicks)
// but they must implement the post method on the element passed.
function setdirty(e) {
	console.log("dirty");
	var p = $(e).closest(".portlet");
	if(!p || !p.length) {
		return;
	}
	var pd = p.find(".portlet-dirty");
	if(pd.length > 0) {
		console.log("already dirty", pd);
		return;
	}
	var pmax = p.find(".portlet-max");
	$("<span class='ui-icon inline ui-icon-disk portlet-dirty'></span>").insertBefore(pmax);
	pmax.closest(".portlet-header").css('background-color', '#EE8800');
	p.find(".portlet-dirty").click(function() {
		e.post(["save"]);
	});
}

// Like setdirty
function setclean(e) {
	var p = $(e).closest(".portlet");
	if(!p || !p.length) {
		return;
	}
	p.find(".portlet-dirty").closest(".portlet-header").css('background-color', '#CC6600');
	p.find(".portlet-dirty").remove();
}

// Like setclean/dirty, but updates the tag
function settag(e, tag) {
	var p = $(e).closest(".portlet");
	if(!p || !p.length) {
		return;
	}
	var tt = p.find(".portlet-header").find("tt");
	tt.html(tag);
	tt.getWordByEvent('click', function tagclick(ev, word) {
			console.log("tag click on ", ev, word);
			e.post(["tag", word]);
		});
}

// move the control to the start of the column
function showcontrol(e, tag) {
	var p = $(e).closest(".portlet");
	if(!p || !p.length) {
		return;
	}
	var c = p.closest(".column");
	if(!c) {
		return;
	}
	$(c).find(".portlet").first().before(p);
}


$(function(){
	document.setdirty = setdirty;
	document.setclean = setclean;
	document.settag = settag;
	document.showcontrol = showcontrol;
});

// el is a portlet
// remove() is not enough, we must close the ws(s)
function removecontrol(el) {
	console.log("closing", el);
	if(!el) {
		return;
	}
	$(el).find(".hasws").each(function() {
		if(!this.ws) {
			console.log("BUG: hasws w/o ws");
			console.log("didn't set d.get(0).ws?");
		} else {
			this.ws.close();
		}
	});
	el.remove();
}

function updportlets() {
	var ps = $(".portlet")
	for(var i = 0; i < ps.length; i++) {
		var p = ps[i];
		if(!p.configured) {
			p.configured = true;
		} else {
			continue;
		}
		$(p).addClass("ui-widget ui-widget-content ui-helper-clearfix ui-corner-all")
		.find(".portlet-header").addClass("ui-widget-header ui-corner-all")
		.prepend("<span class='ui-icon inline ui-icon-minus portlet-toggle'></span>")
		.prepend("<span class='ui-icon inline ui-icon-arrowthick-2-n-s portlet-max'></span>")
		.prepend("<span class='ui-icon inline ui-icon-extlink portlet-drag'></span>")
		.prepend("<span class='ui-icon inline ui-icon-close portlet-close'></span>");
	}
	ps = $(".portlet-max");
	for(var i = 0; i < ps.length; i++) {
		var p = ps[i];
		if(!p.configured) {
			p.configured = true;
		} else {
			continue;
		}
		$(p).click(function(){
			var p0 = $(this).closest(".portlet").get(0);
			var col = $(this).closest(".column");
			$(col).find(".portlet").each(function(){
				var pi = $(this).get(0);
				var self = $(this);
				if(p0 == pi) {
					$(this).find(".portlet-toggle").each(function(){
						if($(this).hasClass("ui-icon-plus")) {
							$(this).toggleClass("ui-icon-minus ui-icon-plus");
							self.find(".portlet-content").toggle();
						}
					});
					return;
				}
				$(this).find(".portlet-toggle").each(function(){
					if($(this).hasClass("ui-icon-minus")) {
						$(this).toggleClass("ui-icon-minus ui-icon-plus");
						self.find(".portlet-content").toggle();
					}
				});
			});
		});
	}
	ps = $(".portlet-toggle");
	for(var i = 0; i < ps.length; i++) {
		var p = ps[i];
		if(!p.configured) {
			p.configured = true;
		} else {
			continue;
		}
		$(p).click(function(){
			var icon = $(this);
			icon.toggleClass("ui-icon-minus ui-icon-plus");
			icon.closest(".portlet").find(".portlet-content").toggle();
		});
	}
	ps = $(".portlet-close");
	for(var i = 0; i < ps.length; i++) {
		var p = ps[i];
		if(!p.configured) {
			p.configured = true;
		} else {
			continue;
		}
		$(p).click(function(){
			var icon = $(this);
			var el = icon.closest(".portlet");
			removecontrol(el)
		});
	}
}

$(function() {
	jQuery.event.props.push('dataTransfer');
	$(".column").sortable({
		connectWith: ".column",
		handle: ".portlet-header",
		cancel: ".portlet-toggle",
		tolerance: "pointer",
		placeholder: "portlet-placeholder ui-corner-all",
		update: function(e, u) {
			console.log("update", e, u);
			pgupdate();
		},
		start: function(e) {
			console.log("start", e);
		},

	});
	updportlets();
	$(".column").on('dragover', function(e) {
		$(this).css("border", "1px black");
		e.dataTransfer.dropEffect = "copy";
		e.preventDefault();
	});
	$(".column").on('dragleave', function(e) {
		$(this).css("border", "0px");
		e.preventDefault();
	});
	$(".column").on('drop', function(e) {
		$(this).css("border", "0px");
		e.preventDefault();
		pgdrop(this, e);
	});
	$("#morecols").on('click', function(e) {
		var ncols = $(".column").length +1;
		document.post(["cols", ""+ncols]);
		var ori = window.location.origin;
		ori += "?ncol=" + ncols;
		location.replace(ori);
	});
	$("#lesscols").on('click', function(e) {
		var ncols = $(".column").length;
		if(ncols > 1) {
			ncols--;
			document.post(["cols", ""+ncols]);
			var ori = window.location.origin;
			ori += "?ncol=" + ncols;
			location.replace(ori);
		}
	});
});

function pgdrop(col, e) {
	var data = e.dataTransfer.getData("Text");
	var id = $(col).attr('id');
	if(data)
		console.log("drop", data, "on", id);
	document.post(["click4", data, id]);
}

function pgupdate() {
	console.log("layout updated");
	var layout=["layout"];
	$(".column").each(function(){
		var col = $(this).attr('id');
		$(this).find(".ui-widget-content").each(function(){
			var el = $(this).attr('id');
			if(el) {
				layout.push(col+"!"+el);
			} else {
				layout.push(col+"!none");
			}
		});
	});
	document.post(layout);
	console.log(layout);
}

function pgapply(ev) {
	if(!ev || !ev.Args || !ev.Args[0]){
		console.log("apply: nil ev");
		return;
	}
	var arg = ev.Args
	switch(arg[0]) {
	case "load":
		var cols = $(".column");
		var col = cols[cols.length-1];
		var first = $(col).find(".portlet");
		if(first && first.length > 0) {
			first.first().before(arg[1]);
		} else {
			$(col).append(arg[1]);
		}
		console.log(col);
		break;
	case "close":
		var id = arg[1];
		$("."+id).each(function() {
			var el = $(this).closest(".portlet");
			removecontrol(e);
		});
		break;
	}
}

function mkpg(id, cid) {
	var wsurl = "wss://" + window.location.host + "/ws/" + cid;
	var ws = new WebSocket(wsurl);
	var post = function(args) {
		if(!ws){
			console.log("no ws");
			return nil;
		}
		if(!args || !args[0]){
			console.log("post: no args");
			return nil;
		}
		var ev = {}
		ev.Id = cid;
		ev.Src = id;
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
	document.post = post
	ws.onopen = function() {
		post(["id"]);
	};
	ws.onmessage = function(ev) {
		// console.log("got msg", e.data);
		var o = JSON.parse(ev.data);
		if(!o || !o.Id) {
			console.log("update: no object id");
			return;
		}
		console.log("update to", o.Id, o.Args);
		pgapply(o);
	};
	ws.onclose = function() {
		console.log("text socket " + wsurl+ " closed\n");
		var nd = document.open("text/html", "replace");
		nd.write("<h3 style><tt>disconnected</tt></h3>");
		nd.close();
		$(document.body).css("background-color", "#ddddc8");
	};
}
