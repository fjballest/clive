/*
	App processes with a channel based interface.

	Each application has a context including

		- A set of IO channels

		- A name space

		- A dot path

		- A set of environment variables

		- Handlers for signals

		- A wait channel and exit status

	The first context created is initialized from OS, and
	following onces share most with the parent context.
	They may later change or dup the resources using
	to stop sharing them.

	All IO channels carry mostly []byte messages.
	Other data may be sent as well.
	In some cases it's context or app. specific data
	not to be forwarded outside of the system.

*/
package app

import (
	"clive/dbg"
	"sync"
	"runtime"
	"os"
	"io"
	"fmt"
	"clive/zx"
	"bytes"
	"errors"
	"reflect"
)

// Argument to Ctx.Handle returning true if the signal was handled.
type SigFun func(sig string) bool

// See FlagPrintf.
type PrintFunc func(fmts string, arg ...interface{}) error

// Application context.
// In, Out, and Err carry []byte as data, but may carry other pieces of
// data as context (eg., zx.Dir)
// When fed into external processes, non []byte messages are discarded.
type Ctx {
	Id int64		// AppId() for this ctx
	Args	[]string	// command line arguments
	Sig chan string	// signals posted
	intr chan string	// copy of signals sent to readers
	Wait chan bool	// cerror(Wait) is the status
	Sts error		// set to the exit status when dead
	lk sync.Mutex
	sigs map[string][]SigFun

	ns zx.Finder	// name space
	dot *cwd		// dot
	env *envSet	// environment
	io  *IOSet		// io chans

	Debug bool	// global debug flag
	exited bool	// Exiting did run for this already
	bg bool		// if true, OS intrs are not posted to this context
			// this is inherited by children apps.
}

var (
	Debug, Verb bool
	ctxs = map[int64]*Ctx{}
	ctxlk sync.Mutex

	usingApp bool
)

func init() {
	dbg.AtExit(appExit)
	dbg.AtIntr(appIntr)
}

func dprintf(fmts string, args ...interface{}) {
	if !Debug {
		return
	}
	dbg.Printf("%d: %s", runtime.AppId(), fmt.Sprintf(fmts, args...))
}

func vprintf(fmts string, args ...interface{}) {
	if !Debug || !Verb {
		return
	}
	dbg.Printf("%d: %s", runtime.AppId(), fmt.Sprintf(fmts, args...))
}

// ns, env, io must be set by the caller
func mkCtx(id int64) *Ctx {
	return &Ctx{
		Args: []string{"noname"},
		Id: id,
		Wait: make(chan bool),
		Sig: make(chan string, 10),
		intr: make(chan string),
	}
}

// Create a new process that starts a new app. context.
// This is the common case for using New().
// The function supplied is run after setting up the context, its Ns, and its env.
// It might further adjust its own context before doing its work.
func Go(fn func(), args ...string) *Ctx {
	xc := make(chan *Ctx, 1)
	go func() {
		defer Exiting()
		x := New()
		x.Args = args
		xc <- x
		close(xc)
		fn()
		Exits(nil)
	}()
	return <-xc
}

// Create a new application context, it's methods are
// implicit and accessed using the runtime application context.
// It's a panic to call New twice in the same process.
// IO, Env, Dot, and Ns are shared with the parent context by default.
// Use NewX() or DupX(), to start using new ones or create dups.
// The first context created initializes them from the underlying OS.
func New() *Ctx {
	usingApp = true
	old := runtime.AppId()
	ctxlk.Lock()
	pc := ctxs[old]
	n := len(ctxs)
	ctxlk.Unlock()
	runtime.NewApp()
	id := runtime.AppId()
	if old == id && n > 0 {
		panic("runtime bug: old ctx == new ctx")
	}
	c := mkCtx(id)
	ctxlk.Lock()
	if ctxs[id] != nil {
		panic("app.new: double call")
	}
	ctxs[id] = c
	ctxlk.Unlock()
	dprintf("new ctx %d\n", c.Id)
	if n == 0 {
		c.Args = os.Args
	}
	if pc != nil {
		pc.io.lk.Lock()
		pc.io.n++
		pc.io.lk.Unlock()
		pc.lk.Lock()
		c.env = pc.env
		c.dot = pc.dot
		c.ns = pc.ns
		c.io = pc.io
		c.bg = pc.bg
		pc.lk.Unlock()
	} else {
		NewEnv(nil)
		NewDot("")
		NewNS(nil)
		NewIO(nil)
	}
	dump("new")
	return c
}

