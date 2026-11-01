package aligner

import (
	"automouthshape/internal/core"
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"golang.org/x/text/unicode/norm"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"
)

type Engine struct {
	Binary, Model, Table string
	pinyin               map[string]string
	vocab                map[string]bool
	simplified           map[string]string
}

//go:embed traditional.tsv
var traditional string

func New(root string) (*Engine, error) {
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	bin := "cpp-ort-aligner"
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	e := &Engine{Binary: filepath.Join(root, runtime.GOOS, bin), Model: filepath.Join(root, "model"), Table: filepath.Join(root, runtime.GOOS, "Chinese_to_Pinyin.txt"), pinyin: map[string]string{}, vocab: map[string]bool{}}
	e.simplified = map[string]string{}
	for _, line := range strings.Split(traditional, "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			fields := strings.Fields(parts[1])
			if len(fields) > 0 {
				e.simplified[parts[0]] = fields[0]
			}
		}
	}
	for _, p := range []string{e.Binary, filepath.Join(e.Model, "model.int8.onnx"), filepath.Join(e.Model, "tokens.txt"), e.Table} {
		if _, err := os.Stat(p); err != nil {
			return nil, fmt.Errorf("离线资源缺失: %s: %w", p, err)
		}
	}
	b, err := os.ReadFile(e.Table)
	if err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		p := strings.SplitN(sc.Text(), "\t", 2)
		if len(p) == 2 {
			fields := strings.Fields(p[1])
			if len(fields) > 0 {
				e.pinyin[p[0]] = plainPinyin(fields[0])
			}
		}
	}
	if err = sc.Err(); err != nil {
		return nil, err
	}
	b, err = os.ReadFile(filepath.Join(e.Model, "tokens.txt"))
	if err != nil {
		return nil, err
	}
	sc = bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := sc.Text()
		pos := strings.LastIndexByte(line, ' ')
		if pos > 0 {
			e.vocab[line[:pos]] = true
		}
	}
	if err = sc.Err(); err != nil {
		return nil, err
	}
	return e, nil
}
func plainPinyin(s string) string {
	var r []rune
	for _, v := range norm.NFD.String(strings.ToLower(s)) {
		if v == '\u0308' && len(r) > 0 && r[len(r)-1] == 'u' {
			r[len(r)-1] = 'v'
			continue
		}
		if !unicode.Is(unicode.Mn, v) {
			r = append(r, v)
		}
	}
	return string(r)
}
func MouthFor(p string) core.Mouth {
	// Main vowel of each Mandarin final, with contracted finals expanded.
	if strings.HasSuffix(p, "iu") {
		return core.U
	}
	if strings.HasSuffix(p, "ui") {
		return core.I
	}
	if strings.Contains(p, "a") {
		return core.A
	}
	if strings.Contains(p, "o") {
		return core.O
	}
	if strings.Contains(p, "e") {
		return core.E
	}
	if strings.Contains(p, "v") {
		return core.V
	}
	if strings.HasPrefix(p, "yu") || strings.HasPrefix(p, "ju") || strings.HasPrefix(p, "qu") || strings.HasPrefix(p, "xu") {
		return core.V
	}
	if strings.Contains(p, "i") {
		return core.I
	}
	return core.U
}

type inputSegment struct {
	Index int     `json:"index"`
	Text  string  `json:"text"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}
type outputSegment struct {
	Index int     `json:"index"`
	Text  string  `json:"text"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Score float64 `json:"score"`
}

