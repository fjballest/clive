/*
	Create authentication keys for Clive.

		usage: auth -options name user secret
		  -d="$HOME/.ssh": clive auth dir
		  -f=false: force write of key file

	Creates a key file at the clive auth dir for the authdomain name
	and user given, containing the key corresponding to the given secret.

	Under flag -f it rewrites the key file even if it exists.
*/
package main

import (
	"clive/app"
	"clive/app/opt"
	"clive/net/auth"
	"os"
)

var (
	dir   string
	force bool
	opts  = opt.New("name user secret [group...]")
)

func main() {
	defer app.Exiting()
	os.Args[0] = "auth"
	app.New()
	dfltdir := auth.KeyDir()
	dir = dfltdir
	opts.NewFlag("d", "adir: clive auth dir", &dir)
	opts.NewFlag("f", "force write of key file when file already exists", &force)
	args, err := opts.Parse(os.Args)
	if err != nil {
		app.Warn("%s", err)
		opts.Usage()
		app.Exits(err)
	}
	if len(args) < 3 {
		opts.Usage()
		app.Exits("usage")
	}
	name, user, secret := args[0], args[1], args[2]
	groups := args[3:]
	file := auth.KeyFile(dir, name)
	fi, _ := os.Stat(file)
	if fi!=nil && !force {
		app.Fatal("key file already exists")
	}
	err = auth.SaveKey(dir, name, user, secret, groups...)
	if err != nil {
		app.Fatal("%s: %s", file, err)
	}
	ks, err := auth.LoadKey(dir, name)
	if err != nil {
		app.Fatal("can't load key: %s", err)
	}
	for _, k := range ks {
		if k.Uid == user {
			app.Warn("%s", file)
			return
		}
	}
	app.Fatal("bad user")
}
