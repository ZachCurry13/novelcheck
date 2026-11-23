package llm

import (
	"context"
	"time"
)

// QuickLimit is how long a cloud model gets while a person waits (Check a book) before the next one is tried.
const QuickLimit = 30 * time.Second

type quickKey struct{}

// Quick marks requests a person is waiting on: a "busy" answer is not retried after a pause (the next model is
// asked at once), and a cloud model gets QuickLimit at most. Background ratings keep the patient behavior.
func Quick(ctx context.Context) context.Context { return context.WithValue(ctx, quickKey{}, true) }

// IsQuick says whether ctx is marked Quick.
func IsQuick(ctx context.Context) bool { v, _ := ctx.Value(quickKey{}).(bool); return v }

// QuickTimeout is limit, shortened to QuickLimit for a cloud model when ctx is Quick.
func QuickTimeout(ctx context.Context, baseURL string, limit time.Duration) time.Duration {
	if IsQuick(ctx) && !IsLocal(baseURL) {
		return min(limit, QuickLimit)
	}
	return limit
}
