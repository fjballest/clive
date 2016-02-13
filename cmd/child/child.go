/*
	Report the set of pids for a command and all its descendants
*/
package child

import (
	"os/exec"
	"strconv"
	"strings"
)

func children(pid int) []int {
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(pid)).Output()
	if err != nil {
		return nil
	}
	toks := strings.Fields(string(out))
	var pids []int
	for _, t := range toks {
		if n, err := strconv.Atoi(t); err == nil {
			pids = append(pids, n)
		}
	}
	return pids
}

func descendants(pid int) []int {
	child := children(pid)
	nchild := make([]int, len(child))
	copy(nchild, child)
	n := len(child)
	for i := 0; i < n; i++ {
		nchild = append(nchild, descendants(child[i])...)
	}
	return nchild
}

// return the list of descendants for the given pid, including pid.
func List(pid int) []int {
	pids := []int{pid}
	return append(pids, descendants(pid)...)
}
