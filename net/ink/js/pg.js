"use strict";
/*
 * clive ink pg tools
 *
 * needs a rewrite.
 * should define a global clive object to contain all the clive globals, and go from there.
 */

var pgdebug = false;

// controls may call this to set the icon for dirty (and get saves on clicks)
// but they must implement the post method on the element passed.
function setdirty(e) {
	if(pgdebug)console.log("dirty");
	var p = $(e).closest(".portlet");
	if(!p || !p.length) {
		console.log("dirty: no portlet");
		return;
	}
	var pd = p.find(".portlet-dirty");
	if(pd.length > 0) {
		console.log("already dirty", pd);
		return;
	}
	var pmax = p.find(".portlet-max");
	$("<span class='ui-icon inline ui-icon-disk portlet-dirty'></span>").insertBefore(pmax);
	pmax.closest(".portlet-header").css('color', 'blue');
	p.find(".portlet-dirty").click(function(ev) {
		ev.stopPropagation();
		e.post(["save"]);
	});
}

// Like setdirty
function setclean(e) {
	var p = $(e).closest(".portlet");
	if(!p || !p.length) {
		return;
	}
	p.find(".portlet-dirty").closest(".portlet-header").css('color', 'black');
	p.find(".portlet-dirty").remove();
}

var oldfocus = undefined;

function setfocus(e) {
	if(pgdebug)console.log("pg focus");
	var p = $(e).closest(".portlet");
	if(!p || !p.length) {
		console.log("focus: no portlet for ", e);
		return;
	}
	var pmax = p.find(".portlet-max");
	if(oldfocus) {
		try {
			oldfocus.closest(".portlet-header").css('background-color', '#CC6600');
		}catch(ex) {
			console.log("setfocus", ex);
		}
	}
	var hdr = pmax.closest(".portlet-header");
	if(pgdebug)console.log("pg hdr ", hdr);
	hdr.css('background-color', '#EE8800');
	oldfocus = pmax;
}

function scrollcol() {
	var child = $(this).find(".portlet").first();
	if(pgdebug)console.log("scroll ", child);
	$(this).append(child);
}

// Like setclean/dirty, but updates the tag
function settag(e, tag) {
	var p = $(e).closest(".portlet");
	if(!p || !p.length) {
		console.log("settag: no portlet");
		return;
	}
	var tt = p.find('.portlet-header').find("tt");
	tt.html(tag);
	return;
	tt.getWordByEvent('click', function tagclick(ev, word) {
			ev.stopPropagation();
			if(pgdebug)console.log("tag click on ", ev, word);
			e.post(["tag", word]);
		});
	
}

// move the control to the start of the column
function showcontrol(e, tag) {
	var p = $(e).closest(".portlet");
	if(!p || !p.length) {
		console.log("showcontrol: no portlet");
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
	document.setfocus = setfocus;
	document.setclean = setclean;
	document.settag = settag;
	document.showcontrol = showcontrol;
});

// el is a portlet
// remove() is not enough, we must close the ws(s)
function removecontrol(el, needpost) {
	console.log("closing", el);
	if(!el) {
		return;
	}
	$(el).find(".clivectl").each(function() {
		if(!this.ws) {
			console.log("BUG: clivectl w/o ws");
			console.log("didn't set d.get(0).ws?");
		} else {
			if(needpost && this.post) {
				this.post(["quit"]);
			}
			var pgid = $(el).attr('pgid')
			if(needpost && pgid) {
				document.post(["quit", pgid]);
			}
			this.ws.close();
		}
	});
	el.remove();
}

