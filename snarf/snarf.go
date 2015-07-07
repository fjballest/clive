/*
	Access to the system clipboard.
*/
package snarf

// BUG(x): As of now, only for darwin

// REFERNCE(x): clive/wax/..., wax window system.

import (
	"os/exec"
	"sync"
)

var snarflk sync.Mutex

// Get the clipboard text
func Get() (string, error) {
	snarflk.Lock()
	defer snarflk.Unlock()
	cmd := exec.Command("/usr/bin/pbpaste")
	txt, err := cmd.Output()
	return string(txt), err
}

// Set the clipbard text
func Set(s string) error {
	snarflk.Lock()
	defer snarflk.Unlock()
	cmd := exec.Command("/usr/bin/pbcopy")
	ifd, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err = cmd.Start(); err != nil {
		ifd.Close()
		return err
	}
	if _, err = ifd.Write([]byte(s)); err != nil {
		ifd.Close()
		return err
	}
	ifd.Close()
	cmd.Wait()
	return nil
}
