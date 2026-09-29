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

// Reasoning reports whether a model thinks before it answers (DeepSeek-R1,
// QwQ, Qwen3, gpt-oss, o1/o3…): it needs room for its thinking, which is
// stripped from the answer (stripThinking).
func Reasoning(model string) bool {
	m := strings.ToLower(model)
	for _, k := range []string{"deepseek-r1", "-r1", "r1:", "qwq", "qwen3", "gpt-oss", "magistral", "reason", "think", "o1-", "o3", "o4-mini"} {
		if strings.Contains(m, k) {
			return true
		}
	}
	return m == "o1" || strings.HasPrefix(m, "o1:")
}

// MaxTokens is how long an answer may be: a verdict is a few hundred tokens,
// plus a reasoning model's thinking.
func MaxTokens(model string) int {
	if Reasoning(model) {
		return 4000
	}
	return 600
}