// Return the current app context.
func AppCtx() *Ctx {
	ctxlk.Lock()
	defer ctxlk.Unlock()
	id := runtime.AppId()
	c := ctxs[id]
	if c == nil {
		return nil
	}
	return c
}

func ctx() *Ctx {
	c := AppCtx()
	if c == nil {
		dbg.Fatal("no context for %d", runtime.AppId())
	}
	return c
}

func Args() []string {
	c := ctx()
	return c.Args
}

func appExit() {
	dprintf("app exit\n")
	ctxlk.Lock()
	for _, c := range ctxs {
		c.post("exit")
	}
	ctxlk.Unlock()
}

func appIntr() bool {
	dprintf("app intr\n")
	handled := false
	ctxlk.Lock()
	for _, c := range ctxs {
		if !c.bg && c.post("intr") {
			handled = true
		}
	}
	ctxlk.Unlock()
	return handled
}

func (c *Ctx) handle(sig string, fn SigFun) {
	c.lk.Lock()
	defer c.lk.Unlock()
	dprintf("c %d: handle '%s'\n", c.Id, sig)
	if c.sigs == nil {
		c.sigs = make(map[string][]SigFun)
	}
	c.sigs[sig] = append(c.sigs[sig], fn)
}

func (c *Ctx) dontHandle(sig string, fn SigFun) {
	c.lk.Lock()
	defer c.lk.Unlock()
	dprintf("c %d: donthandle '%s'\n", c.Id, sig)
	if c.sigs == nil {
		return
	}
	lst := c.sigs[sig]
	fnp := reflect.ValueOf(fn).Pointer()
	for i, h := range lst {
		if reflect.ValueOf(h).Pointer() == fnp {
			lst[i] = nil
			break
		}
	}
}

func (c *Ctx) intrIn(sig string) {
	select {
	case c.intr <- sig:
	default:
	}
}

func (c *Ctx) nbpost(sig string) bool {
	if sig == "exit" {
		return false
	}
	select {
	case c.Sig <- sig:
		dprintf("c %d: post %s\n", c.Id, sig)
		c.intrIn(sig)
		return true
	default:
		dprintf("c %d: post(drop) %s\n", c.Id, sig)
		c.intrIn(sig)
		return false
	}
}

const All = "" // Argument for SigHandler

func (c *Ctx) post(sig string) bool {
	c.lk.Lock()
	handled := false
	for _, h := range c.sigs[All] {
		if h != nil && h(sig) {
			handled = true
			// but call all other handlers also
		}
	}
	for _, h := range c.sigs[sig] {
		if h != nil && h(sig) {
			handled = true
			// but call all other handlers also
		}
	}
	c.lk.Unlock()
	if sig == "exit" {
		return true
	}
	if c.nbpost(sig) {
		handled = true
	}
	return handled
}

// Post a singal to the given app.
func Post(sig string, id int64) {
	ctxlk.Lock()
	c := ctxs[id]
	ctxlk.Unlock()
	if c != nil {
		c.post(sig)
	}
}

// Make the current app ignore OS intrs.
func Bg() {
	c := AppCtx()
	if c != nil {
		c.bg = true
	}
	
}

// Run fn when the named signal arrives (or any signal if the name is empty)
// The fake signal "exit" is posted when the program is about to exit.
// The name "intr" refers to interrupt.
func Handle(sig string, fn SigFun) {
	ctx().handle(sig, fn)
}

// cancel a previous call to Handle with the same arguments
func DontHandle(sig string, fn SigFun) {
	ctx().dontHandle(sig, fn)
}

// Printf to stdout.
func Printf(str string, args ...interface{}) error {
	if !usingApp {
		_, err := fmt.Printf(str, args...)
		return err
	}
	return cprintf(Out(), os.Stdout, str, args...)
}

// Printf to stderr.
func Eprintf(str string, args ...interface{}) error {
	if !usingApp {
		_, err := fmt.Fprintf(os.Stderr, str, args...)
		return err
	}
	return cprintf(Err(), os.Stderr, str, args...)
}

// Eprintf if Debug is set.
func Dprintf(str string, args ...interface{}) error {
	if !usingApp {
		if !Debug {
			return nil
		}
		return Eprintf(str, args...)
	}
	c := ctx()
	if c.Debug {
		return Eprintf(str, args...)
	}
	return nil
}

func cprintf(out chan interface{}, w io.Writer, str string, args ...interface{}) error {
	if out != nil {
		ok := out <- []byte(fmt.Sprintf(str, args...))
		if !ok {
			return cerror(out)
		}
		return nil
	}
	_, err := fmt.Fprintf(w, str, args...)
	return err
}

