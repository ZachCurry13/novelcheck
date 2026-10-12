package analyzer

import (
	"testing"

	"github.com/zachcurry13/novelcheck/internal/store"
)

func TestVisionOrderTriesCloudFirst(t *testing.T) {
	ais := []store.AIConfig{
		{Name: "home", BaseURL: "http://192.168.1.20:11434"},
		{Name: "cloud", BaseURL: "https://generativelanguage.googleapis.com/v1beta/openai"},
		{Name: "also home", BaseURL: "http://127.0.0.1:11434"},
	}
	got := visionOrder(ais)
	if len(got) != 3 || got[0].Name != "cloud" || got[1].Name != "home" || got[2].Name != "also home" {
		t.Fatalf("order: %+v", got)
	}
}
