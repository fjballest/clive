/*
	run a ql command using qlfs on UNIX
*/
package main

import (
	"bytes"
	"clive/app"
	"clive/app/opt"
	"clive/dbg"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

var (
	opts  = opt.New("{cmd}")
	dummy bool
	ql    = "/n/ql"
)

func main() {
	defer app.Exiting()
	os.Args[0] = "Q"
	c := app.New()
	opts.NewFlag("c", "ignored for compatibility", &dummy)
	opts.NewFlag("D", "debug", &c.Debug)
	edir := dbg.Usr
	opts.NewFlag("e", "env: qlfs environment name (defaults to uid)", &edir)
	opts.NewFlag("q", "qldir: qlfs root dir (defaults to /n/ql)", &ql)
	args, err := opts.Parse(os.Args)
	if err != nil {
		app.Warn("%s", err)
		opts.Usage()
		app.Exits(err)
	}
	cmd := ""
	if len(args) == 0 {
		app.Warn("no command given")
		opts.Usage()
		app.Exits(err)
	}
	cmd = strings.Join(args, " ")
	app.Dprintf("run %s\n", cmd)
	_, err = os.Stat(path.Join(ql, "Ctl"))
	if err != nil {
		app.Fatal("qlfs: %s", err)
	}
	env := path.Join(ql, edir)
	_, err = os.Stat(env)
	if err != nil {
		err = os.Mkdir(env, 0775)
	}
	if err != nil {
		app.Fatal("%s", err)
	}

	cdir := fmt.Sprintf("%s/%d", env, os.Getpid())
	if err := os.Mkdir(cdir, 0755); err != nil {
		app.Fatal("%s", err)
	}
	var b bytes.Buffer
	io.Copy(&b, os.Stdin)
	if err := ioutil.WriteFile(path.Join(cdir, "in"), b.Bytes(), 0644); err != nil {
		os.RemoveAll(cdir)
		app.Fatal("writing in: %s", err)
	}
	if err := ioutil.WriteFile(path.Join(cdir, "cmd"), []byte(cmd), 0644); err != nil {
		os.RemoveAll(cdir)
		app.Fatal("writing cmd: %s", err)
	}
	oc := make(chan bool, 2)
	pfd, err := os.Open(path.Join(cdir, "pout"))
	if err != nil {
		app.Warn("out: %s", err)
		oc <- true
	} else {
		go func() {
			io.Copy(os.Stdout, pfd)
			pfd.Close()
			oc <- true
		}()
	}
	efd, err := os.Open(path.Join(cdir, "perr"))
	if err != nil {
		app.Warn("err: %s", err)
		oc <- true
	} else {
		go func() {
			io.Copy(os.Stderr, efd)
			efd.Close()
			oc <- true
		}()
	}
	<-oc
	<-oc
	sts, err := ioutil.ReadFile(path.Join(cdir, "wait"))
	os.RemoveAll(cdir)
	if err != nil {
		app.Exits(err)
	}
	s := string(sts)
	if s == "success" {
		s = ""
	}
	app.Exits(s)
}
