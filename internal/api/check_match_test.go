package api

import "testing"

func TestSameWriter(t *testing.T) {
	same := [][2]string{{"Rowling, J. K.", "J.K. Rowling"}, {"Liz Braswell", "LIZ BRASWELL"}, {"Ilona Andrews & Gordon Andrews", "Ilona Andrews"}, {"James S. A. Corey", "Corey"}}
	for _, p := range same {
		if !sameWriter(p[0], p[1]) {
			t.Errorf("%q and %q are the same writer", p[0], p[1])
		}
	}
	other := [][2]string{{"Abby Jimenez", "Liz Braswell"}, {"Sarah J. Maas", "Sarah Dessen"}}
	for _, p := range other {
		if sameWriter(p[0], p[1]) {
			t.Errorf("%q and %q are different writers", p[0], p[1])
		}
	}
}
