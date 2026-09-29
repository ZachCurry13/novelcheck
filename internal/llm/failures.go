package llm

import (
	"context"
	"errors"
	"net"
	"strings"
)

// HostDown reports whether err means the AI server can't answer at all
// right now: it couldn't be reached (switched off, wrong address) or it
// didn't answer in time. Its other models are skipped then, so a dead or
// stuck server doesn't cost a wait per model before the backup AI.
func HostDown(err error) bool {
	var op *net.OpError
	if errors.As(err, &op) && op.Op == "dial" {
		return true
	}
	var dns *net.DNSError
	return errors.As(err, &dns) || errors.Is(err, context.DeadlineExceeded)
}

// TooLong reports whether the AI refused the request for not fitting its
// context window ("exceeds the available context size", "maximum context
// length", "input length exceeds the context length").
func TooLong(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "context") && (strings.Contains(msg, "exceed") || strings.Contains(msg, "maximum") ||
		strings.Contains(msg, "too long") || strings.Contains(msg, "n_ctx"))
}
