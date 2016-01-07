package zx

import (
	"fmt"
	"errors"
	"clive/net/auth"
	"io"
)

struct rofs {
	fs Fs
}

// Take a fs and make it read-only
// If fs supports get, find, findget, auth, sync, close, then
// the corresponding methods are forwarded, otherwise they
// exist but fail with errors.
func MakeRO(fs Fs) Getter {
	return rofs{fs}
}

func (ro rofs) String() string {
	return fmt.Sprintf("%s", ro.fs)
}

func (ro rofs) Stat(path string) <-chan Dir {
	return ro.fs.Stat(path)
}

func (ro rofs) Get(path string, off, count int64) <-chan []byte {
	if fs, ok := ro.fs.(Getter); ok {
		return fs.Get(path, off, count)
	}
	rc :=  make(chan []byte)
	close(rc, errors.New("RO fs is not not a "))
	return rc
}

func (ro rofs) Find(path, pred string, spref, dpref string, depth0 int) <-chan Dir {
	if fs, ok := ro.fs.(Finder); ok {
		return fs.Find(path, pred, spref, dpref, depth0)
	}
	rc :=  make(chan Dir)
	close(rc, errors.New("RO fs is not not a finder"))
	return rc
}

func (ro rofs) FindGet(path, pred string, spref, dpref string, depth0 int) <-chan interface{} {
	if fs, ok := ro.fs.(FindGetter); ok {
		return fs.FindGet(path, pred, spref, dpref, depth0)
	}
	rc :=  make(chan interface{})
	close(rc, errors.New("RO fs is not not a findgetter"))
	return rc
}

func (ro rofs) Auth(ai *auth.Info) (Fs, error) {
	if fs, ok := ro.fs.(Auther); ok {
		return fs.Auth(ai)
	}
	return nil, errors.New("RO fs is not not an auther")
}

func (ro rofs) Sync(ai *auth.Info) error {
	if fs, ok := ro.fs.(Syncer); ok {
		return fs.Sync()
	}
	return nil
}

func (ro rofs) Close() error {
	if fs, ok := ro.fs.(io.Closer); ok {
		return fs.Close()
	}
	return nil
}

