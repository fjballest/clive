package ink

import (
	"sync"
	"io"
	fpath "path"
	"net/http"
	"clive/cmd"
	"clive/net/auth"
	"html"
	"fmt"
	"strings"
	"strconv"
	"net/url"
	"bytes"
)

// The layout event is sent from the viewer to the page and it goes over
// the pg elements moving those controls with an Id() method to where
// they are, so new pages start with the last layout created by the user.
// The ongoing views are left alone.

// A web page used as a user interface.
// It's itself a control, and posts the events:
//	start
//	end
//	click4 name colname
//
struct Pg {
	*Ctlr
	Tag string
	sync.Mutex
	Path string
	NoAuth bool	// set to true to disable auth
	els [][]io.WriterTo	// of [] of string, Html, io.WriterTo
	idgen int
}

// Elements implementing this may provide the tag as the tittle for the tag bar.
type Tagger interface {
	Tag() string
}

// used on layout changes to locate elements by id
type idder interface {
	GetId() string
}

// Raw HTML text when used as a page element.
type Html string

// External page when used as a page element
type Url string

var (
	jspath = ""
	once sync.Once
)

struct rawEl {
	id, s string
}

struct urlEl {
	rawEl
	tag string
}

// HTML headers to be included in pages using this interface.
var headers = `
<link rel="stylesheet" href="/js/jquery-ui/jquery-ui.css">
<script src="/js/jquery-2.2.0.js"></script>
<script type="text/javascript" src="/js/clive.js"></script>
<script type="text/javascript" src="/js/txt.js"></script>
<script type="text/javascript" src="/js/txt.js"></script>
<script type="text/javascript" src="/js/button.js"></script>
<script type="text/javascript" src="/js/radio.js"></script>
<script src="/js/jquery-ui/jquery-ui.js"></script>
<script src="/js/jquery.get-word-by-event.js"></script>
`

// Write headers to a page so it can support controls.
// Not needed for pages created with NewPg.
// If you do not use NewPg, remember to use AuthHandler
// and HTTPS.
func WriteHeaders(w io.Writer) {
	io.WriteString(w, headers)
}

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
	serveLoginFor("/")
}

// Serve the pages.
// Even if they are NoAuth, it's always through TLS.
func Serve(port string) error {
	if err := http.ListenAndServeTLS(port, auth.ServerPem, auth.ServerKey, nil); err != nil {
		cmd.Warn("%s", err)
		return err
	}
	return nil
}

interface closeder {
	Closed() bool
}

func (pg *Pg) mkstr(el string) rawEl {
	pg.Lock()
	defer pg.Unlock()
	pg.idgen++
	id := fmt.Sprintf("raw%d", pg.idgen)
	return rawEl{id: id, s: el}
}

func (pg *Pg) mkiframe(s string) urlEl {
	u := s
	pg.Lock()
	defer pg.Unlock()
	pg.idgen++
	id := fmt.Sprintf("page%d", pg.idgen)
	s = ` <iframe id="`+id+`frame" src="`+s+`" style="width: 95%; height: 100%;"></iframe>` +
	`<script>
		$(function(){
			$("#`+id+`_0").resizable({handles: "s"});
		});
	</script>`
	return urlEl{rawEl: rawEl{id: id, s: s}, tag: html.EscapeString(u)}
}

func (pg *Pg) mkel(el interface{}) io.WriterTo {
	switch el := el.(type) {
	case io.WriterTo:
		return el
	case string:
		return pg.mkstr(html.EscapeString(el))
	case Html:
		return pg.mkstr(string(el))
	case fmt.Stringer:
		return pg.mkstr(html.EscapeString(el.String()))
	case Url:
		return pg.mkiframe(string(el))
	default:
		cmd.Warn("unknown element type for ink page: %T", el)
		return nil
	}
}

