// +build !darwin !linux
package fuse

import (
	"os"
)

func mount(dir string, ready chan<- struct{}, errp *error) (fusefd *os.File, err error) {
	panic("not implemented")
}
