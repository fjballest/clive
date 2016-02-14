package main

import (
	"clive/cmd/test"
	"clive/dbg"
	"testing"
)

var (
	debug   bool
	dprintf = dbg.FlagPrintf(&debug)

	runs = []test.Run{
		test.Run{
			Line: `echo ☺ | rf | cnt -u`,
			Out: `       1        1        1        2        4  in
`,
		},
		test.Run{
			Line: `(echo z☺ ; echo a b) | rf | cnt -u`,
			Out: `       1        2        3        7        9  in
`,
		},
	}
)

func TestLf(t *testing.T) {
	debug = testing.Verbose()
	test.InstallCmd(t)
	test.Cmds(t, runs)
}
