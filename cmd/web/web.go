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
	opts = opt.New("[dir]")
)

func main() {
	cmd.UnixIO()
	opts.NewFlag("p", "port: port used (8080 by default)", &port)
	args := opts.Parse()
	switch len(args) {
	case 0:
	case 1:
		dir = args[0]
	default:
		cmd.Warn("too many arguments")
		opts.Usage()
	}
	err := http.ListenAndServe(":"+port, http.FileServer(http.Dir(dir)))
	if err != nil {
		cmd.Fatal(err)
	}
}
