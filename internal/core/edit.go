package core

import (
	"fmt"
	"math"
	"slices"
)

type History struct{ undo, redo [][]Segment }

func (h *History) Commit(d *Document, next []Segment) error {
	if e := ValidateTimeline(next, d.Duration); e != nil {
		return e
	}
	if slices.Equal(d.Timeline, next) {
		return nil
	}
	h.undo = append(h.undo, slices.Clone(d.Timeline))
	if len(h.undo) > 100 {
		h.undo = h.undo[1:]
	}
	h.redo = nil
	d.Timeline = slices.Clone(next)
	return nil
}
func (h *History) Undo(d *Document) bool {
	if len(h.undo) == 0 {
		return false
	}
	h.redo = append(h.redo, slices.Clone(d.Timeline))
	d.Timeline = h.undo[len(h.undo)-1]
	h.undo = h.undo[:len(h.undo)-1]
	return true
}
func (h *History) Redo(d *Document) bool {
	if len(h.redo) == 0 {
		return false
	}
	h.undo = append(h.undo, slices.Clone(d.Timeline))
	d.Timeline = h.redo[len(h.redo)-1]
	h.redo = h.redo[:len(h.redo)-1]
	return true
}
func Snap(t float64, fps int) float64 { return math.Round(t*float64(fps)) / float64(fps) }
func Split(d *Document, index int, t float64) ([]Segment, error) {
	if index < 0 || index >= len(d.Timeline) {
		return nil, fmt.Errorf("请先选择口型区间")
	}
	t = Snap(t, d.FPS)
	v := d.Timeline[index]
	if t <= v.Start+1e-8 || t >= v.End-1e-8 {
		return nil, fmt.Errorf("播放头须位于选中区间内部")
	}
	out := slices.Clone(d.Timeline)
	out = append(out[:index], append([]Segment{{v.Start, t, v.Mouth}, {t, v.End, v.Mouth}}, out[index+1:]...)...)
	return out, nil
}
func Merge(d *Document, index int) ([]Segment, error) {
	if index < 0 || index+1 >= len(d.Timeline) {
		return nil, fmt.Errorf("请选择有右侧邻居的区间")
	}
	out := slices.Clone(d.Timeline)
	out[index].End = out[index+1].End
	out = append(out[:index+1], out[index+2:]...)
	return out, nil
}
func Boundary(d *Document, index int, left bool, t float64) []Segment {
	out := slices.Clone(d.Timeline)
	if index < 0 || index >= len(out) {
		return out
	}
	step := 1 / float64(d.FPS)
	t = Snap(t, d.FPS)
	v := out[index]
	if left {
		lo := 0.0
		if index > 0 {
			lo = out[index-1].End
			if math.Abs(lo-v.Start) < 1e-7 {
				lo = out[index-1].Start + step
			}
		}
		hi := v.End - step
		if lo > hi {
			return out
		}
		t = max(lo, min(t, hi))
		out[index].Start = t
		if index > 0 && math.Abs(out[index-1].End-v.Start) < 1e-7 {
			out[index-1].End = t
		}
	} else {
		hi := d.Duration
		if index+1 < len(out) {
			hi = out[index+1].Start
			if math.Abs(hi-v.End) < 1e-7 {
				hi = out[index+1].End - step
			}
		}
		lo := v.Start + step
		if lo > hi {
			return out
		}
		t = max(lo, min(t, hi))
		out[index].End = t
		if index+1 < len(out) && math.Abs(out[index+1].Start-v.End) < 1e-7 {
			out[index+1].Start = t
		}
	}
	return out
}
