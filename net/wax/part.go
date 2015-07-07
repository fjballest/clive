package wax

import (
	"clive/dbg"
	"clive/x/code.google.com/p/go.net/websocket"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"sync"
)

/*
	A part of a user interface. See the package documentation
	for a description. You shouldn't use any field directly.
*/
type Part  {
	*Conn // used only for parts used as a control
	sync.Mutex
	l          *lex
	cmd        *cmd
	w          io.Writer
	fmt        Formatter
	path, name string
	napplies   int
	env        map[string]interface{}
}

/*
	Someone that knows how to present itself into w.
*/
type Presenter interface {
	ShowAt(w io.Writer, nm string) error
}

/*
	Output format implementor for complex go types.
	Decides what to emit to preserve the layout of the data.
	Itemizer is the formatter used by default.
*/
type Formatter interface {
	Start(w io.Writer, name, tag string)
	End(w io.Writer, name, tag string)
	StartGrp(w io.Writer)
	EndGrp(w io.Writer)
	Show(w io.Writer, fmt string, val ...interface{})
}

var (
	Debug   bool
	dprintf = dbg.FlagPrintf(os.Stderr, &Debug)
)

/*
	Create a new part by parsing the format string which is a wax
	template as described in the package description.
*/
func New(fmt string) (*Part, error) {
	p := &Part{
		Conn: &Conn{
			Evc:  make(chan *Ev),
			Updc: make(chan *Ev),
		},
	}
	p.fmt = Itemizer
	cmd, err := p.parse(fmt)
	if err != nil {
		return nil, err
	}
	p.cmd = cmd
	return p, nil
}

/*
	Binds the part to the environment represented by the map.
	Each key in the map can be used as a name in wax commands within
	the part template. The value for each name is that recorded in the
	max.
*/
func (p *Part) SetEnv(args map[string]interface{}) {
	p.Lock()
	defer p.Unlock()
	p.env = make(map[string]interface{})
	if args != nil {
		for k, v := range args {
			p.env[k] = v
		}
	}
}

/*
	Set the part complex gp data formatter to f (defaults to Itemizer)
*/
func (p *Part) SetFmt(f Formatter) {
	p.fmt = f
}

/*
	Apply the part (bound to its current environment) and write
	the resulting HTML to the given writer.
*/
func (p *Part) apply(w io.Writer) error {
	p.Lock()
	defer p.Unlock()
	p.w = w
	if p.cmd == nil {
		return errors.New("part not compiled")
	}
	// a top-level part has no id
	if p.name != "" {
		panic("part used both within a part and as a top-level")
	}
	return p.eval(p.cmd)
}

/*
	For testing. This is equivalent to a call to
	New, then, SetEnv, and then apply.
*/
func applyNew(w io.Writer, fmt string, args map[string]interface{}) error {
	p, err := New(fmt)
	if err != nil {
		return err
	}
	p.fmt = Itemizer
	p.SetEnv(args)
	if err := p.apply(w); err != nil {
		return err
	}
	return nil
}

