package kindleapp

import (
	"os/exec"
	"path/filepath"
	"testing"
)

// The KindleForge package's scripts must at least parse as shell (a Kindle runs them with sh).
func TestKindleForgeScriptsParse(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("no sh here")
	}
	scripts, _ := filepath.Glob("kindleforge/*.sh")
	if len(scripts) != 2 {
		t.Fatalf("install.sh and uninstall.sh: %v", scripts)
	}
	for _, s := range append(scripts, "NovelCheck.sh") {
		if out, err := exec.Command(sh, "-n", s).CombinedOutput(); err != nil {
			t.Fatalf("%s: %v %s", s, err, out)
		}
	}
}
