package pbytes

import (
	"errors"
	"math"
	"unicode/utf8"
)

func m32(data []byte, ui uint32) {
	for i := uint32(0); i < 4; i++ {
		data[i] = byte((ui >> (8 * i)) & 0xff)
	}
}

func m64(data []byte, ui uint64) {
	for i := uint64(0); i < 8; i++ {
		data[i] = byte((ui >> (8 * i)) & 0xff)
	}
}

func MarshalBinary(data []byte, ifc interface{}) error {
	var (
		uix uint64
		ui  uint32
	)

	switch ifc.(type) {
	case byte:
		data[0] = ifc.(byte)
	case float32:
		ui = math.Float32bits(ifc.(float32))
		m32(data, ui)
	case float64:
		uix = math.Float64bits(ifc.(float64))
		m64(data, uix)
	case rune:
		ui = uint32(ifc.(rune))
		m32(data, ui)
	case int:
		ui = uint32(ifc.(int))
		m32(data, ui)
	case uint32:
		ui = ifc.(uint32)
		m32(data, ui)
	case uintptr:
		uix = uint64(ifc.(uintptr))
		for i := uint64(0); i < 8; i++ {
			data[i] = byte((uix >> (8 * i)) & 0xff)
		}
	case uint64:
		uix = ifc.(uint64)
		m64(data, uix)
	case []rune:
		dd := make([]byte, utf8.UTFMax)
		xd := ifc.([]rune)
		n := 0
		for _, r := range xd {
			l := utf8.EncodeRune(dd, r)
			copy(data[n:n+l], dd)
			n += l
		}
	case string:
		dd := make([]byte, utf8.UTFMax)
		xd := ifc.(string)
		n := 0
		for _, r := range xd {
			l := utf8.EncodeRune(dd, r)
			copy(data[n:n+l], dd)
			n += l
		}
	case []byte:
		xd := ifc.([]byte)
		data := make([]byte, len(xd))
		copy(data[0:len(xd)], xd)
	default:
		return errors.New("unknown type for marshal")
	}
	return nil
}

func um32(data []byte) uint32 {
	ui := uint32((uint32(data[3]) << 24) | (uint32(data[2]) << 16) | (uint32(data[1]) << 8) | uint32(data[0]))
	return ui
}

func um64(data []byte) uint64 {
	uix := uint64((uint64(data[3]) << 24) | (uint64(data[2]) << 16) | (uint64(data[1]) << 8) | uint64(data[0]))
	uix |= uint64((uint64(data[7]) << 56) | (uint64(data[6]) << 48) | (uint64(data[5]) << 40) | (uint64(data[4]) << 32))
	return uix
}

func UnmarshalBinary(data []byte, iff interface{}) (interface{}, error) {
	var (
		s   []rune
		uix uint64
		ui  uint32
		ifc interface{}
	)

	switch iff.(type) {
	case byte:
		ifc = data[0]
	case float32:
		ui = um32(data)
		ifc = math.Float32frombits(ui)
	case float64:
		uix = um64(data)
		ifc = math.Float64frombits(uix)
	case rune:
		ifc = rune(um32(data))
	case uint32:
		ifc = um32(data)
	case int:
		ifc = int(um32(data))
	case uint64:
		uix = um64(data)
		ifc = uix
	case uintptr:
		uix = um64(data)
		ifc = uintptr(uix)
	case []rune:
		n := 0
		xd := ifc.([]rune)
		for i := 0; i < len(xd); i++ {
			rv, w := utf8.DecodeRune(data[n:])
			s = append(s, rv)
			n += w
		}
		ifc = s
	case []byte:
		xd := iff.([]byte)
		copy(xd[0:], data)
		ifc = xd
	default:
		return nil, errors.New("unknown type for unmarshal")
	}
	return ifc, nil
}
