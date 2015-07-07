/*
	Simple static web server for UNIX files.
*/
package main

import (
	"clive/app"
	"clive/app/opt"
	"net/http"
	"os"
)

var (
	dir  = "/zx/usr/web"
	port = "8080"
	opts = opt.New("[dir]")
)

func main() {
	defer app.Exiting()
	app.New()
	opts.NewFlag("p", "port: port used (8080 by default)", &port)
	args, err := opts.Parse(os.Args)
	if err != nil {
		app.Warn("%s", err)
		opts.Usage()
		app.Exits(err)
	}
	switch len(args) {
	case 0:
	case 1:
		dir = args[0]
	default:
		app.Warn("too many arguments")
		opts.Usage()
		app.Exits("usage")
	}
	err = http.ListenAndServe(":"+port, http.FileServer(http.Dir(dir)))
	app.Exits(err)
}
