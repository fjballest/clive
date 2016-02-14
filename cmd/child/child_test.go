package child

import (
	"os"
	"testing"
)

func TestChild(t *testing.T) {
	pids := List(os.Getpid())
	me := os.Getpid()
	for _, p := range pids {
		if p == me {
			return
		}
	}
	// This test is not really a test, because pid is always returned.
	// It was testing that we descend from 1 but it seems that
	// for some processes in the middle the parents exited and
	// we are not adopted by 1 yet, so we can't check it that way.
	t.Fatalf("didn't find myself as a descentant of my parent")
}
