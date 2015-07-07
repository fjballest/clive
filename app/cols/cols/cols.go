/*
	UNIX command for clive's cols
*/
package main

import (
	"clive/app"
	"clive/app/cols"
)

func main() {
	defer app.Exiting()
	app.New()
	app.SetIO(app.OSIn(), 0)
	cols.Run()
	app.Exits(nil)
}
