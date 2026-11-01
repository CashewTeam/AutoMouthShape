package aligner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestModelLayout(t *testing.T) {
	root := filepath.Join(t.TempDir(), "中文 空格", "runtime")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	got, err := modelDirectory(root)
	if err != nil || got != filepath.Join(root, "model") {
		t.Fatalf("standalone: %s %v", got, err)
	}
	config := filepath.Join(root, "model-location.json")
	for index, data := range []string{`{"path":"../AutoMouthShape.app/Contents/Resources/runtime/model"}`, `{}`, `{`, `{"path":""}`} {
		if err := os.WriteFile(config, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		got, err = modelDirectory(root)
		if index == 0 {
			want := filepath.Join(filepath.Dir(root), "AutoMouthShape.app", "Contents", "Resources", "runtime", "model")
			if err != nil || got != want {
				t.Fatalf("combined: %s %v", got, err)
			}
		} else if err == nil {
			t.Fatalf("accepted invalid config: %s", data)
		}
	}
}
