/*
	Serve authenticated read-only access to a file tree.
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"net/http"
	"clive/net/ink"
)

func main() {
	opts := opt.New("dir")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	cmd.UnixIO()
	args := opts.Parse()
	if len(args) == 0 {
		opts.Usage()
	}
	h := http.FileServer(http.Dir(args[0]))
	http.HandleFunc("/", ink.AuthHandler(h.ServeHTTP))
	ink.ServeLoginFor("/")
	ink.Serve(":8181")
}
