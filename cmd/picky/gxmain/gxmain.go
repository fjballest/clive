package main

//Testing the gx package, which needs a main

import (
	"clive/cmd/picky/gx"
	"fmt"
	"math"
	"math/rand"
	"time"
)

func polygons(g *gx.Graphics) {
	for i := 6; i >= 3; i-- {
		g.Polygon(100, 100, 100, i, 0)
	}
}

func rainbow(g *gx.Graphics) {
	last := 4
	step := 1
	height := 200
	ht := 4*height

	g.SetPenWidth(200)
	for r := byte(0); r < 128; r += byte(step) {
		g.SetPenRGB(2*r, 255-r, 2*r, 1.0)
		g.Line(last, ht, last+16, ht)
		last = last + 8*step
	}
	last = 4
	ht += height
	for r := byte(0); r < 128; r += byte(step) {
		g.SetPenRGB(r+128, 2*r, 255-2*r, 1.0)
		g.Line(last, ht, last+16, ht)
		last = last + 8*step
	}
	last = 4
	ht += height
	for r := byte(0); r < 128; r += byte(step) {
		g.SetPenRGB(128-r, 128+r, 255-r, 1.0)
		g.Line(last, ht, last+16, ht)
		last = last + 8*step
	}

}

type Point  {
	x, y int
}

const (
	epssimpl  = 0.3
	epssmooth = 1.0
)

func (p Point) length() float32 {
	return float32(math.Sqrt(float64(p.x*p.x + p.y*p.y)))
}

type Fpoint  {
	x, y float32
}

func (p Fpoint) length() float32 {
	return float32(math.Sqrt(float64(p.x*p.x + p.y*p.y)))
}

func dtosegment(p1, pa, pb Point) float32 {
	d := Point{pb.x - pa.x, pb.y - pa.y}
	s := d.length()
	l := float32(d.x*(p1.x-pa.x))/s + float32(d.y*(p1.y-pa.y))/s
	if l <= 0.0 {
		l = 0.0
	} else if l >= 1.0 {
		l = 1.0
	}
	p := Fpoint{float32(pa.x) + l*float32(d.x), float32(pa.y) + l*float32(d.y)}
	dd := Fpoint{p.x - float32(p1.x), p.y - float32(p1.y)}
	return dd.length()
}

//douglas peucker algorithm
func simplify(pts []Point) []Point {
	var pts1, pts2, spts []Point
	dmax := float32(0)
	index := 0
	end := len(pts) - 1
	for i := 1; i < len(pts)-1; i++ {
		d := dtosegment(pts[i], pts[0], pts[end])
		if d > dmax {
			index = i
			dmax = d
		}
	}
	if dmax > epssimpl {
		pts1 = simplify(pts[0 : index+1])
		pts2 = simplify(pts[index:])
		spts = append(pts1[:len(pts1)-1], pts2...)
	} else {
		spts = pts[0:]
	}
	return spts
}

func smooth(pts []Point) []Point {
	var pts1, pts2, spts []Point
	if len(pts) < 3 {
		return pts
	}
	dmax := float32(0)
	index := 0
	end := len(pts) - 1
	for i := 1; i < len(pts)-1; i++ {
		d := dtosegment(pts[i], pts[0], pts[end])
		if d > dmax {
			index = i
			dmax = d
		}
	}
	if dmax > epssmooth {
		avpt1 := avg(pts[index], pts[index-1])
		avpt2 := avg(pts[index], pts[index+1])
		m := avg(avpt1, avpt2)
		m = avg(m, pts[index])
		pts1 = pts[0 : index-1]
		pts2 = pts[index+1:]
		pts1 = append(pts1, avpt1)
		pts1 = append(pts1, m)
		pts1 = append(pts1, avpt2)
		pts2 = smooth(pts2)
		spts = append(pts1, pts2...)
	} else {
		spts = pts
	}
	return spts
}

func head(pts []Point) (h []Point, tail []Point) {
	for i := 0; i < len(pts); i++ {
		if pts[i].x < 0 {
			return h, pts[i+1:]
		}
		h = append(h, pts[i])
	}
	return h, nil
}

func avg(p1, p2 Point) Point {
	return Point{(p1.x + p2.x)/2, (p1.y + p2.y)/2}
}

const Nsmooth = 6

var last int

func trace(g *gx.Graphics, pts []Point) {
	var hd []Point
	if len(pts) < 2 {
		return
	}
	tail := pts
	g.SetPenWidth(5)
	g.SetPenCol(gx.BLACK, 1.0)
	i := 0
	for {
		i++
		hd, tail = head(tail)
		if len(hd) == 0 {
			continue
		}
		if i>last && len(hd)>Nsmooth {
			hd = simplify(hd)
			frozen := hd[:len(hd)-Nsmooth]
			sm := hd[len(hd)-Nsmooth:]
			sm = smooth(sm)
			hd = append(frozen, sm...)
		}
		lpt := hd[0]
		for i := 1; i < len(hd); i++ {
			pt := hd[i]
			g.Line(lpt.x, lpt.y, pt.x, pt.y)
			lpt = pt
		}
		if len(tail) == 0 {
			return
		}
		last = i
	}
}

func redraw(g *gx.Graphics, pts []Point) {
	g.Clear()
	g.SetPenCol(gx.BLACK, 1.0)
	g.SetPenWidth(4)
	g.SetFillCol(gx.GREEN, 1.0)
	polygons(g)
	rainbow(g)
	trace(g, pts)
}

var randsrc *rand.Rand

const Nul = 0

//http://localhost:4242/test
func main() {
	var pts []Point
	randsrc = rand.New(rand.NewSource(0))
	g := gx.OpenGraphics("test")
	x, y, m := 0, 0, 0
	paint := true
	redraw(g, pts)
	g.Flush()
	for {
		time.Sleep(50*time.Millisecond)
		k := g.ReadKeyPress()
		if k != Nul {
			fmt.Printf("Read Key %c\t||\t", k)
		}
		g.ReadMouse(&x, &y, &m)
		if m == 1 {
			paint = true
			pts = append(pts, Point{x, y})
			fmt.Printf("Mouse x: %d y: %d m: %d\t||\t", x, y, m)
		} else if m == 2 {
			pts = nil
			paint = true
		} else if m == 3 {
			break
		} else if m==0 && paint {
			paint = false
			pts = append(pts, Point{-1, -1}) //insert stop
		}
		if paint {
			g.SetPenCol(gx.BLACK, 1.0)
			g.SetPenWidth(1)
			g.SetFillCol(gx.BLACK, 1.0)
			redraw(g, pts)
			g.SetPenCol(gx.BLACK, 1.0)
			g.SetPenWidth(30)
			g.SetFillCol(gx.BLACK, 1.0)
			g.PosText(x+30, y+30, -math.Pi/4)
			fmt.Fprintf(g, "[%d, %d]", x, y)
			fmt.Fprintf(g, "\n")
			fmt.Fprintf(g, "pelota\n")
			fmt.Fprintf(g, "123456789")
			g.SetPenWidth(1)
			g.SetPenCol(gx.YELLOW, 1.0)
			g.SetFillCol(gx.BLUE, 0.5)
			g.Ellipse(x, y, 30, 25, math.Pi*float32(m)/3.0)
			g.SetPenCol(gx.BLACK, 1.0)
			g.SetFillCol(gx.BLACK, 0.5)
			g.Ellipse(x+30, y+30, 2, 2, 0.0)
			g.Flush()
		}
	}
	g.Close()
}
