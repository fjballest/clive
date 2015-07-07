package refs

import (
	"testing"
	"os"
	"strings"
	"clive/app"
	"clive/app/nsutil"
)

func TestLoad(t *testing.T) {
	BibTexOk = false
	c := app.New()
	defer app.Exiting()
	c.Debug = testing.Verbose()
	b, err:= Load("/zx/lib/bib")
	if err != nil {
		t.Fatalf("load: %s", err)
	}
	if testing.Verbose() {
		b.WriteTo(os.Stdout)
	}

	app.Dprintf("cite plan 9 networks:\n")
	refs := b.Cites("plan", "9", "networks")
	for _, r := range refs {
		app.Dprintf("got:\n%s\n", strings.Join(r.Reference(), "\n"))
	}
	if len(refs) != 2 {
		t.Fatalf("wrong count for plan 9 networks at lsub bib")
	}
}

func TestBib2ref(t *testing.T) {
	c := app.New()
	defer app.Exiting()
	c.Debug = testing.Verbose()
	lnc := nsutil.GetLines("/zx/lib/bib/zx.bib")
	rc := bib2ref(lnc)
	for ln := range rc {
		app.Dprintf("%s", ln)
	}
}

func TestBibLoad(t *testing.T) {
	BibTexOk = true
	c := app.New()
	defer app.Exiting()
	c.Debug = testing.Verbose()
	b, err:= Load("/zx/lib/bib")
	if err != nil {
		t.Fatalf("load: %s", err)
	}
	if testing.Verbose() {
		b.WriteTo(os.Stdout)
	}

	app.Dprintf("cite plan 9 networks:\n")
	refs := b.Cites("VisageFS")
	for _, r := range refs {
		app.Dprintf("got:\n%s\n", strings.Join(r.Reference(), "\n"))
	}
	if len(refs) == 0 {
		t.Fatalf("did not find visage in bib")
	}
}

