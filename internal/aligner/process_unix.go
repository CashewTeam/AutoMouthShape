//go:build !windows

package aligner

import (
	"context"
	"os/exec"
)

func configureProcess(cmd *exec.Cmd)                                      {}
func prepareModel(ctx context.Context, tmp, model string) (string, error) { return model, ctx.Err() }
