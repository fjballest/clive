package ostest

import (
	"testing"
	"strings"
)

const tdir = "/tmp/ostest_test"
const tdir2 = "/tmp/ostest_test2"

var xdiffs = []string {
	`/tmp/ostest_test2: chg /1 size 0 vs 50`,
	`/tmp/ostest_test2: chg /2 mode '-rw-r--r--' vs 'drwxr-x---'`,
	`/tmp/ostest_test2: add /2/n2`,
	`/tmp/ostest_test2: chg /a/a1 mtimes '11000000000' vs '1000000000'`,
	`/tmp/ostest_test2: chg /a/a2 mode '-rwxr-x---' vs '-rw-r--r--'`,
	`/tmp/ostest_test2: add /a/b/c`,
	`/tmp/ostest_test2: del /a/n`,
}

func TestDiff(t *testing.T) {
	RmTree(t, tdir)
	RmTree(t, tdir2)
	defer RmTree(t, tdir)
	defer RmTree(t, tdir2)
	MkTree(t, tdir)
	ResetTime()
	MkTree(t, tdir2)

	diffs, err := Diff(WithMtime, tdir, tdir2)
	for _, d := range diffs {
		printf("diff %s\n", d)
	}
	if len(diffs) != 0 {
		t.Fatalf("%d diffs", len(diffs))
	}
	if err != nil {
		t.Fatalf("err %s", err)
	}

	MkChgs(t, tdir)
	MkChgs2(t, tdir2)
	diffs, err = Diff(WithMtime, tdir, tdir2)
	if err != nil {
		t.Fatalf("err %s", err)
	}
	for _, d := range diffs {
		printf("\t`%s`,\n", d)
	}
	if strings.Join(xdiffs, "\n") != strings.Join(diffs, "\n") {
		t.Fatalf("wrong diffs")
	}
}

func TestAsAFile(t *testing.T) {
	RmTree(t, tdir)
	RmTree(t, tdir2)
	defer RmTree(t, tdir)
	defer RmTree(t, tdir2)
	MkTree(t, tdir)
	ResetTime()
	MkTree(t, tdir2)

	AsAFile(t, tdir, tdir2)
}

func TestStats(t *testing.T) {
	RmTree(t, tdir)
	RmTree(t, tdir2)
	defer RmTree(t, tdir)
	defer RmTree(t, tdir2)
	MkTree(t, tdir)
	ResetTime()
	MkTree(t, tdir2)

	Stats(t, tdir, tdir2)
}

func TestGets(t *testing.T) {
	RmTree(t, tdir)
	RmTree(t, tdir2)
	defer RmTree(t, tdir)
	defer RmTree(t, tdir2)
	MkTree(t, tdir)
	ResetTime()
	MkTree(t, tdir2)

	Gets(t, tdir, tdir2)
}

func TestPuts(t *testing.T) {
	RmTree(t, tdir)
	RmTree(t, tdir2)
	defer RmTree(t, tdir)
	defer RmTree(t, tdir2)
	MkTree(t, tdir)
	ResetTime()
	MkTree(t, tdir2)

	Puts(t, tdir, tdir2)
}

func TestMkdirs(t *testing.T) {
	RmTree(t, tdir)
	RmTree(t, tdir2)
	defer RmTree(t, tdir)
	defer RmTree(t, tdir2)
	MkTree(t, tdir)
	ResetTime()
	MkTree(t, tdir2)

	Mkdirs(t, tdir, tdir2)
}

func TestRemoves(t *testing.T) {
	RmTree(t, tdir)
	RmTree(t, tdir2)
	defer RmTree(t, tdir)
	defer RmTree(t, tdir2)
	MkTree(t, tdir)
	ResetTime()
	MkTree(t, tdir2)

	Removes(t, tdir, tdir2)
}

func TestWstats(t *testing.T) {
	RmTree(t, tdir)
	RmTree(t, tdir2)
	defer RmTree(t, tdir)
	defer RmTree(t, tdir2)
	MkTree(t, tdir)
	ResetTime()
	MkTree(t, tdir2)

	Wstats(t, tdir, tdir2)
}

func TestAll(t *testing.T) {
	RmTree(t, tdir)
	RmTree(t, tdir2)
	defer RmTree(t, tdir)
	defer RmTree(t, tdir2)
	MkTree(t, tdir)
	ResetTime()
	MkTree(t, tdir2)

	All(t, tdir, tdir2)
}

func TestFsTest(t *testing.T) {
	RmTree(t, tdir)
	RmTree(t, tdir2)
	defer RmTree(t, tdir)
	defer RmTree(t, tdir2)
	MkTree(t, tdir)
	ResetTime()
	MkTree(t, tdir2)

	AsAFs(t, tdir)
}
