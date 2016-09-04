package zx

import (
	"fmt"
	"time"
)

// A type of change between two trees
type ChgType int

const (
	None    ChgType = iota
	Add             // file was added
	Data            // file data was changed
	Meta            // file metadata was changed
	Del             // file was deleted
	DirFile         // dir replaced with file or file replaced with dir
	Err             // had an error while proceding the dir
	// implies a del of the old tree at file
)

// A change made to a tree wrt another tree
struct Chg {
	Type ChgType
	D    Dir
	Time time.Time
	Err  error
}

func (ct ChgType) String() string {
	switch ct {
	case None:
		return "none"
	case Add:
		return "add"
	case Data:
		return "data"
	case Meta:
		return "meta"
	case Del:
		return "del"
	case DirFile:
		return "dirfile"
	case Err:
		return "error"
	default:
		panic("bad chg type")
	}
}

func (c Chg) String() string {
	switch c.Type {
	case None:
		return "none"
	case Add, Data, Meta:
		return fmt.Sprintf("%s %s", c.Type, c.D)
	case Del:
		return fmt.Sprintf("%s %s", c.Type, c.D)
	case DirFile:
		return fmt.Sprintf("%s %s", c.Type, c.D)
	case Err:
		return fmt.Sprintf("%s %s %s", c.Type, c.D["path"], c.Err)
	default:
		panic("bad chg type")
	}
}
