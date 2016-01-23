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

// A web page used as a user interface.
struct Pg {
	sync.Mutex
	Path string
	els []interface{}	// string, Html, io.WriterTo
}

// Raw HTML text when used as a page element.
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

// Serve the javascript files at /js.
// Only needed if NewPg() is not used.
func ServeJS() {
	once.Do(start)
}

func start() {
	jspath = fpath.Dir(cmd.Dot())
	http.HandleFunc("/js/", jsHandler)
}

// Serve the pages.
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

// Create a new UI page.
// Elements can be strings, Html, or io.WriterTo that know how to write the
// HTML for them (controls implement this interface).
func NewPg(path string, els ...interface{}) *Pg {
	once.Do(start)
	pg := &Pg {
		Path: path,
		els: els,
	}
	hndlr := func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `<html><head><title>Clive web</title>`);
		WriteHeaders(w);
		fmt.Fprintln(w, `</head><body>`);
		pg.Lock()
		defer pg.Unlock()
		for i := 0; i < len(pg.els);  {
			switch el := els[i].(type) {
			case string:
				io.WriteString(w, html.EscapeString(el))
			case Html:
				io.WriteString(w, string(el))
			case io.WriterTo:
				if el, ok := el.(closeder); ok {
					if el.Closed() {
						copy(pg.els[i:], pg.els[i+1:])
						pg.els = pg.els[:len(pg.els)-1]
						continue
					}
				}
				el.WriteTo(w)
			}
			i++
			if i < len(pg.els) {
				fmt.Fprintln(w, `<p><hr><p>`);
			}
		}
		fmt.Fprintln(w, `</body></html>`);
	}
	http.HandleFunc(path, hndlr)
	return pg
}

// Add the given element to the page.
// The page is not reloaded on current viewers.
func (pg *Pg) Add(element interface{}) {
	pg.Lock()
	pg.els = append(pg.els, element)
	pg.Unlock()
}

// Del the given element from the page.
// The element is not closed.
// The page is not reloaded on current viewers.
func (pg *Pg) Del(el interface{}) {
	pg.Lock()
	for i := 0; i < len(pg.els); {
		if pg.els[i] == el {
			copy(pg.els[i:], pg.els[i+1:])
			pg.els = pg.els[:len(pg.els)-1]
		} else {
			i++
		}
	}
	pg.Unlock()
}

