package zx

import (
	"fmt"
	"path"
	"strings"
)

// Make sure s is an absolute path and return it cleaned and never empty.
func UseAbsPath(s string) (string, error) {
	if len(s) == 0 || s[0] != '/' {
		return "", fmt.Errorf("'%s' is not an absolute path", s)
	}
	return path.Clean(s), nil
}

// Return path elements, empty for /
func Elems(p string) []string {
	p = path.Clean(p)
	if p == "/" {
		return []string{}
	}
	if p[0] == '/' {
		p = p[1:]
	}
	return strings.Split(p, "/")
}

// Return the suffix of p relative to base
// Both paths must be absolute or both relative.
// Neither can be empty.
// If there's no such suffix, the empty string is returned.
// The suffix starts with '/' and is "/" if b == p
func Suffix(p, pref string) string {
	if len(pref) == 0 || len(p) == 0 {
		return ""
	}
	p = path.Clean(p)
	if pref == "" {
		return p
	}
	pref = path.Clean(pref)
	if (pref[0] == '/') != (p[0] == '/') {
		return ""
	}
	if pref == "." || pref == "/" {
		return p
	}
	np := len(p)
	npref := len(pref)
	if np < npref {
		return ""
	}
	switch {
	case !strings.HasPrefix(p, pref):
		return ""
	case np == npref:
		return "/"
	case p[npref] != '/':
		return ""
	default:
		return p[npref:]
	}
}

