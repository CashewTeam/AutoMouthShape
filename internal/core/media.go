package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/egoist/mygo/audio"
	_ "github.com/egoist/mygo/audio/mp3"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const SampleRate = 16000

type PCM struct{ Samples []float32 }

func (p *PCM) Duration() float64 { return float64(len(p.Samples)) / SampleRate }
func Decode(data []byte) (*PCM, error) {
	d, _, err := audio.NewDecoder(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if closer, ok := d.(io.Closer); ok {
		defer closer.Close()
	}
	channels, rate := d.Channels(), d.SampleRate()
	if channels < 1 || rate < 1 {
		return nil, fmt.Errorf("音频采样参数无效")
	}
	var all []float32
	buf := make([]float32, 8192*channels)
	for {
		n, e := d.Read(buf)
		if n > 0 {
			all = append(all, buf[:n]...)
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, e
		}
		if n == 0 {
			return nil, io.ErrNoProgress
		}
	}
	frames := len(all) / channels
	if frames == 0 {
		return nil, fmt.Errorf("音频为空")
	}
	mono := make([]float32, frames)
	for f := range frames {
		for c := range channels {
			v := all[f*channels+c]
			if !Finite(float64(v)) {
				return nil, fmt.Errorf("音频含有非法采样")
			}
			mono[f] += v / float32(channels)
		}
	}
	out := make([]float32, int(math.Ceil(float64(frames)*SampleRate/float64(rate))))
	for i := range out {
		x := float64(i) * float64(rate) / SampleRate
		j := min(int(x), frames-1)
		k := min(j+1, frames-1)
		out[i] = mono[j] + float32(x-float64(j))*(mono[k]-mono[j])
	}
	return &PCM{Samples: out}, nil
}
func (p *PCM) WAV(start, end float64) []byte {
	a, b := max(0, int(start*SampleRate)), min(len(p.Samples), int(math.Ceil(end*SampleRate)))
	if b < a {
		b = a
	}
	var out bytes.Buffer
	out.WriteString("RIFF")
	binary.Write(&out, binary.LittleEndian, uint32(36+(b-a)*2))
	out.WriteString("WAVEfmt ")
	binary.Write(&out, binary.LittleEndian, uint32(16))
	for _, v := range []uint16{1, 1} {
		binary.Write(&out, binary.LittleEndian, v)
	}
	binary.Write(&out, binary.LittleEndian, uint32(SampleRate))
	binary.Write(&out, binary.LittleEndian, uint32(SampleRate*2))
	binary.Write(&out, binary.LittleEndian, uint16(2))
	binary.Write(&out, binary.LittleEndian, uint16(16))
	out.WriteString("data")
	binary.Write(&out, binary.LittleEndian, uint32((b-a)*2))
	for _, v := range p.Samples[a:b] {
		binary.Write(&out, binary.LittleEndian, int16(max(-1, min(1, v))*32767))
	}
	return out.Bytes()
}
func ValidateMouths(assets map[Mouth]Asset) (int, int, error) {
	w, h := 0, 0
	for _, m := range Mouths {
		a, ok := assets[m]
		if !ok || len(a.Data) == 0 {
			return 0, 0, fmt.Errorf("缺少 %s 口型 PNG", m.Label())
		}
		img, e := png.Decode(bytes.NewReader(a.Data))
		if e != nil {
			return 0, 0, fmt.Errorf("%s: %w", a.Name, e)
		}
		r := img.Bounds()
		if w == 0 {
			w, h = r.Dx(), r.Dy()
		} else if r.Dx() != w || r.Dy() != h {
			return 0, 0, fmt.Errorf("七张 PNG 的画布尺寸须一致 (%s)", a.Name)
		}
	}
	return w, h, nil
}
func LoadMouthFolder(dir string) (map[Mouth]Asset, error) {
	entries, e := os.ReadDir(dir)
	if e != nil {
		return nil, e
	}
	out := make(map[Mouth]Asset)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		for _, m := range Mouths {
			if name == string(m)+".png" {
				if _, ok := out[m]; ok {
					return nil, fmt.Errorf("%s 有重复文件", m.Label())
				}
				data, e := os.ReadFile(filepath.Join(dir, entry.Name()))
				if e != nil {
					return nil, e
				}
				out[m] = Asset{Name: entry.Name(), Data: data}
			}
		}
	}
	_, _, e = ValidateMouths(out)
	return out, e
}
func Silence(p *PCM, db float64) [][2]float64 {
	const window = SampleRate / 100
	var out [][2]float64
	start := -1
	for i := 0; i < len(p.Samples); i += window {
		end := min(i+window, len(p.Samples))
		var sum float64
		for _, v := range p.Samples[i:end] {
			sum += float64(v) * float64(v)
		}
		silent := 10*math.Log10(max(sum/float64(end-i), 1e-12)) < db
		if silent && start < 0 {
			start = i
		}
		if !silent && start >= 0 {
			if i-start >= SampleRate*8/100 {
				out = append(out, [2]float64{float64(start) / SampleRate, float64(i) / SampleRate})
			}
			start = -1
		}
	}
	if start >= 0 && len(p.Samples)-start >= SampleRate*8/100 {
		out = append(out, [2]float64{float64(start) / SampleRate, p.Duration()})
	}
	return out
}
func ApplySilence(s []Segment, intervals [][2]float64) []Segment {
	out := append([]Segment(nil), s...)
	for _, r := range intervals {
		var next []Segment
		for _, v := range out {
			if v.End <= r[0] || v.Start >= r[1] {
				next = append(next, v)
				continue
			}
			if v.Start < r[0] {
				next = append(next, Segment{v.Start, r[0], v.Mouth})
			}
			next = append(next, Segment{max(v.Start, r[0]), min(v.End, r[1]), Closed})
			if v.End > r[1] {
				next = append(next, Segment{r[1], v.End, v.Mouth})
			}
		}
		out = next
	}
	return Normalize(out)
}
