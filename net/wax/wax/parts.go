package main

import (
	"clive/dbg"
	"clive/net/wax"
	"clive/net/wax/ctl"
)

type Kind int

const (
	PushOnly Kind = iota
	PullOnly
	FullSync
)

func (k Kind) String() string {
	kname := [...]string{"push", "pull", "sync"}
	return kname[int(k)]
}

type tree  {
	Name, Path string
	unexported int
	Sync       Kind "Synchronization kind"
	Peers      []string
}

type repl  {
	Debug bool
	Trees []*tree "Known trees"
}

var (
	evc = make(chan *wax.Ev)
	tTb = ctl.NewBar(
		evc,
		nil,
		ctl.Radio("Push"),
		ctl.Radio("Pull"),
		ctl.Radio("Sync"),
		ctl.Check("Debug"),
		ctl.Button("Go and sync"),
	)
	tEntry = ctl.NewEntry(evc, nil, "Your name?")
	tMnu   = ctl.NewMenu(evc, nil,
		ctl.Label("opt 1"),
		ctl.Label("opt 2"),
		ctl.Menu{
			Label: "opt 3",
			Opts: []interface{}{
				ctl.Label("opt 31"),
				ctl.Label("opt 32"),
			},
		},
	)
	tRepl = &repl{
		Debug: true,
		Trees: []*tree{
			{
				Name:  "absurd name``t1<p ?''",
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
	tCanvas = ctl.NewCanvas(evc, nil, 200, 100,
		[]string{"scale", "proportional"}, // "proportional" or anything else
		[]string{"fill", "grey"},
		[]string{"fillrect", "0", "0", "200", "100"},
		[]string{"fill", "black"},
		[]string{"line", "0", "0", "200", "100"},
	)
	tTxt = ctl.NewText(evc, nil, "A xample text")
)

func testCanvas() *wax.Part {
	p, err := wax.New("$tc$")
	if err != nil {
		dbg.Fatal("new part: %s", err)
	}
	p.SetEnv(map[string]interface{}{"tc": tCanvas})
	return p
}

func testText() *wax.Part {
	t := `- write a ws pg that knows how to display multiple parts with
	  drag and drop and resizes. Play embedding just a chunk of html
	  and later play with embedding something from a different url

	- write a canvas ctl
	- port the old canvas text frame using the canvas ctl
	- write an terminal variant that simply honors In,Out,Err chans
	  on a text frame.
	- use local storage to save the layout editing state for errors
	  and to recover those if desired
	- make sure we can share an inner part with controls multiple times
	 (ie not just repeating controls, but parts with controls, although
	 for this we should probably use iframes)

	- use tls for conns to the registry and peers
	- put in place some kind of auth
	- make the registry a hierarchy, so that we can have
	  registry islands and they sync to each other
	  make it use broadcast to discover other machines nearby
	  and propagate island information

`
	if err := tTxt.Ins([]rune(t), 0); err != nil {
		dbg.Fatal("txt ins: %s", err)
	}
	p, err := wax.New("$txt$")
	if err != nil {
		dbg.Fatal("new part: %s", err)
	}
	p.SetEnv(map[string]interface{}{"txt": tTxt})
	return p

}
func testTreeTb() *wax.Part {
	treepg := `
		$tb$ <br>
		$t.Name$
		$t.Path$
		<ul>
		$for p in t.Peers do$
			<li>
			$p$
		$end$
		</ul>
	`
	p, err := wax.New(treepg)
	if err != nil {
		dbg.Fatal("new part: %s", err)
	}
	tenv := map[string]interface{}{
		"tb": tTb,
		"t":  tRepl.Trees[1],
	}
	p.SetEnv(tenv)
	return p
}

func testFor() *wax.Part {
	forpg := `
		Trees:
		<ul>
		$for t in repl.Trees do$
			<li>
			Name: $t.Name$;
			Path: $t.Path$;
			Kind: $t.Sync$
			<ul>
			$for p in t.Peers do$
				<li>
				$p$
			$end$
			</ul>
		$end$
		</ul>
	`
	p, err := wax.New(forpg)
	if err != nil {
		dbg.Fatal("new part: %s", err)
	}
	tenv := map[string]interface{}{"repl": tRepl}
	p.SetEnv(tenv)
	return p
}

func testAdt() *wax.Part {
	p, err := wax.New(`$repl$`)
	if err != nil {
		dbg.Fatal("new part: %s", err)
	}
	p.SetEnv(map[string]interface{}{"repl": tRepl})
	return p
}

func testTb() *wax.Part {
	tbpg := `
		<p> Single entry<br>
		$entry$
		<p>
		A tool bar:<br>
		$tb$
	`
	p, err := wax.New(tbpg)
	if err != nil {
		dbg.Fatal("new part: %s", err)
	}
	tenv := map[string]interface{}{
		"entry": tEntry,
		"tb":    tTb,
	}
	p.SetEnv(tenv)
	return p
}