// Return a function that calls Printf() but only if flag is set.
func FlagPrintf(flag *bool) PrintFunc {
	lk := &sync.Mutex{}
	return func(fmts string, arg ...interface{}) error {
		if *flag {
			lk.Lock()
			defer lk.Unlock()
			return Printf(fmts, arg...)
		}
		return nil
	}
}

// Return a function that calls Printf() but only if flag is set.
func FlagEprintf(flag *bool) PrintFunc {
	lk := &sync.Mutex{}
	return func(fmts string, arg ...interface{}) error {
		if *flag {
			lk.Lock()
			defer lk.Unlock()
			return Eprintf(fmts, arg...)
		}
		return nil
	}
}

// Printf to stderr, prefixed with program name and terminating with \n.
func Warn(str string, args ...interface{}) {
	c := ctx()
	if len(c.Args) == 0 {
		c.Args = os.Args
	}
	Eprintf("%s[%d]: %s\n", c.Args[0], c.Id, fmt.Sprintf(str, args...))
}

// returns true if the app ctx exited
func Exited() bool {
	ctxlk.Lock()
	defer ctxlk.Unlock()
	return ctxs[runtime.AppId()] == nil
}

// To be deferred in the main program so it recovers from Fatals and returns.
// For safety, Atexits are run by the call to Fatal and not here.
func Exiting() {
	if r := recover(); r != nil {
		dprintf("exiting: %v...\n", r)
		if r == "fatal" {
			return
		}
		panic(r)
	}
	dprintf("exiting...\n")
	dump("exiting")
	ctxlk.Lock()
	id := runtime.AppId()
	c := ctxs[id]
	delete(ctxs, id)
	ctxlk.Unlock()
	if c != nil && c.io != nil {
		c.io.closeAll(nil)
	}
}

func mkSts(args ...interface{}) error {
	if len(args) == 0 || args[0] == nil {
		return nil
	}
	if s, ok := args[0].(string); ok {
		if s == "" {
			return nil
		}
		return fmt.Errorf(s, args[1:]...)
	}
	if e, ok := args[0].(error); ok {
		return e
	}
	return errors.New("fatal")
}

// Exit the app after running atexits.
func Exits(args ...interface{}) {
	fatal(false, args...)
}

// Warn and exit the app after running atexits.
func Fatal(args ...interface{}) {
	fatal(true, args...)
}

func fatal(warn bool, args ...interface{}) {

	c := AppCtx()
	dprintf("fatal...\n")
	dump("fatal")
	if c == nil {
		panic("fatal")
	}
	ctxlk.Lock() 
	n := len(ctxs)
	ctxlk.Unlock()
	sts := mkSts(args...)
	efn := func() {
		c.post("exit")
		c.Sts = sts
		c.io.closeAll(sts)
		close(c.Sig, sts)
		close(c.Wait, sts)
		ctxlk.Lock()
		delete(ctxs, runtime.AppId())
		ctxlk.Unlock()
	}
	if n <= 1 {
		dprintf("c %d: dbg fatal (%d args)\n", c.Id, len(args))
		efn()
		close(out)
		close(err)
		<-outdone
		<-errdone
		os.Args = c.Args
		if warn {
			dbg.Fatal(args...)	// never returns
		} else {
			dbg.Exits(args...)	// never returns
		}
	}
	dprintf("c %d: app fatal (%d args, %d apps)\n", c.Id, len(args), n)
	if sts != nil && warn {
		Warn("%s", sts)
	}
	efn()
	panic("fatal")
}

// Takes locks on everything, don't call while holding locks.
func dump(t string) {
	if !Verb {
		return
	}
	ctxlk.Lock() 
	defer ctxlk.Unlock() 
	if t != "" {
		vprintf("at %s:\n", t)
	}
	for k, c := range ctxs {
		vprintf("[%d]: %s\n", k, c.Sprint())
	}
	vprintf("\n")
}

func (c *Ctx) String() string {
	return fmt.Sprintf("ctx %d %v", c.Id, c.Args)
}

// Return a text dump of the context for debugging.
func (c *Ctx) Sprint() string {
	if c == nil {
		return "<nil ctx>"
	}
	var b bytes.Buffer
	c.lk.Lock()
	fmt.Fprintf(&b, "%s sts %v dot %s\n", c, c.Sts, c.dot.path)
	io := c.io
	c.lk.Unlock()
	io.sprintf(&b)
	return b.String()
}
