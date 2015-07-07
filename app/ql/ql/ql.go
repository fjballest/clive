/*
	unix command for clive ql
*/
package main

import (
	"clive/app"
	"clive/app/ql"
)

func main() {
	defer app.Exiting()
	app.New()
	app.Close(0)
	ql.Run()
	app.Exits(nil)
}
