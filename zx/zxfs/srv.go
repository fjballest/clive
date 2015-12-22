package zxfs

import (
	fs "clive/fuse"
	"clive/x/bazil.org/fuse"
	"clive/zx"
	"fmt"
)

// Serve the tree for FUSE and mount it at the given path.
// Returns when unmounted.
func MountServer(t zx.Tree, mntdir string) error {
	zfs, err := New(t)
	if err != nil {
		return fmt.Errorf("new zxfs: %s", err)
	}
	c, err := fuse.Mount(mntdir)
	if err != nil {
		return fmt.Errorf("mount zxfs: %s", err)
	}
	defer c.Close()
	err = fs.Serve(c, zfs)
	if err != nil {
		return fmt.Errorf("serve zxfs: %s", err)
	}
	<-c.Ready
	if err := c.MountError; err != nil {
		return fmt.Errorf("mount error: %s", err)
	}
	return nil
}
