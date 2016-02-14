package opt

import (
	"fmt"
	"strings"
	"testing"
)

struct chk {
	argv        []string
	flags, args string
}

var okchks = []chk{
	{
		argv:  []string{"ls"},
		flags: "false false false 0",
		args:  "",
	},
	{
		argv:  []string{"ls", "/tmp"},
		flags: "false false false 0",
		args:  "/tmp",
	},
	{
		argv:  []string{"ls", "-l", "/tmp"},
		flags: "true false false 0",
		args:  "/tmp",
	},
	{
		argv:  []string{"ls", "-ld", "/tmp"},
		flags: "true true false 0",
		args:  "/tmp",
	},
	{
		argv:  []string{"ls", "-l", "-d", "/tmp"},
		flags: "true true false 0",
		args:  "/tmp",
	},
	{
		argv:  []string{"ls", "-l"},
		flags: "true false false 0",
	},
	{
		argv:  []string{"ls", "-ld"},
		flags: "true true false 0",
	},
	{
		argv:  []string{"ls", "-l", "-d"},
		flags: "true true false 0",
	},
	{
		argv:  []string{"ls", "-ld", "-i15", "/tmp"},
		flags: "true true false 15",
		args:  "/tmp",
	},
	{
		argv:  []string{"ls", "-ld", "-i", "15", "/tmp"},
		flags: "true true false 15",
		args:  "/tmp",
	},
	{
		argv:  []string{"ls", "-ldi", "15", "/tmp"},
		flags: "true true false 15",
		args:  "/tmp",
	},
}

func TestFlags(t *testing.T) {
	opts := New("[file...]")
	var bl, bd, br bool
	var ival int
	opts.NewFlag("l", "long", &bl)
	opts.NewFlag("d", "long", &bd)
	opts.NewFlag("r", "long", &br)
	opts.NewFlag("i", "ival", &ival)
	for _, c := range okchks {
		bl, bd, br = false, false, false
		args := opts.Parse(c.argv...)
		flg := fmt.Sprintf("%v %v %v %d", bl, bd, br, ival)
		left := strings.Join(args, " ")
		if testing.Verbose() {
			fmt.Printf("flags %s args %s\n", flg, left)
		}
		if flg != c.flags || left != c.args {
			t.Fatal("bad result")
		}
	}
	if testing.Verbose() {
		opts.Usage()
	}
	var pn, mn int
	opts.NewFlag("+num", "add numb", &pn)
	opts.NewFlag("-num", "del numb", &mn)
	args := opts.Parse("ls", "+3", "-ldi", "15", "-5", "/tmp")
	flg := fmt.Sprintf("%v %v %v %d %d %d", bl, bd, br, ival, pn, mn)
	left := strings.Join(args, " ")
	if testing.Verbose() {
		fmt.Printf("flags %s args %s\n", flg, left)
	}
	if flg != `true true false 15 3 -5` || left != "/tmp" {
		t.Fatal("bad arg")
	}
}
