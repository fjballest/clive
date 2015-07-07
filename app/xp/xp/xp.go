/*
	UNIX command for clive's xp
*/
package main

import (
	"clive/app"
	"clive/app/xp"
)

func main() {
	defer app.Exiting()
	app.New()
	app.SetIO(app.OSIn(), 0)
	xp.Run()
	app.Exits(nil)
}
