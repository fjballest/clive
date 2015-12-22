package zux

import (
	"testing"
	"clive/zx/fstest"
	"os"
)

const tdir = "/tmp/zx_test"

func runTest(t *testing.T, fn fstest.TestFunc) {
	os.Args[0] = "zux.test"
	fstest.Verb = testing.Verbose()
	fstest.MkTree(t, tdir)
	defer os.RemoveAll(tdir)

	fs, err := New(tdir)
	if err != nil {
		t.Fatal(err)
	}
	fn(t, fs)
}

func TestStats(t *testing.T) {
	runTest(t, fstest.Stats)
}

func TestGets(t *testing.T) {
	runTest(t, fstest.Gets)
}

func TestFinds(t *testing.T) {
	runTest(t, fstest.Finds)
}

func TestFindGets(t *testing.T) {
	runTest(t, fstest.FindGets)
}

func TestPuts(t *testing.T) {
	runTest(t, fstest.Puts)
}

func TestMkdirs(t *testing.T) {
	runTest(t, fstest.Mkdirs)
}

func TestRemoves(t *testing.T) {
	runTest(t, fstest.Removes)
}

func TestMoves(t *testing.T) {
	runTest(t, fstest.Moves)
}
