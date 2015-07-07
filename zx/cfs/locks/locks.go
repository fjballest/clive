/*
	locks for zx tree implementors
*/
package locks

import (
	"sync"
)

// rwlock on a path for meta and/or data
type pLock {
	n int			// of procs using this
	mlk, dlk sync.RWMutex	// RW lock on this file meta/data
}

// A set of RW locks for file paths.
// Implementation assumes there will be not too many locks in the tree.
type Set {
	lk sync.Mutex		// for the set of locks
	paths map[string] *pLock	// locks in use
	n int			// total sum of n in all locks
	nolocks sync.Cond		// waiting for no locks held
}

func (s *Set) init() {
	s.paths = make(map[string]*pLock)
	s.nolocks.L = &s.lk
}

// Gain a read lock on this path for both meta and data
func (s *Set) RLock(path string) {
	s.rlock(path, true, true)
}

// Gain a read lock on this path for the meta
func (s *Set) RLockMeta(path string) {
	s.rlock(path, true, false)
}

// Gain a read lock on this path for the data
func (s *Set) RLockData(path string) {
	s.rlock(path, false, true)
}

func nlocks(meta, data bool) int {
	if meta && data {
		return 2
	}
	if meta || data {
		return 1
	}
	return 0
}

func (s *Set) rlock(path string, meta, data bool) {
	s.lk.Lock()
	if s.paths == nil {
		s.init()
	}
	p := s.paths[path]
	n := nlocks(meta, data)
	s.n += n
	if p != nil {
		p.n += n
		s.lk.Unlock()
		if meta {
			p.mlk.RLock()
		}
		if data {
			p.dlk.RLock()
		}
		return
	}
	p = &pLock{n: n}
	if meta {
		p.mlk.RLock()
	}
	if data {
		p.dlk.RLock()
	}
	s.paths[path] = p
	s.lk.Unlock()
}

// Release a read lock on this path for both meta and data
func (s *Set) RUnlock(path string) {
	s.runlock(path, true, true)
}

// Release a read lock on this path for meta
func (s *Set) RUnlockMeta(path string) {
	s.runlock(path, true, false)
}

// Release a read lock on this path for data
func (s *Set) RUnlockData(path string) {
	s.runlock(path, false, true)
}

func (s *Set) runlock(path string, meta, data bool) {
	s.lk.Lock()
	defer s.lk.Unlock()
	if s.paths == nil {
		panic("nil paths")
	}
	p := s.paths[path]
	if p == nil {
		panic("locks: runlock without lock")
	}
	n := nlocks(meta, data)
	if p.n < n || s.n < n {
		panic("neg n")
	}
	p.n -= n
	if p.n == 0 {
		delete(s.paths, path)
	}
	if data {
		p.dlk.RUnlock()
	}
	if meta {
		p.mlk.RUnlock()
	}
	s.n -= n
	if s.n == 0 {
		s.nolocks.Signal();
	}
}

// Gain a write lock on this path for both meta and data
func (s *Set) Lock(path string) {
	s.lock(path, true, true)
}

// Gain a write lock on this path for meta
func (s *Set) LockMeta(path string) {
	s.lock(path, true, false)
}

// Gain a write lock on this path for data
func (s *Set) LockData(path string) {
	s.lock(path, false, true)
}

func (s *Set) lock(path string, meta, data bool) {
	s.lk.Lock()
	if s.paths == nil {
		s.init()
	}
	n := nlocks(meta, data)
	p := s.paths[path]
	s.n += n
	if p != nil {
		p.n += n
		s.lk.Unlock()
		if meta {
			p.mlk.Lock()
		}
		if data {
			p.dlk.Lock()
		}
		return
	}
	p = &pLock{n: n}
	if meta {
		p.mlk.Lock()
	}
	if data {
		p.dlk.Lock()
	}
	s.paths[path] = p
	s.lk.Unlock()
}

// Release a write lock on this path for both meta and data
func (s *Set) Unlock(path string) {
	s.unlock(path, true, true)
}

// Release a write lock on this path for meta
func (s *Set) UnlockMeta(path string) {
	s.unlock(path, true, false)
}

// Release a write lock on this path for data
func (s *Set) UnlockData(path string) {
	s.unlock(path, false, true)
}

func (s *Set) unlock(path string, meta, data bool) {
	s.lk.Lock()
	if s.paths == nil {
		panic("nil paths")
	}
	n := nlocks(meta, data)
	p := s.paths[path]
	if p == nil {
		panic("locks: unlock without lock")
	}
	if p.n < n || s.n < n {
		panic("neg n")
	}
	p.n -= n
	if p.n == 0 {
		delete(s.paths, path)
	}
	if data {
		p.dlk.Unlock()
	}
	if meta {
		p.mlk.Unlock()
	}
	s.n -= n
	if s.n == 0 {
		s.nolocks.Signal();
	}
	s.lk.Unlock()
}

// Runs fn while everything is quiescent.
func (s *Set) QuiescentRun(fn func()) {
	s.lk.Lock()
	defer s.lk.Unlock()
	if s.paths == nil {
		s.init()
	}
	for s.n > 0 {
		s.nolocks.Wait();
	}
	fn();
}
