/*
	Create authentication keys for Clive.

	usage: auth [-f] [-d adir] name user secret [group...]
		-d adir: clive auth dir
		-f: force write of key file when file already exists

	Creates a key file at the clive auth dir for the authdomain name
	and user given, containing the key corresponding to the given secret.

	Under flag -f it rewrites the key file even if it exists.
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"clive/net/auth"
	"os"
)

var (
	dir   string
	force bool
	opts  = opt.New("name user secret [group...]")
)

func main() {
	cmd.UnixIO()
	dfltdir := auth.KeyDir()
	dir = dfltdir
	opts.NewFlag("d", "adir: clive auth dir", &dir)
	opts.NewFlag("f", "force write of key file when file already exists", &force)
	args := opts.Parse()
	if len(args) < 3 {
		opts.Usage()
	}
	name, user, secret := args[0], args[1], args[2]
	groups := args[3:]
	file := auth.KeyFile(dir, name)
	fi, _ := os.Stat(file)
	if fi != nil && !force {
		cmd.Fatal("key file already exists")
	}
	err := auth.SaveKey(dir, name, user, secret, groups...)
	if err != nil {
		cmd.Fatal("%s: %s", file, err)
	}
	ks, err := auth.LoadKey(dir, name)
	if err != nil {
		cmd.Fatal("can't load key: %s", err)
	}
	for _, k := range ks {
		if k.Uid == user {
			cmd.Warn("%s", file)
			return
		}
	}
	cmd.Fatal("bad user")
}
