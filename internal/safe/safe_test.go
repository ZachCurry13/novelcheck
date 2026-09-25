package safe

import (
	"errors"
	"strings"
	"testing"
)

func TestRunRecovers(t *testing.T) {
	var nilMap map[string]int
	err := Run("test job", func() error {
		nilMap["boom"] = 1 // panics: assignment to entry in nil map
		return nil
	})
	if err == nil || !strings.Contains(err.Error(), "test job crashed") {
		t.Fatalf("panic should become an error: %v", err)
	}
	want := errors.New("plain")
	if err := Run("x", func() error { return want }); err != want {
		t.Fatalf("errors pass through: %v", err)
	}
	done := make(chan struct{})
	Go("goroutine", func() {
		defer close(done)
		panic("still fine")
	})
	<-done // reaching here means the process survived
}
