package wax

import (
	"bytes"
	"clive/dbg"
	"fmt"
	"os"
	"testing"
)

var printf = dbg.FuncPrintf(os.Stdout, testing.Verbose)

const port = ":9191"

type tree struct {
	Name, Path string "rw"
	unexported int
	Peers      []string
}

type repl struct {
	Debug bool "rw"
	Trees []*tree
}

const tpg = `
		$t.Name$
		$t.Path$
		<ul>
		$for p in t.Peers do$
			<li>
			$p$
		$end$
		</ul>
`

const pg = `
	<article>
	<h1> Replica tool at $port$ </h1>
	<p>
	Flag: $debug$
	<p>
	<ul>
	$for t in repl.Trees do$
		<li>
		$t.Name$
		$t.Path$
		<ul>
		$for p in t.Peers do$
			<li>
			$p$
		$end$
		</ul>
	$end$
	</ul>
	<p>AAA
		$p1$
	<p>BBB
		$show repl.Trees[0].Peers[1]$
	<p>CCC
	$if debug do$
		debug is set
	$else$
		debug is cleared
	$end$
	<p>DDD
	$repl$

	<p>DDD<br>
	$u$
	</article>
`

var p1out = `
		<i>t2</i>
		<i>p2</i>
		<ul>
		
			<li>
			<i>t2</i>
		
		</ul>
`

var p2out = `
	<article>
	<h1> Replica tool at <i>3333</i> </h1>
	<p>
	Flag: <i>true</i>
	<p>
	<ul>
	
		<li>
		<i>t1&lt;p ?&#39;&#39;</i>
		<i>p1</i>
		<ul>
		
			<li>
			<i>t2</i>
		
			<li>
			<i>t4</i>
		
		</ul>
	
		<li>
		<i>t2</i>
		<i>p2</i>
		<ul>
		
			<li>
			<i>t2</i>
		
		</ul>
	
		<li>
		<i>t3</i>
		<i></i>
		<ul>
		
		</ul>
	
	</ul>
	<p>AAA
		
		<i>t2</i>
		<i>p2</i>
		<ul>
		
			<li>
			<i>t2</i>
		
		</ul>

	<p>BBB
		<i>t4</i>
	<p>CCC
	
		debug is set
	
	<p>DDD
	<ul>
<li><b>rw</b>:
<i>true</i>
<li><b>Trees</b>:
<ul>
<li>
<ul>
<li><b>rw</b>:
<i>t1&lt;p ?&#39;&#39;</i>
<li><b>rw</b>:
<i>p1</igo: exit 1
>
<li><b>Peers</b>:
<ul>
<li>
<i>t2</i>
<li>
<i>t4</i>
</ul>

</ul>

<li>
<ul>
<li><b>rw</b>:
<i>t2</i>
<li><b>rw</b>:
<i>p2</i>
<li><b>Peers</b>:
<ul>
<li>
<i>t2</i>
</ul>

</ul>

<li>
<ul>
<li><b>rw</b>:
<i>t3</i>
<li><b>rw</b>:
<i></i>
<li><b>Peers</b>:
<i>-</i>
</ul>

</ul>

</ul>


	<p>DDD<br>
	<ul>
<li><b>Name</b>:
<i>another ui</i>
<li><b>Sub</b>:

		<i>t2</i>
		<i>p2</i>
		<ul>
		
			<li>
			<i>t2</i>
		
		</ul>

</ul>

	</article>
`

type ui struct {
	Name string
	Sub  *Part
}

func TestPart(t *testing.T) {
	r := repl{
		Debug: true,
		Trees: []*tree{
			{
				Name:  "t1<p ?''",
				Path:  "p1",
				Peers: []string{"t2", "t4"},
			},
			{
				Name:  "t2",
				Path:  "p2",
				Peers: []string{"t2"},
			},
			{
				Name: "t3",
			},
		},
	}

	vars1 := map[string]interface{}{
		"t": r.Trees[1],
	}
	p1, err := New(tpg)
	if err != nil {
		dbg.Fatal("new part: %s", err)
	}
	p1.SetEnv(vars1)
	var buf1 bytes.Buffer
	if err := p1.apply(&buf1); err != nil {
		t.Fatalf("apply part: %s", err)
	}
	printf("p1: {%s}\n", buf1.String())
	if buf1.String() != p1out {
		t.Fatalf("wrong p1 output")
	}
	u := ui{"another ui", p1}
	vars := map[string]interface{}{
		"debug": &r.Debug,
		"port":  3333,
		"repl":  r,
		"p1":    p1,
		"u":     u,
	}
	var buf2 bytes.Buffer
	if err := applyNew(&buf2, pg, vars); err != nil {
		t.Fatalf("new part: %s", err)
	}
	printf("p2: {%s}\n", buf2.String())
	if buf2.String() != p2out {
		fmt.Printf("wrong? p2 output: was [%s]", buf2.String())
	}

}
