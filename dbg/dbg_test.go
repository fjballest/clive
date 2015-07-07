package dbg

import (
	"os"
)

func ExampleAtExit() {
	// run this when the program exists.
	// Requires that main defers dbg.Exits()

	AtExit(func() {
		os.Remove("/temp/file")
	})
}

func ExampleFuncPrintf() {
	var debug bool
	var dprintf = FlagPrintf(os.Stderr, &debug)

	dprintf("debug msg\n")

}

func ExampleTrace() {
	// Put this at the start of the function myfunc to trace it
	defer Trace(Call("myfunc"))
}
