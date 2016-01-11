package fstest

import (
	"clive/mblk/rwtest"
	"clive/zx"
	"io"
)

interface testTree {
	zx.Putter
	zx.Getter
	zx.Wstater
	zx.Remover
}

// Wrapper for a ZX Tree to make it look close enough to an OS file.
struct file {
	path string
	t    testTree
	off  int64
}

// Create a file (with operations close to OS.file) for a zx file.
// It has usual read, readat, writeat calls that can be used
// by rwtest tests and other regression tests wrt OS files.
func newFakeOSfile(t testTree, path string) *file {
	return &file{path: path, t: t}
}

func (tf *file) Seek(off int64, whence int) (int64, error) {
	switch whence {
	case 0:
		tf.off = off
	case 1:
		tf.off += off
	case 2:
		st, err := zx.Stat(tf.t, tf.path)
		if err != nil {
			return 0, err
		}
		tf.off = st.Size() + off
	}
	return tf.off, nil
}

func (tf *file) ReadAt(data []byte, off int64) (int, error) {
	dc := tf.t.Get(tf.path, off, int64(len(data)))
	tot := 0
	for d := range dc {
		copy(data[tot:], d)
		tot += len(d)
	}
	if tot == 0 && cerror(dc) == nil {
		return tot, io.EOF
	}
	return tot, cerror(dc)
}

func (tf *file) Read(data []byte) (int, error) {
	n, err := tf.ReadAt(data, tf.off)
	tf.off += int64(n)
	return n, err
}

func (tf *file) Truncate(sz int64) error {
	d := zx.Dir{}
	d.SetSize(sz)
	rc := tf.t.Wstat(tf.path, d)
	<-rc
	return cerror(rc)
}

func (tf *file) WriteAt(data []byte, off int64) (int, error) {
	dc := make(chan []byte, 1)
	dc <- data
	close(dc)
	rc := tf.t.Put(tf.path, nil, off, dc)
	<-rc
	err := cerror(rc)
	if err == nil {
		return len(data), nil
	}
	return 0, err
}

func AsAFile(t Fataler, fs zx.Fs) {
	xfs, ok := fs.(testTree)
	if !ok {
		t.Fatalf("tree is not putter/getter/wstatter/remover")
	}
	dat := []byte{}
	if err := zx.PutAll(xfs, "/testfile", dat); err != nil {
		t.Fatalf("put all: %s", err)
	}
	zxf := newFakeOSfile(xfs, "/testfile")
	rwtest.AsAFile(t, zxf, 1000, 128*1024, 3803)
}
