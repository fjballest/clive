package gx

import (
	"clive/cmd/picky/paminstr"
	"clive/x/code.google.com/p/go.net/websocket"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	BLACK  = 0x000000
	WHITE  = 0xffffff
	GREEN  = 0x0ff00
	RED    = 0xff0000
	BLUE   = 0x0000ff
	YELLOW = 0xffff00
	ORANGE = 0xff9900
	PixelX = 0.5
	PixelY = 0.5

	PickyPort = 4242

	OPAQUE = 1.0
	TRANSP = 0.0
	TLUCID = 0.5

	DefaultXSz = 10000 //only matters if only goes seriously wrong
	DefaultYSz = 10000

	ReloadTime = 500 //UI disconnect timeout, needed for browser reload

	WaitForUiTime = 1000
	NWaitsForUi   = 10
	Debugjs       = false
)

type canvasRenderer struct {
	fn func() *Context
}

func regevent(evkind string) string {
	return fmt.Sprintf(`
					$( "#c" ).%s(function(event) {
						var x = Math.round(event.pageX- $("#c").offset().left) +  1
						var y = Math.round(event.pageY- $("#c").offset().top) +  1
						var ev = {X: x, Y: y, Which: event.which, Type: event.type, Meta: event.metaKey};
						ws.send(JSON.stringify(ev));
						//console.log(JSON.stringify(ev));
					}
				)`, evkind)
}

func (cr canvasRenderer) ServeHTTP(c http.ResponseWriter, req *http.Request) {
	var ctx *Context
	ctx = cr.fn()
	if req.URL.RawQuery == "js" {
		if Debugjs {
			fmt.Fprintf(os.Stdout, "---->")
			ctx.WriteTo(os.Stdout)
		}
		ctx.WriteTo(c)
		if ctx.cursor != "" {
			fmt.Fprintf(c, "\n$(\"#c\").css('cursor', '%s');", ctx.cursor)
		}
		return
	}

	fmt.Fprint(c, `<html><head>`)
	//In order not to depend on the network, I compile it in the binary...
	//fmt.Fprint(c, `<script src="http://ajax.googleapis.com/ajax/libs/jquery/1.4.2/jquery.min.js"></script>`)
	fmt.Fprintf(c, `<script>%s</script>`, jquery)
	fmt.Fprint(c,
		`<script>function init() {`,
		`var c = document.getElementById('c');`,
		`window['g'] = c.getContext('2d'); start();`)
	ctx.WriteTo(c)
	path := req.URL.Path
	fmt.Fprintf(c, `};`)
	fmt.Fprint(c,
		`</script></head>`,
		`<body onload="init()" onresize="resize()">`,
		`<center><canvas id="c" width=300 height=300></canvas>`)
	//no fading for now
	fmt.Fprintf(c, `<script>
				var nsteps = 3;
				var tstep = 20;
				function fadein(sound)  {
									sound.load();
     									sound.play();
									
				}`)
	fmt.Fprintf(c, `function fadeout(sound)  {
									sound.pause();
				}`)
	fmt.Fprintf(c, `function start()  {`)
	if req.URL.RawQuery != "js" {
		fmt.Fprintf(c, `
					var ws;
					var sound = null;
					if ("WebSocket" in window) {
						ws = new WebSocket("ws://localhost:%d/ws");
						ws.onmessage = function (evt){
							 if(evt.data.indexOf("done") == 0){
								var msg = evt.data.split(" ")[1].replace(/_/g, " ");
								ws.onclose = function(){};
								$('body').html('<center><h1>'+msg+'</h1></center>');
							}else if(evt.data.indexOf("play") == 0){
								var snd = evt.data.split(" ")[1]
								if(sound == null || sound.paused){
									sound = new Audio("http://localhost:4242/sound.mp3?"+snd);
									fadein(sound);
									sound.addEventListener('ended', function(){fadeout(sound)});
								}
							}else if(evt.data == "stop"){
								if(sound != null){
									fadeout(sound);
									sound = null;
								}
							}else{
								$.getScript('%s?js');
							}
						};
						ws.onclose = function(){ 
							$('body').html('<center><h1>Program Exited</h1></center>');
						};
						ws.onerror = function(){ 
							$('body').html('<center><h1>Program Exited</h1></center>');
						};
				`, PickyPort, path)
		fmt.Fprintf(c, `
					}else{
    						 // The browser doesn't support WebSocket
						alert("This browser does not support Websockets, need a modern browser");
						$('body').html('<h1>Program Exited</h1>');
						os.Exit(1);
					}
				`)
	}
	fmt.Fprintf(c, `
				$( "#c" ).attr("tabindex", "0")
				.focus();
			`)
	fmt.Fprintf(c, `
				$( "#c" ).bind('contextmenu', function(e) {
					return false;
					}
				);`)
	if ctx.cursor != "" {
		fmt.Fprintf(c, "\n$(\"#c\").css('cursor', '%s');", ctx.cursor)
	}
	fmt.Fprintf(c, regevent("keydown"))
	fmt.Fprintf(c, regevent("keyup"))
	fmt.Fprintf(c, regevent("keypress"))
	fmt.Fprintf(c, regevent("mousedown"))
	fmt.Fprintf(c, regevent("mouseup"))
	fmt.Fprintf(c, regevent("mousemove"))
	fmt.Fprintf(c, `
				c.width = $(window).width()-20;
				c.height = $(window).height()-20;
			`)
	fmt.Fprint(c, `};`)
	fmt.Fprintf(c, `function resize()  {`)
	fmt.Fprintf(c, `
            			var c = document.getElementById("c");
				if(c != null){
					c.width = $(window).width()-20;
					c.height = $(window).height()-20;
				}
			`)
	fmt.Fprint(c, `};`)
	fmt.Fprint(c,
		`</script></center>`,
		`</body></html>`)
}

