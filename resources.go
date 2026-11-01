package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Combined packages explicitly locate their shared resources relative to the
// native bundle. Standalone builds use their own resources directory.
func resourceDirectory(base string) (string, error) {
	data, err := os.ReadFile(filepath.Join(base, "resource-location.json"))
	if errors.Is(err, os.ErrNotExist) {
		return base, nil
	}
	if err != nil {
		return "", err
	}
	var location struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(data, &location); err != nil {
		return "", fmt.Errorf("共享资源路径配置无效: %w", err)
	}
	path := filepath.FromSlash(location.Path)
	if strings.TrimSpace(path) == "" || filepath.IsAbs(path) || filepath.VolumeName(path) != "" {
		return "", fmt.Errorf("共享资源路径须为非空相对路径")
	}
	return filepath.Join(base, path), nil
}
