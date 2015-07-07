/*
	Large and sparse buffers for clive programs.
*/
package bufs

import (
	"io"
	"sync"
	"sort"
	"errors"
	"clive/dbg"
	"os"
	"crypto/sha1"
	"fmt"
)

// Popular sizes.
const (
	KiB = 1024
	MiB = KiB*1024
	GiB = MiB*1024
	TiB = GiB*1024
)

// for holes, data is nil.
type block  {
	off  int
	data []byte
	n int	// len(data) or size for holes.
}

/*
	A buffer made out of blocks of data, perhaps with holes.
	Sequential reads/writes are more efficient that random ones.
	It might be large and sparse, beware that asking for
	a string or the array of bytes might run out of memory quickly.
	if Mutex is allocated, it can be used by multiple procs safely.
*/
type Blocks  {
	blks  []*block
	*sync.Mutex
}

// Like an open fd for a block, with its own offset.
type BlockFd {
	*Blocks
	seqrw bool // set if sequential read/write is going on
	rwoff int	// next read off (if seqrd)
	rwi int	// next blk idx (if seqrd)
	rwboff int	// next blk off (if seqrd)
}

const nBlockBufs = 1024

var (
	// Default block size for Blocks in bytes
	// Kept as a var for testing.
	Size = 16*KiB

	// Max number of cached buffers.
	// Change only at init time.
	Count = 128

	debug bool
	dprintf = dbg.FlagPrintf(os.Stdout, &debug)

	blkbufc = make(chan []byte, nBlockBufs)
)

/*
	Return the string for the buffer
*/
func (b *Blocks) String() string {
	return string(b.Bytes())
}