type Key struct {
	k  byte
	sc byte
}

type Graphics struct {
	sync.Mutex
	name      string
	isclosed  bool
	haslostui bool
	seeneof   bool //readxxx returned eof
	ctx, zctx *Context
	ws        *websocket.Conn

	nkpresses, nmpresses int
	mbut                 int
	mx, my               int
	keyssc               map[byte]bool
	lastsc               byte

	lasttext *TextPos
	penwd    int

	keyq      []*Key
	keywaitc  chan<- bool
	unreadkey byte
	unreadsc  byte

	pencol  string
	fillcol string
}

var (
	graphics    *Graphics
	graphicsLck sync.Mutex
)

func (g *Graphics) Line(x1, y1, x2, y2 int) {
	g.Lock()
	defer g.Unlock()
	g.CheckOpen()
	g.zctx.BeginPath()
	g.zctx.Set("strokeStyle", g.pencol)
	g.zctx.Set("fillStyle", g.fillcol)
	g.zctx.Set("lineWidth", g.penwd*(PixelX+PixelY)/2)
	g.zctx.MoveTo(PixelX*float32(x1), PixelY*float32(y1))
	g.zctx.LineTo(PixelX*float32(x2), PixelY*float32(y2))
	g.zctx.ClosePath()
	g.zctx.Stroke()
	g.zctx.Fill()
}

func colfmt(color uint, opacity float32) string {
	b := color & 0xff
	g := (color >> 8) & 0xff
	r := (color >> 16) & 0xff
	colstr := fmt.Sprintf("rgba(%d,%d,%d,%g)", r, g, b, opacity)
	return colstr
}

func (g *Graphics) Ellipse(x, y, radx, rady int, angle float32) {
	g.Lock()
	defer g.Unlock()
	g.CheckOpen()
	g.zctx.Set("strokeStyle", g.pencol)
	g.zctx.Set("fillStyle", g.fillcol)
	g.zctx.Set("lineWidth", 2*g.penwd*(PixelX+PixelY)/2)
	g.zctx.ellipse(PixelX*float32(x), PixelY*float32(y), PixelX*float32(radx), PixelY*float32(rady), angle)
}

