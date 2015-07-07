/*
	UNIX command for clive's wr
*/
package main

import (
	"clive/app"
	"clive/app/wr"
	"os"
)

func main() {
	os.Stdin.Close()
	defer app.Exiting()
	app.New()
	app.Close(0)
	wr.Run()
	app.Exits(nil)
}
