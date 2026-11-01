package core

import (
	"errors"
	"fmt"
	"math"
	"sort"
)

const Version = "0.0.5"

type Mouth string

const (
	A      Mouth = "a"
	O      Mouth = "o"
	E      Mouth = "e"
	I      Mouth = "i"
	U      Mouth = "u"
	V      Mouth = "v"
	Closed Mouth = "closed"
)

var Mouths = []Mouth{A, O, E, I, U, V, Closed}

func (m Mouth) Label() string {
	if m == Closed {
		return "闭口"
	}
	if m == V {
		return "Ü"
	}
	return string(m[0] - 32)
}
func (m Mouth) Valid() bool {
	for _, v := range Mouths {
		if v == m {
			return true
		}
	}
	return false
}

type Cue struct {
	Index int     `json:"index"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}
type Character struct {
	Cue    int     `json:"cue"`
	Text   string  `json:"text"`
	Pinyin string  `json:"pinyin"`
	Start  float64 `json:"start"`
	End    float64 `json:"end"`
	Score  float64 `json:"score"`
}
type Segment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Mouth Mouth   `json:"mouth"`
}
type Asset struct {
	Name string
	Data []byte
}
type Document struct {
	Name       string
	Audio      Asset
	Subtitle   Asset
	Mouths     map[Mouth]Asset
	Cues       []Cue
	Characters []Character
	Timeline   []Segment
	Duration   float64
	FPS        int
	SilenceDB  float64
}

func NewDocument() *Document {
	return &Document{Name: "未命名工程", Mouths: make(map[Mouth]Asset), FPS: 30, SilenceDB: -40}
}
func ValidFPS(v int) bool   { return v == 24 || v == 25 || v == 30 || v == 60 }
func Finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func ValidateTimeline(s []Segment, duration float64) error {
	if !Finite(duration) || duration <= 0 {
		return errors.New("音频时长无效")
	}
	prev := 0.0
	for i, v := range s {
		if !Finite(v.Start) || !Finite(v.End) || v.Start < prev-1e-8 || v.End <= v.Start || v.End > duration+1e-6 || !v.Mouth.Valid() {
			return fmt.Errorf("第 %d 个口型区间无效或重叠", i+1)
		}
		prev = v.End
	}
	return nil
}

// MouthAt is the sole time sampling rule shared by playback and export. Gaps close the mouth.
func MouthAt(s []Segment, t float64) Mouth {
	n := sort.Search(len(s), func(i int) bool { return s[i].End > t })
	if n < len(s) && s[n].Start <= t {
		return s[n].Mouth
	}
	return Closed
}
func Normalize(s []Segment) []Segment {
	out := make([]Segment, 0, len(s))
	for _, v := range s {
		if v.End <= v.Start {
			continue
		}
		n := len(out)
		if n > 0 && out[n-1].Mouth == v.Mouth && math.Abs(out[n-1].End-v.Start) < 1e-7 {
			out[n-1].End = v.End
		} else {
			out = append(out, v)
		}
	}
	return out
}