// Create a new UI multicolum page, authenticated.
// Elements can be strings, Html, Url, or io.WriterTo that know how to write the
// HTML for them (controls implement this interface).
// The tag line for each element comes from its Tag method if it's a Tagger.
func NewColsPg(path string, cols ...[]interface{}) *Pg {
	once.Do(start)
	pg := &Pg {
		Ctlr: newCtlr("pg"),
		Path: path,
		els: make([][]io.WriterTo, len(cols)),
	}
	for i, c := range cols {
		for _, el := range c {
			nel := pg.mkel(el)
			if nel != nil {
				pg.els[i] = append(pg.els[i], nel)
			}
		}
	}
	hndlr := func(w http.ResponseWriter, r *http.Request) {
		tag := pg.Tag
		if tag == "" {
			tag = "Clive"
		}
		title := html.EscapeString(tag)
		fmt.Fprintln(w, `<html><head><title>`+title+`</title>`);
		WriteHeaders(w);

		values, _ := url.ParseQuery(r.URL.RawQuery)
		if v := values["ncol"]; len(v) > 0 {
			nc, err := strconv.Atoi(v[0])
			if err == nil && nc > 0 {
				cmd.Dprintf("changing layout to %v columns...\n", nc)
				pg.setNumCols(nc)
			}
		}
		fmt.Fprintln(w, `<script type="text/javascript" src="/js/pg.js"></script>`);
		pcent := 100/len(pg.els)
		fmt.Fprintln(w, `
		<style>
		body {
			background-color: #ddddc8;
			min-width: 520px;
		}
		.ui-widget-content {background-color: #ddddc8; }
		.column {width: `+strconv.Itoa(pcent)+`%;  float: left; padding-bottom: 10px; }
		.portlet { margin: 0 0 0 0; padding: 0.1em; background-color: #ddddc8;}
		.portlet-header { padding: 0.2em 0.2em; margin-bottom: 0.5em; 
			position: relative; background-color: #CC6600}
		.portlet-toggle { position: absolute; top: 50%; right: 0; margin-top: -8px; }
		.portlet-content { padding: 0.2em; }
		.portlet-placeholder { border: 1px dotted black; margin: 0 1em 1em 0; height: 30px; }
		.ui-icon.inline { display:inline-block; }
		.ui-widget-header.center { text-align:center; }
		</style>`)
		fmt.Fprintln(w, `</head><body>`);
		pg.Lock()
		defer pg.Unlock()
		for i := 0; i < len(pg.els); i++ {
			pre := fmt.Sprintf(`<div id="column%d" class="column">`, i)
			if i == 0 {
				pre += `<span id="morecols"><tt>more</tt></span> `
				pre += `<span id="lesscols"><tt>less</tt></span><p> `
			} else {
				pre += `<p>`
			}
			pg.els[i] = writeEls(w, pg.els[i],
				pre,
				`<div class="portlet"><div class="portlet-header">`,
				`</div><div class="portlet-content">`,
				`</div></div>`,
				`</div>`)
		}
		fmt.Fprintf(w, `<script>$(function() { mkpg("%s", "%s"); });`+"\n</script>\n",
			pg.newViewId(), pg.Id)
		fmt.Fprintln(w, `</body></html>`);
	}
	go func() {
		for e := range pg.in {
			pg.handle(e)
		}
	}()
	http.HandleFunc(path, AuthHandler(hndlr))
	return pg
}

// Create a new single column UI page, authenticated.
// Elements can be strings, Html, or io.WriterTo that know how to write the
// HTML for them (controls implement this interface).
func NewPg(path string, els ...interface{}) *Pg {
	return NewColsPg(path, els)
}

func (r rawEl) WriteTo(w io.Writer) (tot int64, err error) {
	n ,err := fmt.Fprintln(w, `<div id="`+r.id+`_0" class="ui-widget-content `+r.id+`">`)
	tot += int64(n);
	if err != nil {
		return tot, err
	}
	n ,err = fmt.Fprintln(w, r.s)
	tot += int64(n);
	if err != nil {
		return tot, err
	}
	n ,err = fmt.Fprintln(w, `</div>`)
	tot += int64(n);
	return tot, err
}

func (r rawEl) GetId() string {
	// this returns the cid, actually
	return r.id
}

func (e urlEl) Tag() string {
	return e.tag
}

func writeEl(w io.Writer, el io.WriterTo, pre, mid, post string) {
	if len(pre) > 0 {
		fmt.Fprintln(w, pre)
	}
	if t, ok := el.(Tagger); ok {
		fmt.Fprintln(w, `<tt>`+
			html.EscapeString(t.Tag())+
			`</tt>`)
	}
	if len(mid) > 0 {
		fmt.Fprintln(w, mid)
	}
	el.WriteTo(w)
	if len(post) > 0 {
		fmt.Fprintln(w, post)
	}
}

