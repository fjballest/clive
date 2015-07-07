package syscallx

import (
	"errors"
)

/* This is the source file for syscallx_darwin_*.go, to regenerate run

   ./generate

*/

// cannot use dest []byte here because OS X getxattr really wants a
// NULL to trigger size probing, size==0 is not enough
//
//sys getxattr(path string, attr string, dest *byte, size int, position uint32, options int) (sz int, err error)

func Getxattr(path string, attr string, dest []byte) (sz int, err error) {
	return 0, errors.New("xattr not implemented for openbsd")
}

//sys listxattr(path string, dest []byte, options int) (sz int, err error)

func Listxattr(path string, dest []byte) (sz int, err error) {
	return 0, errors.New("xattr not implemented for openbsd")
}

//sys setxattr(path string, attr string, data []byte, position uint32, flags int) (err error)

func Setxattr(path string, attr string, data []byte, flags int) (err error) {
	return errors.New("xattr not implemented for openbsd")
}

//sys removexattr(path string, attr string, options int) (err error)

func Removexattr(path string, attr string) (err error) {
	return errors.New("xattr not implemented for openbsd")
}
