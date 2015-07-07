/*
	wax test command
*/
package main

import (
	"clive/dbg"
	"clive/net/wax"
	"clive/net/wax/ctl"
	"clive/net/wax/index"
	"flag"
	"fmt"
	"os"
)

var (
	port = ":9191"
	test = true
)

func testParts() []string {
	go func() {
		for ev := range evc {
			fmt.Printf("client ev %v\n", ev)
		}
		fmt.Printf("evc closed: ", cerror(evc))
	}()
	tx := testText()
	tx.Serve("/txt")
	tc := testCanvas()
	tc.Serve("/tc")
	tb1 := testTb()
	tb1.Serve("/tb1")
	tb2 := testTb()
	tb2.Serve("/tb2")
	tt := testTreeTb()
	tt.Serve("/tt")
	tf := testFor()
	tf.Serve("/tf")
	ta := testAdt()
	ta.Serve("/ta")
	return []string{"/txt", "/tc", "/tb1", "/tb2", "/tt", "/tf", "ta"}
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: %s -flags\n", os.Args[0])
	flag.PrintDefaults()
	os.Exit(1)
}

func main() {
	os.Args[0] = "wax"
	flag.Usage = usage
	flag.BoolVar(&test, "t", false, "create some test parts")
	flag.BoolVar(&wax.Verbose, "v", false, "verbose")
	flag.StringVar(&port, "p", "9191", "port")
	flag.Parse()
	wax.Verbose = true
	ctl.Debug = true
	wax.ServeLogin("/", "/index")
	index.ServeAt("/index", testParts())
	if err := wax.Serve(":" + port); err != nil {
		dbg.Fatal("serve: %s", err)
	}
}
