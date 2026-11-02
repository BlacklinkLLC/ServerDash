package plugins

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateDefaults(t *testing.T) {
	mf := Manifest{
		ID: "demo", Name: "Demo",
		Widgets: []Widget{{ID: "w1", Command: "echo hi"}},
		Actions: []Action{{ID: "a1", Command: "true", Role: "root"}},
	}
	if err := mf.validate(); err != nil {
		t.Fatal(err)
	}
	if mf.Widgets[0].Format != "text" || mf.Widgets[0].Interval != 60 {
		t.Fatalf("widget defaults not applied: %+v", mf.Widgets[0])
	}
	if mf.Actions[0].Role != "admin" {
		t.Fatalf("unknown role should fall back to admin, got %q", mf.Actions[0].Role)
	}
}

func TestValidateRejects(t *testing.T) {
	bad := []Manifest{
		{ID: "Bad ID", Name: "x"},
		{ID: "ok", Name: ""},
		{ID: "ok", Name: "x", Widgets: []Widget{{ID: "w", Command: "true", Format: "html"}}},
		{ID: "ok", Name: "x", Widgets: []Widget{{ID: "ww", Command: "true"}, {ID: "ww", Command: "true"}}},
		{ID: "ok", Name: "x", Settings: []Setting{{Key: "bad key"}}},
	}
	for i, mf := range bad {
		if err := mf.validate(); err == nil {
			t.Errorf("case %d: expected an error", i)
		}
	}
}

// Bundled plugins must all load, so a broken manifest fails the build.
func TestBundledPluginsLoad(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "plugins")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skip("no bundled plugins folder")
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := load(filepath.Join(dir, e.Name())); err != nil {
			t.Errorf("%s: %v", e.Name(), err)
		}
	}
}
