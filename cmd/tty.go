// +build bsd darwin freebsd openbsd

package cmd

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func tput(w string) (int, error) {
	ifd, err := os.Open("/dev/tty")
	if err != nil {
		return 0, err
	}
	defer ifd.Close()
	ofd, err := os.Create("/dev/tty")
	if err != nil {
		return 0, err
	}
	defer ofd.Close()
	cmd := exec.Command("tput", w)
	cmd.Stdin = ifd
	cmd.Stderr = ofd
	tpout, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	out := strings.TrimSpace(string(tpout))
	return strconv.Atoi(string(out))
}

// Return cols of the current tty window
func Cols() (int, error) {
	return tput("cols")
}

// Return rows of the current tty window
func Rows() (int, error) {
	return tput("rows")
}
