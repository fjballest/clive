package wax

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"reflect"
	"unicode"
	"unicode/utf8"
)

func (p *Part) lookup(e *el) (interface{}, string, error) {
	if e == nil {
		return nil, "", fmt.Errorf("nil element")
	}
	if e.op != tId {
		return nil, "", fmt.Errorf("bug: lookup: not id")
	}
	ei, ok := p.env[e.name]
	if !ok {
		return nil, "", fmt.Errorf("undefined '%s'", e.name)
	}
	return e.lookupAt(ei)
}

func istrue(ei interface{}) bool {
	v := reflect.ValueOf(ei)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface,
		reflect.Map, reflect.Ptr, reflect.Slice:
		if v.IsNil() {
			return false
		}
	}
	switch v.Kind() {
	case reflect.Bool:
		return v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64:
		return v.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64:
		return v.Uint() != 0
	case reflect.Interface, reflect.Ptr:
		return istrue(v.Elem().Interface())
	case reflect.Array, reflect.String, reflect.Chan,
		reflect.Map, reflect.Slice:
		return v.Len() != 0
	case reflect.Struct:
		return true
	}
	return true
}

func loop(ei interface{}, c chan interface{}) {
	defer close(c)
	v := reflect.ValueOf(ei)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface,
		reflect.Map, reflect.Ptr, reflect.Slice:
		if v.IsNil() {
			return
		}
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Ptr:
		loop(v.Elem().Interface(), c)
	case reflect.Array, reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			el := v.Index(i).Interface()
			c <- el
		}
	case reflect.Map:
		keys := v.MapKeys()
		for i := 0; i < len(keys); i++ {
			el := v.MapIndex(keys[i]).Interface()
			c <- el
		}
	case reflect.Struct:
		nf := v.NumField()
		t := v.Type()
		for i := 0; i < nf; i++ {
			fld := t.Field(i)
			r, _ := utf8.DecodeRuneInString(fld.Name)
			if !unicode.IsUpper(r) {
				continue
			}
			el := v.Field(i).Interface()
			c <- el
		}
	default:
		c <- ei
	}
}

type itemizer {}

// A Formatter for parts generating lists of items.
var Itemizer itemizer

type divider {}

// A Formatter for parts generating nested div tags.
var Divider divider

func (i itemizer) Show(w io.Writer, s string, val ...interface{}) {
	fmt.Fprintf(w, "<i>")
	fmt.Fprintf(w, s, val...)
	fmt.Fprintf(w, "</i>")
}

func (i itemizer) Start(w io.Writer, name, tag string) {
	if tag != "" {
		name = html.EscapeString(tag)
	}
	if name != "" {
		fmt.Fprintf(w, "<li><b>%s</b>:\n", name)
	} else {
		fmt.Fprintf(w, "<li>\n")
	}
}

func (i itemizer) End(w io.Writer, name, tag string) {
	fmt.Fprintf(w, "\n")
}

func (i itemizer) StartGrp(w io.Writer) {
	fmt.Fprintf(w, "<ul>\n")
}

func (i itemizer) EndGrp(w io.Writer) {
	fmt.Fprintf(w, "</ul>\n")
}

func (i divider) Show(w io.Writer, s string, val ...interface{}) {
	fmt.Fprintf(w, "<span>")
	fmt.Fprintf(w, s, val...)
	fmt.Fprintf(w, "</span>")
}

func (i divider) Start(w io.Writer, name, tag string) {
	if tag != "" {
		name = html.EscapeString(tag)
	}
	if name != "" {
		fmt.Fprintf(w, "<div ><b>%s</b>:\n", name)
	} else {
		fmt.Fprintf(w, "<div >\n")
	}
}

func (i divider) End(w io.Writer, name, tag string) {
	fmt.Fprintf(w, "</div>\n")
}

func (i divider) StartGrp(w io.Writer) {
	fmt.Fprintf(w, "%s\n", `<div class="grp">`)
}

func (i divider) EndGrp(w io.Writer) {
	fmt.Fprintf(w, "</div>\n")
}

/*
	Make part a Presenter. This is not for you to call.
*/
func (p *Part) ShowAt(w io.Writer, nm string) error {
	if p.name == "" {
		p.name = nm
	}
	p.w = w
	err := p.eval(p.cmd)
	p.napplies++
	return err
}

/*
	Make part a Presenter. This is not for you to call.
*/
func (p *Part) Id() string {
	return p.name + "_0"
}

func (p *Part) xshow(w io.Writer, ei interface{}, nm string) error {
	if s, ok := ei.(Controller); ok {
		s.Mux(p.Evc, p.Updc)
	}
	if s, ok := ei.(Presenter); ok {
		return s.ShowAt(w, nm)
	}
	return p.show(w, ei, nm)
}

