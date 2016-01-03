package zx

import (
	"errors"
	"strings"
)

// Popular errors
var (
	ErrNotExist  = errors.New("no such file or directory")
	ErrExists    = errors.New("file already exists")
	ErrIsDir     = errors.New("file is a directory")
	ErrNotDir    = errors.New("not a directory")
	ErrPerm      = errors.New("permission denied")
	ErrBug       = errors.New("buggered or not implemented")
	ErrNotEmpty  = errors.New("directory not empty")
	ErrRO        = errors.New("resource is read-only")
	ErrBadCtl    = errors.New("bad ctl request")
	ErrNotSuffix = errors.New("not an inner path")
	ErrBadType   = errors.New("bad file type")
)

func IsNotExist(e error) bool {
	if e == nil {
		return false
	}
	if e == ErrNotExist {
		return true
	}
	s := e.Error()
	return strings.Contains(s, "does not exist") ||
		strings.Contains(s, "no such file") ||
		strings.Contains(s, "not found")
}

func IsNotEmpty(e error) bool {
	if e == nil {
		return false
	}
	if e == ErrNotEmpty {
		return true
	}
	s := e.Error()
	return strings.Contains(s, "not empty")
}

func IsExists(e error) bool {
	if e == nil {
		return false
	}
	if e == ErrExists {
		return true
	}
	s := e.Error()
	return strings.Contains(s, "already exists") ||
		strings.Contains(s, "file exists")
}

func IsPerm(e error) bool {
	if e == nil {
		return false
	}
	if e == ErrPerm {
		return true
	}
	s := e.Error()
	return strings.Contains(s, "permission denied")
}
