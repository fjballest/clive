/*
	file system watcher
*/
package fswatch

import (
	"clive/cmd"
	"syscall"
	fpath "path"
	"io/ioutil"
	"errors"
)

// Watcher for file system changes
// (unix; not ZX)
struct Watcher {
	kfd int
	did uint64
	fds map[uint64]string
	evs []syscall.Kevent_t
	once bool
	rc chan string
}

// Create a new watcher
func New() (*Watcher, error) {
	fd, err := syscall.Kqueue()
	if err != nil {
		return nil, err
	}
	w := &Watcher {
		did: ^uint64(0),
		kfd: fd,
		fds: map[uint64]string{},
	}
	return w, nil
}

// Arrange for w to be done after the first change reported.
func (w *Watcher) Once() {
	w.once = true
}

func (w *Watcher) add1(p string) error {
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

// Add a file to the watcher list.
// Must be called before watching changes.
// If the file is a directory all the files in it are also watched (w/o recur. for subdirs)
func (w *Watcher) Add(p string) error {
	if w.rc != nil {
		return errors.New("can't add (yet) while watching")
	}
	if err := w.add1(p); err != nil {
		return err
	}
	ents, err := ioutil.ReadDir(p)
	if err == nil {
		for _, e := range ents {
			path := fpath.Join(p, e.Name())
			if err := w.add1(path); err != nil {
				cmd.Dprintf("wath %s: %s\n", path, err)
			}
		}
	}
	return nil
}

func (w *Watcher) change(rc chan string) bool { 
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
			// cmd.Dprintf("%s %x\n", p, e.Fflags)
			if ok = rc <- p; !ok || w.once {
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
	return !w.once
}

func (w *Watcher) changes(rc chan string) {
	for w.change(rc) {
		ofds := w.fds
		w.fds = map[uint64]string{}
		for _, p := range ofds {
			w.Add(p)
		}
	}
	for fd := range w.fds {
		syscall.Close(int(fd))
	}
	close(rc)
	syscall.Close(w.kfd)
}

// Report changes through the returned chan.
func (w *Watcher) Changes() chan string {
	rc := make(chan string, 10)
	if w.rc != nil {
		close(rc, "already watching")
		return rc
	}
	w.rc = rc
	go w.changes(rc)
	return rc
}
