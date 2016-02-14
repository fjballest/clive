package main

import (
	"clive/cmd"
	"strconv"
	"time"
)

func main() {
	cmd.UnixIO()
	nc := cmd.New(func() {
		cmd.Printf("hi from here\n")
		cmd.Fatal("oops!")
		println("XXX")
	})
	args := cmd.Args()
	cmd.Printf("args %v\n", args)
	wc := nc.Waitc()
	cmd.Printf("Hi there!\n")
	cmd.Printf("Hi there!\n")
	cmd.Printf("Hi there!\n")
	if len(args) > 1 {
		n, _ := strconv.Atoi(args[1])
		ns := time.Duration(n)
		time.Sleep(ns * time.Second)
	} else {
		time.Sleep(time.Second)
	}
	<-wc
	cmd.Printf("done\n")
}
