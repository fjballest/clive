/*
	UNIX command for clive's pf
*/
package main

import (
	"clive/app"
	"clive/app/pf"
)

func main() {
	defer app.Exiting()
	app.New()
	pf.Run()
	app.Exits(nil)
}
