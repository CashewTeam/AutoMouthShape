//go:build windows

package aligner

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

func configureProcess(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }

// The upstream CLI accepts narrow argv. Stage its model beside the ASCII-named
// job files so no Unicode path is passed through C++ argv or narrow fopen.
// Go supplies the working directory and executable to Windows as UTF-16.
func prepareModel(ctx context.Context, tmp, model string) (string, error) {
	dest := filepath.Join(tmp, "model")
	if err := os.Mkdir(dest, 0700); err != nil {
		return "", err
	}
	for _, name := range []string{"model.int8.onnx", "tokens.txt"} {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		src, err := os.Open(filepath.Join(model, name))
		if err != nil {
			return "", err
		}
		out, err := os.Create(filepath.Join(dest, name))
		if err != nil {
			src.Close()
			return "", err
		}
		buffer := make([]byte, 1024*1024)
		for {
			if err = ctx.Err(); err != nil {
				break
			}
			var n int
			n, err = src.Read(buffer)
			if n > 0 {
				if _, writeErr := out.Write(buffer[:n]); writeErr != nil {
					err = writeErr
					break
				}
			}
			if err == io.EOF {
				err = nil
				break
			}
			if err != nil {
				break
			}
		}
		src.Close()
		closeErr := out.Close()
		if err != nil {
			return "", err
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	return "model", nil
}
