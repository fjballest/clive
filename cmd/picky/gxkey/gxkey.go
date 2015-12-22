package main

//Testing the gx package, which needs a main

import (
	"clive/cmd/picky/gx"
	"fmt"
	"time"
)

//http://localhost:12347/picky
func main() {
	g := gx.OpenGraphics("key")
	b := make([]byte, 2)
	for {
		time.Sleep(50 * time.Millisecond)
		n, err := g.Read(b)
		fmt.Println(n, err, b)
	}
	g.Close()
}
