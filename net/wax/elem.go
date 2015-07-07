package wax

import (
	"fmt"
	"reflect"
	"strconv"
)

type el  {
	op   tok // tId, tDot, tLbra
	name string
	idx  string
	next *el
}

func (e *el) String() string {
	if e == nil {
		return ""
	}
	if e.op == tId {
		return fmt.Sprintf("%s%s", e.name, e.next)
	}
	if e.op == tDot {
		return fmt.Sprintf(".%s%s", e.name, e.next)
	}
	if e.op == tLbra {
		return fmt.Sprintf("[%s]%s", e.name, e.next)
	}
	return "???"
}

func parseEl(s string) (*el, error) {
	p := &Part{
		l: newLex(s),
	}
	p.l.tfn = p.l.inside
	return p.parseElem()
}

func getfield(v reflect.Value, nm string) (interface{}, error) {
	t := v.Type()
	_, ok := t.FieldByName(nm)
	if !ok {
		return nil, fmt.Errorf("%s is not a field", nm)
	}
	// TODO: tag
	ei := v.FieldByName(nm).Interface()
	return ei, nil
}

func sliceidx(v reflect.Value, nm string) (interface{}, error) {
	idx, err := strconv.Atoi(nm)
	if err != nil {
		return nil, fmt.Errorf("%s not an index", nm)
	}
	if idx<0 || idx>=v.Len() {
		return nil, fmt.Errorf("out of range")
	}
	ei := v.Index(idx).Interface()
	return ei, nil
}

func mapidx(v reflect.Value, nm string) (interface{}, error) {
	t := v.Type()
	keyt := t.Key()
	var k reflect.Value
	switch keyt.Kind() {
	case reflect.String:
		k = reflect.ValueOf(nm)
	case reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64:
		idx, err := strconv.ParseInt(nm, 0, 64)
		if err != nil {
			return nil, fmt.Errorf("%s not an index", nm)
		}
		k = reflect.ValueOf(idx)
	case reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64:
		idx, err := strconv.ParseUint(nm, 0, 64)
		if err != nil {
			return nil, fmt.Errorf("%s not an index", nm)
		}
		k = reflect.ValueOf(uint(idx))
	default:
		return nil, fmt.Errorf("%s: can't handle index type", nm)
	}
	if !k.Type().ConvertibleTo(keyt) {
		return nil, fmt.Errorf("invalid map key type")
	}
	k = k.Convert(keyt)
	return v.MapIndex(k).Interface(), nil

}

func (e *el) lookupAt(ei interface{}) (interface{}, string, error) {
	if e == nil {
		return nil, "", fmt.Errorf("nil element")
	}
	if e.op != tId {
		return nil, "", fmt.Errorf("bug: lookup: not id")
	}
	nm := e.name
	for e.next != nil {
		e = e.next
		var err error
		v := reflect.ValueOf(ei)
		k := v.Kind()
		for k==reflect.Interface || k==reflect.Ptr {
			if v.IsNil() {
				err := fmt.Errorf("%s is nil", nm)
				return nil, "", err
			}
			v = v.Elem()
			k = v.Kind()
		}
		nm = e.name
		switch e.op {
		case tDot:
			if k != reflect.Struct {
				err := fmt.Errorf("%s is not a struct", nm)
				return nil, "", err
			}
			if ei, err = getfield(v, nm); err != nil {
				return nil, "", err
			}
		case tLbra:
			switch k {
			case reflect.Array, reflect.Slice:
				if ei, err = sliceidx(v, nm); err != nil {
					return nil, "", err
				}
			case reflect.Map:
				if ei, err = mapidx(v, nm); err != nil {
					return nil, "", err
				}
			default:
				err := fmt.Errorf("not an array or map", nm)
				return nil, "", err
			}
		default:
			panic("wax part elem lookup bug")
		}
	}
	return ei, nm, nil
}
