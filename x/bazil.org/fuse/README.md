This is bazil.org/fuse retrieved from their git (which omes from rsc's fuse packages),
modified to fix a couple of bugs related to macos and to fit a little bit better with clive.

-

bazil.org/fuse -- Filesystems in Go
===================================

`bazil.org/fuse` is a Go library for writing FUSE userspace
filesystems.

It is a from-scratch implementation of the kernel-userspace
communication protocol, and does not use the C library from the
project called FUSE. `bazil.org/fuse` embraces Go fully for safety and
ease of programming.

Here’s how to get going:

    go get bazil.org/fuse

Website: http://bazil.org/fuse/

Github repository: https://github.com/bazillion/fuse

API docs: http://godoc.org/bazil.org/fuse

Our thanks to Russ Cox for his fuse library, which this project is
based on.
