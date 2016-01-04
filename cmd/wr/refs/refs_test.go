package refs

import (
	"clive/cmd"
	"os"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	BibTexOk = false
	c := cmd.AppCtx()
	c.Debug = testing.Verbose()
	b, err := Load("/zx/lib/bib")
	if err != nil {
		t.Fatalf("load: %s", err)
	}
	if testing.Verbose() {
		b.WriteTo(os.Stdout)
	}

	cmd.Dprintf("cite plan 9 networks:\n")
	refs := b.Cites("plan", "9", "networks")
	for _, r := range refs {
		cmd.Dprintf("got:\n%s\n", strings.Join(r.Reference(), "\n"))
	}
	if len(refs) != 2 {
		t.Fatalf("wrong count for plan 9 networks at lsub bib")
	}
}

func TestBib2ref(t *testing.T) {
	c := cmd.AppCtx()
	c.Debug = testing.Verbose()
	lnc := b2s(cmd.ByteLines(cmd.Get("/zx/lib/bib/zx.bib", 0, -1)))
	rc := bib2ref(lnc)
	for ln := range rc {
		cmd.Dprintf("%s", ln)
	}
}

func TestBibLoad(t *testing.T) {
	BibTexOk = true
	c := cmd.AppCtx()
	c.Debug = testing.Verbose()
	b, err := Load("/zx/lib/bib")
	if err != nil {
		t.Fatalf("load: %s", err)
	}
	if testing.Verbose() {
		b.WriteTo(os.Stdout)
	}

	cmd.Dprintf("cite plan 9 networks:\n")
	refs := b.Cites("VisageFS")
	for _, r := range refs {
		cmd.Dprintf("got:\n%s\n", strings.Join(r.Reference(), "\n"))
	}
	if len(refs) == 0 {
		t.Fatalf("did not find visage in bib")
	}
}
