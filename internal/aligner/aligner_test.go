package aligner

import (
	"automouthshape/internal/core"
	"automouthshape/internal/demo"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPinyinMouths(t *testing.T) {
	for p, want := range map[string]core.Mouth{"hao": core.A, "wo": core.O, "men": core.E, "ni": core.I, "lu": core.U, "lv": core.V, "ju": core.V, "yu": core.V, "liu": core.U, "gui": core.I} {
		if got := MouthFor(p); got != want {
			t.Fatalf("%s -> %s != %s", p, got, want)
		}
	}
	if got := plainPinyin("lǜ"); got != "lv" {
		t.Fatal(got)
	}
}
func TestChineseIntegration(t *testing.T) {
	if os.Getenv("AMS_INTEGRATION") == "" {
		t.Skip("set AMS_INTEGRATION=1 to run the bundled binary and model")
	}
	root, e := filepath.Abs("../../resources/runtime")
	if e != nil {
		t.Fatal(e)
	}
	engine, e := New(root)
	if e != nil {
		t.Fatal(e)
	}
	for _, traditional := range []bool{false, true} {
		d, p, e := demo.LoadDemo()
		if e != nil {
			t.Fatal(e)
		}
		if traditional {
			r := strings.NewReplacer("欢", "歡", "们", "們", "测", "測", "试", "試", "语", "語")
			for i := range d.Cues {
				d.Cues[i].Text = r.Replace(d.Cues[i].Text)
			}
		}
		chars, timeline, e := engine.Analyze(context.Background(), d, p, nil)
		if e != nil {
			t.Fatal(e)
		}
		if len(chars) != 20 {
			t.Fatalf("characters %d", len(chars))
		}
		prev := -1.0
		for _, ch := range chars {
			if ch.Start <= prev || ch.End <= ch.Start {
				t.Fatalf("bad character %+v", ch)
			}
			prev = ch.Start
		}
		if e = core.ValidateTimeline(timeline, p.Duration()); e != nil {
			t.Fatal(e)
		}
	}
	for _, fixture := range []string{"repeated-slow.wav", "repeated-fast.wav"} {
		data, err := os.ReadFile(filepath.Join("testdata", fixture))
		if err != nil {
			t.Fatal(err)
		}
		pcm, err := core.Decode(data)
		if err != nil {
			t.Fatal(err)
		}
		d := core.NewDocument()
		d.Duration = pcm.Duration()
		d.Cues = []core.Cue{{Index: 1, Start: 0, End: d.Duration, Text: "你好你好。妈妈慢慢说话。我们一起测试。"}}
		chars, timeline, err := engine.Analyze(context.Background(), d, pcm, nil)
		if err != nil {
			t.Fatalf("%s: %v", fixture, err)
		}
		if len(chars) != 16 {
			t.Fatalf("%s: %d characters", fixture, len(chars))
		}
		if err = core.ValidateTimeline(timeline, d.Duration); err != nil {
			t.Fatal(err)
		}
	}
	d, p, e := demo.LoadDemo()
	if e != nil {
		t.Fatal(e)
	}
	longDoc := core.NewDocument()
	longPCM := &core.PCM{}
	for repeat := 0; repeat < 7; repeat++ {
		offset := float64(len(longPCM.Samples)) / core.SampleRate
		longPCM.Samples = append(longPCM.Samples, p.Samples...)
		for _, cue := range d.Cues {
			cue.Index = len(longDoc.Cues) + 1
			cue.Start += offset
			cue.End += offset
			longDoc.Cues = append(longDoc.Cues, cue)
		}
	}
	longDoc.Duration = longPCM.Duration()
	characters, timeline, err := engine.Analyze(context.Background(), longDoc, longPCM, nil)
	if err != nil {
		t.Fatalf("30 second batch boundary: %v", err)
	}
	if len(characters) != 140 {
		t.Fatalf("batch character count: %d", len(characters))
	}
	if err = core.ValidateTimeline(timeline, longPCM.Duration()); err != nil {
		t.Fatal(err)
	}
	mismatch := *d
	mismatch.Cues = []core.Cue{{Index: 1, Start: 0, End: p.Duration(), Text: "苹果香蕉葡萄西瓜今天下雨空气清新"}}
	if _, _, err = engine.Analyze(context.Background(), &mismatch, p, nil); err == nil {
		t.Fatal("mismatched transcript accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, e = engine.Analyze(ctx, d, p, nil); !errors.Is(e, context.Canceled) {
		t.Fatalf("cancel %v", e)
	}
	silent := &core.PCM{Samples: make([]float32, len(p.Samples))}
	if _, _, e = engine.Analyze(context.Background(), d, silent, nil); e == nil {
		t.Fatal("silent input accepted")
	}
	d.Cues[0].Text = "123"
	if _, _, e = engine.Analyze(context.Background(), d, p, nil); e == nil {
		t.Fatal("unsupported input accepted")
	}
	engine.Binary = filepath.Join(root, "missing-binary")
	d.Cues[0].Text = "你好"
	if _, _, e = engine.Analyze(context.Background(), d, p, nil); e == nil {
		t.Fatal("process failure accepted")
	}
}
