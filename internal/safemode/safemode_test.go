package safemode

import (
	"testing"
	"time"
)

func TestCrashLoopFlagAndLeave(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvVar, "")
	now := time.Now()
	// Two quick starts that never ended cleanly are fine; the third is a crash loop.
	if r := Starting(dir, now); r != "" {
		t.Fatalf("first start: %q", r)
	}
	if r := Starting(dir, now.Add(time.Minute)); r != "" {
		t.Fatalf("second start: %q", r)
	}
	if r := Starting(dir, now.Add(2*time.Minute)); r != ByCrash {
		t.Fatalf("third quick start: %q", r)
	}
	// A clean run forgets them; starts long apart don't add up.
	Clean(dir)
	if r := Starting(dir, now.Add(3*time.Minute)); r != "" {
		t.Fatalf("after a clean run: %q", r)
	}
	Starting(dir, now.Add(20*time.Minute))
	if r := Starting(dir, now.Add(40*time.Minute)); r != "" {
		t.Fatalf("starts far apart: %q", r)
	}

	// The flag, and leaving safe mode starts the background work once.
	Clean(dir)
	_ = SetFlag(dir, true)
	reason := Starting(dir, now.Add(time.Hour))
	if reason != ByFlag {
		t.Fatalf("flag: %q", reason)
	}
	started := 0
	st := NewState(dir, reason, func() { started++ })
	if err := st.Leave(); err != nil || started != 1 || st.Reason() != "" || FlagSet(dir) {
		t.Fatalf("leave: %v started=%d reason=%q flag=%v", err, started, st.Reason(), FlagSet(dir))
	}

	// Set in the environment, it can't be left from the app.
	t.Setenv(EnvVar, "true")
	if r := Starting(dir, now.Add(2*time.Hour)); r != ByEnv {
		t.Fatalf("env: %q", r)
	}
	if err := NewState(dir, ByEnv, func() {}).Leave(); err != ErrByEnv {
		t.Fatalf("leave env: %v", err)
	}
}
