package fstest

import (
	"clive/zx"
)

var (
	rms    = []string{"/d", "/e/f", "/e", "/a/a2"}
	badrms = []string{"/", "/xxzx", "/a"}
)

func Removes(t Fataler, xfs zx.Fs) {
	fs, ok := xfs.(zx.Remover)
	if !ok {
		t.Fatalf("not a Remover")
	}
	for i, p := range rms {
		Printf("rm #%d %s\n", i, p)
		rc := fs.Remove(p)
		err := <-rc
		if err != nil || cerror(rc) != nil {
			t.Fatalf("did fail")
		}
	}
	for i, p := range badrms {
		Printf("rm #%d %s\n", i, p)
		rc := fs.Remove(p)
		err := <-rc
		Printf("sts %v\n", err)
		if err == nil {
			t.Fatalf("didn't fail")
		}
	}
}