/*
	Return the contents in a single array.
*/
func (b *Blocks) Bytes() []byte {
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

func (b *Blocks) dump() {
	if !debug {
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
	dbg.HexDump(os.Stdout, blk.data, blk.off)
}

/*
	Return the length in bytes.
	Ok if b is nil
*/
func (b *Blocks) Len() int {
	if b != nil && b.Mutex != nil {
		b.Lock()
		defer b.Unlock()
	}
	return b.len()
}

func (b *Blocks) len() int {
	if b==nil || len(b.blks)==0 {
		return 0
	}
	last := b.blks[len(b.blks)-1]
	return last.off + last.n
}

/*
	Drop the content
	Ok if b is nil
*/
func (b *Blocks) Reset() {
	b.Truncate(0)
}

func (b *Blocks) seek(pos int) (nblk, blkoff int) {
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

/*
	Truncate at size n,
	if n is 0, it's ok if b is nil.
*/
func (b *Blocks) Truncate(n int64) error {
	dprintf("truncate %d\n", n)
	defer b.dump()
	if b != nil && b.Mutex != nil {
		b.Lock()
		defer b.Unlock()
	}
	return b.truncate(int(n))
}

func (b *Blocks) grow(nz int) error {
	if len(b.blks) > 0 {
		last := b.blks[len(b.blks)-1]
		if last.data == nil  {
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
			if lz > cap(last.data) - last.n {
				lz = cap(last.data) - last.n
			}
			if lz > 0 {
				last.data=last.data[:len(last.data)+lz]
				last.n += lz
				nz -= lz
			}
		}
	}
	// grow: now add holes for the extra nz bytes (not a single huge hole)
	off := b.len()
	for cnt := 0; cnt < nz; {
		hsz := nz-cnt
		if hsz > Size {
			hsz = Size
		}
		b.blks = append(b.blks, &block{off: off, n: hsz})
		cnt += hsz
		off += hsz
	}
	return nil
}

func recycle (blks []*block) {
	for _, b := range blks {
		if !Recycle(b.data) {
			break
		}
	}
}

func balloc() []byte {
	return New()
}

var zeros = make([]byte, 4096)

// Zero out the given buffer
func Clear(b []byte) {
	zb := b
	for len(zb) > 0 {
		n := copy(zb, zeros)
		zb = zb[n:]
	}
}


/*
	Return a new buffer.
*/
func New() []byte {
	select {
	case b := <- blkbufc:
		b = b[:cap(b)]
		Clear(b)
		if len(b) < Size {
			panic("short buf in bufs")
		}
		return b
	default:
		return make([]byte, Size)
	}
}

/*
	Release a buffer.
	Returns false if the buferr was thrown away and not recycled.
	If you recicle a bunch of buffers, you might stop in that case.
*/
func Recycle(buf []byte) bool {
	if cap(buf) < Size {
		return true
	}
	select {
	case blkbufc <- buf:
		return true
	default:
		return false
	}
}

func (b *Blocks) truncate(n int) error {
	sz := b.len()
	if n == sz {
		return nil
	}
	if b==nil {
		return errors.New("offset out of range in truncate")
	}
	if n >=sz {
		return b.grow(n-sz)
	}
	nb, boff := b.seek(n)
	if boff == 0 {
		recycle(b.blks[nb:])
		b.blks = b.blks[:nb]
		return nil
	}
	recycle(b.blks[nb+1:])
	b.blks = b.blks[:nb+1]
	blk := b.blks[nb]
	old := blk.n
	blk.n = int(n) - blk.off
	if blk.data != nil {
		// we always assume that new data is zeroed, so we must
		// clear when we shrink.
		for i := blk.n; i < old; i++ { blk.data[i] = 0 }
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

/*
	Write more data to the buffer (append).
	The data is copied.
*/
func (b *Blocks) Write(p []byte) (n int, err error) {
	if debug {
		dprintf("write [%d]{%s}\n", len(p), dbg.HexStr(p, 10))
		defer b.dump()
	}
	if b != nil && b.Mutex != nil {
		b.Lock()
		defer b.Unlock()
	}
	return b.write(p)
}

func (b *Blocks) write(p []byte) (n int, err error) {
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
			if left  > 0 {
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
		np := balloc()
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

/*
	Write more data to the buffer at the given offset.
	The data is not inserted, it is written.
	Holes are ok.
*/
func (b *Blocks) WriteAt(p []byte, at int64) (n int, err error) {
	if debug {
		dprintf("write [%d]{%s} at %d\n", len(p), dbg.HexStr(p, 10), at)
		defer b.dump()
	}
	if b != nil && b.Mutex != nil {
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
			if nbwr > blk.n - boff {
				nbwr = blk.n - boff
			}
			if blk.data == nil {
				blk.data = balloc()
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

func (b *Blocks) WriteString(s string) (n int, err error) {
	return b.Write([]byte(s))
}

// Prepare to read from b, sets the read offset to 0.
// Not really needed if read was never called on the buffer.
func (b *Blocks) Open() (*BlockFd, error) {
	if b == nil {
		return nil, nil
	}
	br := &BlockFd{Blocks: b}
	return br, nil
}

// Done reading from b (reset read offset to 0)
// Not really needed if the buffer is not going to be reread
func (b *BlockFd) Close() error {
	return nil
}

/*
	Any write into the buffer resets the read offset to 0.
	Ok if b is nil.
*/
func (b *BlockFd) Read(p []byte) (int, error) {
	if b != nil && b.Mutex != nil {
		b.Lock()
		defer b.Unlock()
	}
	n , err := b.read(p)
	if debug && n > 0 {
		dprintf("read [%d] -> [%d]{%s}\n", len(p), n, dbg.HexStr(p[:n], 20))
	} else if debug {
		dprintf("read [%d] -> [0]\n", len(p))
	}
	return n, err
}

func (b *BlockFd) Seek(uoff int64, whence int) (int64, error) {
	off := int(uoff)
	if int64(off) != uoff {
		panic("overflow")
	}
	if b.Mutex != nil {
		b.Lock()
		defer b.Unlock()
	}
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

func (b *BlockFd) Write(p []byte) (int, error) {
	b.Lock()
	off := b.rwoff
	b.Unlock()
	nw, err := b.WriteAt(p, int64(off))
	b.Lock()
	b.rwoff += nw
	b.rwi, b.rwboff = b.seek(b.rwoff)
	b.Unlock()
	return nw, err
}

func (b *BlockFd) read(p []byte) (int, error) {
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
		if nr > blk.n - b.rwboff {
			nr = blk.n - b.rwboff
		}
		if blk.data == nil {
			blk.data = balloc()
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

func (b *Blocks) ReadAt(p []byte, roff int64) (int, error) {
	br, _ := b.Open()
	n, err := br.ReadAt(p, roff)
	return n, err
}

func (b *BlockFd) ReadAt(p []byte, roff int64) (int, error) {
	if b != nil && b.Mutex != nil {
		b.Lock()
		defer b.Unlock()
	}
	off := int(roff)
	if off >= b.len() {
		dprintf("read [%d] at %d -> [0]\n", len(p), roff)
		return 0, io.EOF
	}
	if b.rwoff == off {
		n, err := b.read(p)
		if debug && n > 0 {
			dprintf("read [%d] at %d -> [%d]{%s}\n", len(p), roff, n, dbg.HexStr(p[:n], 20))
		} else if debug {
			dprintf("read [%d] at %d -> [0]\n", len(p), roff)
		}
		return n, err
	}
	b.rwoff = off
	b.rwi, b.rwboff = b.seek(off)
	if b.rwi >= len(b.blks) {
		return 0, io.EOF
	}
	b.rwoff = off
	n, err := b.read(p)
	if debug && n > 0 {
		dprintf("read [%d] at %d -> [%d]{%s}\n", len(p), roff, n, dbg.HexStr(p[:n], 20))
	} else if debug {
		dprintf("read [%d] at %d -> [0]\n", len(p), roff)
	}
	return n, err
}

/*
	Receive data from datac, writing it to the end of the buffer.
	Stops on the first empty message or when the channel is closed.
	Returns the receive error or nil upon eof.
*/
func (b *Blocks) RecvFrom(datac <-chan []byte) (n int, err error) {
	tot := 0
	for {
		data := <-datac
		if len(data) == 0 {
			break
			continue
		}
		if _, err := b.Write(data); err != nil {
			close(datac, err)
			return tot, err
		}
		tot += len(data)
	}
	return tot, cerror(datac)
}

/*
	Receive data from datac, writing it to the given pos.
	Stops on the first empty message or when the channel is closed.
	Returns the receive error or nil upon eof.
*/
func (b *Blocks) RecvAtFrom(roff int64, datac <-chan []byte) (n int64, err error) {
	off := int(roff)
	tot := 0
	for {
		data := <-datac
		if len(data) == 0 {
			break
			continue
		}
		if _, err := b.WriteAt(data, int64(off+tot)); err != nil {
			close(datac, err)
			return int64(tot), err
		}
		tot += len(data)
	}
	return int64(tot), cerror(datac)
}

/*
	Write the contents of b to w.
*/
func (b *Blocks) WriteTo(w io.Writer) (int64, error) {
	if b != nil && b.Mutex != nil {
		b.Lock()
		defer b.Unlock()
	}
	tot := 0
	for i := 0; i < len(b.blks); i++ {
		blk := b.blks[i]
		if blk.data == nil {
			blk.data = balloc()
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

/*
	Send the contents of b[off:off+cnt] to the given chan.
	if count < 0 it means send everything starting at off.
	The data is copied.
	Returns the number of bytes, number of messages and
	the error status.
	The channel is not closed.
	Ok if b is nil.
*/
func (b *Blocks) SendTo(soff, scount int64, c chan<- []byte) (int64, int, error) {
	if b != nil && b.Mutex != nil {
		b.Lock()
		defer b.Unlock()
	}
	off := int(soff)
	count := int(scount)
	if off >= b.len() {
		return 0, 0, nil
	}
	if count < 0 {
		count = b.len() - off
	}
	nb, boff := b.seek(off)
	tot := 0
	nm := 0
	for tot < count && nb < len(b.blks) {
		blk := b.blks[nb]
		n := count - tot
		if n > blk.n - boff {
			n = blk.n - boff
		}
		if blk.data == nil {
			zeros := make([]byte, n)
			if ok := c <- zeros; !ok {
				return int64(tot), nm, cerror(c)
			}
		} else {
			dat := make([]byte, n)
			copy(dat, blk.data[boff:])
			if ok := c <- dat; !ok {
				return int64(tot), nm, cerror(c)
			}
		}
		tot += n
		boff = 0
		nb++
		nm++
	}
	return int64(tot), nm, nil
}

/*
 * Return a SHA1 sum for the blocks
*/
func (b *Blocks) Sum() string {
	h := sha1.New()
	for _, blk := range b.blks {
		if blk.data == nil {
			blk.data = make([]byte, blk.n)
		}
		h.Write(blk.data)
	}
	sum := h.Sum(nil)
	return fmt.Sprintf("%040x", sum)
}
