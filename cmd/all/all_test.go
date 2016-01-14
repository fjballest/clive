package main

import (
	"testing"
	"clive/dbg"
	"clive/cmd/test"
)

var (
	debug bool
	dprintf = dbg.FlagPrintf(&debug)

	runs = []test.Run{
		test.Run {
			Line: `all 1 2 | cnt -mu`,
			Out: `       0  1
       1  2
       1  total
`,
		},
		test.Run {
			Line: `all -1 1 2 | cnt -mu`,
			Out: `       1  in
`,
		},
	}
)

func TestLf(t *testing.T) {
	debug = testing.Verbose()
	test.InstallCmd(t)
	test.Cmds(t, runs)
}
