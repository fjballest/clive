package sre

import (
	"fmt"
)

// debug aid
func tokStr(tok rune) string {
	switch tok {
	case tSTART:
		return "start"
	case tRPAREN:
		return ")"
	case tLPAREN:
		return "("
	case tOR:
		return "|"
	case tCAT:
		return "cat"
	case tSTAR:
		return "*"
	case tPLUS:
		return "+"
	case tQUEST:
		return "?"
	case tANY:
		return "."
	case tWORD:
		return "\\w"
	case tBLANK:
		return "\\s"
	case tNOP:
		return "nop"
	case tBOL:
		return "^"
	case tEOL:
		return "$"
	case tCCLASS:
		return "[]"
	case tNCCLASS:
		return "[^]"
	case tEND:
		return "eof"
	case '\n':
		return "\\n"
	case '\t':
		return "\\t"
	case ' ':
		return "_"
	case cRange:
		return "-"
	default:
		if tok < 32 {
			return fmt.Sprintf("%#x", tok)
		}
		if tok&tQUOTE != 0 {
			return fmt.Sprintf("\\%c", tok)
		}
	}
	return fmt.Sprintf("%c", tok)
}

// debug
func (i inst) String() string {
	s := fmt.Sprintf("%s\t\\%d\tl %#x\tr %#x",
		tokStr(i.op), i.subid, i.left, i.right)
	if len(i.class) == 0 {
		return s
	}
	s += "\t{"
	for nc, c := range i.class {
		if nc > 0 {
			s += " "
		}
		s += tokStr(c)
	}
	return s + "}"
}

// Debug: return a printable program, including the entire NFA machine program.
func (prg *ReProg) String() string {
	s := fmt.Sprintf("entry: %#x back: %v ids: %d\n",
		prg.entry, prg.back, prg.cursubid)
	for ni, i := range prg.code {
		s += fmt.Sprintf("%#x\t%s\n", ni, i)
	}
	return s
}
