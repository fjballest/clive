package zx

import (
	"clive/net/auth"
	"fmt"
)

// To check with 0111 | 0222 | 0444.
func (d Dir) Can(ai *auth.Info, what int) bool {
	if d == nil {
		return true
	}
	mode := int(d.Mode())
	if ai.InGroup(d["Uid"]) {
		return mode&what != 0
	}

	if ai.InGroup(d["Gid"]) {
		return mode&what&077 != 0
	}

	return mode&what&07 != 0
}

// can this auth info get data from this file?
// If ai is nil, owner's role is assumed.
func (d Dir) CanGet(ai *auth.Info) bool {
	return d.Can(ai, 0444)
}

// can this auth info put data in this file?
// If ai is nil, owner's role is assumed.
func (d Dir) CanPut(ai *auth.Info) bool {
	return d.Can(ai, 0222)
}

// can this auth info exec this file?
// If ai is nil, owner's role is assumed.
func (d Dir) CanExec(ai *auth.Info) bool {
	return d.Can(ai, 0111)
}

// can this auth info walk this file?
// If ai is nil, owner's role is assumed.
func (d Dir) CanWalk(ai *auth.Info) bool {
	return d.Can(ai, 0111)
}

// can this auth info do this wstat to this file?
// If ai is nil, owner's role is assumed.
// path, addr, type, wuid, and name are never updated
// so they are ignored.
// Only the owner can udpate the mode
// Updating the size is ok if CanPut(), and it's ignored for directories.
// The owner can update the group or the owner if it's a
// member of the target owner/group.
// The owner and group members may update other attributes
func (d Dir) CanWstat(ai *auth.Info, nd Dir) error {
	if ai == nil && nd["size"] != "" && d["type"] == "d" {
		return fmt.Errorf("size: %s", ErrIsDir)
	}
	if len(d) == 0 || len(nd) == 0 || ai == nil {
		return nil
	}
	isowner := ai.InGroup(d["uid"])
	for k, v := range nd {
		if d[k] == v {
			continue
		}
		switch k {
		case "path", "addr", "type", "wuid", "name":
			// ignored
		case "mode":
			if !isowner {
				return fmt.Errorf("mode: %s", ErrPerm)
			}
		case "size":
			if !d.CanPut(ai) {
				return fmt.Errorf("size: %s", ErrPerm)
			}
		case "uid", "gid":
			if !isowner || (!ai.InGroup(v) && !ai.InGroup("elf")) {
				return fmt.Errorf("%s: %s", k, ErrPerm)
			}
		case "mtime":
			fallthrough
		default:
			if !isowner && !ai.InGroup(d["gid"]) {
				return fmt.Errorf("size: %s", ErrPerm)
			}

		}
	}
	return nil
}
