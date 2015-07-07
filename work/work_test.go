package work

import (
	"testing"
	"time"
	"sync/atomic"
	"unicode"
)

const (
	tout = 15*time.Second
	ntests = 20
)

var (
	calls [2*ntests]rune
	ncalls int32
)

func ExamplePool() {
	// Create a pool of 5 workers
	p := NewPool(5)
	// Run 20 functions in the pool
	dc := make(chan bool, 20)
	for i := 0; i < 20; i++ {
		p.Go(dc, func() {
			dprintf("running this...\n")
		})
	}
	// Now we know all of them have at least start
	// and at least 15 of them have finished
	// Wait until all of them are done
	for i := 0; i < 20; i++ {
		<-dc
	}
	// Or we might instead close the pool
	p.Close()
	// and wait until all remaining workers are done
	p.Wait()
}

func TestNewPool(t *testing.T) {
	Debug = testing.Verbose()
	p := NewPool(5)
	donec := make(chan bool)
	go func() {
		p.Close()
		p.Wait()
		close(donec)
	}()
	select {
	case <- donec:
	case <-time.After(tout):
		t.Fatalf("poll timed out (deadlock?)")
	}
}

func fakefn(r rune, t time.Duration) {
	n := atomic.AddInt32(&ncalls, 1)
	calls[n-1] = r
	dprintf("fake call %c\n", r)
	time.Sleep(t)
	r = unicode.ToUpper(r)
	n = atomic.AddInt32(&ncalls, 1)
	calls[n-1] = r
}

func plot(trz string, ncalls int) {
	c := 'a'
	dprintf("trace: %s\n", trz)
	ch := map[bool]string{true: "-", false: " "}
	for i := 0; i < ncalls; i++ {
		st := c+rune(i)
		end := unicode.ToUpper(st)
		sted := false
		for _, ev := range trz {
			switch ev {
			case st, end:
				dprintf("+")
				sted = !sted
			default:
				dprintf(ch[sted])
			}
		}
		dprintf("\n")
	}
}

func TestPoolGo(t *testing.T) {
	Debug = testing.Verbose()
	p := NewPool(5)
	donec := make(chan bool)
	go func() {
		ncalls = 0
		// we must be able to sched up to 5 calls
		// the next calls must be called by our own process
		rc := make(chan bool, ntests)
		for i := 0; i < ntests; i++ {
			r := rune('a'+i)
			p.Go(rc, func() {
				fakefn(r, time.Second)
			})
		}
		dprintf("all started\n")
		for i := 0; i < ntests; i++ {
			<-rc
		}
		dprintf("all done\n")
		p.Close()
		p.Wait()
		close(donec)
	}()
	select {
	case <- donec:
		dprintf("calls:\n")
		trace := string(calls[:int(ncalls)])
		plot(trace, ntests)
		if ncalls < 2*ntests {
			t.Fatalf("wrong number of calls")
		}
		t.Logf("sched '%s'", trace)
	case <-time.After(tout):
		t.Fatalf("poll timed out (deadlock?)")
	}
}

