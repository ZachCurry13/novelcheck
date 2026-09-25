// Package safe keeps a panic in one background job from taking the whole
// app down. An unrecovered panic in any goroutine ends the process, and
// TrueNAS then restarts the container, dropping whatever was in progress.
package safe

import (
	"fmt"
	"log"
	"runtime/debug"
)

// Run calls fn, turning a panic into an error (logged with its stack).
func Run(what string, fn func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			log.Printf("%s crashed: %v\n%s", what, p, debug.Stack())
			err = fmt.Errorf("%s crashed unexpectedly: %v", what, p)
		}
	}()
	return fn()
}

// Go runs fn in its own goroutine without risking the app.
func Go(what string, fn func()) {
	go func() { _ = Run(what, func() error { fn(); return nil }) }()
}
