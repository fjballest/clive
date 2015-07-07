package wax

import (
	"clive/dbg"
	"clive/net/auth"
	"fmt"
	"net/http"
	"os"
	"path"
	"sync"
)

/*
	From where to fetch js code and related files.
*/
var JSpath = "/Users/nemo/gosrc/src/clive/net/wax"

var (
	started   bool
	startedlk sync.Mutex
	Verbose   bool
	vprintf   = dbg.FlagPrintf(os.Stderr, &Verbose)
)

func jsHandler(w http.ResponseWriter, r *http.Request) {
	p := path.Clean(r.URL.Path)
	p = path.Join(JSpath, p)
	vprintf("serving %s\n", p)
	http.ServeFile(w, r, p)
}

/*
	Start the wax web server.
*/
func Serve(port string) error {
	startedlk.Lock()
	defer startedlk.Unlock()
	if started {
		return fmt.Errorf("already started")
	}
	started = true
	http.HandleFunc("/js/", jsHandler)
	if auth.TLSserver != nil {
		dbg.Warn("wax: listening at https://%s:%s\n", dbg.Sys, port)
		return http.ListenAndServeTLS(port, auth.ServerPem, auth.ServerKey, nil)
	}
	dbg.Warn("wax: listening at http://%s:%s\n", dbg.Sys, port)
	return http.ListenAndServe(port, nil)
}