/*
	If an element implements this interface, Refresh is called before
	showing the item.
*/
type Refresher interface {
	Refresh()
}

func (p *Part) show(w io.Writer, ei interface{}, nm string) error {
	xp := p // make sure we are a Presenter and don't use p
	p = nil // except to call show again

	if xp.fmt == nil {
		panic("no fmt")
	}
	if rei, ok := ei.(Refresher); ok {
		rei.Refresh()
	}
	v := reflect.ValueOf(ei)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface,
		reflect.Map, reflect.Ptr, reflect.Slice:
		if v.IsNil() {
			xp.fmt.Show(w, "-")
			return nil
		}
	}
	switch v.Kind() {
	case reflect.Bool:
		xp.fmt.Show(w, "%v", v.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64:
		xp.fmt.Show(w, "%v", ei)
	case reflect.Interface, reflect.Ptr:
		el := v.Elem()
		return xp.xshow(w, el.Interface(), nm)
	case reflect.String:
		xp.fmt.Show(w, "%s", html.EscapeString(v.String()))
	case reflect.Chan:
		xp.fmt.Show(w, "?")
	case reflect.Array, reflect.Slice:
		xp.fmt.StartGrp(w)
		for i := 0; i < v.Len(); i++ {
			elname := "" // fmt.Sprintf("%dth", i)
			xp.fmt.Start(w, elname, "")
			el := v.Index(i).Interface()
			err := xp.xshow(w, el, elname)
			if err != nil {
				return err
			}
			xp.fmt.End(w, elname, "")
		}
		xp.fmt.EndGrp(w)
	case reflect.Map:
		xp.fmt.StartGrp(w)
		keys := v.MapKeys()
		for i := 0; i < len(keys); i++ {
			var buf bytes.Buffer
			err := xp.xshow(&buf, keys[i].Interface(), "")
			if err != nil {
				return err
			}
			elname := buf.String()
			xp.fmt.Start(w, elname, "")
			el := v.MapIndex(keys[i]).Interface()
			if err = xp.xshow(w, el, elname); err != nil {
				return err
			}
			xp.fmt.End(w, elname, "")
		}
		xp.fmt.EndGrp(w)
	case reflect.Struct:
		xp.fmt.StartGrp(w)
		nf := v.NumField()
		t := v.Type()
		for i := 0; i < nf; i++ {
			fld := t.Field(i)
			r, _ := utf8.DecodeRuneInString(fld.Name)
			if !unicode.IsUpper(r) {
				continue
			}
			xp.fmt.Start(w, fld.Name, string(fld.Tag))
			el := v.Field(i).Interface()
			err := xp.xshow(w, el, fld.Name)
			if err != nil {
				return err
			}
			xp.fmt.End(w, fld.Name, "")
		}
		xp.fmt.EndGrp(w)
	}
	return nil
}

func (p *Part) eval(c *cmd) error {
	if c == nil {
		return fmt.Errorf("nil cmd")
	}
	switch c.op {
	case tShow, tEdit: // show or edit elem
		ei, en, err := p.lookup(c.e)
		dprintf("eval: %s %s %s\n", c.op, c.e, en)
		if err != nil {
			return err
		}
		if err := p.xshow(p.w, ei, en); err != nil {
			return err
		}
	case tDo: // block
		dprintf("eval: %s{\n", c.op)
		for i := 0; i < len(c.cmds); i++ {
			if err := p.eval(c.cmds[i]); err != nil {
				return err
			}
		}
		dprintf("eval: }%s\n", c.op)
	case tText: // just show it
		dprintf("eval: %s %s\n", c.op, c.txt)
		fmt.Fprintf(p.w, "%s", c.txt)
	case tFor:
		dprintf("eval: for %s in elem<%s> do ", c.txt, c.e)
		old, ok := p.env[c.txt]
		defer func() {
			if ok {
				p.env[c.txt] = old
			} else {
				delete(p.env, c.txt)
			}
		}()
		ei, _, err := p.lookup(c.e)
		if err != nil {
			return err
		}
		cc := make(chan interface{})
		go loop(ei, cc)
		for ei := range cc {
			p.env[c.txt] = ei
			dprintf("eval: loop on %s", c.txt)
			for i := 0; i < len(c.cmds); i++ {
				if err := p.eval(c.cmds[i]); err != nil {
					close(cc, err)
					return err
				}
			}
		}
	case tIf:
		dprintf("eval: if %s do  ", c.e)
		ei, _, err := p.lookup(c.e)
		if err != nil {
			return err
		}
		if istrue(ei) {
			return p.eval(c.cmds[0])
		}
		if len(c.cmds) == 1 {
			return nil
		}
		return p.eval(c.cmds[1])
	default:
		return fmt.Errorf("eval: op %s", c.op)
	}
	dprintf("eval %s ok\n", c.op)
	return nil
}
