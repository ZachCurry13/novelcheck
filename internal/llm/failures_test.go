package llm

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
)

func TestFailures(t *testing.T) {
	dial := &net.OpError{Op: "dial", Err: errors.New("connection refused")}
	if !HostDown(fmt.Errorf("post: %w", dial)) || !HostDown(fmt.Errorf("post: %w", context.DeadlineExceeded)) || HostDown(errors.New("invalid JSON")) {
		t.Fatal("HostDown")
	}
	for msg, want := range map[string]bool{
		`400 {"error":{"message":"the request exceeds the available context size, try increasing it","type":"exceed_context_size_error","n_ctx":4096}}`: true,
		"This model's maximum context length is 8192 tokens": true,
		"level must be 0-5, got ":                            false,
	} {
		if TooLong(errors.New(msg)) != want {
			t.Fatalf("TooLong(%q) != %v", msg, want)
		}
	}
}

func TestReasoning(t *testing.T) {
	for m, want := range map[string]bool{"deepseek-r1:1.5b": true, "qwen3:8b": true, "qwq": true, "gpt-oss:20b": true,
		"qwen2.5:7b": false, "llama3.1:8b": false, "llama3.2": false, "gpt-4o-mini": false, "o3-mini": true} {
		if Reasoning(m) != want {
			t.Errorf("%s: %v", m, !want)
		}
	}
	if MaxTokens("deepseek-r1:7b") <= MaxTokens("qwen2.5:7b") {
		t.Fatal("reasoning models need room to think")
	}
}
