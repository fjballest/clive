/*
	UNIX command for clive's jn
*/
package main

import (
	"clive/app"
	"clive/app/jn"
)

func main() {
	defer app.Exiting()
	app.New()
	jn.Run()
	app.Exits(nil)
}
