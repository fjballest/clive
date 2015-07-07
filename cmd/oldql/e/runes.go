package e

import (
	"clive/sre"
)

/*
	Interface for a text provider
*/
type Text interface {
	sre.Text
	GetRunes(p0, p1 int) <-chan []rune
}

/*
	Type implementing a generic text provider interface
	relying on an array of runes
*/
type Runes []rune

func (t *Runes) Len() int {
	return len(*t)
}

func (t *Runes) GetRune(n int) rune {
	return (*t)[n]
}

func (t *Runes) GetRunes(p0, p1 int) <-chan []rune {
	c := make(chan []rune)
	go func() {
		c <- (*t)[p0:p1]
		close(c)
	}()
	return c
}
