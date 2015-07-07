/*
	UNIX command for clive's lns
*/
package main

import (
	"clive/app"
	"clive/app/lns"
)

func main() {
	defer app.Exiting()
	app.New()
	app.SetIO(app.OSIn(), 0)
	lns.Run()
	app.Exits(nil)
}