func (g *Graphics) SetPenCol(color uint, opacity float32) {
	colstr := colfmt(color, opacity)
	g.Lock()
	defer g.Unlock()
	g.CheckOpen()
	g.pencol = colstr
}

func (g *Graphics) SetPenRGB(r, gr, b byte, opacity float32) {
	colstr := fmt.Sprintf("rgba(%d,%d,%d,%g)", r, gr, b, opacity)
	g.Lock()
	defer g.Unlock()
	g.CheckOpen()
	g.pencol = colstr
}

func (g *Graphics) SetFillCol(color uint, opacity float32) {
	colstr := colfmt(color, opacity)
	g.Lock()
	defer g.Unlock()
	g.fillcol = colstr
}

func (g *Graphics) SetFillRGB(r, gr, b byte, opacity float32) {
	colstr := fmt.Sprintf("rgba(%d,%d,%d,%g)", r, gr, b, opacity)
	g.Lock()
	defer g.Unlock()
	g.fillcol = colstr
}

func (g *Graphics) SetPenWidth(width int) {
	g.Lock()
	defer g.Unlock()
	g.CheckOpen()
	g.penwd = width
}

func (g *Graphics) Clear() {
	g.Lock()
	defer g.Unlock()
	g.CheckOpen()
	*g.zctx = Context{}
	g.lasttext = nil
	g.zctx.ClearRect(0, 0, DefaultXSz, DefaultYSz)
}

func (g *Graphics) lostui(ws *websocket.Conn) {
	g.Lock()
	if g.ws == ws {
		g.ws = nil
	}
	g.Unlock()
	time.Sleep(time.Duration(ReloadTime) * time.Millisecond)
	l := &graphicsLck
	l.Lock()
	defer l.Unlock()
	g.Lock()
	if g.ws == nil {
		g.haslostui = true
		if g.keywaitc != nil {
			g.keywaitc <- true
		}
	}
	g.Unlock()
}

//After the wait we start loosing events
//this is an inevitable race if we want the progress to
//continue when there is no UI
func (g *Graphics) nouiyet() bool {
	var hasui bool
	for i := 0; i < NWaitsForUi; i++ {
		time.Sleep(time.Duration(WaitForUiTime/NWaitsForUi) * time.Millisecond)
		g.Lock()
		ws := g.ws
		hasui = ws != nil
		g.Unlock()
		if hasui {
			break
		}
	}
	return hasui
}

func (g *Graphics) Flush() (errret error) {
	g.Lock()
	ws := g.ws
	g.CheckOpen()
	*g.ctx = *g.zctx
	g.Unlock()
	if ws == nil && g.nouiyet() { //no UI yet
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			g.lostui(ws)
		}
	}()
	if _, err := ws.Write([]byte("refresh")); err != nil {
		ws.Close()
		g.lostui(ws)
		return nil
	}
	return nil
}

func cos(x float32) float32 {
	return float32(math.Cos(float64(x)))
}
func sin(x float32) float32 {
	return float32(math.Sin(float64(x)))
}

func (g *Graphics) Polygon(x, y, radius, nsides int, rotateAngle float32) {
	r := ((PixelX + PixelY) / 2) * float32(radius)
	sector := (math.Pi * 2.0) / float32(nsides)
	if nsides < 3 {
		panic("not enough sides")
	}
	g.Lock()
	defer g.Unlock()
	g.CheckOpen()
	zctx := g.zctx
	zctx.Save()
	zctx.Set("strokeStyle", g.pencol)
	zctx.Set("fillStyle", g.fillcol)
	zctx.BeginPath()
	zctx.Set("lineWidth", 2*g.penwd*(PixelX+PixelY)/2)
	zctx.Translate(PixelX*float32(x), PixelY*float32(y))
	rotateAngle += -math.Pi + math.Pi*float32(nsides-2)/2.0
	if nsides == 4 {
		rotateAngle += -math.Pi / 4.0
	}
	zctx.Rotate(rotateAngle)
	zctx.MoveTo(r, 0)
	for i := 1; i < nsides; i++ {
		zctx.LineTo(r*cos(sector*float32(i)), r*sin(sector*float32(i)))
	}
	zctx.ClosePath()
	zctx.Stroke()
	zctx.Fill()
	zctx.Restore()
}

