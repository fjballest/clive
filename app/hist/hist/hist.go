/*
	UNIX command for clive's hist
*/
package main

import (
	"clive/app"
	"clive/app/hist"
)

func main() {
	defer app.Exiting()
	app.New()
	app.SetIO(app.OSIn(), 0)
	hist.Run()
	app.Exits(nil)
}
