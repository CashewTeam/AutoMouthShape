package core

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"
)

func Export(ctx context.Context, parent string, d *Document, progress func(int, int)) (dir string, err error) {
	if !ValidFPS(d.FPS) {
		return "", fmt.Errorf("不支持的帧率")
	}
	if len(d.Timeline) == 0 {
		return "", fmt.Errorf("请先完成分析")
	}
	if err = ValidateTimeline(d.Timeline, d.Duration); err != nil {
		return "", err
	}
	if _, _, err = ValidateMouths(d.Mouths); err != nil {
		return "", err
	}
	dir, err = os.MkdirTemp(parent, "AutoMouthShape_"+time.Now().Format("20060102_150405")+"_")
	if err != nil {
		return "", err
	}
	marker := filepath.Join(dir, "INCOMPLETE.txt")
	if err = os.WriteFile(marker, []byte("导出未完成；请勿将此目录当作完整序列。"), 0644); err != nil {
		return dir, err
	}
	count := int(math.Ceil(d.Duration * float64(d.FPS)))
	for n := 0; n < count; n++ {
		if err = ctx.Err(); err != nil {
			return dir, err
		}
		m := MouthAt(d.Timeline, float64(n)/float64(d.FPS))
		path := filepath.Join(dir, fmt.Sprintf("frame_%06d.png", n+1))
		if err = os.WriteFile(path, d.Mouths[m].Data, 0644); err != nil {
			return dir, err
		}
		if progress != nil && (n%30 == 0 || n+1 == count) {
			progress(n+1, count)
		}
	}
	err = os.Remove(marker)
	return dir, err
}
