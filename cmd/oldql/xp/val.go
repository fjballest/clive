package xp

import (
	"clive/cmd"
	"clive/zx"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"time"
)

type value interface{}

func Bval(v value) bool {
	if n, ok := v.(bool); ok {
		return n
	}
	if n, ok := v.(uint64); ok {
		return n != 0
	}
	if n, ok := v.(float64); ok {
		return n != 0
	}
	return Ival(v) != 0
}

func Nval(v value) float64 {
	if n, ok := v.(bool); ok {
		if n {
			return 1
		}
		return 0
	}
	if n, ok := v.(float64); ok {
		return n
	}
	if n, ok := v.(uint64); ok {
		return float64(n)
	}
	if s, ok := v.(string); ok {
		if n, err := strconv.ParseFloat(s, 64); err == nil {
			return n
		}
		if n, err := strconv.ParseUint(s, 0, 64); err == nil {
			return float64(n)
		}
	}
	if t, ok := v.(time.Time); ok {
		return float64(t.Unix())
	}
	return 0
}

func Ival(v value) uint64 {
	if n, ok := v.(bool); ok {
		if n {
			return 1
		}
		return 0
	}
	if n, ok := v.(uint64); ok {
		return n
	}
	if s, ok := v.(string); ok {
		if n, err := strconv.ParseUint(s, 0, 64); err == nil {
			return n
		}
		if n, err := strconv.ParseFloat(s, 64); err == nil {
			return uint64(n)
		}
	}
	if t, ok := v.(time.Time); ok {
		return uint64(t.Unix())
	}
	return 0
}

func add(v1, v2 value) value {
	_, ok1 := v1.(uint64)
	_, ok2 := v2.(uint64)
	if ok1 && ok2 {
		return Ival(v1) + Ival(v2)
	}
	return Nval(v1) + Nval(v2)
}

func sub(v1, v2 value) value {
	_, ok1 := v1.(uint64)
	_, ok2 := v2.(uint64)
	if ok1 && ok2 {
		return Ival(v1) - Ival(v2)
	}
	return Nval(v1) - Nval(v2)
}

func mul(v1, v2 value) value {
	_, ok1 := v1.(uint64)
	_, ok2 := v2.(uint64)
	if ok1 && ok2 {
		return Ival(v1) * Ival(v2)
	}
	return Nval(v1) * Nval(v2)
}

func minus(v1 value) value {
	_, ok1 := v1.(uint64)
	if ok1 {
		return -Ival(v1)
	}
	return -Nval(v1)
}

func div(v1, v2 value) value {
	_, ok1 := v1.(float64)
	_, ok2 := v2.(float64)
	if ok1 || ok2 {
		if Nval(v2) == 0 {
			panic("divide by 0")
		}
		return Nval(v1) / Nval(v2)
	}
	if Ival(v2) == 0 {
		panic("divide by 0")
	}
	return Ival(v1) / Ival(v2)
}

func mod(v1, v2 value) value {
	_, ok1 := v1.(uint64)
	n2, ok2 := v2.(uint64)
	if ok1 && ok2 {
		if n2 == 0 {
			panic("divide by 0")
		}
		return Ival(v1) % Ival(v2)
	}
	f2 := Nval(v2)
	if f2 == 0 {
		panic("divide by 0")
	}
	return math.Remainder(Nval(v1), f2)
}

func pow(v1, v2 value) value {
	return math.Pow(Nval(v1), Nval(v2))
}

func cmp(v1, v2 value) int {
	t1, okt1 := v1.(time.Time)
	t2, okt2 := v2.(time.Time)
	if okt1 && okt2 {
		if t1.Before(t2) {
			return -1
		}
		if t1.After(t2) {
			return 1
		}
		return 0
	}
	s1, ok1 := v1.(string)
	s2, ok2 := v2.(string)
	if ok1 && ok2 {
		if s1 == s2 {
			return 0
		}
		return 1
	}
	_, ok1 = v1.(uint64)
	_, ok2 = v2.(uint64)
	if ok1 || ok2 {
		switch n1, n2 := Ival(v1), Ival(v2); {
		case n1 < n2:
			return -1
		case n1 > n2:
			return 1
		default:
			return 0
		}
	}
	switch n := Nval(v1) - Nval(v2); {
	case n < 0:
		return -1
	case n > 0:
		return 1
	default:
		return 0
	}
}

func attr(aname string, v1 value) (value, error) {
	fname, ok := v1.(string)
	if !ok {
		return nil, errors.New("not a file name")
	}
	fname, _ = filepath.Abs(fname)
	_, ts, ns, err := cmd.ResolveTree(fname)
	if err != nil {
		panic(err)
	}
	t := ts[0]
	spath := ns[0]
	d, err := zx.Stat(t, spath)
	if err != nil {
		panic(err)
	}
	if aname == "r" {
		return d.Int("mode")&0444 != 0, nil
	}
	if aname == "w" {
		return d.Int("mode")&0222 != 0, nil
	}
	if aname == "x" {
		return d.Int("mode")&0111 != 0, nil
	}
	if aname == "mode" {
		return fmt.Sprintf("0%o", d.Int("mode")&0777), nil
	}
	if aname == "size" {
		return uint64(d.Int64("size")), nil
	}
	if aname == "mtime" {
		return d.Time("mtime"), nil
	}
	return d[aname], nil
}
