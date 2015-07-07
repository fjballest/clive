package snarf

import (
	"testing"
)

func test(s string, t *testing.T) {
	if err := Set(s); err != nil {
		t.Fatalf("set: %s", err)
	}
	o, err := Get()
	if err != nil {
		t.Fatalf("get: %s", err)
	}
	if o != s {
		t.Fatalf("got '%s' and not '%s'", o, s)
	}
}

func TestCb(t *testing.T) {
	test("", t)
	test(`hola
caracola
`, t)
}
