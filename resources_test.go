package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResourceDirectory(t *testing.T) {
	root := filepath.Join(t.TempDir(), "中文 空格")
	base := filepath.Join(root, "AutoMouthShape.app", "Contents", "Resources")
	if err := os.MkdirAll(base, 0700); err != nil { t.Fatal(err) }
	if got, err := resourceDirectory(base); err != nil || got != base { t.Fatalf("standalone: %q %v", got, err) }
	config := filepath.Join(base, "resource-location.json")
	if err := os.WriteFile(config, []byte(`{"path":"../../.."}`), 0600); err != nil { t.Fatal(err) }
	if got, err := resourceDirectory(base); err != nil || got != root { t.Fatalf("shared: %q %v", got, err) }
	for _, value := range []string{`{`, `{}`, `{"path":""}`, `{"path":"/absolute"}`} {
		if err := os.WriteFile(config, []byte(value), 0600); err != nil { t.Fatal(err) }
		if _, err := resourceDirectory(base); err == nil { t.Fatalf("accepted invalid config %s", value) }
	}
}
