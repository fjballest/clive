package main

import (
	"io"
	"net/http"
	"clive/x/code.google.com/p/go.net/websocket"
	"clive/cmd"
	"clive/net/auth"
	fpath "path"
	"fmt"
)

var jspath string

func EchoServer(ws *websocket.Conn) {
	io.Copy(ws, ws)
}

func TxtServer(ws *websocket.Conn) {
}

func jsHandler(w http.ResponseWriter, r *http.Request) {
	p := fpath.Clean(r.URL.Path)
	p = fpath.Join(jspath, p)
	cmd.Warn("serving %s\n", p)
	http.ServeFile(w, r, p)
}

func rHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%s\n", xhtml)
}

func main() {
	cmd.UnixIO()
	jspath = fpath.Dir(cmd.Dot())
	http.HandleFunc("/", rHandler)
	http.Handle("/clive", websocket.Handler(EchoServer))
//	http.Handle("/ws/txt1", websocket.Handler(txtServer))
	http.HandleFunc("/js/", jsHandler)
	if err := http.ListenAndServeTLS(":8181", auth.ServerPem, auth.ServerKey, nil); err != nil {
		cmd.Fatal(err)
	}
}

var xhtml=`<html>
<head>
<title>testing</title>
<script src="https://ajax.googleapis.com/ajax/libs/jquery/2.2.0/jquery.min.js"></script>
<script type="text/javascript" src="/js/clive.js"></script>
<script type="text/javascript" src="/js/txt.js"></script>
</head>

Testing text
<div id="txt1" class="txt1", tabindex="1" style="padding=0; margin:0; width:100%%;height:100%%;">
<canvas id="txt1c" class="mayresize txt1c hastag" width="300" height="128" style="border:1px solid black;"></canvas>
</div>

<script>
	$(function(){
		var d = $("#txt1");
		var x = $("#txt1c").get(0);
		x.tag = "foo bar";
		x.lines = [];
		x.lines.push({txt: "hi there", off: 0, eol: true});
		x.lines.push({txt: "and there", off: 9, eol: true});
		x.lines.push({txt: "", off: 19, eol: true});
		document.mktext(d, x, 0, "txt1", "txt1");
	});
</script>
</html>
`
