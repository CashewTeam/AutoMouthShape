package core

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type manifest struct {
	Format     int              `json:"format"`
	AppVersion string           `json:"app_version"`
	Name       string           `json:"name"`
	Audio      string           `json:"audio"`
	Subtitle   string           `json:"subtitle"`
	Mouths     map[Mouth]string `json:"mouths"`
	Cues       []Cue            `json:"cues"`
	Characters []Character      `json:"characters"`
	Timeline   []Segment        `json:"timeline"`
	Duration   float64          `json:"duration"`
	FPS        int              `json:"fps"`
	SilenceDB  float64          `json:"silence_db"`
}

func SaveProject(path string, d *Document) error {
	if _, _, e := ValidateMouths(d.Mouths); e != nil {
		return e
	}
	if e := ValidateTimeline(d.Timeline, d.Duration); e != nil {
		return e
	}
	if !ValidFPS(d.FPS) {
		return fmt.Errorf("帧率无效")
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".ams-save-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	z := zip.NewWriter(f)
	add := func(name string, data []byte) error {
		w, e := z.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		if e != nil {
			return e
		}
		_, e = w.Write(data)
		return e
	}
	m := manifest{Format: 1, AppVersion: Version, Name: d.Name, Audio: "audio" + filepath.Ext(d.Audio.Name), Subtitle: "subtitles.srt", Mouths: map[Mouth]string{}, Cues: d.Cues, Characters: d.Characters, Timeline: d.Timeline, Duration: d.Duration, FPS: d.FPS, SilenceDB: d.SilenceDB}
	for _, mouth := range Mouths {
		m.Mouths[mouth] = "mouths/" + string(mouth) + ".png"
		if e = add(m.Mouths[mouth], d.Mouths[mouth].Data); e != nil {
			z.Close()
			f.Close()
			return e
		}
	}
	if e = add(m.Audio, d.Audio.Data); e == nil {
		e = add(m.Subtitle, d.Subtitle.Data)
	}
	if e == nil {
		var b []byte
		b, e = json.MarshalIndent(m, "", "  ")
		if e == nil {
			e = add("project.json", b)
		}
	}
	if ze := z.Close(); e == nil {
		e = ze
	}
	if e == nil {
		e = f.Sync()
	}
	if ce := f.Close(); e == nil {
		e = ce
	}
	if e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
func OpenProject(path string) (*Document, *PCM, error) {
	z, e := zip.OpenReader(path)
	if e != nil {
		return nil, nil, e
	}
	defer z.Close()
	files := map[string]*zip.File{}
	for _, f := range z.File {
		if _, exists := files[f.Name]; exists {
			return nil, nil, fmt.Errorf("工程内有重复条目: %s", f.Name)
		}
		files[f.Name] = f
	}
	read := func(name string) ([]byte, error) {
		f, ok := files[name]
		if !ok {
			return nil, fmt.Errorf("工程缺少 %s", name)
		}
		if f.UncompressedSize64 > 1<<30 {
			return nil, fmt.Errorf("工程资源过大: %s", name)
		}
		r, e := f.Open()
		if e != nil {
			return nil, e
		}
		defer r.Close()
		return io.ReadAll(io.LimitReader(r, (1<<30)+1))
	}
	b, e := read("project.json")
	if e != nil {
		return nil, nil, e
	}
	var m manifest
	if e = json.Unmarshal(b, &m); e != nil {
		return nil, nil, e
	}
	if m.Format != 1 {
		return nil, nil, fmt.Errorf("不支持的工程格式版本: %d", m.Format)
	}
	if !ValidFPS(m.FPS) || !Finite(m.SilenceDB) || m.SilenceDB < -90 || m.SilenceDB > -5 {
		return nil, nil, fmt.Errorf("工程设置无效")
	}
	d := NewDocument()
	d.Name = m.Name
	d.Cues = m.Cues
	d.Characters = m.Characters
	d.Timeline = m.Timeline
	d.FPS = m.FPS
	d.SilenceDB = m.SilenceDB
	d.Audio.Name = filepath.Base(m.Audio)
	d.Audio.Data, e = read(m.Audio)
	if e != nil {
		return nil, nil, e
	}
	pcm, e := Decode(d.Audio.Data)
	if e != nil {
		return nil, nil, e
	}
	d.Duration = pcm.Duration()
	d.Subtitle.Name = "subtitles.srt"
	d.Subtitle.Data, e = read(m.Subtitle)
	if e != nil {
		return nil, nil, e
	}
	d.Cues, e = ParseSRT(d.Subtitle.Data)
	if e != nil {
		return nil, nil, e
	}
	for _, mouth := range Mouths {
		data, e := read(m.Mouths[mouth])
		if e != nil {
			return nil, nil, e
		}
		d.Mouths[mouth] = Asset{Name: string(mouth) + ".png", Data: data}
	}
	if _, _, e = ValidateMouths(d.Mouths); e != nil {
		return nil, nil, e
	}
	if e = ValidateTimeline(d.Timeline, d.Duration); e != nil {
		return nil, nil, e
	}
	return d, pcm, nil
}