func writeEls(w io.Writer, els []io.WriterTo, pre, elpre, elmid, elpost, post string) []io.WriterTo {
	fmt.Fprintln(w, pre)
	for i := 0; i < len(els);  {
		el := els[i]
		if el, ok := el.(closeder); ok {
			if el.Closed() {
				copy(els[i:], els[i+1:])
				els = els[:len(els)-1]
				continue
			}
		}
		writeEl(w, el, elpre, elmid, elpost)
		i++
	}
	fmt.Fprintln(w, post)
	return els
}

// Add the given element to the page.
// The element is always added to the last column and can be
// a string, Html, io.WriterTo, or fmt.Stringer.
// The string returned can be used to remove the element later.
func (pg *Pg) Add(el interface{}) (string, error) {
	nel := pg.mkel(el)
	if nel == nil {
		return "", fmt.Errorf("unknown element type %T", el)
	}
	var buf bytes.Buffer
	// XXX: TODO: We must call a function to convert this into a portlet.
	// must do what updportlets() does in pg.js but just for the elements
	// in the new portlet.
	writeEl(&buf, nel,
		`<div class="portlet"><div class="portlet-header">`,
		`</div><div class="portlet-content">`,
		`</div></div>
		<script>
		updportlets();
		</script>
	`)
	pg.out <- &Ev{Id: pg.Id, Src: "app", Args:[]string{"load", buf.String()}}
	pg.Lock()
	defer pg.Unlock()
	pg.els[len(pg.els)-1] = append(pg.els[len(pg.els)-1], nel)
	x := nel.(idder)
	return x.GetId(), nil
}

// Delete the element with the given id from the page (see Add for the id).
func (pg *Pg) Del(id string) {
	el := pg.dettach(id)
	if el != nil {
		pg.out <- &Ev{Id: pg.Id, Src: "app",
			Args: []string{"close", id},
		}
	}
}

func (pg *Pg) dettach(cid string) io.WriterTo {
	for i, c := range pg.els {
		for j, el := range c {
			ir, ok := el.(idder)
			if !ok || ir.GetId() != cid {
				continue
			}
			copy(c[j:], c[j+1:])
			pg.els[i] = c[:len(c)-1]
			cmd.Dprintf("el %s from col %d...\n", cid, i)
			return el
		}
	}
	return nil
}

func (pg *Pg) layout(args []string) {
	cols := map[string][]string{}
	colnames := []string{}
	for _, a := range args {
		toks := strings.Split(a, "!")
		if len(toks) != 2 {
			continue
		}
		col, ok := cols[toks[0]]
		if !ok {
			colnames = append(colnames, toks[0])
		}
		nm := toks[1]
		// convert the id into a cid
		if r := strings.LastIndexByte(nm, '_'); r >= 0 {
			nm = nm[:r]
		}
		cols[toks[0]] = append(col, nm)
	}
	pg.Lock()
	defer pg.Unlock()
	for len(colnames) > len(pg.els) {
		pg.els = append(pg.els, []io.WriterTo{})
	}
	for ci, cname := range colnames {
		for _, ename := range cols[cname] {
			if ename == "none" {
				continue
			}
			cmd.Dprintf("layout for %d %s %s\n", ci, cname, ename)
			el := pg.dettach(ename)
			if el != nil {
				cmd.Dprintf("el %s to col %d\n", ename, ci)
				pg.els[ci] = append(pg.els[ci], el)
			}
		}
	}
}

func (pg *Pg) setNumCols(n int) {
	if n <= 0 {
		return
	}
	pg.Lock()
	defer pg.Unlock()
	for n > len(pg.els) {
		pg.els = append(pg.els, []io.WriterTo{})
	}
	if n == len(pg.els) {
		return
	}
	for n < len(pg.els) {
		last := pg.els[len(pg.els)-1]
		pg.els = pg.els[:len(pg.els)-1]
		i := 0
		for _, el := range last {
			pg.els[i] = append(pg.els[i], el)
			i = (i+1)%len(pg.els)
		}
	}
}

func (pg *Pg) handle(wev *Ev) {
	if wev==nil || len(wev.Args)<1 {
		return
	}
	ev := wev.Args
	cmd.Dprintf("%s: ev %v\n", pg.Id, ev)
	switch ev[0] {
	case "start":
		pg.post(wev)
	case "end":
		pg.post(wev)
	case "click4":
		pg.post(wev)
	case "layout":
		if len(ev) < 2 {
			return
		}
		pg.layout(ev[1:])
	default:
		cmd.Dprintf("%s: unhandled %v\n", pg.Id, ev)
		return
	}
}