function maxpl(pl) {
	var ismin = false;
	var icon = $(pl).find(".portlet-toggle").first();
	if(!icon.hasClass("ui-icon-plus")){
		return false;
	}
	if(pgdebug)console.log("maxpl ", icon);
	$(pl).find('.portlet-content').toggle();
	icon.toggleClass("ui-icon-minus ui-icon-plus");
	pl.find(".clivectl").each(function() {
		if(this.addsize) {
			this.addsize(0);
		}
	});
	return true;
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
		var hdr = $(p).addClass("ui-widget ui-widget-content ui-helper-clearfix ui-corner-all")
			.find(".portlet-header");
		$(hdr).on('click', function(e) {
			if(pgdebug)console.log("tag click");
			scrollcol.call($(this).closest(".column"), e);
		});
		hdr.addClass("ui-widget-header ui-corner-all")
		.prepend("<span class='ui-icon inline ui-icon-minus portlet-toggle'></span>")
		.prepend("<span class='ui-icon inline ui-icon-triangle-2-n-s portlet-incr2'></span>")
		.prepend("<span class='ui-icon inline ui-icon-triangle-1-n portlet-decr'></span>")
		.prepend("<span class='ui-icon inline ui-icon-triangle-1-s portlet-incr'></span>")
		.prepend("<span class='ui-icon inline ui-icon-triangle-1-e portlet-max'></span>")
		.prepend("<span class='ui-icon inline ui-icon-close portlet-close'></span>");
		hdr.on('contextmenu', function(){return false;});
	}
	ps = $(".portlet-max");
	for(var i = 0; i < ps.length; i++) {
		var p = ps[i];
		if(!p.configured) {
			p.configured = true;
		} else {
			continue;
		}
		$(p).click(function(e){
			e.stopPropagation();
			var pl = $(this).closest(".portlet");
			if(maxpl(pl)) {
				return;
			}
			var p0 = pl.get(0);
			var col = $(this).closest(".column");
			$(col).find(".portlet").each(function(){
				var pi = $(this).get(0);
				var self = $(this);
				// let's minimize everything.
				if(false && p0 == pi) {
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
		$(p).click(function(e){
			e.stopPropagation();
			var icon = $(this);
			icon.toggleClass("ui-icon-minus ui-icon-plus");
			var pl = icon.closest(".portlet");
			pl.find(".portlet-content").toggle();
			pl.find(".clivectl").each(function() {
				if(this.addsize) {
					this.addsize(0);
				}
			});
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
		$(p).click(function(e){
			e.stopPropagation();
			var icon = $(this);
			var el = icon.closest(".portlet");
			removecontrol(el, true)
		});
	}
	ps = $(".portlet-incr");
	for(var i = 0; i < ps.length; i++) {
		var p = ps[i];
		if(!p.configured) {
			p.configured = true;
		} else {
			continue;
		}
		$(p).click(function(e){
			e.stopPropagation();
			var icon = $(this);
			var el = icon.closest(".portlet");
			maxpl(el);
			$(el).find(".clivectl").each(function() {
				if(this.addsize) {
					this.addsize(1);
				}
			});
		});
	}
	ps = $(".portlet-incr2");
	for(var i = 0; i < ps.length; i++) {
		var p = ps[i];
		if(!p.configured) {
			p.configured = true;
		} else {
			continue;
		}
		$(p).click(function(e){
			e.stopPropagation();
			var icon = $(this);
			var el = icon.closest(".portlet");
			maxpl(el);
			$(el).find(".clivectl").each(function() {
				if(this.addsize) {
					this.addsize(2);
				}
			});
		});
	}
	ps = $(".portlet-decr");
	for(var i = 0; i < ps.length; i++) {
		var p = ps[i];
		if(!p.configured) {
			p.configured = true;
		} else {
			continue;
		}
		$(p).click(function(e){
			e.stopPropagation();
			var icon = $(this);
			var el = icon.closest(".portlet");
			maxpl(el);
			$(el).find(".clivectl").each(function() {
				if(this.addsize) {
					this.addsize(-1);
				}
			});
		});
	}
}

function pgdrop(col, e) {
	var data = e.dataTransfer.getData("Text");
	var id = $(col).attr('id');
	if(data)
		if(pgdebug)console.log("drop", data, "on", id);
	document.post(["click4", data, id]);
}

function pgupdate() {
	if(pgdebug)console.log("layout updated");
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
	if(pgdebug)console.log(layout);
}

function pgapply(ev) {
	if(!ev || !ev.Args || !ev.Args[0]){
		console.log("apply: nil ev");
		return;
	}
	var arg = ev.Args
	switch(arg[0]) {
	case "load":
		if(arg.length < 2){
			console.log(this.divid, "apply: short load");
			break;
		}
		var cols = $(".column");
		var n = cols.length-1;
		if (arg.length > 2) {
			n = parseInt(arg[2]);
		}
		if(n < 0 || n >= cols.length) {
			n = cols.length-1;
		}
		if(pgdebug)console.log("load at col ", n, cols.length);
		var col = cols[n];
		var first = $(col).find(".portlet");
		if(first && first.length > 0) {
			first.first().before(arg[1]);
		} else {
			$(col).append(arg[1]);
		}
		if(pgdebug)console.log(col);
		break;
	case "close":
		if(arg.length < 2){
			console.log(this.divid, "apply: short close");
			break;
		}
		var id = arg[1];
		$("."+id).each(function() {
			var el = $(this).closest(".portlet");
			removecontrol(el, false);
		});
		break;
	}
}

function smooth(fn) {
	var to;
	return function(e) {
		var self = this;
		var args = arguments;
		var defer = function() {
			if (to) {
				clearTimeout(to);
				to = null;
			}
			fn.apply(self, args);
		};
		if(to) {
			clearTimeout(to);
		}
		to = setTimeout(defer, 30);
	};
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
		if(pgdebug)console.log("update to", o.Id, o.Args);
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

$(function() {
	jQuery.event.props.push('dataTransfer');
	$(".column").sortable({
		connectWith: ".column",
		handle: ".portlet-header",
		cancel: ".portlet-toggle",
		tolerance: "pointer",
		placeholder: "portlet-placeholder ui-corner-all",
		update: function(e, u) {
			if(pgdebug)console.log("update", e, u);
			pgupdate();
		},
		start: function(e) {
			if(pgdebug)console.log("start", e);
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
	// $(".column").on('mousewheel', smooth(scrollcol));
	// $("body").css("overflow", "hidden");
	
});
