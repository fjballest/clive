/*
	Simple static web server for UNIX files.
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"net/http"
)

var (
	dir  = "/zx/usr/web"
	port = "8080"
	tport = "8083" 
	opts = opt.New("[dir]")
)

func main() {
	cmd.UnixIO()
	opts.NewFlag("p", "port: port used (8080 by default)", &port)
	opts.NewFlag("t", "port: TLS port (8083 by default)", &tport)
	args := opts.Parse()
	switch len(args) {
	case 0:
	case 1:
		dir = args[0]
	default:
		cmd.Warn("too many arguments")
		opts.Usage()
	}
	cert := "/zx/lib/webcert.pem"
	key := "/zx/lib/webkey.pem"
	go http.ListenAndServeTLS(":"+tport, cert, key, http.FileServer(http.Dir(dir)))
	err := http.ListenAndServe(":"+port, http.FileServer(http.Dir(dir)))
	if err != nil {
		cmd.Fatal(err)
	}
}