func (e *Engine) Analyze(ctx context.Context, d *core.Document, pcm *core.PCM, progress func(float64, string)) ([]core.Character, []core.Segment, error) {
	if len(d.Cues) == 0 {
		return nil, nil, fmt.Errorf("没有有效字幕")
	}
	if d.Cues[len(d.Cues)-1].End > pcm.Duration()+.1 {
		return nil, nil, fmt.Errorf("字幕时间超出音频时长")
	}
	sil := core.Silence(pcm, d.SilenceDB)
	silentDuration := 0.0
	for _, r := range sil {
		silentDuration += r[1] - r[0]
	}
	if silentDuration >= pcm.Duration()-.08 {
		return nil, nil, fmt.Errorf("音频没有可用于对齐的有效语音")
	}
	tmp, err := os.MkdirTemp("", "automouthshape-align-*")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(tmp)
	model, err := prepareModel(ctx, tmp, e.Model)
	if err != nil {
		return nil, nil, err
	}
	var all []core.Character
	for begin := 0; begin < len(d.Cues); {
		if err = ctx.Err(); err != nil {
			return nil, nil, err
		}
		end := begin + 1
		for end < len(d.Cues) && d.Cues[end].End-d.Cues[begin].Start <= 30 {
			end++
		}
		offset := max(0, d.Cues[begin].Start-.5)
		limit := min(pcm.Duration(), d.Cues[end-1].End+.5)
		var in []inputSegment
		var chars []core.Character
		for _, cue := range d.Cues[begin:end] {
			for _, r := range cue.Text {
				if unicode.IsPunct(r) || unicode.IsSpace(r) {
					continue
				}
				ch := string(r)
				target := ch
				if s, ok := e.simplified[ch]; ok {
					target = s
				}
				if !unicode.Is(unicode.Han, r) || !e.vocab[target] {
					return nil, nil, fmt.Errorf("字幕 %d 含未支持的字符 %q；请将数字或外文改为中文读法", cue.Index, ch)
				}
				py, ok := e.pinyin[ch]
				if !ok {
					return nil, nil, fmt.Errorf("%q 缺少拼音映射", ch)
				}
				in = append(in, inputSegment{Index: len(in) + 1, Text: target, Start: max(0, cue.Start-offset), End: cue.End - offset})
				chars = append(chars, core.Character{Cue: cue.Index, Text: ch, Pinyin: py})
			}
		}
		if len(in) == 0 {
			return nil, nil, fmt.Errorf("字幕批次没有可对齐的汉字")
		}
		if float64(len(in))/(limit-offset) > 20 {
			return nil, nil, fmt.Errorf("字幕字数与音频区间明显不匹配")
		}
		input, _ := json.Marshal(struct {
			Segments []inputSegment `json:"segments"`
		}{in})
		if err = os.WriteFile(filepath.Join(tmp, "input.json"), input, 0600); err != nil {
			return nil, nil, err
		}
		if err = os.WriteFile(filepath.Join(tmp, "audio.wav"), pcm.WAV(offset, limit), 0600); err != nil {
			return nil, nil, err
		}
		// File output avoids mixing the CLI's diagnostics with JSON. Each process owns one batch.
		cmd := exec.CommandContext(ctx, e.Binary, "--audio", "audio.wav", "--model", model, "--json-input", "input.json", "--json-output", "output.json", "--language", "cmn", "--threads", "4")
		cmd.Dir = tmp
		configureProcess(cmd)
		var log bytes.Buffer
		cmd.Stdout = &log
		cmd.Stderr = &log
		if progress != nil {
			progress(float64(begin)/float64(len(d.Cues)), fmt.Sprintf("对齐字幕 %d–%d", begin+1, end))
		}
		if err = cmd.Run(); err != nil {
			if ctx.Err() != nil {
				return nil, nil, ctx.Err()
			}
			return nil, nil, fmt.Errorf("本地对齐失败: %w\n%s", err, log.String())
		}
		output, err := os.ReadFile(filepath.Join(tmp, "output.json"))
		if err != nil {
			return nil, nil, err
		}
		var result struct {
			Segments []outputSegment `json:"segments"`
		}
		if err = json.Unmarshal(output, &result); err != nil {
			return nil, nil, fmt.Errorf("对齐器 JSON 无效: %w", err)
		}
		if len(result.Segments) != len(chars) {
			return nil, nil, fmt.Errorf("逐字结果不完整: 期望 %d，收到 %d", len(chars), len(result.Segments))
		}
		prev := -1.0
		var confidenceSum float64
		for i, r := range result.Segments {
			if r.Text != in[i].Text || r.Index != i+1 || !core.Finite(r.Start) || !core.Finite(r.End) || !core.Finite(r.Score) || r.Start < 0 || r.End < r.Start || r.Start <= prev || r.End > limit-offset+.04 {
				return nil, nil, fmt.Errorf("字 %q 的 CTC 时间结果非法或未能对齐", chars[i].Text)
			}
			confidenceSum += max(0, min(1, r.Score))
			// Upstream CTC ends are inclusive. A single-frame spike is a valid 20 ms span.
			chars[i].Start = r.Start + offset
			chars[i].End = min(limit, r.End+offset+.02)
			chars[i].Score = r.Score
			prev = r.Start
		}
		if confidenceSum/float64(len(chars)) < .35 {
			return nil, nil, fmt.Errorf("字幕批次的整体对齐分数过低，请检查字幕与音频是否对应")
		}
		all = append(all, chars...)
		begin = end
	}
	var timeline []core.Segment
	cursor := 0.0
	cueEnds := map[int]float64{}
	for _, c := range d.Cues {
		cueEnds[c.Index] = c.End
	}
	for i, ch := range all {
		if ch.Start < cursor-1e-7 {
			return nil, nil, fmt.Errorf("字幕批次的时间范围重叠，请检查字幕")
		}
		end := max(ch.End, cueEnds[ch.Cue])
		if i+1 < len(all) {
			end = min(end, all[i+1].Start)
		}
		end = min(end, pcm.Duration())
		if ch.Start > cursor {
			timeline = append(timeline, core.Segment{Start: cursor, End: ch.Start, Mouth: core.Closed})
		}
		if end <= ch.Start {
			return nil, nil, fmt.Errorf("字 %q 没有有效展示区间", ch.Text)
		}
		timeline = append(timeline, core.Segment{Start: ch.Start, End: end, Mouth: MouthFor(ch.Pinyin)})
		cursor = end
	}
	if cursor < pcm.Duration() {
		timeline = append(timeline, core.Segment{Start: cursor, End: pcm.Duration(), Mouth: core.Closed})
	}
	timeline = core.ApplySilence(core.Normalize(timeline), sil)
	if err = core.ValidateTimeline(timeline, pcm.Duration()); err != nil {
		return nil, nil, err
	}
	return all, timeline, nil
}
