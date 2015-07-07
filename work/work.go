/*
	A pool of worker processes.

*/
package work

import (
	"clive/dbg"
	"os"
	"sync"
)

type call struct {
	fn func()
	donec chan bool
}

// A pool of work to do.
type Pool {
	calls chan call
	wg sync.WaitGroup
	closed bool
}

var (
	Debug bool
	dprintf = dbg.FlagPrintf(os.Stderr, &Debug)
)

// create a pool of workers with up to n concurrent works.
func NewPool(n int) *Pool {
	p := &Pool{calls: make(chan call)}
	for i := 0; i < n; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	return p
}

func (p* Pool) worker(id int) {
	defer p.wg.Done()
	dprintf("worker#%d started\n", id)
	defer dprintf("worker#%d terminated\n", id)
	for c := range p.calls {
		dprintf("worker#%d fn()...\n", id)
		c.fn()
		dprintf("worker#%d ...fn()\n", id)
		c.donec <- true
	}
}

// Terminate the pool.
// Running functions continue to run but no further work is accepted
// causing a panic instead.
func (p *Pool) Close() {
	p.closed = true
	close(p.calls)
}

// Wait until all workers are done, closing if the pool is not yet closed.
func (p *Pool) Wait() {
	p.Close()
	p.wg.Wait()
}

// Run fn in a worker from the pool.
// If there is no worker available, await until there is one
// Either way, return a chan that is sent-to when the call completes.
// The channel is the one passed or a new one if it is nil.
func (p *Pool) Go(donec chan bool, fn func()) chan bool {
	if p.closed {
		panic("closed pool")
	}
	dprintf("go call\n")
	if donec == nil {
		donec = make(chan bool, 1)
	}
	p.calls <- call{fn: fn, donec: donec}
	return donec
}

// Run fn in a worker from the pool.
// If there is no worker available, run the function in the caller process.
// Either way, return a chan that is sent-to when the call completes.
// The channel is the one passed or a new one if it is nil.
func (p *Pool) Call(donec chan bool, fn func()) chan bool {
	if p.closed {
		panic("closed pool")
	}
	dprintf("go call\n")
	if donec == nil {
		donec = make(chan bool, 1)
	}
	select {
	case p.calls <- call{fn: fn, donec: donec}:
	default:
		fn()
		donec <- true
	}
	return donec
}