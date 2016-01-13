package main

import (
	"clive/cmd"
	"time"
)

func main() {
	cmd.UnixIO()
	nc := cmd.New(func() {
		cmd.Printf("hi from here\n")
		cmd.Fatal("oops!")
		println("XXX")
	})
	wc := nc.Waitc()
	cmd.Printf("Hi there!\n")
	cmd.Printf("Hi there!\n")
	cmd.Exit()
	cmd.Printf("Hi there!\n")
	println("....")
	time.Sleep(10)
	<-wc
	println("....")
}
