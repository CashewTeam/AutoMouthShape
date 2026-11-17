package core_test

import (
	"automouthshape/internal/core"
	"automouthshape/internal/demo"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
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
	if e := core.ValidateMouthImages(d.Mouths); e == nil {
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

func TestJPEGMouthsFolderProjectAndExport(t *testing.T) {
	d, _ := loadDemo(t)
	dir := t.TempDir()
	for i, m := range core.Mouths {
		asset := d.Mouths[m]
		name := string(m) + ".png"
		if i%2 == 0 {
			img, _, err := image.Decode(bytes.NewReader(asset.Data))
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if err := jpeg.Encode(&buf, img, nil); err != nil {
				t.Fatal(err)
			}
			asset.Data = buf.Bytes()
			name = string(m) + ".JpEg"
		}
		if err := os.WriteFile(filepath.Join(dir, name), asset.Data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	mouths, err := core.LoadMouthFolder(dir)
	if err != nil {
		t.Fatal(err)
	}
	d.Mouths = mouths
	path := filepath.Join(dir, "mixed.amsproj")
	if err := core.SaveProject(path, d); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := core.OpenProject(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range core.Mouths {
		if !bytes.Equal(d.Mouths[m].Data, loaded.Mouths[m].Data) {
			t.Fatalf("mouth %s changed", m)
		}
	}
	loaded.Timeline = []core.Segment{{Start: 0, End: loaded.Duration, Mouth: core.A}}
	out, err := core.Export(context.Background(), dir, loaded, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(out, "frame_000001.png"))
	if err != nil {
		t.Fatal(err)
	}
	got, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format != "png" {
		t.Fatalf("export format %q: %v", format, err)
	}
	want, _, err := image.Decode(bytes.NewReader(loaded.Mouths[core.A].Data))
	if err != nil {
		t.Fatal(err)
	}
	if got.Bounds() != want.Bounds() {
		t.Fatal("export dimensions changed")
	}
	for y := 0; y < got.Bounds().Dy(); y++ {
		for x := 0; x < got.Bounds().Dx(); x++ {
			if got.At(x, y) != want.At(x, y) {
				r1, g1, b1, a1 := got.At(x, y).RGBA()
				r2, g2, b2, a2 := want.At(x, y).RGBA()
				if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
					t.Fatalf("pixel changed at %d,%d", x, y)
				}
			}
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "a.jpg"), loaded.Mouths[core.A].Data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := core.LoadMouthFolder(dir); err == nil {
		t.Fatal("duplicate formats accepted")
	}
}

func TestSRTEmptyCues(t *testing.T) {
	input := "1\r\n00:00:00,000 --> 00:00:00,500\r\n\r\n2\r\n00:00:00,500 --> 00:00:01,000\r\n你好\r\n\r\n3\r\n00:00:01,000 --> 00:00:02,000\r\n   \r\n\r\n4\r\n00:00:02,000 --> 00:00:03,000\r\n世界\r\n\r\n5\r\n00:00:03,000 --> 00:00:04,000\r\n"
	cues, err := core.ParseSRT([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(cues) != 2 || cues[0].Index != 2 || cues[0].Text != "你好" || cues[1].Index != 4 || cues[1].Start != 2 {
		t.Fatalf("unexpected cues: %+v", cues)
	}
	for _, input := range []string{
		"1\n00:00:00,000 --> 00:00:01,000\n",
		"1\n00:00:01,000 --> 00:00:00,000\n",
		"1\n00:00:00,000 --> 00:00:01,000\n00:00:00,000 --> 00:00:00,500\n",
	} {
		if _, err := core.ParseSRT([]byte(input)); err == nil {
			t.Fatalf("accepted invalid subtitles %q", input)
		}
	}
}

func TestDifferentMouthCanvases(t *testing.T) {
	d, _ := loadDemo(t)
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, 96, 64)), nil); err != nil {
		t.Fatal(err)
	}
	d.Mouths[core.A] = core.Asset{Name: "new.jpg", Data: b.Bytes()}
	path := filepath.Join(t.TempDir(), "unfinished.amsproj")
	if err := core.SaveProject(path, d); err != nil {
		t.Fatal(err)
	}
	loaded, _, err := core.OpenProject(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(loaded.Mouths[core.A].Data, b.Bytes()) {
		t.Fatal("asset changed")
	}
	folder := t.TempDir()
	for _, m := range core.Mouths {
		if err := os.WriteFile(filepath.Join(folder, loaded.Mouths[m].Name), loaded.Mouths[m].Data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := core.LoadMouthFolder(folder); err != nil {
		t.Fatal(err)
	}
	loaded.Timeline = []core.Segment{{Start: 0, End: loaded.Duration / 2, Mouth: core.A}, {Start: loaded.Duration / 2, End: loaded.Duration, Mouth: core.O}}
	out, err := core.Export(context.Background(), t.TempDir(), loaded, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, frame := range []struct {
		name          string
		width, height int
	}{{"frame_000001.png", 96, 64}, {fmt.Sprintf("frame_%06d.png", int(math.Ceil(loaded.Duration*float64(loaded.FPS)))), 512, 512}} {
		data, err := os.ReadFile(filepath.Join(out, frame.name))
		if err != nil {
			t.Fatal(err)
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		if img.Bounds().Dx() != frame.width || img.Bounds().Dy() != frame.height {
			t.Fatalf("%s dimensions changed: %v", frame.name, img.Bounds())
		}
	}
	loaded.FPS = 24
	if err := core.SaveProject(path, loaded); err != nil {
		t.Fatal(err)
	}
	loaded, _, err = core.OpenProject(path)
	if err != nil || loaded.FPS != 24 {
		t.Fatalf("overwrite failed: %v", err)
	}
}
