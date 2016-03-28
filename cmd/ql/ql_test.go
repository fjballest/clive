package main

import (
	"clive/cmd"
	"clive/cmd/test"
	"clive/dbg"
	"testing"
)

var (
	debug   bool
	dprintf = dbg.FlagPrintf(&debug)

	runs = []test.Run{
		test.Run{
			Line: `echo ☺`,
			Out: `☺
`,
		},
		test.Run{
			Line: "echo `{}pquoted]`",
			Out: `{}pquoted]
`,
		},
		test.Run{
			Line: `echo a \
			b`,
			Out: `a b
`,
		},
		test.Run{
			Line: `echo a ; echo b`,
			Out: `a
b
`,
		},
		test.Run{
			Line: `x=(a b c) ; echo $x`,
			Out: `a b c
`,
		},
		test.Run{
			Line: `x=(a b c) ; echo (z)^$x`,
			Out: `za zb zc
`,
		},
		test.Run{
			Line: `x=(a b c) ; echo (z)^$^x`,
			Out: `za b c
`,
		},
		test.Run{
			Line: `x=(a b c) ; echo $x^(d e e)`,
			Out: `ad be ce
`,
		},
		test.Run{
			Line: `x=(a b c) ; echo $#x ; echo $x[1] ; echo $x[10]`,
			Out: `3
b

`,
		},
		test.Run{
			Line: `x=(a b c) ; x[2] = z ; x[3] = z; echo $x`,
			Out: `a b z z
`,
		},
		test.Run{
			Line: `z = ([a b] [c] [d e f]) ; echo $z ; echo $#z ; echo $z[a]`,
			Out: `a c d
3
b
`,
		},
		test.Run{
			Line: `z = ([a b] [c] [d e f]) ; x=$z[d]; echo $x $#x`,
			Out: `e f 2
`,
		},
		test.Run{
			Line: `z = ([a b] [c] [d e f]) ; z[x] = (a b c) ; echo $z ; echo $z[x]`,
			Out: `a c d x
a b c
`,
		},
		test.Run{
			Line: `echo ☺ | rf | cnt -u`,
			Out: `       1        1        1        2        4  in
`,
		},
		test.Run{
			Line: `rf <2 | cnt -lu`,
			Out: `    4096  in
`,
		},
		test.Run{
			Line: `rf <2 | 
				cnt -lu`,
			Out: `    4096  in
`,
		},
		test.Run{
			Line: `rf <2 >/tmp/3 ; rf <2 >>/tmp/3 ; cnt -lu </tmp/3`,
			Out: `    8192  in
`,
		},
		test.Run{
			Line: `rf <[in] 2 >[out]/tmp/3 ; cnt -lu </tmp/3`,
			Out: `    4096  in
`,
		},
		test.Run{
			Line: `{echo a ; echo b} | wc -l`,
			Out: `       2
`,
		},
		test.Run{
			Line: `{lf -u 1 ; lf -u fdsafdsfa } >[out,err]/tmp/errs ; cat /tmp/errs`,
			Out: `- rw-r--r--      0 /tmp/cmdtest/1
lf: stat /tmp/cmdtest/fdsafdsfa: no such file or directory
lf: stat /tmp/cmdtest/fdsafdsfa: no such file or directory
`,
		},
		test.Run{
			Line: `rf <2 >/tmp/3 ; rf <2 >>[out,err]/tmp/3 ; cnt -lu </tmp/3`,
			Out: `    8192  in
`,
		},
		test.Run{
			// BUG? race here?
			Line: `{ rf } <[in] 2 >[out]/tmp/3 ; cnt -lu </tmp/3`,
			Out: `    4096  in
`,
		},
		test.Run{
			Line: `echo ☺ | { rf } | cnt -u`,
			Out: `       1        1        1        2        4  in
`,
		},
		test.Run{
			Line: `for x a b c { for y c d e { echo $x $y }}`,
			Out: `a c
a d
a e
b c
b d
b e
c c
c d
c e
`,
		},
		test.Run{
			Line: `for x a b c { for y c d e { echo $x $y }} > /tmp/3 ; cnt -lu /tmp/3`,
			Out: `       9  /tmp/3
`,
		},
		test.Run{
			Line: `{echo 1 2 ; echo 3 4}  | rf | words | for x { echo got $x }`,
			Out: `got 1
got 2
got 3
got 4
`,
		},
		test.Run{
			Line: `{echo 1 2 ; echo 3 4}  | rf | lns | for x { echo got $x }`,
			Out: `got 1 2
got 3 4
`,
		},
		test.Run{
			Line: `{echo 1 2 ; echo 3 4}  | rf | all | for x { echo got $x }`,
			Out: `got 1 2
3 4
`,
		},
		test.Run{
			Line: `cond { true ;  echo c ; false ; echo a } or { echo b } or {echo x} `,
			Out: `c
b
`,
		},
		test.Run{
			Line: `x =3; x = <{expr $x + 1 | rf} ; echo $x`,
			Out: `4
`,
		},
		test.Run{
			Line: `while test 33 -lt 4 { echo y }`,
			Out:  ``,
		},
		test.Run{
			Line: `x=1; while test $x -lt 4 { echo $x; x =<{expr $x + 1 | rf} }`,
			Out: `1
2
3
`,
		},
		test.Run{
			Line: `pf <[in2]{seq 5|rf} <[in3]{seq 6 |rf}`,
			Out: `c ---------      0 |<in2
1
2
3
4
5
c ---------      0 |<in3
1
2
3
4
5
6
`,
		},
		test.Run{
			Line: `fn f { echo x $argv0 $#argv $argv y } ; f a b c ; f c d e `,
			Out: `x f 3 a b c y
x f 3 c d e y
`,
		},
		test.Run{
			Line: `echo $argv0 $argv`,
			Out: `ql -c echo $argv0 $argv
`,
		},
	}
)

func TestQl(t *testing.T) {
	debug = testing.Verbose()
	for i, r := range runs {
		runs[i].Line = "ql -c '" + r.Line + "'"
	}
	test.InstallCmd(t)
	test.Cmds(t, runs)
}

func TestPath(t *testing.T) {
	t.Logf("path %v", cmd.GetEnvList("path"))
	t.Logf("path %v", cmd.GetEnv("PATH"))
	t.Logf("path %v", cmd.Path())
	if p := cmd.LookPath("sh"); p != "/bin/sh" {
		t.Fatalf("sh is %q\n", p)
	}
	if p := cmd.LookPath("./sh"); p != "" {
		t.Fatalf("sh is %q\n", p)
	}
}
