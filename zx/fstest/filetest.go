package fstest

import (
	"clive/zx"
	"strconv"
	"clive/bufs/rwtest"
	"io"
)

// Wrapper for a ZX Tree to make it look close enough to an OS file.
type File struct {
	path string
	t zx.RWTree
	off int64
}

// Create a File (with operations close to OS.File) for a zx file.
// It has usual read, readat, writeat calls that can be used
// by rwtest tests and other regression tests wrt OS Files.
func NewFakeOSFile(t zx.RWTree, path string) *File {
	return &File{path: path, t: t}
}

func (tf *File) Seek(off int64, whence int) (int64, error) {
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
		tf.off = st.Int64("size") + off
	}
	return tf.off, nil
}

func (tf *File) ReadAt(data []byte, off int64) (int, error) {
	dc := tf.t.Get(tf.path, off, int64(len(data)), "")
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

func (tf *File) Read(data []byte) (int, error) {
	n, err := tf.ReadAt(data, tf.off)
	tf.off += int64(n)
	return n, err
}

func (tf *File) Truncate(sz int64) error {
	d := zx.Dir{"size": strconv.FormatInt(sz, 10)}
	return <- tf.t.Wstat(tf.path, d)
}

func (tf *File) WriteAt(data []byte, off int64) (int, error) {
	dc := make(chan []byte, 1)
	dc <- data
	close(dc)
	rc := tf.t.Put(tf.path, nil, off, dc, "")
	<-rc
	err := cerror(rc)
	if err == nil {
		return len(data), nil
	}
	return 0, err
}

func AsAFile(t Fataler, fss ...zx.Tree) {
	if len(fss) == 0 {
		t.Fatalf("no fs given")
	}
	fs := fss[0].(zx.RWTree)
	dat := []byte{}
	if err := zx.PutAll(fs, "/testfile", zx.Dir{"mode": "0640"}, dat); err != nil {
		t.Fatalf("put all: %s", err)
	}
	zxf := NewFakeOSFile(fs, "/testfile")
	rwtest.AsAFile(t, zxf, 1000, 128 * 1024, 3803)
}