func (g *Graphics) TextHeight() int {
	g.Lock()
	defer g.Unlock()
	g.CheckOpen()
	zctx := g.zctx
	return 3.0 * zctx.TextHeight(g.penwd)
}

func (g *Graphics) PosText(x, y int, rotateAngle float32) {
	g.Lock()
	defer g.Unlock()
	g.CheckOpen()
	g.lasttext = &TextPos{x: x, y: y, angle: rotateAngle}
}

func (g *Graphics) _Text(text string) {
	size := g.penwd
	tp := g.lasttext
	zctx := g.zctx
	zctx.Set("fillStyle", g.fillcol)
	zctx.Translate(PixelX*float32(tp.x), PixelY*float32(tp.y))
	sz := (PixelX + PixelY) * float32(size)
	size = int(sz)
	g.lasttext = zctx.Text(size, text, tp.angle)
}

func (g *Graphics) Text(text string) {
	g.Lock()
	defer g.Unlock()
	g.CheckOpen()
	size := g.penwd
	tp := g.lasttext
	zctx := g.zctx
	if tp == nil {
		g.lasttext = &TextPos{}
		tp = g.lasttext
	}
	if tp.end == 0 && tp.start == 0 { //never or just positioned
		zctx.Save()
		g._Text(text)
		zctx.Restore()
		return
	}
	g.lasttext = zctx.AddText(g.lasttext, size, text)
}

func (g *Graphics) Write(p []byte) (n int, err error) {
	g.Text(string(p))
	return len(p), nil
}

func NewGraphics(name string) (g *Graphics) {
	ctx := new(Context)
	ctx.ClearRect(0, 0, DefaultXSz, DefaultYSz)
	ctx.Save()
	zctx := new(Context)
	zctx.ClearRect(0, 0, DefaultXSz, DefaultYSz)
	g = new(Graphics)
	g.ctx = ctx
	g.zctx = zctx
	g.name = name
	g.keyssc = make(map[byte]bool, 1)
	return g
}

func (g *Graphics) Name() string {
	g.Lock()
	defer g.Unlock()
	return g.name
}

//should have the lock
func (g *Graphics) CheckOpen() {
	if g.isclosed {
		panic("g is closed")
	}
}

func wsdone(ws *websocket.Conn, msg string) {
	if _, err := ws.Write([]byte("done " + msg)); err != nil {
		ws.Close()
	}
}

func (g *Graphics) Close() {
	g.Lock()
	defer g.Unlock()
	g.isclosed = true
	if g.ws == nil {
		return
	}
	wsdone(g.ws, "Graphics_Closed")
	g.ws = nil
}

const (
	AsciiSpc = 0x20
	ASciiDel = 0x7f
	BadKey   = 0xfe
)

func (g *Graphics) deletesc(sc byte) {
	for i := 0; i < len(g.keyq); i++ {
		if g.keyq[i].sc == sc {
			g.keyq[i].k = paminstr.Nul
		}
	}
}

func (g *Graphics) ReadKeyPress() byte {
	g.Lock()
	if g.seeneof {
		panic("graphx read: already reported EOF")
	}
	defer g.Unlock()
	if g.haslostui {
		g.seeneof = true
		return paminstr.Eof
	}
	if g.nkpresses <= 0 {
		return paminstr.Nul
	}
	i := 0
	n := 0
	if len(g.keyssc) != 0 {
		n = rand.Intn(len(g.keyssc))
	}
	for v := range g.keyssc {
		if i == n {
			g.deletesc(v)
			return v
		}
		i++
	}
	return paminstr.Nul
}

