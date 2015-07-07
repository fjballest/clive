/*
	ql is the clive shell
*/
package main

import (
	"bufio"
	"bytes"
	"clive/cmd"
	"clive/cmd/opt"
	"clive/dbg"
	"io/ioutil"
	"os"
)

var (
	cmdarg               string
	opts                 = opt.New("[file [arg...]]")
	debug, verb, debugNs bool
	dprintf              = dbg.FlagPrintf(os.Stderr, &debug)
	vprintf              = dbg.FlagPrintf(os.Stderr, &verb)
	Interactive          bool
	lexer                *lex
	Prompter             *cmd.Reader
)

func interactive(in *bufio.Reader) (isintr bool) {
	// if there's a C-c while we are reading,
	// the lexer will panic with ErrIntr,
	// to signal us that we must discard the current
	// input and parsing and start again.
	defer func() {
		lvl = 0
		nerrors = 0
		if err := recover(); err != nil {
			if err == ErrIntr {
				isintr = true
				return
			}
			panic(err)
		}
	}()
	lexer = newLex("stdin", in)
	yyParse(lexer)
	return false
}

var (
	prompt  = "> "
	prompt2 = ">> "
)

func main() {
	defer dbg.Exits("errors")
	os.Args[0] = "ql"
	opts.NewFlag("c", "cmd: execute this command and exit", &cmdarg)
	opts.NewFlag("D", "debug", &debug)
	opts.NewFlag("V", "verbose debug", &verb)
	opts.NewFlag("N", "debug ns", &debugNs)
	args, err := opts.Parse(os.Args)
	if err != nil {
		opts.Usage(os.Stderr)
		dbg.Fatal(err)
	}

	cmd.Debug = debugNs
	debugYacc = debug
	debugExec = verb
	cmd.MkNS()

	var in inText
	Interactive = cmd.IsTTY(os.Stdin)
	if Interactive {
		dprintf("interactive\n")
		if xprompt := os.Getenv("prompt"); xprompt != "" {
			prompt = xprompt
		} else {
			os.Setenv("prompt", prompt)
		}
		if xprompt2 := os.Getenv("prompt2"); xprompt2 != "" {
			prompt2 = xprompt2
		} else {
			os.Setenv("prompt2", prompt2)
		}
	} else {
		dprintf("script\n")
		IntrExits = true
	}
	var argv0, iname string
	var argv []string
	if cmdarg != "" {
		in = bytes.NewBufferString(cmdarg + "\n")
		argv0 = "ql"
		iname = "flag-c"
		argv = args
	} else if len(args) == 0 {
		if Interactive {
			SetEnvList("argv0", "ql")
			SetEnvList("argv", args...)
			Argv = args
			rdr := cmd.NewReader(os.Stdin, os.Stdout, prompt)
			Prompter = rdr
			for interactive(bufio.NewReader(rdr)) {
				dprintf("*** interrupted\n")
				rdr.Flush()
			}
			dbg.Exits("")
		}
		iname = "stdin"
		in = bufio.NewReader(os.Stdin)
		argv0 = "ql"
	} else {
		argv0 = args[0]
		iname = args[0]
		argv = args[1:]
		idata, err := ioutil.ReadFile(iname)
		if err != nil {
			dbg.Fatal("open: %s: %s", iname, err)
		}
		in = bytes.NewBuffer(idata)
	}

	SetEnvList("argv0", argv0)
	SetEnvList("argv", argv...)
	Argv = argv
	lexer = newLex(iname, in)
	yyParse(lexer)
	dbg.Exits(os.Getenv("status"))
}
