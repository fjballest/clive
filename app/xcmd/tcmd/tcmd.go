/*
	test command for xcmd.

	Issues a message each second for the given number of seconds.
*/
package main

import (
	"clive/dbg"
	"os"
	"strconv"
	"time"
)

func main() {
	defer dbg.Exits("")
	os.Args[0] = "tcmd"
	n := 10
	if len(os.Args) > 1 {
		n, _ = strconv.Atoi(os.Args[1])
	}
	for i := 0; i < n; i++ {
		dbg.Warn("T %d", i)
		time.Sleep(time.Second)
	}
}
