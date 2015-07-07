/*
	UNIX command for clive's rem
*/
package main

import (
	"clive/app"
	"clive/app/rem"
)

func main() {
	defer app.Exiting()
	app.New()
	rem.Run()
	app.Exits(nil)
}
