package cfs

import (
	"sync"
	"fmt"
	"runtime"
	"clive/dbg"
)

type lockTrz {
	rid string	// set to "" when unlocked
	pc uintptr	
	file string
	line int
}

// Used to trace missing unlocks
type lockTrzs {
	sync.Mutex
	locks map[int64] []lockTrz
}

func (lk lockTrz) String() string {
	return fmt.Sprintf("%s:%d locked %s", lk.file, lk.line, lk.rid)
}

func (t *lockTrzs) Locking(rid string, skip int) {
	if t == nil {
		return
	}
	id := runtime.GoId()
	lk := lockTrz{rid: rid}
	lk.pc, lk.file, lk.line, _ = runtime.Caller(skip+1)
	t.Lock()
	if t.locks == nil {
		t.locks = make(map[int64][]lockTrz)
	}
	locks := t.locks[id]
	t.locks[id] = append(locks, lk)
	t.Unlock()
}

func (t *lockTrzs) Unlocking(rid string) {
	if t == nil {
		return
	}
	id := runtime.GoId()
	t.Lock()
	defer t.Unlock()
	if t.locks == nil {
		t.locks = make(map[int64][]lockTrz)
	}
	locks := t.locks[id]
	n := len(locks)
	for i := 0; i < n; i++ {
		if locks[i].rid == rid {
			if i < n - 1 {
				locks[i] = locks[n-1]
			}
			locks = locks[:n-1]
			if len(locks) == 0 {
				delete(t.locks, id)
			} else {
				t.locks[id] = locks
			}
			return
		}
	}
	panic("unlock without lock")
}

func (t *lockTrzs) NoLocks() {
	if t == nil {
		return
	}
	id := runtime.GoId()
	t.Lock()
	defer t.Unlock()
	locks := t.locks[id]
	if len(locks) == 0 {
		return
	}
	for _, lk := range locks {
		dbg.Warn("nolocks: %s", lk)
	}
	panic("nolocks with locks")
}
