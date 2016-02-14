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
			Line: `lf -u`,
			Out: `d rwxr-xr-x      0 /tmp/cmdtest
- rw-r--r--      0 /tmp/cmdtest/1
- rw-r--r--  30.9k /tmp/cmdtest/2
d rwxr-xr-x      0 /tmp/cmdtest/a
d rwxr-xr-x      0 /tmp/cmdtest/d
d rwxr-xr-x      0 /tmp/cmdtest/e
`,
			Err: ``,
		},
		test.Run{
			Line: `lf -u /fsdfsd`,
			Out:  ``,
			Err: `lf: stat /fsdfsd: no such file or directory
`,
			Fails: true,
		},
		test.Run{
			Line: `lf -gu 2 |sed 10q`,
			Out: `- rw-r--r--  30.9k /tmp/cmdtest/2
/2 0
/2 1
/2 2
/2 3
/2 4
/2 5
/2 6
/2 7
/2 8
`,
			Err: ``,
		},
		test.Run{
			Line: `lf -gu 2 |tail -10`,
			Out: `/2 4086
/2 4087
/2 4088
/2 4089
/2 4090
/2 4091
/2 4092
/2 4093
/2 4094
/2 4095
`,
			Err: ``,
		},
	}
)

func TestLf(t *testing.T) {
	debug = testing.Verbose()
	test.InstallCmd(t)
	test.Cmds(t, runs)
}
