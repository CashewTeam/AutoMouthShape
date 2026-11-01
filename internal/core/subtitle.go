package core

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

func parseTime(s string) (float64, error) {
	p := strings.Split(strings.TrimSpace(strings.ReplaceAll(s, ",", ".")), ":")
	if len(p) != 3 {
		return 0, fmt.Errorf("时间码无效: %s", s)
	}
	h, e := strconv.Atoi(p[0])
	if e != nil || h < 0 {
		return 0, fmt.Errorf("时间码无效: %s", s)
	}
	m, e := strconv.Atoi(p[1])
	if e != nil || m < 0 || m >= 60 {
		return 0, fmt.Errorf("时间码无效: %s", s)
	}
	sec, e := strconv.ParseFloat(p[2], 64)
	if e != nil || !Finite(sec) || sec < 0 || sec >= 60 {
		return 0, fmt.Errorf("时间码无效: %s", s)
	}
	return float64(h*3600+m*60) + sec, nil
}
func ParseSRT(data []byte) ([]Cue, error) {
	if !utf8.Valid(data) {
		return nil, fmt.Errorf("字幕必须使用 UTF-8 编码")
	}
	text := strings.TrimSpace(strings.TrimPrefix(strings.ReplaceAll(string(data), "\r\n", "\n"), "\ufeff"))
	if text == "" {
		return nil, fmt.Errorf("字幕文件为空")
	}
	var out []Cue
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); {
		if strings.TrimSpace(lines[i]) == "" {
			i++
			continue
		}
		idx, e := strconv.Atoi(strings.TrimSpace(lines[i]))
		if e != nil {
			return nil, fmt.Errorf("字幕第 %d 行缺少序号", i+1)
		}
		i++
		if i >= len(lines) {
			return nil, fmt.Errorf("字幕 %d 缺少时间码", idx)
		}
		times := strings.Split(lines[i], "-->")
		if len(times) != 2 {
			return nil, fmt.Errorf("字幕 %d 时间码格式错误", idx)
		}
		i++
		start, e := parseTime(times[0])
		if e != nil {
			return nil, e
		}
		end, e := parseTime(times[1])
		if e != nil {
			return nil, e
		}
		var content []string
		for i < len(lines) && strings.TrimSpace(lines[i]) != "" {
			content = append(content, strings.TrimSpace(lines[i]))
			i++
		}
		value := strings.Join(content, " ")
		if end <= start || value == "" {
			return nil, fmt.Errorf("字幕 %d 内容或时间范围无效", idx)
		}
		if len(out) > 0 && start < out[len(out)-1].End {
			return nil, fmt.Errorf("字幕 %d 与上一条重叠", idx)
		}
		out = append(out, Cue{Index: idx, Start: start, End: end, Text: value})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("没有有效字幕")
	}
	return out, nil
}