func (p *Part) ws() {
	wspath := path.Join(p.path, "ws")
	vprintf("ws ready at %s\n", wspath)
	http.Handle(wspath, websocket.Handler(func(ws *websocket.Conn) {
		vprintf("%s: ws started\n", wspath)
		defer vprintf("%s: ws done\n", wspath)
		var buf [8*1024]byte
		defer ws.Close()
		inc := make(chan *Ev)
		outc := make(chan *Ev)
		p.Mux(inc, outc)
		// spawn the writer proc
		go func() {
			vprintf("%s: ws writer started\n", wspath)
			defer vprintf("%s: ws writer done\n", wspath)
			defer close(inc, "writer hangup")
			defer close(outc, "writer hangup")
			for ev := range outc {
				m, err := json.Marshal(ev)
				if err != nil {
					dprintf("%s update: marshal: %s\n", wspath, err)
					return
				}
				vprintf("%s update: %v\n", wspath, ev)
				if _, err := ws.Write(m); err != nil {
					dprintf("%s update: %v wr: %s\n", wspath, ev, err)
					return
				}
				dprintf("%s update: %v done\n", wspath, ev)
			}
			dprintf("%s: writer: outc closed: %s\n", wspath, cerror(outc))
		}()

		// do the reader proc
		for {
			defer close(inc, "reader hangup")
			defer close(outc, "reader hangup")
			n, err := ws.Read(buf[0:])
			if err != nil {
				vprintf("%s: ev read: %s\n", wspath, err)
				return
			}
			if n == 0 {
				continue
			}
			ev, err := ParseEv(buf[:n])
			if err != nil {
				dprintf("%s: ev parse: %s\n", wspath, err)
				continue
			}
			vprintf("%s event: %v\n", wspath, ev)
			if ok := inc <- ev; !ok {
				dprintf("%s: ev send: %s\n", wspath, cerror(inc))
				return
			}
			dprintf("%s event: %v posted \n", wspath, ev)
			if p.Updc == nil {
				dprintf("%s: ev %v: nil updc\n", wspath, ev)
				continue
			}

			if ev.reflects() {
				dprintf("%s event: %v reflecting\n", wspath, ev)
				p.Updc <- ev
			}
			dprintf("%s event: %v done\n", wspath, ev)
		}
	}))
}

var parthead = `
<html>
<head>
	<link rel="stylesheet" type="text/css" href="/js/jq/themes/base/jquery-ui.css"/>
	<script type="text/javascript" src="/js/aes.js"></script>
	<script type="text/javascript" src="/js/jq/jquery.js"></script>
	<script type="text/javascript" src="/js/jq/ui/jquery-ui.js"></script>
	<script type="text/javascript" src="/js/txt.js"></script>
	<style>
  		.ui-menu { width: 150px; }
  	</style>
	</head>
`

var parttail = `</html>`

/*
	Make part serve the given path.
	After the call, the (url) path is handled by the HTML
	produced by the
*/
func (p *Part) Serve(path string) {
	p.Lock()
	defer p.Unlock()
	if p.path != "" {
		panic(fmt.Errorf("already serving '%s'", p.path))
	}
	if path == "" {
		panic("serve with no path")
	}
	p.path = path
	if p.Evc==nil || p.Updc==nil {
		panic("part: no evc/updc")
	}
	p.ws()
	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if !Auth(w, r) {
			return
		}
		fmt.Fprintf(w, "%s\n", parthead)

		js := `
			var xuri = window.location;
			var wsuri = "ws:";
			if(xuri.protocol === "https:" || xuri.protocol === "https://") {
				wsuri = "wss:";
			}
			wsuri += "//" + xuri.hostname + ":" + xuri.port + "` + path + `" + "/ws";
			console.log("part socket at " + wsuri);
			partws = new WebSocket(wsuri);
			partws.onmessage = function(e){
				var o = JSON.parse(e.data);
				if(!o || !o.Id) {
					console.log("update: no objet id");
					return;
				}
				console.log($("."+o.Id));
				var some = false;
				$("."+o.Id).each(function(i){
					if(this.update){
						some = true;
						this.update(o)
					}
				});
				if(!some)
					console.log("update: " + e.data);
			}
			partws.onclose = function() {
				console.log("ws closed: done");
				var nd = document.open("text/html", "replace")
				nd.write("<b>program exited</b>")
				nd.close();
			};
			console.log(partws);
			$(window).on('resize', function() {
				var dx = $(window).width();
				var dy = $(window).height();
				console.log("window resize", dx, dy);
				$(".mayresize").each(function(){
					if(!this.mayresize){
						console.log("no resize? ", this);
						return;
					}
					this.mayresize();
				});
			});
		`
		fmt.Fprintf(w, "<script> %s </script>\n", js)
		if err := p.apply(w); err != nil {
			panic(fmt.Errorf("part apply: %s", err))
		}
		fmt.Fprintf(w, "%s\n", parttail)
	})
}
