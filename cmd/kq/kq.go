/*
	whatch file system changes
*/
package main

import (
	"clive/cmd"
	"clive/cmd/opt"
	"syscall"
	fpath "path"
	"io/ioutil"
)

struct Watches {
	kfd int
	did uint64
	fds map[uint64]string
	evs []syscall.Kevent_t
}

var (
	opts = opt.New("file")
	notux bool
)

func NewWatches() (*Watches, error) {
	fd, err := syscall.Kqueue()
	if err != nil {
		return nil, err
	}
	w := &Watches {
		did: ^uint64(0),
		kfd: fd,
		fds: map[uint64]string{},
	}
	return w, nil
}

func (w *Watches) Add(p string) error {
	fd, err := syscall.Open(p, syscall.O_RDONLY, 0)
	if err != nil {
		return err
	}
	id := uint64(fd)
	w.fds[id] = p
	ev := syscall.Kevent_t{
		Ident:  id,
		Filter: syscall.EVFILT_VNODE,
		Flags:  syscall.EV_ADD | syscall.EV_ENABLE | syscall.EV_ONESHOT | syscall.EV_CLEAR,
		Fflags: syscall.NOTE_DELETE | syscall.NOTE_WRITE | syscall.NOTE_RENAME,
		Data:   0,
		Udata:  nil,
	}
	if len(w.evs) == 0 {
		w.did = id
	}
	w.evs = append(w.evs, ev)
	return nil
}

func (w *Watches) watch1(rc chan string) bool {
	if len(w.fds) == 0 {
		return false
	}
	// wait for events
	isdir := false
Loop:	for !isdir {
		// create kevent
		events := make([]syscall.Kevent_t, 2*len(w.evs))
		_, err := syscall.Kevent(w.kfd, w.evs, events, nil)
		if err != nil {
			close(rc, err)
			return false
		}
		// check if there was an event and process it
		for _, e := range events {
			p, ok := w.fds[e.Ident]
			if !ok {
				cmd.Dprintf("no events\n")
				continue
			}
			cmd.Dprintf("%s %x\n", p, e.Fflags)
			if ok := rc <- p; !ok {
				break Loop
			}
			if e.Fflags&syscall.NOTE_DELETE != 0 {
				syscall.Close(int(e.Ident))
				delete(w.fds, e.Ident)
			}
			if e.Ident == w.did {
				isdir = true
			}
		}
	}
	for fd := range w.fds {
		syscall.Close(int(fd))
	}
	return true
}

func (w *Watches) watch(rc chan string) {
	for w.watch1(rc) {
		ofds := w.fds
		w.fds = map[uint64]string{}
		for _, p := range ofds {
			w.Add(p)
		}
	}
	for fd := range w.fds {
		syscall.Close(int(fd))
	}
	syscall.Close(w.kfd)
}

func (w *Watches) Watch() chan string {
	rc := make(chan string, 10)
	go w.watch(rc)
	return rc
}

func kq(dir string) {
	w, err := NewWatches()
	if err != nil {
		cmd.Fatal("kqueue: %s", err)
	}

	cmd.Dprintf("(re)read %s\n", dir)
	if err := w.Add(dir); err != nil {
		cmd.Fatal("kqueue: %s", err)
	}
	ents, err := ioutil.ReadDir(dir)
	if err == nil {
		for _, e := range ents {
			path := fpath.Join(dir, e.Name())
			if err := w.Add(path); err != nil {
				cmd.Dprintf("wath %s: %s\n", path, err)
			}
		}
	}
	pc := w.Watch()
	if err != nil {
		cmd.Fatal("watch: %s", err)
	}
	for p := range pc {
		cmd.Warn("%s", p)
	}
	if err := cerror(pc); err != nil {
		cmd.Fatal(err)
	}
	cmd.Exit(nil)
}

func main() {
	cmd.UnixIO("err")
	c := cmd.AppCtx()
	opts.NewFlag("D", "debug", &c.Debug)
	opts.NewFlag("u", "don't use unix out", &notux)
	args := opts.Parse()
	if !notux {
		cmd.UnixIO("out")
	}
	if len(args) != 1{
		opts.Usage()
	}
	for {
		kq(args[0])
	}
}
