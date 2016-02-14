/*
	Memory block buffers
*/
package mblk

import (
	"clive/dbg"
	"errors"
	"io"
	"sort"
	"sync"
)

// Popular sizes.
const (
	KiB = 1024
	MiB = KiB * 1024
	GiB = MiB * 1024
	TiB = GiB * 1024
)

// for holes, data is nil.
struct block {
	off  int
	data []byte
	n    int // len(data) or size for holes.
}

// A buffer made out of blocks of data, perhaps with holes.
// Sequential reads/writes are more efficient that random ones.
// It might be large and sparse, beware that asking for
// a string or the array of bytes might run out of memory quickly.
// It can be safely used by multiple processes, there's a lock inside.
struct Buffer {
	blks []*block
	sync.Mutex
}

// Like an open fd for a block, with its own offset.
struct BufferFd {
	*Buffer
	seqrw  bool // set if sequential read/write is going on
	rwoff  int  // next read off (if seqrd)
	rwi    int  // next blk idx (if seqrd)
	rwboff int  // next blk off (if seqrd)
}

var (
	// Default block size for Buffer in bytes
	// Kept as a var for testing.
	Size = 16 * KiB

	// Max number of cached buffers.
	// Change only at init time.
	Count = 128

	zeros = make([]byte, 4096)

	Debug   bool
	dprintf = dbg.FlagPrintf(&Debug)
)

// Return the string for the buffer
func (b *Buffer) String() string {
	return string(b.Bytes())
}

// Return the contents in a single array.
func (b *Buffer) Bytes() []byte {
	bytes := make([]byte, b.Len())
	if len(bytes) == 0 {
		return bytes
	}
	for _, blk := range b.blks {
		if blk.data != nil {
			copy(bytes[blk.off:], blk.data[:])
		}
	}
	return bytes
}

func (b *Buffer) dump() {
	if !Debug {
		return
	}
	if b == nil {
		dprintf("<nil blocks>\n")
		return
	}
	dprintf("blocks:\n")
	for _, blk := range b.blks {
		blk.dump()
	}
	dprintf("\n")
}

func (blk *block) dump() {
	dprintf("%4d: [%d]\n", blk.off, blk.n)
	if blk.data == nil {
		dprintf("\tzeros\n")
		return
	}
	dprintf("%s\n", dbg.HexStr(blk.data, blk.n))
}

// Return the length in bytes.
// Ok if b is nil
func (b *Buffer) Len() int {
	if b != nil {
		b.Lock()
		defer b.Unlock()
	}
	return b.len()
}

func (b *Buffer) len() int {
	if b == nil || len(b.blks) == 0 {
		return 0
	}
	last := b.blks[len(b.blks)-1]
	return last.off + last.n
}

// Drop the content
// Ok if b is nil
func (b *Buffer) Reset() {
	b.Truncate(0)
}

func (b *Buffer) seek(pos int) (nblk, blkoff int) {
	if b == nil || pos <= 0 {
		return 0, 0
	}
	if pos >= b.len() {
		return len(b.blks), 0
	}
	i := sort.Search(pos, func(i int) bool {
		return i >= len(b.blks) || b.blks[i].off+b.blks[i].n > pos
	})
	if i >= len(b.blks) {
		// should not happen
		return len(b.blks), 0
	}
	return i, pos - b.blks[i].off
}

// Truncate at size n,
// if n is 0, it's ok if b is nil.
func (b *Buffer) Truncate(n int64) error {
	dprintf("truncate %d\n", n)
	defer b.dump()
	if b != nil {
		b.Lock()
		defer b.Unlock()
	}
	return b.truncate(int(n))
}

func (b *Buffer) grow(nz int) error {
	if len(b.blks) > 0 {
		last := b.blks[len(b.blks)-1]
		if last.data == nil {
			if last.n < Size {
				lz := Size - last.n
				if lz > nz {
					lz = nz
				}
				last.n += lz
				nz -= lz
			}
		} else {
			lz := nz
			if lz > cap(last.data)-last.n {
				lz = cap(last.data) - last.n
			}
			if lz > 0 {
				last.data = last.data[:len(last.data)+lz]
				last.n += lz
				nz -= lz
			}
		}
	}
	// grow: now add holes for the extra nz bytes (not a single huge hole)
	off := b.len()
	for cnt := 0; cnt < nz; {
		hsz := nz - cnt
		if hsz > Size {
			hsz = Size
		}
		b.blks = append(b.blks, &block{off: off, n: hsz})
		cnt += hsz
		off += hsz
	}
	return nil
}

// Zero out the given buffer
func Clear(b []byte) {
	zb := b
	for len(zb) > 0 {
		n := copy(zb, zeros)
		zb = zb[n:]
	}
}

