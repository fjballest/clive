package main

import (
	"io"
	"net/http"
	"clive/x/code.google.com/p/go.net/websocket"
	"clive/cmd"
	"clive/cmd/opt"
	"clive/net/auth"
	"clive/net/web"
	fpath "path"
	"fmt"
)

var jspath, txthtml string

func EchoServer(ws *websocket.Conn) {
	io.Copy(ws, ws)
}

func jsHandler(w http.ResponseWriter, r *http.Request) {
	p := fpath.Clean(r.URL.Path)
	p = fpath.Join(jspath, p)
	cmd.Warn("serving %s\n", p)
	http.ServeFile(w, r, p)
}

func rHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%s\n", xhtml)
	
	fmt.Fprintf(w, "%s\n", txthtml)
	fmt.Fprintf(w, "</body></html>\n")
}

func main() {
	opts := opt.New("")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	cmd.UnixIO()
	opts.Parse()
	jspath = fpath.Dir(cmd.Dot())
	http.HandleFunc("/", rHandler)
	http.Handle("/clive", websocket.Handler(EchoServer))
	t := web.NewText("txt1")
	txthtml = t.HTML()
	http.HandleFunc("/js/", jsHandler)
	if err := http.ListenAndServeTLS(":8181", auth.ServerPem, auth.ServerKey, nil); err != nil {
		cmd.Fatal(err)
	}
}

var xhtml=`<html>
<head>
<title>testing</title>
<script src="/js/jquery-2.2.0.js"></script>
<script type="text/javascript" src="/js/clive.js"></script>
<script type="text/javascript" src="/js/txt.js"></script>
</head>
<body>
Testing text
`
