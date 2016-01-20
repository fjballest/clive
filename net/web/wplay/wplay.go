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

var JSpath = "/Users/nemo/gosrc/src/clive/net/web"

func EchoServer(ws *websocket.Conn) {
	io.Copy(ws, ws)
}

func jsHandler(w http.ResponseWriter, r *http.Request) {
	p := fpath.Clean(r.URL.Path)
	p = fpath.Join(JSpath, p)
	cmd.Warn("serving %s\n", p)
	http.ServeFile(w, r, p)
}

func rHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%s\n", xhtml)
}

func main() {
	cmd.UnixIO()
	http.HandleFunc("/", rHandler)
	http.Handle("/clive", websocket.Handler(EchoServer))
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
</head>

Testing sockets

</html>
`
