package zx

import (
	"clive/dbg"
	"clive/net/auth"
	"fmt"
	"errors"
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

// can this auth info read this file?
// If ai is nil, owner's role is assumed.
func (d Dir) CanRead(ai *auth.Info) bool {
	return d.Can(ai, 0444)
}

// can this auth info write this file?
// If ai is nil, owner's role is assumed.
func (d Dir) CanWrite(ai *auth.Info) bool {
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

var errCantRemove = errors.New("attribute cannot be removed")

// can this wstat be performed?
// If ai is nil, owner's role is assumed.
// Use this to check that it look fine even under no permission checking.
// The user can set only mode, mtime, size, and attrs starting with Upper runes but
// for Wuid.
// Attributes that the user can't set are ignored.
// However if there are only attributes that the user can't set and they try to
// change the value, an error is granted.
// Mode can be changed only by the owner
// mtime can be changed by the owner and anyone with write permissions.
// size can be changed by anyone with write permissions.
// uid/gid can be changed only by the owner if also a member of the target id or elf.
// Other usr attrs can be changed only by the owner and anyone with write permissions.
func (d Dir) CanWstat(ai *auth.Info, nd Dir) error {
	if d == nil || nd == nil {
		return nil
	}
	isowner := ai.InGroup(d["Uid"])
	some := false
	somecant := ""
	for k, v := range nd {
		if !IsUsr(k) {
			if v == "" {
				return fmt.Errorf("%s: %s", k, errCantRemove)
			}
			somecant = k
			continue
		}
		if k == "size" && d["type"] == "d" && v != d[k] {
			somecant = k
			continue
		}
		some = true
		if v == d[k] {
			continue // no change really
		}
		switch k {
		case "mode":
			if v == "" {
				return fmt.Errorf("mode: %s", errCantRemove)
			}
			if !isowner {
				return fmt.Errorf("mode: %s", dbg.ErrPerm)
			}
		case "size":
			if v == "" {
				return fmt.Errorf("%s: %s", k, errCantRemove)
			}
			if !d.CanWrite(ai) {
				return fmt.Errorf("%s: %s", k, dbg.ErrPerm)
			}
		case "Uid", "Gid":
			if v == "" {
				return fmt.Errorf("%s: %s", k, errCantRemove)
			}
			if !isowner || (!ai.InGroup(v) && !ai.InGroup("elf"))  {
				return fmt.Errorf("%s: %s", k, dbg.ErrPerm)
			}
		case "mtime":
			if v == "" {
				return fmt.Errorf("%s: %s", k, errCantRemove)
			}
			fallthrough
		default:
			if !isowner && !d.CanWrite(ai) {
				return fmt.Errorf("%s: %s", k, dbg.ErrPerm)
			}
		}
	}
	if !some && somecant != "" {
		return  fmt.Errorf("%s: %s", somecant, dbg.ErrPerm)
	}
	return nil
}
