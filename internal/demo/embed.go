package demo

import (
	"automouthshape/internal/core"
	"embed"
	"encoding/json"
	"fmt"
)

//go:embed data
var Demo embed.FS

func LoadDemo() (*core.Document, *core.PCM, error) {
	d := core.NewDocument()
	d.Name = "中文口型示例"
	read := func(p string) ([]byte, error) { return Demo.ReadFile("data/" + p) }
	var e error
	d.Audio.Name = "example.wav"
	d.Audio.Data, e = read(d.Audio.Name)
	if e != nil {
		return nil, nil, e
	}
	d.Subtitle.Name = "example.srt"
	d.Subtitle.Data, e = read(d.Subtitle.Name)
	if e != nil {
		return nil, nil, e
	}
	for _, m := range core.Mouths {
		b, e := read("mouths/" + string(m) + ".png")
		if e != nil {
			return nil, nil, e
		}
		d.Mouths[m] = core.Asset{Name: string(m) + ".png", Data: b}
	}
	if _, _, e = core.ValidateMouths(d.Mouths); e != nil {
		return nil, nil, e
	}
	pcm, e := core.Decode(d.Audio.Data)
	if e != nil {
		return nil, nil, e
	}
	d.Duration = pcm.Duration()
	d.Cues, e = core.ParseSRT(d.Subtitle.Data)
	if e != nil {
		return nil, nil, e
	}
	b, e := read("analysis.json")
	if e != nil {
		return nil, nil, e
	}
	var analysis struct {
		Characters []core.Character `json:"characters"`
		Timeline   []core.Segment   `json:"timeline"`
	}
	if e = json.Unmarshal(b, &analysis); e != nil {
		return nil, nil, e
	}
	d.Characters = analysis.Characters
	d.Timeline = analysis.Timeline
	if e = core.ValidateTimeline(d.Timeline, d.Duration); e != nil {
		return nil, nil, fmt.Errorf("示例时间轴无效: %w", e)
	}
	return d, pcm, nil
}
