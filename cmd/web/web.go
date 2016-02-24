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
	local := false
	opts.NewFlag("p", "port: port used (8080 by default)", &port)
	opts.NewFlag("t", "port: TLS port (8083 by default)", &tport)
	opts.NewFlag("l", "localhost and TLS only", &local)
	args := opts.Parse()
	switch len(args) {
	case 0:
		if local {
			dir = "/zx"
		}
	case 1:
		dir = args[0]
	default:
		cmd.Warn("too many arguments")
		opts.Usage()
	}
	cert := "/zx/lib/webcert.pem"
	key := "/zx/lib/webkey.pem"
	addr := ":"
	go func() {
		err := http.ListenAndServeTLS(addr+tport, cert, key, http.FileServer(http.Dir(dir)))
		if err != nil {
			cmd.Fatal(err)
		}
	}()
	go func() {
		if local {
			return
		}
		err := http.ListenAndServe(addr+port, http.FileServer(http.Dir(dir)))
		if err != nil {
			cmd.Fatal(err)
		}
	}()
	c := make(chan bool)
	<-c
}
