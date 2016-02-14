// Copyright 2013 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package ipv6

type sysSockoptLen uint32

const (
	sysSizeofPacketInfo   = 0x14
	sysSizeofMulticastReq = 0x14
	sysSizeofICMPFilter   = 0x20
)

struct sysPacketInfo {
	IP      [16]byte
	IfIndex uint32
}

struct sysMulticastReq {
	IP      [16]byte
	IfIndex uint32
}