func (g *Graphics) ReadKeyPresses(kr []byte) {
	var p []byte
	g.Lock()
	if g.seeneof {
		panic("graphx read: already reported EOF")
	}
	defer g.Unlock()
	if g.haslostui {
		kr[0] = paminstr.Eof
		g.seeneof = true
		for i := 1; i < len(kr); i++ {
			kr[i] = paminstr.Nul
		}
		return
	}
	if g.nkpresses <= 0 {
		return
	}
	nk := len(kr)
	if len(g.keyssc) < nk {
		nk = len(g.keyssc)
	}
	for v := range g.keyssc {
		p = append(p, v)
	}
	n := len(p)
	if n != 0 {
		j := rand.Intn(n)
		j = j % n
		for i := 0; i < nk; i++ {
			kr[i] = p[j]
			g.deletesc(kr[i])
			j = (1 + j) % n
		}
	}
	for i := nk; i < len(kr); i++ {
		kr[i] = paminstr.Nul
	}
}

func (g *Graphics) _Read(p []byte) (n int, err error, sc byte) {
	i, j, l := 0, 0, 0
	if g.seeneof {
		panic("graphx read: already reported EOF")
	}
	if g.haslostui {
		g.seeneof = true
		return 0, io.EOF, 0
	}
Retry:
	if len(g.keyq) == 0 {
		if g.keywaitc != nil {
			panic("only one reader!")
		}
		wc := make(chan bool, 1) //the buffering is just to decouple
		g.keywaitc = wc
		g.Unlock()
		<-wc
		g.Lock()
		g.keywaitc = nil
	}
	if g.seeneof || g.haslostui {
		g.seeneof = true
		return 0, io.EOF, 0
	}
	l = len(p)
	if l > len(g.keyq) {
		l = len(g.keyq)
	}
	i = 0
	j = 0
	for _, key := range g.keyq {
		if key.k != paminstr.Nul {
			p[i] = key.k
			sc = key.sc
			i++
		}
		j++
		if i == l {
			break
		}
	}
	copy(g.keyq, g.keyq[j:])
	nt := len(g.keyq)
	g.keyq = g.keyq[:nt-j]
	if i == 0 {
		goto Retry //was misled by Nul keys taken away non-blocking read
	}
	if len(g.keyq) == 0 {
		g.keyq = nil
	}
	return i, nil, sc
}

func (g *Graphics) Read(p []byte) (n int, err error) {
	g.Lock()
	defer g.Unlock()
	n, err, _ = g._Read(p)
	return n, err
}

func (g *Graphics) ReadRune() (r rune, n int, err error) {
	var (
		p  [1]byte
		sc byte
	)
	g.Lock()
	defer g.Unlock()
	n, err, sc = g._Read(p[:])
	if err != nil {
		return 0, n, err
	}
	r = rune(p[0])
	g.unreadkey = p[0]
	g.unreadsc = sc
	return r, 1, nil
}

func (g *Graphics) UnreadRune() error {
	g.Lock()
	defer g.Unlock()
	l := len(g.keyq)
	g.keyq = append(g.keyq, nil)
	copy(g.keyq[0:l], g.keyq[1:l+1])
	g.keyq[0] = &Key{k: g.unreadkey, sc: g.unreadsc}
	return nil
}

func (g *Graphics) ReadMouse(x, y, nbut *int) {
	g.Lock()
	defer g.Unlock()
	*x = g.mx
	*y = g.my
	*nbut = g.mbut
}

type JqEvent struct {
	Which int
	X     int
	Y     int
	Type  string
	Meta  bool
}

