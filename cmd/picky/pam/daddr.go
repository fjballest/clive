package main

import (
	"clive/cmd/picky/pbytes"
	"unsafe"
)

//This file isolates converting pointers to slices which is needed
//to convert back and forth between the serialized bytes in the
//stack and the go data types. This simplifies greatly the arithmetic
//for the instructions. The only problems have to do with the runtime
//loosing track of the pointers, but the only time this happens is with
//*Ptr which are never freed anyway so that leaks can be tracked.
//if later they are freed in any way (for example if there are too many of them),
//probably some kind of LRU scheme could be used,
//then something has to be done to keep them alive while they are in the stack.

func popptr() *Ptr {
	p := (*Ptr)(unsafe.Pointer(uintptr(pop64())))
	return p
}

func popslice(n int) []byte {
	p := uintptr(pop64())
	return sliceptr(p, n)
}

func pushdaddr(p *byte) {
	pp := uintptr(unsafe.Pointer(p))
	push64(uint64(pp))
}

func pushduaddr(p uintptr) {
	push64(uint64(p))
}

func popduaddr() uintptr {
	p := uintptr(pop64())
	return p
}

//Hack to obtain a slice out of a pointer
// it may come from the stack or from something
// you want to marshal/unmarshal
// reflection would make it safer and *much* slower
func sliceptr(p uintptr, n int) []byte {
	pp := make([]byte, n)
	bptr := (*uintptr)(unsafe.Pointer(&pp))
	*bptr = p
	return pp
}

const p64sz = 8

func ptrPtr(p uintptr) (pt *Ptr) {
	if unsafe.Sizeof(pt) > p64sz {
		panic("pointers are too big")
	}
	ps := sliceptr(p, p64sz)
	ifc, err := pbytes.UnmarshalBinary(ps, p)
	if err != nil {
		panic("ptrPtr marshal")
	}
	return (*Ptr)(unsafe.Pointer(ifc.(uintptr)))
}

func Ptrptr(pt *Ptr, p uintptr) {
	if unsafe.Sizeof(pt) > p64sz {
		panic("pointers are too big")
	}
	ps := sliceptr(p, p64sz)
	err := pbytes.MarshalBinary(ps, uintptr(unsafe.Pointer(pt)))
	if err != nil {
		panic("xnew marshal")
	}
}

func ptrU64(p *byte) (u uint64) {
	ps := sliceptr(uintptr(unsafe.Pointer(p)), p64sz)
	ifc, err := pbytes.UnmarshalBinary(ps, uintptr(unsafe.Pointer(p)))
	if err != nil {
		panic("ptrPtr marshal")
	}
	return ifc.(uint64)
}

func ptrU32(p *byte) (u uint32) {
	ps := sliceptr(uintptr(unsafe.Pointer(p)), p32sz)
	ifc, err := pbytes.UnmarshalBinary(ps, u)
	if err != nil {
		panic("ptrPtr marshal")
	}
	u = ifc.(uint32)
	return u
}
