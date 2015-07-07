package ql

import (
	"testing"
	"clive/app"
	"clive/dbg"
	"os"
	"io"
)

func TestParseError(t *testing.T) {
	app.New()
	defer app.Exiting()
	ql := func() {
		app.AppCtx().Debug = testing.Verbose()
		Run()
	}
	x := app.Go(ql, "ql", "-n", "-c", "pw{d")
	<- x.Wait
	if x.Sts == nil {
		t.Logf("warn: ql didn't fail")
	}
}

func TestParseCmd(t *testing.T) {
	app.New()
	defer app.Exiting()
	ql := func() {
		app.AppCtx().Debug = testing.Verbose()
		Run()
	}
	x := app.Go(ql, "ql", "-n", "-c", "pwd")
	<- x.Wait
	if x.Sts != nil {
		t.Fatalf("did fail")
	}
}

/*
	`|gf ../jn/1 ../jn/2 | jn -1`,
	`|gf ../jn/1 ../jn/2 | srt -r 1n -r 2`,
	`|lf /tmp/t,~c | rem -n`,
	`|lf /tmp/t,type=-|mvm -cn /tmp `,
	`|gf ../echo,1 | trex -r  'aeiou' '' `,
	`|gf ../echo,1 | gr -x '(^[a-z][^\n]*$)' '(^}$)' | gr -xef Run | pf -x`,
	`|gf ../echo,1 | gr -xf  '(^[a-z][^\n]*\n)' '(^}\n)' | gr -xevf 'Run' | pf -x `,
	`|gf ../echo,1 | gr -x '(^[a-z][^\n]*$)' '(^}$)' | gr -xvef Run | pf -x`,
	`|gf ../echo,1 | gr -xf  '(^[a-z][^\n]*\n)' '(^}\n)' | gr -xef Run | srt -x | pf -sx`,
	`|gf /tmp/echo,1 | gx  '(^[a-z][^\n]*\n)' '(^}\n)' | gg Run | gv foobar | pf -x`,
	`|gf /tmp/echo,1 | gx  '(^[a-z][^\n]*\n)' '(^}\n)' | gg Run | 
		gp '~m.*&1' | trex -u | pf -x`,
*/
var cmds = []string {
/*
	`,1`,
	`,1|pf -l`,
	`/tmp | cd ; |pwd`
	`/tmp |cd |pf`,
	`/tmkkp |cd |pf`,
	`,- |> gr '^var '`,
	`{date;pwd}`,
	`|{date; pwd } | trex -u`,
	`exit oops`,
	`|exit oops`,
	`|sleep 5`,
	`-|trex -u`,
	`trex -u <`,
	`flag`,
	`flag +D; flag -D`,
	`type sleep`,
	`|echo <{.,1|pf -l}`,
	`|echo <<{.,1 |pf -l}`,
	`|echo <<<{.,1 |pf -l}`,
	`|echo ' quoted {}$
string'`,
	`|echo a >/tmp/a; |cat /tmp/a`, 
	`|echo a >>/tmp/a; |cat /tmp/a`, 
	`/tmp/a /fdsfds >[2]/tmp/c| pf >/tmp/b ; |cat /tmp/b; |cat /tmp/c`,
	`lf /tmp/a /fdsfds  |[21] pf >/tmp/b ; |cat /tmp/b`,
	`|ls /tmp/a /fdsfds  >[2=1] ; date`,
	`|echo a |> grep x  >[2]/dev/null | cat |[21] wc >[21] /tmp/b ; |cat /tmp/b`,
	`|echo script name is $argv0 and has $#argv args`,
	`v1←x ; |echo $v1`,
	`v1 = {a b c} ; |echo $v1 $v1[1] $#v1`,
	`v1 = {a b c} ; v1[2] = x; |echo $v1 $v1[1] $#v1`,
	`v1 = {a b c} ; v1[2] = <{|echo a b c}; |echo $v1 $v1[1] $#v1`,
	`v1 ={[temp]/tmp [home]/usr/foo} ; v1[temp] = {foo bar}; |echo $v1 X $v1[home] X $v1[temp] $#v1`,
	`{echo a ; echo b } &x; wait x`,
	`|echo $status`,
	`argv ={a b c}; for arg $argv {
	echo arg is $arg
} > /tmp/a &x; wait x; |cat /tmp/a`,
	`func testfn {
	ls /tmp y z
	echo ls status is $status
	echo testfn has $#argv args $argv
	 /fsfs
	status = {testfn: $status}
}
	|testfn a b c
	|echo x $status x
`,
	`{echo a ; echo  b} | >{
	pf
	trex -u
} ; |echo`,
	`/tmp/df, |> diffs  <|{|gf ../diffs,}`,
	`|: 3 + 3 '<' 2`,
	`. && /fsd >[2] /dev/null && /fdsfds || |echo x && sdsd && |echo x || |echo q && |echo w`,

	`. && /fsd >[2] /dev/null && {
	/fdsfds
} || |echo x && sdsd && {
	|echo x 
} || |echo q && /fdsfds && {
	echo y
} || {
	echo else
}
`,
	`,- | for file {
		echo file is $file
	}`,
	`,~fns |> words | for w {
		echo word is $w
	}`,
	`,~fns |> lns | for ln {
		echo line is $ln
		flds = <{echo $ln}
		echo $#flds fields:  $flds^'X'
	}`,
	`x = 3; while : $x '>' 0 {
		echo $x
		x = <{: $x - 1}
	}`,
*/
	`,~fns |> lns | for ln {
		echo line is $ln
		flds = <{echo $ln}
		echo $#flds fields:  $flds^'X'
	}`,
}

func TestCmds(t *testing.T) {
	os.Args[0] = "ql.test"
	app.Debug = testing.Verbose() && false
	app.Verb = testing.Verbose() && false
	app.New()
	app.AppCtx().Debug = testing.Verbose()
	dbg.ExitDumpsStacks = testing.Verbose()
	defer app.Exiting()
	inc := make(chan interface{}, 3)
	inc <- []byte("hi\n")
	inc <- []byte("there\n")
	close(inc)
	app.SetIO(inc, 0)
	ql := func() {
		app.AppCtx().Debug = testing.Verbose()
		Run()
	}
	for _, c := range cmds {
		args := []string{"ql", "-c", c}
		if testing.Verbose() {
			args = []string{"ql", "-X", "-c", c}
		}
		x := app.Go(ql, args...)
		<- x.Wait
		if x.Sts != nil {
			t.Logf("did fail with sts %v", x.Sts)
		}
	}
}

func TestParseFile(t *testing.T) {
	app.New()
	defer app.Exiting()
	ql := func() {
		app.AppCtx().Debug = testing.Verbose()
		Run()
	}
	x := app.Go(ql, "ql", "-n", "example")
	<- x.Wait
	if x.Sts != nil {
		t.Fatalf("did fail")
	}
}

func TestParseStdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %s", err)
	}
	fd, err := os.Open("example")
	if err != nil {
		t.Fatalf("ex: %s", err)
	}
	defer fd.Close()
	go func() {
		io.Copy(w, fd)
		w.Close()		
	}()
	os.Stdin = r
	app.New()
	defer app.Exiting()
	ql := func() {
		c := app.AppCtx()
		c.Debug = testing.Verbose()
		app.SetIO(app.OSIn(), 0)
		Run()
	}
	x := app.Go(ql, "ql", "-n")
	<- x.Wait
	if x.Sts != nil {
		t.Fatalf("did fail")
	}
}
