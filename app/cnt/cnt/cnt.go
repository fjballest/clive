/*
	UNIX command for clive's cnt
*/
package main

import (
	"clive/app"
	"clive/app/cnt"
)

func main() {
	defer app.Exiting()
	app.New()
	app.SetIO(app.OSIn(), 0)
	cnt.Run()
	app.Exits(nil)
}
