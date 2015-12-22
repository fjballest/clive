package ctl

import (
	"bytes"
	"clive/net/wax"
	"fmt"
	"io"
)

type Canvas struct {
	*wax.Conn
	cmds   []Cmd
	dx, dy int
}

type Cmd []string

func NewCanvas(in, out chan *wax.Ev, dx, dy int, cmd ...Cmd) *Canvas {
	return &Canvas{
		Conn: &wax.Conn{Evc: in, Updc: out},
		dx:   dx,
		dy:   dy,
		cmds: cmd,
	}
}

func (cmd Cmd) String() string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "[")
	for i := 0; i < len(cmd); i++ {
		if i > 0 {
			fmt.Fprintf(&buf, ", ")
		}
		fmt.Fprintf(&buf, `"%s"`, cmd[i])
	}
	fmt.Fprintf(&buf, "]")
	return buf.String()
}

func (c *Canvas) Cmds() []Cmd {
	c.Lock()
	defer c.Unlock()
	return c.cmds
}

func (c *Canvas) Draw(cmd ...Cmd) {
	c.Lock()
	defer c.Unlock()
	c.cmds = append(c.cmds, cmd...)
	for i := 0; i < len(cmd); i++ {
		args := []string{"draw"}
		args = append(args, cmd[i]...)
		c.Updc <- &wax.Ev{Id: c.Id, Args: args}
	}
}

func (c *Canvas) ShowAt(w io.Writer, nm string) error {
	if c.Id == "" {
		c.Id = nm
	} else {
		nm = c.Id
	}
	fmt.Fprintf(w, `<canvas id="%s" class="mayresize" width="%d" height="%d" style="border:1px solid black; padding=0; margin:0; "></canvas>`+"\n",
		nm, c.dx, c.dy)
	fmt.Fprintf(w, "<script>\n%s\n", cfuncs)
	fmt.Fprint(w, `
		$(function(){
			var c = $("#`+nm+`").get(0);
			c.cmd = ccmd;
			c.redraw = credraw;
			c.mayresize = cmayresize;
			c.cmds = [
	`)
	for i := 0; i < len(c.cmds); i++ {
		if i > 0 {
			fmt.Fprintf(w, ", ")
		}
		fmt.Fprintf(w, "%s", c.cmds[i])
	}
	fmt.Fprintf(w, `];
		c.dx = %d;
		c.dy = %d;
		c.redraw(false);
		c.mayresize();`+"\n", c.dx, c.dy)

	fmt.Fprintf(w, "});\n</script>\n")

	return nil
}

/*
	Initialize a canvas object c
*/
var cfuncs = `
	function credraw(isrsz) {
		var ctx = this.getContext("2d");
		for(var i in this.cmds) {
			if(!isrsz || (this.cmds[i][0] != "size" && this.cmds[i][0] != "scale")){
				this.cmd(ctx, this.cmds[i]);
			}
		}
	}

	function cmayresize() {
		var p = $(this).parent();
		var dx = p.width();
		var dy = p.height();
		console.log('canvas resized', dx, dy);
		ctx = this.getContext("2d");
		if(this.dx > 0 && this.dy > 0 && dx > 0 && dy > 0){
			if(this.scalekind == "proportional"){
				var ratio = this.dx/this.dy;
				var nx = dx
				var scl = 1.0;
				if(ratio < 0.0001)
					return;
 				if(dx/ratio > dy){
					scl = dy/this.dy;
				} else {
					scl = dx/this.dx;
				}
				this.width = this.dx * scl;
				this.height = this.dy * scl;
				ctx.scale(scl, scl);
			} else {
				var sx = dx/this.dx;
				var sy = dy/this.dy;
				this.width = dx;
				this.height = dy;
				ctx.scale(sx, sy);
			}
			console.log('canvas scale', sx, sy);
			this.redraw(true);
		}
	}

	function ccmd(ctx, args) {
		if(!this || !args || !args[0])
			return;
		console.log("canvas cmd ", args);
		ctx.beginPath();
		switch(args[0]){
		case "scale":	/* "proportional" or anything else, see cmayresize() */
			this.scalekind = args[1];
			break;
		case "size":
			if(args.length != 3){
				console.log("usage: size|fixsize dx dy");
				break;
			}
			var dx = parseInt(args[1]);
			var dy = parseInt(args[2]);
			if(dx < 30)
				dx = 30;
			if(dy < 30)
				dy = 30;
			if(dx > 5000)
				dx = 5000;
			if(dy > 5000)
				dy = 5000;
			this.width = dx;
			this.height = dy;
			this.dx = dx;
			this.dy = dy;
			break;
		case "fill":
			if(args.length != 2){
				console.log("usage: fill #nnnnnn");
				break;
			}
			ctx.fillStyle = args[1];
			break;
		case "col":
			if(args.length != 2){
				console.log("usage: col #nnnnnn");
				break;
			}
			ctx.strokeStyle = args[1];
			break;
		case "wid":
			if(args.length != 2){
				console.log("usage: wid n");
				break;
			}
			ctx.lineWidth = parseInt(args[1]);
			break;
		case "cap":
			if(args.length != 2){
				console.log("usage: cap butt|round|square");
				break;
			}
			ctx.lineCap = args[1];
			break;
		case "font":
			if(args.length != 2){
				console.log("usage: font spec");
				break;
			}
			ctx.font = args[1];
			break;
		case "join":
			if(args.length != 2){
				console.log("usage: join bevel|round|miter");
				break;
			}
			ctx.lineJoin = args[1];
			break;
		case "rect":
		case "fillrect":
		case "clear":
			if(args.length != 5){
				console.log("usage: rect|fillrect|clear x0 y0 x1 y1");
				break;
			}
			var x0 = parseInt(args[1]);
			var y0 = parseInt(args[2]);
			var x1 = parseInt(args[3]);
			var y1 = parseInt(args[4]);
			switch(args[0]){
			case "rect":
				ctx.strokeRect(x0, y0, x1-0, y1-y0);
				break;
			case "fillrect":
				ctx.fillRect(x0, y0, x1-0, y1-y0);
				break; 
			case "clear":
				ctx.clearRect(x0, y0, x1-0, y1-y0);
				break;
			}
			break;
		case "line":
		case "fillline":
			if(args.length < 5 || (args.length-1)%4 != 0){
				console.log("usage: line|fillline x0 y0 x1 y1 [x2 y2...]");
				break;
			}
			var x0 = parseInt(args[1]);
			var y0 = parseInt(args[2]);
			ctx.moveTo(x0, y0);
			for(var i = 3; i < args.length-1; i += 2) {
				var x = parseInt(args[i]);
				var y = parseInt(args[i+1]);
				ctx.lineTo(x, y);
			}
			ctx.stroke();
			if(args[0] == "fillline")
				ctx.fill();
			break;
		case "arc":
			if(args.length != 6){
				console.log("usage: arc|fillarc x y r sangle eangle");
				break;
			}
			var x = parseInt(args[1]);
			var y = parseInt(args[2]);
			var r = parseInt(args[3]);
			var d0 = parseFloat(args[4]);
			var d1 = parseFloat(args[5]);
	
			ctx.arc(x, y, r, d0, d1);
			ctx.stroke();
			if(args[0] == "fillarc")
				ctx.fill();
			break;
		case "text":
			if(args.length != 4){
				console.log("usage: text x y str");
				break;
			}
			var x = parseInt(args[1]);
			var y = parseInt(args[2]);
			var wid = ctx.lineWidth;
			ctx.lineWidth = 1;
			ctx.strokeText(args[3], x, y);
			ctx.lineWidth = wid;
			break;
		}
	}
`
