package fuse

import (
	"syscall"
)

const (
	ENODATA = Errno(syscall.ENOENT)
)

func init() {
	panic("openbsd not supported")
}

var errnoNames = map[Errno]string{
	ENOSYS: "ENOSYS",
	ESTALE: "ESTALE",
	ENOENT: "ENOENT",
	EIO:    "EIO",
	EPERM:  "EPERM",
	EINTR:  "EINTR",
}

func translateGetxattrError(err Error) Error {
	return err
}