func (b *Buffer) truncate(n int) error {
	sz := b.len()
	if n == sz {
		return nil
	}
	if b == nil {
		return errors.New("offset out of range in truncate")
	}
	if n >= sz {
		return b.grow(n - sz)
	}
	nb, boff := b.seek(n)
	if boff == 0 {
		b.blks = b.blks[:nb]
		return nil
	}
	b.blks = b.blks[:nb+1]
	blk := b.blks[nb]
	old := blk.n
	blk.n = int(n) - blk.off
	if blk.data != nil {
		// we always assume that new data is zeroed, so we must
		// clear when we shrink.
		for i := blk.n; i < old; i++ {
			blk.data[i] = 0
		}
		blk.data = blk.data[:blk.n]
	}
	return nil
}

func bufsz(sz int) int {
	if sz <= Size {
		return Size
	}
	return sz
}

// Write more data to the buffer (append).
// The data is copied.
func (b *Buffer) Write(p []byte) (n int, err error) {
	if Debug {
		dprintf("write [%d]{%s}\n", len(p), dbg.HexStr(p, 10))
		defer b.dump()
	}
	if b != nil {
		b.Lock()
		defer b.Unlock()
	}
	return b.write(p)
}

func (b *Buffer) write(p []byte) (n int, err error) {
	nn := int64(b.len()) + int64(len(p))
	if nn != int64(int(nn)) {
		panic("blocks: overflow: int is too short")
	}
	if len(p) == 0 {
		return 0, nil
	}
	nw := len(p)
	// Grow a block if we can
	if len(b.blks) > 0 {
		last := b.blks[len(b.blks)-1]
		if last.data != nil {
			sz := len(last.data)
			left := cap(last.data) - sz
			if left > nw {
				left = nw
			}
			if left > 0 {
				last.data = last.data[:sz+left]
				nc := copy(last.data[sz:], p)
				last.n += nc
				p = p[left:]
			}
			if len(p) == 0 {
				return nw, nil
			}
		}
	}
	for len(p) > 0 {
		np := make([]byte, Size)
		sz := cap(np)
		if sz > len(p) {
			sz = len(p)
		}
		np = np[:sz]
		copy(np, p)
		b.blks = append(b.blks, &block{b.len(), np, sz})
		p = p[sz:]
	}
	return nw, nil
}

// Write more data to the buffer at the given offset.
// The data is not inserted, it is rewritten.
// Holes are ok.
func (b *Buffer) WriteAt(p []byte, at int64) (n int, err error) {
	if Debug {
		dprintf("write [%d]{%s} at %d\n", len(p), dbg.HexStr(p, 10), at)
		defer b.dump()
	}
	if b != nil {
		b.Lock()
		defer b.Unlock()
	}
	woff := int(at)
	nw := len(p)
	if woff > b.len() {
		b.truncate(woff)
		b.dump()
		// and continue...
	}
	if woff < b.len() {
		// overwrite existing data
		off := woff
		nwr := b.len() - off
		if nwr > len(p) {
			nwr = len(p)
		}
		nb, boff := b.seek(off)
		for nwr > 0 {
			blk := b.blks[nb]
			nbwr := nwr
			if nbwr > blk.n-boff {
				nbwr = blk.n - boff
			}
			if blk.data == nil {
				blk.data = make([]byte, Size)
				blk.data = blk.data[:blk.n]
			}
			copy(blk.data[boff:], p[:nbwr])
			p = p[nbwr:]
			nwr -= nbwr
			nb++
			boff = 0
		}
	}
	b.write(p)
	return nw, nil
}

// Write a string into the buffer
func (b *Buffer) WriteString(s string) (n int, err error) {
	return b.Write([]byte(s))
}

// Prepare to read from b, sets the read offset to 0.
// Not really needed if read was never called on the buffer.
func (b *Buffer) Open() (*BufferFd, error) {
	if b == nil {
		return nil, nil
	}
	br := &BufferFd{Buffer: b}
	return br, nil
}

// Done reading from b (reset read offset to 0)
// Not really needed if the buffer is not going to be reread
func (b *BufferFd) Close() error {
	return nil
}

// Any write into the fd resets the read offset to 0.
// Ok if b is nil.
func (b *BufferFd) Read(p []byte) (int, error) {
	if b != nil {
		b.Lock()
		defer b.Unlock()
	}
	n, err := b.read(p)
	if Debug {
		if n > 0 {
			dprintf("read [%d] -> [%d]{%s}\n", len(p), n, dbg.HexStr(p[:n], 20))
		} else {
			dprintf("read [%d] -> [0]\n", len(p))
		}
	}
	return n, err
}

func (b *BufferFd) Seek(uoff int64, whence int) (int64, error) {
	off := int(uoff)
	if int64(off) != uoff {
		panic("overflow")
	}
	b.Lock()
	defer b.Unlock()
	switch whence {
	case 0:
		b.rwoff = off
	case 1:
		b.rwoff = b.rwoff + off
	case 2:
		b.rwoff = b.len() + off
	}
	if b.rwoff < 0 {
		b.rwoff = 0
	}
	b.rwi, b.rwboff = b.seek(b.rwoff)
	return int64(b.rwoff), nil
}

func (b *BufferFd) Write(p []byte) (int, error) {
	b.Lock()
	off := b.rwoff
	b.rwoff += len(p)
	b.Unlock()
	nw, err := b.WriteAt(p, int64(off))
	b.Lock()
	b.rwi, b.rwboff = b.seek(b.rwoff)
	b.Unlock()
	return nw, err
}