func scantokey(k uint, shift bool, meta bool) byte {
	if k >= 'A' && k <= 'Z' {
		return byte(k - 'A' + 'a')
	}
	switch {
	case k == '[' && meta: //meta (command or windows) key
		return paminstr.MetaLeft
	case k == ']' && meta: //meta (command or windows) key
		return paminstr.MetaRight
	}
	switch k {
	case '\t':
		return paminstr.Tab
	case 0xbc:
		k = ','
	case 0xbd:
		k = '-'
	case 0xbe:
		k = '.'
	case 0xbf:
		k = '/'
	case 0xc0:
		k = ','
	case 0xba:
		k = ';'
	case 0xbb:
		k = '='
	case 0xdb:
		k = '['
	case 0xdd:
		k = ']'
	case 0xde:
		k = '\''
	case 0x10:
		return paminstr.Shift
	case 0x11:
		return paminstr.Ctrl
	case '&':
		return paminstr.Up
	case '(':
		return paminstr.Down
	case '%':
		return paminstr.Left
	case '\'':
		return paminstr.Right
	case '\r':
		return paminstr.Return
	case 0x8:
		return paminstr.Del
	}
	if k >= AsciiSpc && k < ASciiDel {
		return byte(k)
	}
	return BadKey
}

var sndstr = []string{
	paminstr.Woosh:      "Woosh",
	paminstr.Beep:       "Beep",
	paminstr.Sheep:      "Sheep",
	paminstr.Phaser:     "Phaser",
	paminstr.Rocket:     "Rocket",
	paminstr.ANote:      "ANote",
	paminstr.AsharpNote: "AsharpNote",
	paminstr.BNote:      "BNote",
	paminstr.CNote:      "CNote",
	paminstr.CsharpNote: "CsharpNote",
	paminstr.DNote:      "DNote",
	paminstr.DsharpNote: "DsharpNote",
	paminstr.ENote:      "ENote",
	paminstr.FNote:      "FNote",
	paminstr.FsharpNote: "FsharpNote",
	paminstr.GNote:      "GNote",
	paminstr.GsharpNote: "GsharpNote",
	paminstr.Bomb:       "Bomb",
	paminstr.Fail:       "Fail",
	paminstr.Tada:       "Tada",
}

func (g *Graphics) Play(snd int) {
	sname := sndstr[snd]
	g.nouiyet()
	g.Lock()
	defer g.Unlock()
	if g.ws == nil { //no UI yet
		return
	}
	if _, err := g.ws.Write([]byte("play " + sname)); err != nil {
		g.ws.Close()
	}
	return
}

func (g *Graphics) Stop() {
	g.nouiyet()
	g.Lock()
	defer g.Unlock()
	if g.ws == nil { //no UI yet
		return
	}
	if _, err := g.ws.Write([]byte("stop")); err != nil {
		g.ws.Close()
	}
	return
}

func (g *Graphics) key(e JqEvent) {
	g.Lock()
	defer g.Unlock()
	k := scantokey(uint(e.Which), g.keyssc[paminstr.Shift], e.Meta)
	if e.Type == "keyup" {
		//fmt.Printf("keyup %v\n", e)
		g.nkpresses--
		if k != BadKey {
			delete(g.keyssc, k)
		}
		return
	}
	g.nkpresses++
	if k != BadKey && !g.keyssc[k] {
		//fmt.Printf("keydown %v\n", e)
		g.lastsc = k
		g.keyssc[k] = true
	}
}

func (g *Graphics) keypress(e JqEvent) {
	g.Lock()
	defer g.Unlock()
	k := e.Which

	//fmt.Printf("keypress %v\n", e)
	if k == '\r' {
		for _, v := range paminstr.EOL {
			g.keyq = append(g.keyq, &Key{byte(v), g.lastsc})
		}
		if g.keywaitc != nil {
			g.keywaitc <- true
		}
	} else if k >= AsciiSpc && k < ASciiDel {
		key := &Key{k: byte(k), sc: g.lastsc}
		g.keyq = append(g.keyq, key)
		if g.keywaitc != nil {
			g.keywaitc <- true
		}
	}
}

func (g *Graphics) mousebut(e JqEvent) {
	g.Lock()
	defer g.Unlock()
	if e.Type == "mousedown" {
		g.nmpresses++
	} else {
		g.nmpresses--
	}
	g.mx = e.X
	g.my = e.Y
	if g.nmpresses == 0 {
		g.mbut = 0
	} else {
		g.mbut = e.Which
	}
}

