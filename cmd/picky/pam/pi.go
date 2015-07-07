package main

import (
	"bufio"
	"bytes"
	"clive/cmd/picky/paminstr"
	"clive/cmd/picky/pbytes"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"runtime"
	godebug "runtime/debug"
	"runtime/pprof"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const Poison = 242

type Vent  {
	name   string // of variable or constant
	tid    uint   // type
	addr   uint32 // in memory (offset for args, l.vars.)
	fname  string
	lineno int
	val    string // initial value as a string, or "".
	fields []Vent // aggregate members
}

type Tent  {
	name   string   // of the type
	fmt    rune     // value format character
	first  int      // legal value or index
	last   int      // idem
	nitems int      // # of values or elements
	sz     uint     // in memory for values
	etid   uint     // element type id
	lits   []string // names for literals
	fields []Vent   // only name, tid, and addr defined
}

type Pent  {
	name   string // for procedure/function
	addr   uint   // for its code in text
	nargs  int    // # of arguments
	nvars  int    // # of variables
	retsz  int    // size for return type or 0
	argsz  int    // size for arguments in stack
	varsz  int    // size for local vars in stack
	fname  string
	lineno int
	args   []Vent // Var descriptors for args
	vars   []Vent // Var descriptors for local vars.
}

type Pc  {
	pc     uint32
	fname  string
	lineno uint
	next   *Pc  //Pc with leaks; for leaks
	n      uint // # of leaks in this Pc; for leaks
}

type FileSt  {
	bin    *bufio.Reader
	fname  string
	lineno uint
	binln  []rune
	toks   []string
}

//ints here are indexes in bytes
type MachSt  {
	text                     []uint32
	stack                    []byte
	globend, stackend, maxsp int
	// regs	program counter, procedure id
	//		stack pointer, frame pointer, (local) var pointer, argument pointer
	pc, procid     uint32
	sp, fp, vp, ap int
}

type MachAbs  {
	entry  uint
	ninstr uint
	tents  []Tent
	vents  []Vent
	pents  []Pent
	pcs    []Pc
}

const (
	sz32Bits = 4
)

var (
	debug          map[rune]int = make(map[rune]int)
	waitforwindows              = false
	statflag       bool
	mst            MachSt
	mabs           MachAbs
	fst            *FileSt
)

func (m *MachAbs) findpc(pc uint32) *Pc {
	for i := 0; i < len(m.pcs); i++ {
		if m.pcs[i].pc >= pc {
			if i > 0 {
				i--
			}
			return &m.pcs[i]
		}
	}
	return nil
}

func done(sts string) {
	flushall()
	pprof.StopCPUProfile()
	if statflag {
		fmt.Fprintf(os.Stderr, "%d instructions executed\n", mabs.ninstr)
		tot := mst.globend + mst.maxsp + sz32Bits*len(mst.text)
		tot += nheap
		fmt.Fprintf(os.Stderr, "%d bytes used", tot)
		fmt.Fprintf(os.Stderr, " (%d code +", sz32Bits*len(mst.text))
		fmt.Fprintf(os.Stderr, " %d data +", mst.globend)
		fmt.Fprintf(os.Stderr, " %d heap +", nheap)
		fmt.Fprintf(os.Stderr, " %d stack)\n", mst.maxsp)
	}
	if sts == "" {
		undisposed()
	}
	if waitforwindows {
		finishwait()
	}
	if sts != "" {
		if sts != "panic" {
			fmt.Fprintf(os.Stderr, "error: %s\n", sts)
		}
		os.Exit(1)
	}
	os.Exit(0)
}

func pop8() byte {
	if mst.sp < 1 {
		panic("stack underflow")
	}
	mst.sp -= 1
	return mst.stack[mst.sp]
}

func top8() byte {
	if mst.sp < 1 {
		panic("stack underflow")
	}
	return mst.stack[mst.sp]
}

func push8(c byte) {
	if int(mst.sp)+1 > mst.stackend {
		panic("stack overflow")
	}
	mst.stack[mst.sp] = c
	mst.sp += 1
}

func pop32() uint32 {
	var (
		n   uint32
		err error
		ifc interface{}
	)
	if mst.sp < 4 {
		panic("stack underflow")
	}
	mst.sp -= 4
	ifc, err = pbytes.UnmarshalBinary(mst.stack[mst.sp:], n)
	if err != nil {
		panic("pop32 unmarshal")
	}
	n = ifc.(uint32)
	return n
}

func top32() uint32 {
	var (
		n   uint32
		err error
		ifc interface{}
	)
	if mst.sp < 4 {
		panic("stack underflow")
	}
	ifc, err = pbytes.UnmarshalBinary(mst.stack[mst.sp:], n)
	if err != nil {
		panic("top32 unmarshal")
	}
	n = ifc.(uint32)
	return n
}

func push32(c uint32) {
	if int(mst.sp)+4 > mst.stackend {
		panic("stack overflow")
	}
	err := pbytes.MarshalBinary(mst.stack[mst.sp:], c)
	if err != nil {
		panic("push32")
	}
	mst.sp += 4
}

func pop64() uint64 {
	var (
		n   uint64
		err error
		ifc interface{}
	)
	if mst.sp < 8 {
		panic("stack underflow")
	}
	mst.sp -= 8
	ifc, err = pbytes.UnmarshalBinary(mst.stack[mst.sp:], n)
	if err != nil {
		panic("pop64 unmarshal")
	}
	n = ifc.(uint64)
	return n
}

func top64() uint64 {
	var (
		n   uint64
		err error
		ifc interface{}
	)
	if mst.sp < 8 {
		panic("stack underflow")
	}
	ifc, err = pbytes.UnmarshalBinary(mst.stack[mst.sp:], n)
	if err != nil {
		panic("top64 unmarshal")
	}
	n = ifc.(uint64)
	return n
}

func push64(c uint64) {
	if int(mst.sp)+8 > mst.stackend {
		panic("stack overflow")
	}
	err := pbytes.MarshalBinary(mst.stack[mst.sp:], c)
	if err != nil {
		panic("push64")
	}
	mst.sp += 8
}

func popr() float64 {
	var (
		n   float32
		err error
		ifc interface{}
	)
	if mst.sp < 4 {
		panic("stack underflow")
	}
	mst.sp -= 4
	ifc, err = pbytes.UnmarshalBinary(mst.stack[mst.sp:], n)
	if err != nil {
		panic("popr unmarshal")
	}
	n = ifc.(float32)
	c := float64(n)
	if math.IsNaN(c) {
		panic("invalid  operation on float")
	}
	return c
}

func topr() float64 {
	var (
		n   float32
		err error
		ifc interface{}
	)
	if mst.sp < 4 {
		panic("stack underflow")
	}
	ifc, err = pbytes.UnmarshalBinary(mst.stack[mst.sp:], n)
	if err != nil {
		panic("topr unmarshal")
	}
	n = ifc.(float32)
	c := float64(n)
	if math.IsNaN(c) {
		panic("invalid operation on float")
	}
	return c
}

func pushr(c float64) {
	if math.IsNaN(c) {
		panic("invalid operation on float")
	}
	n := float32(c)
	if int(mst.sp)+4 > mst.stackend {
		panic("stack overflow")
	}
	err := pbytes.MarshalBinary(mst.stack[mst.sp:], n)
	if err != nil {
		panic("pushr")
	}
	mst.sp += 4
}

func pushn(xd []byte) {
	if int(mst.sp)+len(xd) > mst.stackend {
		panic("stack overflow")
	}
	err := pbytes.MarshalBinary(mst.stack[mst.sp:], xd)
	if err != nil {
		panic("pushn")
	}
	mst.sp += len(xd)
}

func popn(sz int) string {
	var (
		rv  rune
		err error
		ifc interface{}
	)
	r := make([]byte, sz)
	if mst.sp < sz {
		panic("stack underflow")
	}
	mst.sp -= sz
	ifc, err = pbytes.UnmarshalBinary(mst.stack[mst.sp:mst.sp+sz], r)
	if err != nil {
		panic("popn unmarshal")
	}
	p := ifc.([]byte)
	s := make([]byte, 0)
	for i := 0; i < sz; i += 4 {
		rv, _ = utf8.DecodeRune(p[i : i+4])
		s = append(s, byte(rv))
	}
	return string(s)
}

func pushdata(v *Vent) {
	t := tfetch(int(v.tid))
	switch t.fmt {
	case 'i', 'e', 'b', 'c', 'f', 'h', 'g', 'u':
		n, err := strconv.ParseInt(v.val, 0, 64)
		if err != nil {
			panic("expected int")
		}
		push32(uint32(n))
	case 'r', 'l':
		r, err := strconv.ParseFloat(v.val, 64)
		if err != nil {
			panic("expected float")
		}
		pushr(r)
	case 'p':
		n, err := strconv.ParseInt(v.val, 0, 64)
		if err != nil {
			panic("expected int")
		}
		push64(uint64(n))
		break
	case 'a', 'R':
		if len(v.fields) == 0 {
			s := fmt.Sprintf("pushdata: %v: no fields", v)
			panic(s)
		}
		for i := 0; i < t.nitems; i++ {
			pushdata(&v.fields[i])
		}
	case 'X', 'F':
		s := fmt.Sprintf("pushdata: X and F aggregates not supported")
		panic(s)
	case 's':
		s := v.val
		for _, r := range s {
			push32(uint32(r))
		}
		break
	default:
		s := fmt.Sprintf("pushdata: bad type fmt '%c'", t.fmt)
		panic(s)
	}
}

func poison(p []byte) {
	if len(p) == 0 {
		return
	}
	for n := range p {
		p[n] = Poison ^ p[n]
		//
		// All addresses are aligned to word boundaries in
		// the stack. This makes poisoned memory to be odd
		// as an address, and xdispose() and xptr() check for it
		// to detect references to not initialized pointers.
		p[n] |= 1
	}

}

const Stack = 64*1024*1024

func datainit() {
	sz := int(Stack)
	if len(mabs.vents) > 0 {
		v := &mabs.vents[len(mabs.vents)-1]
		sz += int(v.addr) + int(mabs.tents[v.tid].sz)
	}
	mst.stack = make([]byte, sz)
	mst.stackend = sz
	mst.sp = 0
	for _, v := range mabs.vents {
		if int(v.addr) != mst.sp {
			s := fmt.Sprintf("bad data '%s' addr %#x", v.name, v.addr)
			panic(s)
		}
		if debug['D'] != 0 {
			fmt.Fprintf(os.Stderr, "data '%s'\tdaddr %x va %x sz %x val '%s'\n",
				v.name, v.addr, mst.sp, mabs.tents[v.tid].sz, v.val)
		}
		if v.val != "" {
			pushdata(&v)
		} else {
			sz = int(mabs.tents[v.tid].sz)
			poison(mst.stack[mst.sp : mst.sp+sz])
			mst.sp += sz
		}
	}
	mst.globend = mst.sp
	// fake activation record for main
	mst.ap = mst.sp
	mst.vp = mst.sp
	poison(mst.stack[mst.sp : mst.sp+mabs.pents[mabs.entry].varsz])
	mst.sp += mabs.pents[mabs.entry].varsz
	pushdaddr(nil)
	pushdaddr(nil)
	pushdaddr(nil)
	push32(^uint32(0))
	push32(^uint32(0))
	mst.fp = mst.sp
	mst.procid = uint32(mabs.entry)
	if debug['D'] != 0 {
		fmt.Fprintf(os.Stderr, "stack %x globend %x end %x size %#x\n",
			mst.sp, mst.globend, mst.stackend, mst.stackend-mst.sp)
	}
	mst.maxsp = mst.sp
}

func dumpxstck(nn int) {
	n := nn
	fmt.Fprintf(os.Stderr, "stack:\t\tsp %x fp %x vp %x ap %x\n", mst.sp, mst.fp, mst.vp, mst.ap)
	for e := mst.sp - 4; e >= mst.globend; e -= 4 {
		ux := (uint32(mst.stack[e+3])<<24) | (uint32(mst.stack[e+2])<<16) | (uint32(mst.stack[e+1])<<8) | uint32(mst.stack[e])
		fmt.Fprintf(os.Stderr, "%#x\t%#x\n", e, ux)
		n--
		if n <= 0 {
			break
		}
	}
	fmt.Fprintf(os.Stderr, "%v\n", mst.stack[mst.sp-4*nn:mst.sp])
	fmt.Fprintf(os.Stderr, "\n")
}

func fetch() uint {
	if int(mst.pc) >= len(mst.text) {
		s := fmt.Sprintf("bad program counter %#ux", mst.pc)
		panic(s)
	}
	i := mst.text[mst.pc]
	mst.pc++
	return uint(i)
}

func tfetch(tid int) *Tent {
	if tid<0 || tid>=len(mabs.tents) {
		errs := fmt.Sprintf("bad tid: %d", tid)
		panic(errs)
	}
	return &mabs.tents[tid]
}

func idx(tid int) {
	addr := popduaddr()
	t := tfetch(tid)
	v := int(pop32())
	if v<t.first || v>t.last {
		panic("index value out of range")
	}
	addr += uintptr((v - t.first)*int(mabs.tents[t.etid].sz))
	pushduaddr(addr)
}

//
// type-check value of type tid, about to be copied,
// and return its size.
//
func tchk(t *Tent, s interface{}) uint {
	var (
		ep int
		f  float32
	)
	switch t.fmt {
	case 'b', 'c', 'i', 'e', 'h', 'z', 'u':
		switch s.(type) {
		case []byte:
			eep, err := pbytes.UnmarshalBinary(s.([]byte), int32(ep))
			if err != nil {
				panic("tchk umarshal")
			}
			ep = int(eep.(int32))
		case *int:
			ep = *s.(*int)
		default:
			panic("wrong type in tchk")
		}
		if ep<t.first || ep>t.last {
			panic("assigned value out of range")
		}
	case 'l':
		ifc, err := pbytes.UnmarshalBinary(mst.stack[mst.sp:], f)
		if err != nil {
			panic("tchk float")
		}
		f := ifc.(float32)
		if f<float32(t.first) || f>float32(t.last) {
			panic("assigned value out of range")
		}
	case 'p':
		// check pointer value
	case 'a':
		//
		// Arrays may be used as buffer space,
		// and it is not reasonable to require the
		// entire array to be ok regarding values.
		// So, disable this check.
		//
	case 'R':
		cp := s.([]byte) //TODO offsets, etc, see original picky and reslice
		for i := 0; i < t.nitems; i++ {
			nt := &mabs.tents[t.fields[i].tid]
			tchk(nt, cp[0:int(nt.sz)])
			cp = cp[nt.sz:]
		}
	}
	return t.sz
}

func castr(t *Tent, r float64) {
	if t.fmt == 'r' {
		pushr(r)
	} else {
		e := int(r)
		tchk(t, &e)
		push32(uint32(e))
	}
}

func cast(t *Tent, v int) {
	if t.fmt == 'r' {
		pushr((float64)(v))
	} else {
		tchk(t, &v)
		push32(uint32(v))
	}
}

//
// Caller pushes args,
// call saves space for lvars and 4 saved regs.
// The callee may call push/pop.
// ret is called with sp == fp (if there is no ret value)
// it restores pc, fp, vp, and ap, and moves the
// return value found at the top of the stack to ap.
//
// Right before call:
//
//		<- sp
//	arg
//	arg
//	arg
//	XXX
//
// Right before ret:
//
//		<- sp
//	ret
//	ret
//	savedpc	<- fp
//
// After ret:
//
//		<- sp
//	ret
//	ret	(<- old ap)
//	XXX
//
// That is, a call+ret replaces args in the stack with result.
//

func call(pid int) {
	if pid<0 || pid>=len(mabs.pents) {
		panic("bad pid")
	}
	spc := mst.pc
	sfp := mst.fp
	svp := mst.vp
	sap := mst.ap

	mst.ap = mst.sp - mabs.pents[pid].argsz
	mst.vp = mst.sp
	poison(mst.stack[mst.sp : mst.sp+mabs.pents[pid].varsz])
	mst.sp += mabs.pents[pid].varsz
	push32(uint32(sap))
	push32(uint32(svp))
	push32(uint32(sfp))
	push32(mst.procid)
	push32(spc)
	mst.fp = mst.sp
	mst.pc = uint32(mabs.pents[pid].addr)
	mst.procid = uint32(pid)
	//fmt.Printf("call %d\n", pid) DEBUG
}

func ret(pid int) {

	if pid == int(mabs.entry) {
		if debug['X'] != 0 {
			fmt.Fprintf(os.Stderr, "program complete.\n")
		}
		done("")
	}
	p := &mabs.pents[pid]
	mst.sp -= p.retsz
	retval := mst.sp
	spc := pop32()
	mst.procid = pop32()
	sfp := int(pop32())
	svp := int(pop32())
	sap := int(pop32())
	mst.sp = mst.ap
	if p.retsz > 0 {
		copy(mst.stack[mst.sp:mst.sp+p.retsz], mst.stack[retval:retval+p.retsz])
	}
	mst.sp += p.retsz
	mst.fp = sfp
	mst.ap = sap
	mst.vp = svp
	mst.pc = spc
}

func finishwait() {
	b := bufio.NewReader(os.Stdin)
	b.ReadRune()
}

func pushbool(b bool) {
	if b {
		push32(1)
	} else {
		push32(0)
	}
}

func pami() {
	//debug['X'] = 10
	//debug['M'] = 10

	pop2u := func() (uint32, uint32) {
		return pop32(), pop32()
	}
	pop2i := func() (int32, int32) {
		return int32(pop32()), int32(pop32())
	}
	pop2r := func() (float64, float64) {
		return popr(), popr()
	}
	pop2slice := func(n int) ([]byte, []byte) {
		return popslice(n), popslice(n)
	}
	if int(mabs.entry) >= len(mabs.pents) {
		panic("bad entry number")
	}
	mst.pc = uint32(mabs.pents[mabs.entry].addr)
	if debug['X'] != 0 {
		fmt.Fprintf(os.Stderr, "entry pc %#x\n", mst.pc)
	}
	for int(mst.pc) < len(mst.text) {
		mabs.ninstr++
		if debug['S'] != 0 {
			dumpxstck(10*debug['S'])
		}
		if debug['D'] > 1 {
			dumpglobals()
		}
		ir := fetch()
		ic := paminstr.IC(uint32(ir))
		it := paminstr.IT(uint32(ir))
		if debug['X'] != 0 {
			if paminstr.Hasarg(uint32(ir)) {
				if it == paminstr.ITreal {
					fmt.Fprintf(os.Stderr, "%#05x:\t%v\t%#x (%g)\n", mst.pc-1, paminstr.Instr(ir), mst.text[mst.pc], float64(mst.text[mst.pc]))
				} else {
					fmt.Fprintf(os.Stderr, "%#05x:\t%v\t%#x\n", mst.pc-1, paminstr.Instr(ir), mst.text[mst.pc])
				}
			} else {
				fmt.Fprintf(os.Stderr, "%#05x:\t%v\n", mst.pc-1, paminstr.Instr(ir))
			}
		}

		switch ic {
		case paminstr.ICnop: // nop
		case paminstr.ICptr:
			pushdaddr(xptr(popptr()))
		case paminstr.ICindir: // indir  n -sp +sp
			n := int(fetch())
			d1 := popslice(n)
			if int(mst.sp)+n > mst.stackend {
				panic("stack overflow")
			}
			//fmt.Printf("INDIRING %d %#x %v\n", n, *bptr, d1);
			copy(mst.stack[mst.sp:mst.sp+n], d1)
			mst.sp += n
		case paminstr.ICpush: // push n +sp
			// both real and int
			push32(uint32(fetch()))
		case paminstr.ICjmp: // jmp addr
			mst.pc = uint32(fetch())
		case paminstr.ICjmpt: // jmpt addr
			taddr := fetch()
			if pop32() != 0 {
				mst.pc = uint32(taddr)
			}
		case paminstr.ICjmpf: // jmpf addr
			taddr := uint32(fetch())
			if pop32() == 0 {
				mst.pc = taddr
			}
		case paminstr.ICidx: // idx tid  -sp -sp +sp
			//substitutes addr[i] ->elem addr
			idx(int(fetch()))
		case paminstr.ICfld: // fld n -sp +sp
			//TODO BUG, how to do size?
			//substitutes addr.a ->elem addr
			n := uintptr(fetch())
			d1 := popduaddr()
			pushduaddr(d1 + n)
		case paminstr.ICdata: // data n +sp
			//push n inmediate
			for n := fetch(); n > 0; n -= 4 {
				push32(uint32(fetch()))
			}
		case paminstr.ICdaddr: // daddr n +sp
			//push address for data
			n := int(fetch())
			pushdaddr(&mst.stack[n])
		case paminstr.ICadd: // add -sp -sp +sp
			if it == paminstr.ITreal {
				pushr(popr() + popr())
			} else {
				push32(pop32() + pop32())
			}
		case paminstr.ICsub: // sub -sp -sp +sp
			if it == paminstr.ITreal {
				r1, r2 := pop2r()
				pushr(r1 - r2)
			} else {
				a1, a2 := pop2u()
				push32(a1 - a2)
			}

		case paminstr.ICminus: // minus -sp +sp
			if it == paminstr.ITreal {
				pushr(-popr())
			} else {
				push32(-pop32())
			}
		case paminstr.ICnot: // not -sp +sp
			b := pop32()
			pushbool(b == 0)
		case paminstr.ICor: // or -sp -sp +sp
			a1, a2 := pop2u()
			pushbool(a1!=0 || a2!=0)
		case paminstr.ICand: // and -sp -sp +sp
			a1, a2 := pop2u()
			pushbool(a1!=0 && a2!=0)
		case paminstr.ICeq: // eq -sp -sp +sp
			switch it {
			case paminstr.ITreal:
				r1, r2 := pop2r()
				pushbool(r1 == r2)
			case paminstr.ITaddr:
				d1 := popduaddr()
				d2 := popduaddr()
				pushbool(d1 == d2)
			default:
				a1, a2 := pop2u()
				pushbool(a1 == a2)
			}
		case paminstr.ICeqm: // eqm  n -sp +sp
			n := int(fetch())
			d1, d2 := pop2slice(n)
			pushbool(bytes.Compare(d1[:n], d2[:n]) == 0)
		case paminstr.ICne: // ne -sp -sp +sp
			switch it {
			case paminstr.ITreal:
				r1, r2 := pop2r()
				pushbool(r1 != r2)
			case paminstr.ITaddr:
				d1 := popduaddr()
				d2 := popduaddr()
				pushbool(d1 != d2)
			default:
				a1, a2 := pop2u()
				pushbool(a1 != a2)
			}

		case paminstr.ICnem: // eqm  n -sp +sp
			n := int(fetch())
			d1, d2 := pop2slice(n)
			pushbool(bytes.Compare(d1[:n], d2[:n]) != 0)
		case paminstr.ICcast: // cast tid -sp +sp
			t := tfetch(int(fetch()))
			if it == paminstr.ITreal {
				castr(t, popr())
			} else {
				cast(t, int(int32(pop32())))
			}

		case paminstr.ICle: // le -sp -sp +sp
			if it == paminstr.ITreal {
				r1, r2 := pop2r()
				pushbool(r1 <= r2)
			} else {
				a1, a2 := pop2i()
				pushbool(a1 <= a2)
			}

		case paminstr.ICge: // ge -sp -sp +sp
			if it == paminstr.ITreal {
				r1, r2 := pop2r()
				pushbool(r1 >= r2)
			} else {
				a1, a2 := pop2i()
				pushbool(a1 >= a2)
			}

		case paminstr.ICpow: // pow -sp -sp +sp
			if it == paminstr.ITreal {
				r1, r2 := pop2r()
				pushr(math.Pow(r1, r2))
			} else {
				a1, a2 := pop2u()
				push32(uint32(math.Pow(float64(a1), float64(a2))))
			}

		case paminstr.IClt: // lt -sp -sp +sp
			if it == paminstr.ITreal {
				r1, r2 := pop2r()
				pushbool(r1 < r2)
			} else {
				a1, a2 := pop2i()
				pushbool(a1 < a2)
			}

		case paminstr.ICgt: // gt -sp -sp +sp
			if it == paminstr.ITreal {
				r1, r2 := pop2r()
				pushbool(r1 > r2)
			} else {
				a1, a2 := pop2i()
				pushbool(a1 > a2)
			}

		case paminstr.ICmul: // mul -sp -sp +sp
			if it == paminstr.ITreal {
				r1, r2 := pop2r()
				pushr(r1*r2)
			} else {
				a1, a2 := pop2i()
				push32(uint32(a1*a2))
			}

		case paminstr.ICdiv: // div -sp -sp +sp
			if it == paminstr.ITreal {
				r1, r2 := pop2r()
				if r2 == 0.0 {
					panic("divide by 0.0")
				}
				pushr(r1/r2)
			} else {
				a1, a2 := pop2i()
				if a2 == 0 {
					panic("divide by 0")
				}
				push32(uint32(a1/a2))
			}

		case paminstr.ICmod: // mod -sp -sp +sp
			a1, a2 := pop2i()
			if a2 == 0 {
				panic("divide by zero")
			}
			push32(uint32(a1%a2))

		case paminstr.ICcall: // call pid
			n := uint32(fetch())
			if (n&paminstr.PAMbuiltin) != 0 {
				n &= ^uint32(paminstr.PAMbuiltin)
				if n >= paminstr.Nbuiltins {
					s := fmt.Sprintf("bad builtin call #%d", n)
					panic(s)
				}
				//fmt.Printf("BUILTIN %#x\n", n)
				builtin[n]()
			} else {
				call(int(n))
			}
		case paminstr.ICret: // ret pid
			ret(int(fetch()))

		case paminstr.ICarg: // arg n +sp
			pushdaddr(&mst.stack[mst.ap+int(fetch())])
		case paminstr.IClvar: // lvar n +sp
			pushdaddr(&mst.stack[mst.vp+int(fetch())])
		case paminstr.ICstom: // stom  n -sp -sp
			t := tfetch(int(fetch()))
			n := int(t.sz)
			d1, d2 := pop2slice(n)
			if bytes.Compare(d1[:n], d2[:n]) != 0 {
				tchk(t, d2)
				copy(d1[:n], d2)
			}

		case paminstr.ICsto: // sto  n -sp -sp
			t := tfetch(int(fetch()))
			n := int(t.sz)
			d1 := popslice(n)
			if mst.sp-n < 0 {
				panic("stack underflow")
			}
			mst.sp -= n
			tchk(t, mst.stack[mst.sp:mst.sp+n])
			copy(d1, mst.stack[mst.sp:mst.sp+n])

		default:
			panic("unknown instruction")
		}
	}
	panic("bad program counter")
}

func (t *Tent) String() string {
	s := ""
	if t == nil {
		return s
	}
	s += fmt.Sprintf("type %s %c %d..%d n=%d sz=%d etid=%d\n",
		t.name, t.fmt, t.first, t.last, t.nitems, t.sz, t.etid)
	switch t.fmt {
	case 'e':
		for i := 0; i < t.nitems; i++ {
			s += fmt.Sprintf("\t%s\n", t.lits[i])
		}
	case 'R':
		for i := 0; i < t.nitems; i++ {
			v := &t.fields[i]
			s += fmt.Sprintf("\t%s %d %d\n", v.name, v.tid, v.addr)
		}
	}
	return s
}

var vtabs int

func (v *Vent) String() string {
	s := ""
	s += fmt.Sprintf("var %s tid %d addr %d val %s %s:%d\n",
		v.name, v.tid, v.addr, v.val, v.fname, v.lineno)
	if v.fields == nil {
		return s
	}
	t := tfetch(int(v.tid))
	vtabs++
	for i := 0; i < t.nitems; i++ {
		for j := 0; j < vtabs; j++ {
			s += fmt.Sprintf("\t")
		}
		s += fmt.Sprintf("%v", v.fields[i])
	}
	vtabs--
	return s
}

func (p *Pent) String() string {
	s := ""
	if p == nil {
		return "<nil>"
	}
	s += fmt.Sprintf("prog %s addr %d #args %d #vars %d ret %d %s:%d\n",
		p.name, p.addr, p.nargs, p.nvars, p.retsz,
		p.fname, p.lineno)
	for i := 0; i < p.nargs; i++ {
		s += fmt.Sprintf("\tp%v", &p.args[i])
	}
	for i := 0; i < p.nvars; i++ {
		s += fmt.Sprintf("\tl%v", &p.vars[i])
	}
	return s
}

func badbin(s string) {
	pprof.StopCPUProfile()
	fmt.Fprintf(os.Stderr, "%s:%d: %s\n", fst.fname, fst.lineno, s)
	os.Exit(1)
}

func (f *FileSt) getln() string {
	var (
		err error
	)
	binln := ""
	for {
		f.lineno++
		binln, err = f.bin.ReadString('\n')
		if err != nil {
			badbin("truncated file")
		}
		if binln[0] != '#' {
			break
		}
	}
	return binln
}

// ScanCEscWords is a split function for a Scanner that returns each
// space-separated/quoted word of text, with surrounding spaces/quotes deleted. It will
// never return an empty string. The definition of space is set by
// unicode.IsSpace.
func ScanCEscWords(data []byte, atEOF bool) (advance int, token []byte, err error) {
	var r, lr rune
	// Skip leading spaces.
	start := 0
	quoting := false
	for width := 0; start < len(data); start += width {
		r, width = utf8.DecodeRune(data[start:])
		if r == '\'' {
			if !quoting {
				quoting = true
				continue
			}
		}
		if !unicode.IsSpace(r) || quoting {
			break
		}
	}
	if atEOF && len(data)==0 {
		return 0, nil, nil
	}
	nslash := false
	// Scan until space, marking end of word.
	for width, i := 0, start; i < len(data); i += width {
		r, width = utf8.DecodeRune(data[i:])
		if !quoting {
			if unicode.IsSpace(r) {
				return i + width, data[start:i], nil
			}
		}
		if nslash {
			//I only permit backquoted tabs for now
			if r == 't' {
				nslash = false
			} else {
				panic("bad char in word")
			}
		}
		if quoting {
			if r == '\\' {
				nslash = true
			} else if lr == '\'' {
				/* if this is the third, we have found the infamous ''' start */
				if r != '\'' {
					s := string(data[start : i-1])
					s = strings.Replace(s, "''", "'", -1)
					s = strings.Replace(s, "\\t", "	", -1)
					return i + width, []byte(s), nil
				}
			}
		}
		if lr=='\'' && r=='\'' {
			lr = rune(0)
		} else {
			lr = r
		}
	}
	// If we're at EOF, we have a final, non-empty, non-terminated word. Return it.
	if atEOF && len(data)>start {
		return len(data), data[start:], nil
	}
	// Request more data.
	return 0, nil, nil
}

func (f *FileSt) rtoks() int {
	ln := f.getln()
	scanner := bufio.NewScanner(strings.NewReader(ln))
	scanner.Split(ScanCEscWords)
	f.toks = nil
	ntoks := 0
	for scanner.Scan() {
		ntoks++
		f.toks = append(f.toks, scanner.Text())
	}
	return ntoks
}

func (f *FileSt) rhdr() {

	if f.rtoks() != 2 {
		badbin("bad entry header")
	}
	if f.toks[0] != "entry" {
		badbin("no entry")
	}
	n, err := strconv.ParseInt(f.toks[1], 0, 64)
	if err != nil {
		badbin("bad entry")
	}
	mabs.entry = uint(n)
	if debug['F'] != 0 {
		fmt.Fprintf(os.Stderr, "entry %d\n", mabs.entry)
	}
}

func (f *FileSt) rventfield() *Vent {
	switch f.rtoks() {
	case 3:
		addr, err := strconv.ParseInt(f.toks[2], 0, 64)
		if err != nil {
			badbin("bad vent")
		}
		for i := 0; i<len(mabs.vents) && mabs.vents[i].name!=""; i++ {
			if mabs.vents[i].addr == uint32(addr) {
				return &mabs.vents[i]
			}
		}
		badbin("aggregate field address not found")
	case 6:
		v := new(Vent)
		f.rvent(v)
		return v
	default:
		badbin("bad aggregate field")
	}
	return nil
}

func (f *FileSt) rvent(v *Vent) {
	var (
		err error
		n   int64
	)
	v.name = f.toks[0]
	n, err = strconv.ParseInt(f.toks[1], 0, 64)
	if err != nil {
		badbin("bad vent")
	}
	v.tid = uint(n)
	n, err = strconv.ParseInt(f.toks[2], 0, 64)
	if err != nil {
		badbin("bad vent")
	}
	v.addr = uint32(n)
	if int(v.tid) >= len(mabs.tents) {
		badbin("bad type id")
	}
	v.fname = f.toks[4]
	n, err = strconv.ParseInt(f.toks[5], 0, 64)
	if err != nil {
		badbin("bad vent")
	}
	v.lineno = int(n)
	v.val = ""
	switch mabs.tents[v.tid].fmt {
	case 'f':
		if v.name == "stdin" {
			v.val = "0"
		} else if v.name == "stdout" {
			v.val = "1"
		} else if v.name == "stdgraph" {
			v.val = "-1"
		} else if f.toks[3] == "-" {
			v.val = f.toks[3]
		}
	case 'a', 'R':
		if f.toks[3] != "-" {
			n, err = strconv.ParseInt(f.toks[3], 0, 64)
			if err != nil {
				badbin("bad vent")
			}
			if n <= 0 {
				badbin("aggregate arity <= 0")
			}
			v.val = "$aggr"
			for i := 0; i < int(n); i++ {
				v.fields = append(v.fields, *f.rventfield())
			}
		}
	default:
		v.val = f.toks[3]
	}
}

func (f *FileSt) parseinttok(errs string, i int) int {
	n, err := strconv.ParseInt(f.toks[i], 0, 64)
	if err != nil {
		badbin(errs)
	}
	return int(n)
}

func (f *FileSt) rfield(v *Vent) {
	if f.rtoks() != 3 {
		badbin("bad field entry")
	}
	v.name = f.toks[0]
	errs := "bad vent"
	v.tid = uint(f.parseinttok(errs, 1))
	v.addr = uint32(f.parseinttok(errs, 2))
	if int(v.tid) >= len(mabs.tents) {
		badbin("bad type id")
	}
}
func (f *FileSt) rtype(i uint) {
	var t Tent
	if f.rtoks() != 8 {
		badbin("bad type entry")
	}
	mabs.tents = append(mabs.tents, t)
	mabs.tents[i].name = f.toks[1]
	mabs.tents[i].fmt, _ = utf8.DecodeRuneInString(f.toks[2])
	errs := "bad rtype"
	mabs.tents[i].first = f.parseinttok(errs, 3)
	mabs.tents[i].last = f.parseinttok(errs, 4)
	mabs.tents[i].nitems = f.parseinttok(errs, 5)
	mabs.tents[i].sz = uint(f.parseinttok(errs, 6))
	mabs.tents[i].etid = uint(uint(f.parseinttok(errs, 7)))
	switch mabs.tents[i].fmt {
	case 'e':
		for n := 0; n < mabs.tents[i].nitems; n++ {
			ln := f.getln()
			mabs.tents[i].lits = append(mabs.tents[i].lits, ln[0:len(ln)-1])
		}
		break
	case 'R':
		mabs.tents[i].fields = make([]Vent, mabs.tents[i].nitems)
		for n := 0; n < mabs.tents[i].nitems; n++ {
			f.rfield(&mabs.tents[i].fields[n])
		}
		break
	}
	if debug['F'] != 0 {
		fmt.Fprintf(os.Stderr, "tid %d %v", i, &mabs.tents[i])
	}
}

func (f *FileSt) rpc(i uint) {
	var (
		lfname string
		err    error
		pc     Pc
	)

	if f.rtoks() != 3 {
		badbin("bad pc entry")
	}
	n, err := strconv.ParseInt(f.toks[0], 16, 64)
	if err != nil {
		badbin("bad rpc")
	}
	mabs.pcs = append(mabs.pcs, pc)
	mabs.pcs[i].pc = uint32(n)

	if lfname=="" && f.toks[1]==lfname {
		mabs.pcs[i].fname = lfname
	} else {
		mabs.pcs[i].fname = f.toks[1]
	}
	lfname = mabs.pcs[i].fname
	n, err = strconv.ParseInt(f.toks[2], 0, 64)
	if err != nil {
		badbin("bad rpc")
	}
	mabs.pcs[i].lineno = uint(n)
	if debug['F'] != 0 {
		fmt.Fprintf(os.Stderr, "%05x\t%s:%d\n", mabs.pcs[i].pc, mabs.pcs[i].fname, mabs.pcs[i].lineno)
	}
}

func (f *FileSt) rvar(i uint) {
	var v Vent
	if f.rtoks() != 6 {
		badbin("bad var entry")
	}
	mabs.vents = append(mabs.vents, v)
	f.rvent(&mabs.vents[i])
	if debug['F'] != 0 {
		fmt.Fprintf(os.Stderr, "%v", &mabs.vents[i])
	}
}
func (f *FileSt) rproc(i uint) {
	var p Pent
	if f.rtoks() != 10 {
		badbin("bad proc entry")
	}
	mabs.pents = append(mabs.pents, p)
	n, err := strconv.ParseInt(f.toks[0], 0, 64)
	if err != nil {
		badbin("bad rproc")
	}
	if uint(n) != i {
		badbin("bad proc entry id")
	}
	mabs.pents[i].name = f.toks[1]

	errs := "bad rproc"
	mabs.pents[i].addr = uint(f.parseinttok(errs, 2))
	mabs.pents[i].nargs = f.parseinttok(errs, 3)
	mabs.pents[i].nvars = f.parseinttok(errs, 4)
	mabs.pents[i].retsz = f.parseinttok(errs, 5)
	mabs.pents[i].argsz = f.parseinttok(errs, 6)
	mabs.pents[i].varsz = f.parseinttok(errs, 7)

	mabs.pents[i].fname = f.toks[8]

	n, err = strconv.ParseInt(f.toks[9], 0, 64)
	if err != nil {
		badbin("bad rproc")
	}
	mabs.pents[i].lineno = int(n)
	if mabs.pents[i].nargs > 0 {
		mabs.pents[i].args = make([]Vent, mabs.pents[i].nargs)
		for j := 0; j < mabs.pents[i].nargs; j++ {
			if f.rtoks() != 6 {
				badbin("bad proc arg entry")
			}
			f.rvent(&mabs.pents[i].args[j])
		}
	}
	if mabs.pents[i].nvars > 0 {
		mabs.pents[i].vars = make([]Vent, mabs.pents[i].nvars)
		for j := 0; j < mabs.pents[i].nvars; j++ {
			if f.rtoks() != 6 {
				badbin("bad proc lvar entry")
			}
			f.rvent(&mabs.pents[i].vars[j])
		}
	}
	if debug['F'] != 0 {
		fmt.Fprintf(os.Stderr, "%v", &mabs.pents[i])
	}
}

func (f *FileSt) rtext() {
	var r float64
	if f.rtoks() != 2 {
		badbin("bad text header")
	}
	if f.toks[0] != "text" {
		badbin("unexpected tab")
	}
	n, err := strconv.ParseInt(f.toks[1], 0, 64)
	if err != nil {
		badbin("bad rtext")
	}
	ntext := int(n)
	mst.text = make([]uint32, ntext)
	if debug['F'] != 0 {
		fmt.Fprintf(os.Stderr, "tab text[%d]\n", ntext)
	}
	ndata := 0
	for i := 0; i < ntext; i++ {
		ntoks := f.rtoks()
		// feature: ignore extra toks in line
		if ntoks < 2 {
			badbin("bad text entry")
		}
		if ndata > 0 {
			n, err = strconv.ParseInt(f.toks[1], 0, 64)
			if err != nil {
				badbin("bad rtext")
			}
			mst.text[i] = uint32(n)
			if debug['F'] != 0 {
				fmt.Fprintf(os.Stderr, "%05x\t%#x\n", i, mst.text[i])
			}
			ndata -= 4
			continue
		}
		if f.toks[1][0]<'a' || f.toks[1][0]>'z' {
			n, err = strconv.ParseInt(f.toks[1], 0, 64)
			if err != nil {
				badbin("bad rtext")
			}
			mst.text[i] = uint32(n)
		} else {
			mst.text[i] = uint32(paminstr.Icode(f.toks[1]))
		}
		if !paminstr.Hasarg(mst.text[i]) {
			if debug['F'] != 0 {
				fmt.Fprintf(os.Stderr, "%05x\t%v\n", i, paminstr.Instr(mst.text[i]))
			}
			continue
		}
		if ntoks < 3 {
			errs := fmt.Sprintf("missing argument for %#x %v", mst.text[i], paminstr.Instr(mst.text[i]))
			panic(errs)
		}
		if i == ntext-1 {
			panic("truncated instruction")
		}
		ir := mst.text[i]
		if paminstr.IT(ir)==paminstr.ITreal && paminstr.IC(ir)!=paminstr.ICcast {
			r, err = strconv.ParseFloat(f.toks[2], 64)
			if err != nil {
				badbin("bad rtext")
			}
			i++
			mst.text[i] = math.Float32bits(float32(r))
			if debug['F'] != 0 {
				fmt.Fprintf(os.Stderr, "%05x\t%v\t%e\n", i-1, paminstr.Instr(ir), r)
			}
		} else {
			n, err = strconv.ParseInt(f.toks[2], 0, 64)
			if err != nil {
				badbin("bad rtext")
			}
			i++
			mst.text[i] = uint32(n)
			if paminstr.IC(ir) == paminstr.ICdata {
				ndata = int(mst.text[i])
				if (ndata%4) != 0 {
					badbin("bad data argument in text")
				}
			}
			if debug['F'] != 0 {
				fmt.Fprintf(os.Stderr, "%05x\t%v\t%#x\n", i-1, paminstr.Instr(ir), mst.text[i])
			}
		}
	}
	if ndata > 0 {
		panic("truncated instruction")
	}
}

func (f *FileSt) rtab(name string, rf func(uint)) (np uint) {
	if f.rtoks() != 2 {
		errs := fmt.Sprintf("bad %s header", name)
		badbin(errs)
	}
	if f.toks[0] != name {
		badbin("unexpected tab")
	}
	n, err := strconv.ParseInt(f.toks[1], 0, 64)
	if err != nil {
		panic("expected int")
	}
	np = uint(n)
	if debug['F'] != 0 {
		fmt.Fprintf(os.Stderr, "tab %s[%d]\n", name, np)
	}
	for i := uint(0); i < np; i++ {
		rf(i)
	}
	return np
}

type Derr string

func (d Derr) Error() string {
	return string(d)
}

type Dflag  {
	name rune
}

func (d *Dflag) Set(s string) error {
	_, ok := debug[d.name]
	if ok {
		debug[d.name]++
	} else {
		debug[d.name] = 1
	}
	return nil
}

func (d *Dflag) String() string {
	return fmt.Sprintf("[%d]", debug[d.name])
}

func (d *Dflag) IsBoolFlag() bool {
	return true
}

var (
	dmpflag bool
	eflag   Dflag
	mflag   Dflag
	iflag   Dflag
	sflag   Dflag
	dflag   Dflag
	xflag   Dflag
	fflag   Dflag
	cflag   Dflag
	wflag   bool
)

func init() {
	flag.Var(&mflag, "M", "globals debug")
	mflag.name = 'M'
	flag.Var(&iflag, "I", "file i/o debug")
	iflag.name = 'I'
	flag.Var(&sflag, "S", "stack debug")
	sflag.name = 'S'
	flag.Var(&dflag, "D", "memalloc debug")
	dflag.name = 'D'
	flag.Var(&xflag, "X", "instruction execution debug")
	xflag.name = 'X'
	flag.Var(&fflag, "F", "binary file debug")
	fflag.name = 'F'
	flag.Var(&cflag, "C", "cpu profiling")
	cflag.name = 'C'
	flag.Var(&cflag, "E", "mem profiling")
	eflag.name = 'E'

	flag.BoolVar(&dmpflag, "d", false, "dump stack on error")
	flag.BoolVar(&statflag, "s", false, "dump statistics")
	flag.BoolVar(&wflag, "w", false, "don't wait at the end in windows")

}

func main() {
	defer func() {
		if e := recover(); e != nil {
			errs := fmt.Sprint(e)
			if strings.HasPrefix(errs, "runtime error:") {
				errs = strings.Replace(errs, "runtime error:", "pam error:", 1)
			}
			flushall()
			s := ""
			pe := mabs.findpc(mst.pc)
			if pe != nil {
				s += fmt.Sprintf("%s:%d: ", pe.fname, pe.lineno)
			}
			s += errs
			s += fmt.Sprintf("\npc=%#x, sp=%#x, fp=%#x\n", mst.pc, mst.sp, mst.fp)
			fmt.Fprintf(os.Stderr, "%s", s)
			if dmpflag {
				fmt.Fprintf(os.Stderr, "%s", godebug.Stack())
			}
			done("panic")
		}
	}()

	randsrc = rand.New(rand.NewSource(0))
	if runtime.GOOS == "windows" {
		paminstr.EOL = "\r\n"
		waitforwindows = true
	}
	flag.Parse()
	if wflag {
		waitforwindows = false
	}
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}
	args := flag.Args()
	if debug['C'] != 0 {
		f, err := os.Create(args[0] + ".cpuprofile")
		if err != nil {
			done(fmt.Sprint(err))
		}
		defer f.Close()
		pprof.StartCPUProfile(f)
	}

	if debug['E'] != 0 {
		defer func() {
			f, err := os.Create(args[0] + ".memprofile")
			if err != nil {
				done(fmt.Sprint(err))
			}
			pprof.WriteHeapProfile(f)
			f.Close()
			pprof.StartCPUProfile(f)
		}()
	}

	fst = new(FileSt)
	fst.fname = args[0]
	fd, err := os.Open(fst.fname)
	defer fd.Close()
	if err != nil {
		panic("bad file")
	}
	fst.bin = bufio.NewReader(fd)
	fst.rhdr()
	fst.rtab("types", fst.rtype)
	fst.rtab("vars", fst.rvar)
	fst.rtab("procs", fst.rproc)
	fst.rtext()
	fst.rtab("pcs", fst.rpc)
	datainit()
	fnsinit()
	pami()
	done("")
}