func (b *BufferFd) read(p []byte) (int, error) {
	if b.len() == 0 {
		dprintf("read: eof\n")
		return 0, io.EOF
	}
	if b.rwoff >= b.len() {
		dprintf("read: off %d: eof\n", b.rwoff)
		return 0, io.EOF
	}
	tot := 0
	var err error
	for tot < len(p) && b.rwi < len(b.blks) {
		blk := b.blks[b.rwi]
		nr := len(p) - tot
		if nr > blk.n-b.rwboff {
			nr = blk.n - b.rwboff
		}
		if blk.data == nil {
			blk.data = make([]byte, Size)
			blk.data = blk.data[:blk.n]
		}
		copy(p[tot:], blk.data[b.rwboff:b.rwboff+nr])
		tot += nr
		b.rwboff += nr
		b.rwoff += nr
		if b.rwboff >= blk.n {
			b.rwi++
			b.rwboff = 0
			if b.rwi >= len(b.blks) {
				err = io.EOF
			}
		}
	}
	if tot == 0 {
		return 0, io.EOF
	}
	return tot, err
}

func (b *Buffer) ReadAt(p []byte, roff int64) (int, error) {
	br, _ := b.Open()
	n, err := br.ReadAt(p, roff)
	return n, err
}

func (b *BufferFd) ReadAt(p []byte, roff int64) (n int, err error) {
	if b != nil {
		b.Lock()
		defer b.Unlock()
	}
	off := int(roff)
	if off >= b.len() {
		dprintf("read [%d] at %d -> [0]\n", len(p), roff)
		return 0, io.EOF
	}
	if b.rwoff == off {
		n, err = b.read(p)
	} else {
		b.rwoff = off
		b.rwi, b.rwboff = b.seek(off)
		if b.rwi >= len(b.blks) {
			return 0, io.EOF
		}
		b.rwoff = off
		n, err = b.read(p)
	}
	if Debug {
		if n > 0 {
			dprintf("read [%d] at %d -> [%d]{%s}\n",
				len(p), roff, n, dbg.HexStr(p[:n], 20))
		} else {
			dprintf("read [%d] at %d -> [0]\n", len(p), roff)
		}
	}
	return n, err
}

// Write the contents of b to w.
func (b *Buffer) WriteTo(w io.Writer) (int64, error) {
	if b != nil {
		b.Lock()
		defer b.Unlock()
	}
	tot := 0
	for i := 0; i < len(b.blks); i++ {
		blk := b.blks[i]
		if blk.data == nil {
			blk.data = make([]byte, Size)
			blk.data = blk.data[:blk.n]
		}
		nw, err := w.Write(blk.data)
		tot += nw
		if err != nil {
			return int64(tot), err
		}
	}
	return int64(tot), nil
}

// Send the contents of b[off:off+cnt] to the given chan.
// if count < 0 it means send everything starting at off.
// The data is copied and the buffer is unlocked during sends
// Returns the number of bytes, number of messages and
// the error status.
// The channel is not closed.
// Ok if b is nil.
func (b *Buffer) SendTo(soff, scount int64, c chan<- []byte) (int64, int, error) {
	if b == nil || scount == 0 {
		return 0, 0, nil
	}
	b.Lock()
	count := int(scount)
	off := int(soff)
	if off >= b.len() {
		b.Unlock()
		return 0, 0, nil
	}
	tot := 0
	b.Unlock()
	bfd, err := b.Open()
	if err != nil {
		return 0, 0, err
	}
	defer bfd.Close()
	bfd.Seek(soff, 0)
	nm := 0
	for count < 0 || tot < count {
		sz := Size
		if count > 0 && count-tot < sz {
			sz = count - tot
		}
		buf := make([]byte, sz)
		nr, err := bfd.Read(buf)
		if nr == 0 {
			break
		}
		buf = buf[:nr]
		tot += nr
		if err != nil && err != io.EOF {
			return int64(tot), nm, err
		}
		if ok := c <- buf; !ok {
			return int64(tot), nm, cerror(c)
		}
		nm++
	}
	return int64(tot), nm, nil
}

// Receive the contents starting at off from the given chan.
// if off < 0 it means the initial size of the buffer.
// The data is copied.
// The buffer is not locked during receives
// Returns the number of bytes, number of messages and
// the error status.
// The channel is not closed.
func (b *Buffer) RecvFrom(soff int64, c <-chan []byte) (int64, int, error) {
	b.Lock()
	b.Unlock()
	bfd, err := b.Open()
	if err != nil {
		return 0, 0, err
	}
	defer bfd.Close()
	if soff < 0 {
		bfd.Seek(0, 2)
	} else {
		bfd.Seek(soff, 0)
	}
	tot := 0
	nm := 0
	for data := range c {
		nw, err := bfd.Write(data)
		tot += nw
		nm++
		if err != nil {
			return int64(tot), nm, err
		}
	}
	return int64(tot), nm, cerror(c)
}
