/*
	Tools use for debugging and to issue diagnostics.
*/
package dbg

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"bytes"
)

// Variables initialized at init time with the user, system, home directory
// and temporary directory names.
var (
	cwd string

	Usr = "none"

	Sys = "sargazos"

	Home = "/tmp"

	Tmp = "/tmp"
)

func init() {
	cwd, _ = os.Getwd()
	u, err := user.Current()
	if err == nil {
		Usr = strings.ToLower(u.Username)
		Home = u.HomeDir
	}
	s, err := os.Hostname()
	if err == nil {
		toks := strings.SplitN(s, ".", 2)
		Sys = strings.ToLower(toks[0])
	}
	t := os.TempDir()
	if t!="" && runtime.GOOS!="darwin" {
		Tmp = t
	}
}

// Argument to AtExit.
type ExitFun func()

// Argument to AtIntr. return true to handle
type IntrFun func() bool

var (
	exits = []ExitFun{}
	intrs = []IntrFun{}
	exitslk sync.Mutex
)

func FirstErr(err ...error) error {
	for _, e := range err {
		if e != nil {
			return e
		}
	}
	return nil
}

// Arrange for fn to be called upon SIGINT, SIGTERM, dbg.Exits, or dbg.Fatal.
func AtExit(fn ExitFun) {
	exitslk.Lock()
	exits = append(exits, fn)
	exitslk.Unlock()

}

// Arrange for fn to be called upon SIGINT and ignore if it returns true.
func AtIntr(fn IntrFun) {
	exitslk.Lock()
	intrs = append(intrs, fn)
	exitslk.Unlock()
}

func atExits() {
	for i := len(exits) - 1; i >= 0; i-- {
		exits[i]()
	}
}

func init() {
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGUSR1)
	go func() {
	Loop:
		for {
			s := <-sc
			if s == syscall.SIGUSR1 {
				buf := make([]byte, 64*1024)
				n := runtime.Stack(buf, true)
				os.Stderr.Write(buf[:n])
				continue
			}
			if s == os.Interrupt {
				exitslk.Lock()
				for _, fn := range intrs {
					if fn() {
						exitslk.Unlock()
						continue Loop
					}
				}
				exitslk.Unlock()
			}
			Fatal("interrupted")
		}
	}()
}

// Popular errors
var (
	ErrNotExist = errors.New("no such file or directory")
	ErrExists   = errors.New("file already exists")
	ErrIsDir    = errors.New("file is a directory")
	ErrNotDir   = errors.New("not a directory")
	ErrPerm     = errors.New("permission denied")
	ErrBug      = errors.New("buggered or not implemented")
	ErrNotEmpty = errors.New("directory not empty")
	ErrRO       = errors.New("resource is read-only")
	ErrBadCtl   = errors.New("bad ctl request")
	ErrIntr = errors.New("interrupted")
	ErrUsage = errors.New("wrong use")
)

// Std go IsNotExist sucks.
func IsNotExist(e error) bool {
	if e == nil {
		return false
	}
	if e == ErrNotExist {
		return true
	}
	s := e.Error()
	return strings.Contains(s, "does not exist") ||
		strings.Contains(s, "no such file") ||
		strings.Contains(s, "not found")
}

func IsExists(e error) bool {
	if e == nil {
		return false
	}
	if e == ErrExists {
		return true
	}
	s := e.Error()
	return strings.Contains(s, "already exists") ||
		strings.Contains(s, "file exists")
}

func IsPerm(e error) bool {
	if e == nil {
		return false
	}
	if e == ErrPerm {
		return true
	}
	s := e.Error()
	return strings.Contains(s, "permission denied")
}

// Printf to stderr.
func Printf(str string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, str, args...)
}

// Printf to stderr, prefixed with program name and terminating with \n.
func Warn(str string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "%s: %s\n",
		os.Args[0], fmt.Sprintf(str, args...))
}

// To be deferred at main or used to terminate the program while
// honoring AtExits. (NB: INT and TERM signals also honor atexits).
// Exit with 0 or 1 depending on the args (like Fatal), but do not print.
func Exits(args ...interface{}) {
	fatal(false, args...)
}