func (g *Graphics) mouse(e JqEvent) {
	g.Lock()
	defer g.Unlock()
	g.mx = e.X
	g.my = e.Y
}

func currentgraphics() *Graphics {
	l := &graphicsLck
	l.Lock()
	defer l.Unlock()
	g := graphics
	return g
}

func events(ws *websocket.Conn) {
	g := currentgraphics()
	defer func() {
		if r := recover(); r != nil {
			g.lostui(ws)
		}
	}()
	defer func() {
		g.lostui(ws)
		ws.Close()
	}()
	var e JqEvent
	for {
		if err := websocket.JSON.Receive(ws, &e); err != nil {
			break
		}
		if g == nil {
			continue
		}
		e.X = int(float32(e.X) / PixelX)
		e.Y = int(float32(e.Y) / PixelY)
		switch {
		case e.Type == "keypress":
			g.keypress(e)
		case strings.HasPrefix(e.Type, "key"):
			g.key(e)
		case e.Type == "mousemove":
			g.mouse(e)
		case strings.HasPrefix(e.Type, "mouse"):
			g.mousebut(e)
		}
		g = currentgraphics()
	}
}

func getHandler() *Context {
	c := new(Context)
	g := currentgraphics()
	if g == nil {
		return c
	}
	g.Lock()
	defer g.Unlock()
	*c = *g.ctx
	return c
}

// Echo the data received on the WebSocket.
func wshandle(ws *websocket.Conn) {
	hadui := false
	g := currentgraphics()

	g.Lock()
	oldws := g.ws
	g.ws = ws
	if oldws != nil {
		hadui = true //for now only one UI
	}
	lostui := g.haslostui
	g.Unlock()
	//allow for reconnect, disable this
	if false && lostui {
		wsdone(ws, "Lost_UI")
		return
	}
	if hadui {
		wsdone(oldws, "Another_UI_connected")
	}
	events(ws)
}

var sounds = map[string]*[]byte{
	"Woosh":      &woosh,
	"Beep":       &beep,
	"Sheep":      &sheep,
	"Phaser":     &phaser,
	"Rocket":     &rocket,
	"ANote":      &anote,
	"AsharpNote": &asharpnote,
	"BNote":      &bnote,
	"CNote":      &cnote,
	"CsharpNote": &csharpnote,
	"DNote":      &dnote,
	"DsharpNote": &dsharpnote,
	"ENote":      &enote,
	"FNote":      &fnote,
	"FsharpNote": &fsharpnote,
	"GNote":      &gnote,
	"GsharpNote": &gsharpnote,
	"Bomb":       &bomb,
	"Fail":       &fail,
	"Tada":       &tada,
}

func soundHandler(w http.ResponseWriter, r *http.Request) {
	w.Write(*sounds[r.URL.RawQuery])
}

func OpenGraphics(name string) *Graphics {
	l := &graphicsLck
	l.Lock()
	if graphics != nil {
		l.Unlock()
		panic("graphics already open")
	}
	graphics = NewGraphics(name)
	g := graphics
	l.Unlock()
	go func() {
		picky := &canvasRenderer{fn: getHandler}
		upath := fmt.Sprintf("/%s", name)
		http.HandleFunc("/sound.mp3", soundHandler)
		http.Handle(upath, picky)
		http.Handle("/ws", websocket.Handler(wshandle))
		err := http.ListenAndServe(fmt.Sprintf(":%d", PickyPort), nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "gopen: for \"%s\" UI program already running", name)
			os.Exit(1)
		}
		g.nouiyet()
	}()
	return g
}

func (g *Graphics) IsLost() bool {
	g.Lock()
	defer g.Unlock()
	g.CheckOpen()
	return g.haslostui
}

func (g *Graphics) Cursor(isvisib bool) {
	g.Lock()
	defer g.Unlock()
	if isvisib {
		g.zctx.Cursor("auto")
	} else {
		g.zctx.Cursor("none")
	}
}
