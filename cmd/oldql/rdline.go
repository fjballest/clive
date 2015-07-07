package main

/*
	This is an example of how to use readline.
	but DONT use it.

	To use this, the lines from XXX to YYY must be
	a comment.

import (
	"io"
)

XXX
#include <stdio.h>
#include <stdlib.h>
#include <readline/readline.h>
#include <readline/history.h>
#cgo LDFLAGS: -lreadline
YYY
import "C"
import "unsafe"

func ReadLine(prompt string) (string, error) {
	var cPrompt *C.char;
	if prompt != "" {
		cPrompt = C.CString(prompt)
	}
	cLine := C.readline(cPrompt);
	if cPrompt != nil {
		C.free(unsafe.Pointer(cPrompt))
	}
	if cLine == nil {
		return "", io.EOF
	}

	line := C.GoString(cLine);
	C.free(unsafe.Pointer(cLine));
	return line, nil
}

*/
