package web

import (
	"sync"
	"io"
	fpath "path"
	"net/http"
	"clive/cmd"
	"clive/net/auth"
	"html"
	"fmt"
)

struct Pg {
	sync.Mutex
	Path string
	Els []interface{}	// string, Html, io.WriterTo
}

// unscaped text as html
type Html string

var (
	jspath = ""
	once sync.Once
)


func jsHandler(w http.ResponseWriter, r *http.Request) {
	p := fpath.Clean(r.URL.Path)
	p = fpath.Join(jspath, p)
	cmd.Warn("serving %s\n", p)
	http.ServeFile(w, r, p)
}

func ServeJS() {
	once.Do(start)
}

func start() {
	jspath = fpath.Dir(cmd.Dot())
	http.HandleFunc("/js/", jsHandler)
}

func Serve() error {
	if err := http.ListenAndServeTLS(":8181", auth.ServerPem, auth.ServerKey, nil); err != nil {
		cmd.Warn("%s", err)
		return err
	}
	return nil
}

interface closeder {
	Closed() bool
}

func NewPg(path string, els ...interface{}) *Pg {
	once.Do(start)
	pg := &Pg {
		Path: path,
		Els: els,
	}
	hndlr := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "%s\n", pgstart)
		pg.Lock()
		defer pg.Unlock()
		for i := 0; i < len(pg.Els);  {
			switch el := els[i].(type) {
			case string:
				io.WriteString(w, html.EscapeString(el))
			case Html:
				io.WriteString(w, string(el))
			case io.WriterTo:
				if el, ok := el.(closeder); ok {
					if el.Closed() {
						copy(pg.Els[i:], pg.Els[i+1:])
						pg.Els = pg.Els[:len(pg.Els)-1]
						continue
					}
				}
				el.WriteTo(w)
			}
			i++
		}
		fmt.Fprintf(w, "%s\n", pgend)
	}
	http.HandleFunc(path, hndlr)
	return pg
}

var pgstart =`<html>
<head>
<title>wedit</title>
` + Headers + `
</head>
<body>
`
var pgend = `</body></html>`
