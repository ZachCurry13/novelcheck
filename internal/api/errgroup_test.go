package api

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestRepeatedErrorsAreOneLine(t *testing.T) {
	e := func(at, detail string) store.ServerError {
		return store.ServerError{At: at, Method: "background", Path: "deep scan", Detail: detail}
	}
	g := groupErrors([]store.ServerError{e("12:03", "segfault pid 41"), e("12:02", "segfault pid 77"), e("12:01", "other")}, 40)
	if len(g) != 2 || g[0].Count != 2 || g[0].First != "12:02" || g[1].Count != 1 {
		t.Fatalf("got %+v", g)
	}
}