// Warn and exit after running atexits.
func Fatal(args ...interface{}) {
	fatal(true, args...)
}

var ExitDumpsStacks bool
func xexit(sts int) {
	if ExitDumpsStacks {
		var buf [64*1024]byte
		n := runtime.Stack(buf[0:], true)
		os.Stderr.Write(buf[:n])
	}
	os.Exit(sts)
}

func fatal(warn bool, args ...interface{}) {
	atExits()
	if len(args)==0 || args[0]==nil {
		xexit(0)
	}
	if s, ok := args[0].(string); ok {
		if s == "" {
			xexit(0)
		}
		if warn {
			Warn(s, args[1:]...)
		}
	} else if e, ok := args[0].(error); ok {
		if e == nil {
			xexit(0)
		}
		if warn {
			Warn("%s", e)
		}
	} else if warn {
		Warn("fatal")
	}
	xexit(1)
}

// Trace a function call.
func Trace(s string) {
	fmt.Printf("end %s\n", s)
}

// Trace a function call. See Trace and its example.
func Call(s string) string {
	if _, file, lno, ok := runtime.Caller(1); ok {
		rel, _ := filepath.Rel(cwd, file)
		s = fmt.Sprintf("%s %s:%d", s, rel, lno)
	}
	fmt.Printf("bgn %s\n", s)
	return s
}

// See FuncPrintf and FlagPrintf.
type PrintFunc func(fmts string, arg ...interface{}) (int, error)

/*
	Return a function that calls fmt.Fprintf(w, ...) but only if fn returns true.
*/
func FuncPrintf(w io.Writer, fn func() bool) PrintFunc {
	lk := &sync.Mutex{}
	return func(fmts string, arg ...interface{}) (int, error) {
		if fn() {
			lk.Lock()
			defer lk.Unlock()
			return fmt.Fprintf(w, fmts, arg...)
		}
		return 0, nil
	}
}

/*
	Return a function that calls fmt.Fprintf(w,...) but only if flag is set.
*/
func FlagPrintf(w io.Writer, flag *bool) PrintFunc {
	lk := &sync.Mutex{}
	return func(fmts string, arg ...interface{}) (int, error) {
		if *flag {
			lk.Lock()
			defer lk.Unlock()
			return fmt.Fprintf(w, fmts, arg...)
		}
		return 0, nil
	}
}

/*
	Do an hex dump to the given writer
*/
func HexDump(w io.Writer, data []byte, off0 int) {
	for i := 0; i < len(data); i++ {
		if i%16 == 0 {
			fmt.Fprintf(w, "\n%4d  ", off0+i)
		}
		fmt.Fprintf(w, " %02x", data[i])
	}
	fmt.Fprintf(w, "\n")
}

/*
	Return a string with an hex dump of at most n bytes from data
	for debug
*/
func HexStr(data []byte, n int) string {
	var b bytes.Buffer
	if len(data)%2 != 0 {
		fmt.Fprintf(&b, " %02x", data[0])
		data = data[1:]
	}
	if n > len(data) {
		n = len(data)
	}
	for i := 0; i < n/2; i++ {
		fmt.Fprintf(&b, " %02x", data[i])
	}
	if 2*n < len(data) {
		fmt.Fprintf(&b, "...")
	}
	for i := 0; i < n/2; i++ {
		fmt.Fprintf(&b, " %02x", data[len(data)-i-1])
	}
	return b.String()
}

func Text(s string, n int) string {
	if n < 10 {
		n = 10
	}
	var b bytes.Buffer
	for _, r := range s {
		switch r {
		case '\n':
			fmt.Fprintf(&b, "\\n")
		case '\t':
			fmt.Fprintf(&b, "\\t")
		default:
			b.WriteRune(r)
		}
	}
	ns := b.String()
	if len(ns) > n {
		ns = ns[:n/2] + "..." + ns[len(ns)-n/2:]
	}
	return "[" + ns + "]"
}
