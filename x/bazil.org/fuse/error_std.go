// +build !darwin,!openbsd

package fuse

var errnoNames = map[Errno]string{
	ENOSYS:  "ENOSYS",
	ESTALE:  "ESTALE",
	ENOENT:  "ENOENT",
	EIO:     "EIO",
	EPERM:   "EPERM",
	EINTR:   "EINTR",
	ENODATA: "ENODATA",
}

func translateGetxattrError(err Error) Error {
	return err
}
