/*
	users
*/
package u

import (
	"strings"
	"os"
	"os/user"
	"runtime"
)

// Variables initialized at init time with the user, system, home directory
// and temporary directory names.
var (
	cwd string
	Uid = "none"
	Sys = "sargazos"
	Home = "/tmp"
	Tmp = "/tmp"
)

func init() {
	cwd, _ = os.Getwd()
	u, err := user.Current()
	if err == nil {
		Uid = strings.ToLower(u.Username)
		Home = u.HomeDir
	}
	s, err := os.Hostname()
	if err == nil {
		toks := strings.SplitN(s, ".", 2)
		Sys = strings.ToLower(toks[0])
	}
	t := os.TempDir()
	if t != "" && runtime.GOOS != "darwin" {
		Tmp = t
	}
}

