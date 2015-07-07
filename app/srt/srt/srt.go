/*
	UNIX command for clive's srt
*/
package main

import (
	"clive/app"
	"clive/app/srt"
)

func main() {
	defer app.Exiting()
	app.New()
	app.SetIO(app.OSIn(), 0)
	srt.Run()
	app.Exits(nil)
}
