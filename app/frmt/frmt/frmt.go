/*
	UNIX command for clive's frmt
*/
package main

import (
	"clive/app"
	"clive/app/frmt"
)

func main() {
	defer app.Exiting()
	app.New()
	app.SetIO(app.OSIn(), 0)
	frmt.Run()
	app.Exits(nil)
}
