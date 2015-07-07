/*
	UNIX command for clive's flds
*/
package main

import (
	"clive/app"
	"clive/app/flds"
)

func main() {
	defer app.Exiting()
	app.New()
	app.SetIO(app.OSIn(), 0)
	flds.Run()
	app.Exits(nil)
}
