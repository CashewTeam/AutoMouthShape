package core_test

import (
	"automouthshape/internal/core"
	"automouthshape/internal/demo"
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func loadDemo(t *testing.T) (*core.Document, *core.PCM) {
	t.Helper()
	d, p, e := demo.LoadDemo()
	if e != nil {
		t.Fatal(e)
	}
	return d, p
}
func TestSRTValidation(t *testing.T) {
	for _, s := range []string{"", "1\n00:00:02,000 --> 00:00:01,000\n坏时间", "1\n00:00:00,000 --> 00:00:01,000\n甲\n\n2\n00:00:00,500 --> 00:00:01,500\n乙", string([]byte{0xff})} {
		if _, e := core.ParseSRT([]byte(s)); e == nil {
			t.Fatalf("accepted invalid SRT %q", s)
		}
	}
	c, e := core.ParseSRT([]byte("\ufeff1\r\n00:00:00,050 --> 00:00:01,100\r\n你好\r\n世界\r\n"))
	if e != nil || len(c) != 1 || c[0].Text != "你好 世界" || c[0].Start != .05 {
		t.Fatalf("SRT %+v %v", c, e)
	}
}
func TestSilenceMinimumAndOverlay(t *testing.T) {
	p := &core.PCM{Samples: make([]float32, 16000)}
	for i := range p.Samples {
		p.Samples[i] = .3
	}
	for i := 1600; i < 2720; i++ {
		p.Samples[i] = 0
	}
	for i := 4800; i < 6400; i++ {
		p.Samples[i] = 0
	}
	sil := core.Silence(p, -40)
	if len(sil) != 1 || sil[0][0] != .3 || sil[0][1] != .4 {
		t.Fatalf("silence %+v", sil)
	}
	s := core.ApplySilence([]core.Segment{{Start: 0, End: 1, Mouth: core.A}}, sil)
	for _, v := range []struct {
		t float64
		m core.Mouth
	}{{.299, core.A}, {.3, core.Closed}, {.399, core.Closed}, {.4, core.A}} {
		if got := core.MouthAt(s, v.t); got != v.m {
			t.Fatalf("at %v: %s", v.t, got)
		}
	}
}
func TestEditingAndHistory(t *testing.T) {
	d := core.NewDocument()
	d.Duration = 2
	d.Timeline = []core.Segment{{Start: 0, End: 1, Mouth: core.A}, {Start: 1, End: 2, Mouth: core.O}}
	initial := slices.Clone(d.Timeline)
	h := &core.History{}
	next := core.Boundary(d, 0, false, 1.4)
	if next[0].End != 1.4 || next[1].Start != 1.4 {
		t.Fatalf("boundary %+v", next)
	}
	if e := h.Commit(d, next); e != nil {
		t.Fatal(e)
	}
	if !h.Undo(d) || !slices.Equal(initial, d.Timeline) {
		t.Fatal("undo")
	}
	if !h.Redo(d) || !slices.Equal(next, d.Timeline) {
		t.Fatal("redo")
	}
	cut, e := core.Split(d, 0, .5)
	if e != nil || len(cut) != 3 {
		t.Fatalf("split %+v %v", cut, e)
	}
	if e = h.Commit(d, cut); e != nil {
		t.Fatal(e)
	}
	merged, e := core.Merge(d, 0)
	if e != nil || len(merged) != 2 || merged[0].End != 1.4 {
		t.Fatalf("merge %+v %v", merged, e)
	}
	if e = core.ValidateTimeline(core.Boundary(d, 1, true, 10), 2); e != nil {
		t.Fatal(e)
	}
}
func TestPortableProjectAndExport(t *testing.T) {
	d, p := loadDemo(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "中文 工程.amsproj")
	if e := core.SaveProject(path, d); e != nil {
		t.Fatal(e)
	}
	moved := filepath.Join(dir, "移动后")
	if e := os.Mkdir(moved, 0755); e != nil {
		t.Fatal(e)
	}
	newPath := filepath.Join(moved, "工程.amsproj")
	if e := os.Rename(path, newPath); e != nil {
		t.Fatal(e)
	}
	loaded, decoded, e := core.OpenProject(newPath)
	if e != nil {
		t.Fatal(e)
	}
	if loaded.Duration != p.Duration() || decoded.Duration() != p.Duration() || !slices.Equal(d.Timeline, loaded.Timeline) {
		t.Fatal("portable roundtrip changed timeline/audio")
	}
	for _, m := range core.Mouths {
		if !bytes.Equal(d.Mouths[m].Data, loaded.Mouths[m].Data) {
			t.Fatal("mouth changed", m)
		}
	}
	for _, fps := range []int{24, 25, 30, 60} {
		loaded.FPS = fps
		out, e := core.Export(context.Background(), dir, loaded, nil)
		if e != nil {
			t.Fatal(e)
		}
		entries, e := os.ReadDir(out)
		if e != nil {
			t.Fatal(e)
		}
		count := int(math.Ceil(loaded.Duration * float64(fps)))
		if len(entries) != count {
			t.Fatalf("fps %d: %d != %d", fps, len(entries), count)
		}
		for i, entry := range entries {
			b, e := os.ReadFile(filepath.Join(out, entry.Name()))
			if e != nil {
				t.Fatal(e)
			}
			want := loaded.Mouths[core.MouthAt(loaded.Timeline, float64(i)/float64(fps))].Data
			if !bytes.Equal(b, want) {
				t.Fatalf("preview/export mismatch frame %d", i)
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, e := core.Export(ctx, dir, d, nil)
	if !errors.Is(e, context.Canceled) {
		t.Fatalf("cancel: %v", e)
	}
	if _, e = os.Stat(filepath.Join(out, "INCOMPLETE.txt")); e != nil {
		t.Fatal("missing incomplete marker", e)
	}
}
func TestInvalidAssetsAndPCM(t *testing.T) {
	d, p := loadDemo(t)
	delete(d.Mouths, core.V)
	if _, _, e := core.ValidateMouths(d.Mouths); e == nil {
		t.Fatal("missing mouth accepted")
	}
	decoded, e := core.Decode(p.WAV(0, p.Duration()))
	if e != nil {
		t.Fatal(e)
	}
	if math.Abs(decoded.Duration()-p.Duration()) > 1.0/core.SampleRate {
		t.Fatal("PCM roundtrip duration")
	}
	if _, e = core.Decode([]byte("invalid")); e == nil {
		t.Fatal("bad audio accepted")
	}
}

func TestMP3Decode(t *testing.T) {
	data, e := os.ReadFile("testdata/example.mp3")
	if e != nil {
		t.Fatal(e)
	}
	pcm, e := core.Decode(data)
	if e != nil {
		t.Fatal(e)
	}
	if pcm.Duration() < 5 || pcm.Duration() > 5.5 {
		t.Fatalf("MP3 duration %.3f", pcm.Duration())
	}
	var peak float32
	for _, v := range pcm.Samples {
		peak = max(peak, float32(math.Abs(float64(v))))
	}
	if peak < .05 {
		t.Fatal("MP3 decoded to silence")
	}
}
