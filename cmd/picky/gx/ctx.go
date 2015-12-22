package gx

import (
	"bufio"
	"clive/cmd/picky/paminstr"
	"fmt"
	"io"
	"math"
	"strings"
)

type call struct {
	name   string
	args   []interface{}
	setval interface{}
}

type Context struct {
	cursor string
	v      []call
}

//many thanks to kyle consalus

func (c call) WriteTo(name string, out *bufio.Writer) {
	out.WriteString(name)
	out.WriteByte('.')
	out.WriteString(c.name)
	if c.setval == nil {
		out.WriteByte('(')
		for i, v := range c.args {
			if i != 0 {
				out.WriteByte(',')
			}
			fmt.Fprintf(out, "%#v", v)
		}
		out.WriteString(")")
	} else {
		fmt.Fprintf(out, "=%#v", c.setval)
	}
	out.WriteString(";\n")
}

func (c *Context) call(fname string, args ...interface{}) {
	c.v = append(c.v, call{name: fname, args: args})
}

func (c *Context) Save() {
	c.call("save")
}

func (c *Context) Restore() {
	c.call("restore")
}

func (c *Context) Set(propname string, value interface{}) {
	c.v = append(c.v, call{name: propname, setval: value})
}

func (c *Context) BeginPath() {
	c.call("beginPath")
}

func (c *Context) ClosePath() {
	c.call("closePath")
}

func (c *Context) ClearRect(x0, y0, x1, y1 float32) {
	c.call("clearRect", x0, y0, x1, y1)
}

func (c *Context) StrokeRect(x0, y0, x1, y1 float32) {
	c.call("strokeRect", x0, y0, x1, y1)
}

func (c *Context) Fill() {
	c.call("fill")
}

func (c *Context) Stroke() {
	c.call("stroke")
}

func (c *Context) Arc(x, y float32, radius float32, a0, a1 float32, conn bool) {
	c.call("arc", x, y, radius, a0, a1, conn)
}

func (c *Context) MoveTo(x, y float32) {
	c.call("moveTo", x, y)
}

func (c *Context) LineTo(x, y float32) {
	c.call("lineTo", x, y)
}

func (c *Context) FillText(text string, x, y float32) {
	c.call("fillText", text, x, y)
}

func (c *Context) Rotate(angle float32) {
	c.call("rotate", angle)
}

func (c *Context) Scale(x, y float32) {
	c.call("scale", x, y)
}

func (c *Context) Translate(x, y float32) {
	c.call("translate", x, y)
}

func (c *Context) WriteTo(out io.Writer) {
	w := bufio.NewWriter(out)
	for _, v := range c.v {
		v.WriteTo("g", w)
	}
	w.Flush()
}

func (c *Context) circle(x, y float32, radius float32) {
	c.Save()
	c.Arc(x, y, radius, 0, 2*math.Pi, true)
	c.Restore()
}

func (c *Context) ellipse(x, y float32, radiusX float32, radiusY float32, rotateAngle float32) {
	max := radiusX
	if radiusY > radiusX {
		max = radiusY
	}
	c.Save()
	c.Save()
	c.Translate(x, y)
	c.Rotate(rotateAngle)
	c.Scale(radiusX/max, radiusY/max)
	c.BeginPath()
	c.Arc(0, 0, max, 0, 2*math.Pi, true)
	c.ClosePath()
	c.Fill()
	c.Restore()
	c.Stroke()
	c.Restore()
}

func (c *Context) Cursor(s string) {
	c.cursor = s
}

const (
	TextHt    = 3.0
	TextScale = 1.350
)

func (c *Context) TextHeight(size int) int {
	return int(TextHt * float32(size) / 6.0)
}

func (c *Context) TextWidth(size int) int {
	return int(float32(size))
}

type TextPos struct {
	start, end int //index in array in ctx structure
	x, y       int //text position
	angle      float32
}

func (c *Context) Text(size int, text string, angle float32) *TextPos {
	c.Rotate(angle)
	tp := &TextPos{}
	c.Set("font", fmt.Sprintf("%dpt Consolas", int(TextScale*float32(size)*PixelX)))
	tp.start = len(c.v) - 1
	s := strings.Split(text, paminstr.EOL)
	ht := c.TextHeight(size)
	yrel := 0
	for _, v := range s {
		if v != "" {
			c.call("fillText", v, 0, yrel)
		}
		yrel += ht
		tp.x = int(float32(len(v)*c.TextWidth(size)) * PixelX)
	}
	tp.end = len(c.v) - 1
	nextl := strings.HasSuffix(text, paminstr.EOL)
	if nextl {
		tp.x = 0
	} else {
		yrel -= ht
	}
	tp.y = yrel
	return tp
}

func (c *Context) AddText(tp *TextPos, size int, text string) *TextPos {
	if c.v[tp.end].name != "fillText" && c.v[tp.end].name != "font" {
		panic("cannot add text at " + c.v[tp.end].name)
	}
	s := tp.start
	e := tp.end
	nn := make([]call, len(c.v[e+1:]))
	copy(nn, c.v[e+1:])
	c.v = c.v[:e+1]
	ht := c.TextHeight(size)
	yrel := tp.y
	xrel := tp.x
	lstsz := 0
	str := strings.Split(text, paminstr.EOL)
	for _, v := range str {
		if v != "" {
			e++
			c.call("fillText", v, xrel, yrel)
		}
		yrel += ht
		xrel = 0
		lstsz = int(float32(len(v)*c.TextWidth(size)) * PixelX)
	}
	c.v = append(c.v, nn...)
	nextl := strings.HasSuffix(text, paminstr.EOL)
	if nextl {
		xrel = 0
	} else {
		xrel = lstsz + tp.x
		yrel -= ht
	}
	return &TextPos{start: s, end: e, x: xrel, y: yrel}

}
