package desktop

import (
	"automouthshape/internal/core"
	"bytes"
	"github.com/egoist/mygo/ui"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"path/filepath"
	"testing"
)

func TestTimelineEditingUI(t *testing.T) {
	root, e := filepath.Abs("../../resources/runtime")
	if e != nil {
		t.Fatal(e)
	}
	a, e := New(root)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Player.Close()
	tt := ui.NewTester(a.View, 1440, 940)
	if !tt.HasText("口型时间轴") {
		t.Fatal("timeline missing")
	}
	chars := a.Doc.Characters
	for _, tc := range []struct {
		button     string
		from, want float64
	}{
		{"后一音", 0, chars[0].Start},
		{"后一音", chars[0].Start, chars[1].Start},
		{"前一音", chars[1].Start, chars[0].Start},
		{"前一音", (chars[0].Start + chars[1].Start) / 2, chars[0].Start},
		{"后一音", (chars[0].Start + chars[1].Start) / 2, chars[1].Start},
		{"前一音", 0, 0},
		{"后一音", chars[len(chars)-1].Start, chars[len(chars)-1].Start},
		{"前一音", a.Doc.Duration, chars[len(chars)-1].Start},
	} {
		a.seek(tc.from)
		tt.Frame()
		if e = tt.Click(tc.button); e != nil {
			t.Fatal(e)
		}
		if a.Position != tc.want {
			t.Fatalf("%s from %v: got %v, want %v", tc.button, tc.from, a.Position, tc.want)
		}
	}
	a.Doc.Characters = nil
	a.seek(0)
	tt.Frame()
	for _, button := range []string{"前一音", "后一音"} {
		if e = tt.Click(button); e != nil {
			t.Fatal(e)
		}
		if a.Position != 0 {
			t.Fatal("navigation without alignment changed position")
		}
	}
	a.Doc.Characters = chars
	a.seek(0)
	a.Selected = 1
	tt.Key(0, ui.KeyRight)
	if a.Position != 1.0/30 {
		t.Fatal("keyboard frame step", a.Position)
	}
	if e = tt.Click("替换为 A"); e != nil {
		t.Fatal(e)
	}
	if a.Doc.Timeline[1].Mouth != core.A {
		t.Fatal("mouth edit")
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	// Drag the boundary of a real mouth interval and undo the whole gesture.
	box, ok := tt.Find("口型编辑时间轴")
	if !ok {
		t.Fatal("timeline canvas missing")
	}
	before := a.Doc.Timeline[1].End
	x := box.X + float32(before/a.Doc.Duration)*box.W
	y := box.Y + 150
	tt.Press(x, y)
	tt.Move(x+18, y)
	tt.Release(x+18, y)
	if a.Doc.Timeline[1].End == before {
		t.Fatal("boundary drag did not edit timeline")
	}
	if err := core.ValidateTimeline(a.Doc.Timeline, a.Doc.Duration); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyZ)
	if a.Doc.Timeline[1].End != before {
		t.Fatal("drag undo did not restore boundary")
	}
	if !tt.HasText("区间编辑") {
		t.Fatal("inspector missing")
	}
}

func TestAssignMouthUpdatesPreviewsWithDifferentCanvas(t *testing.T) {
	root, err := filepath.Abs("../../resources/runtime")
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Player.Close()
	a.Doc.Timeline = []core.Segment{{Start: 0, End: a.Doc.Duration, Mouth: core.A}}
	tt := ui.NewTester(a.View, 1440, 940)
	before := tt.Image()
	img := image.NewRGBA(image.Rect(0, 0, 96, 64))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{R: 255, A: 255}}, image.Point{}, draw.Src)
	var data bytes.Buffer
	if err := jpeg.Encode(&data, img, nil); err != nil {
		t.Fatal(err)
	}
	asset := core.Asset{Name: "custom.jpg", Data: data.Bytes()}
	if err := a.assignMouth(core.A, asset); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	after := tt.Image()
	// Both the sidebar thumbnail and main preview must paint the new red image.
	for _, region := range []image.Rectangle{image.Rect(0, 0, 228, 700), image.Rect(250, 150, 1150, 600)} {
		changed := false
		for y := region.Min.Y; y < region.Max.Y; y++ {
			for x := region.Min.X; x < region.Max.X; x++ {
				r, g, b, _ := after.At(x, y).RGBA()
				if r > 50000 && g < 10000 && b < 10000 && before.At(x, y) != after.At(x, y) {
					changed = true
				}
			}
		}
		if !changed {
			t.Fatalf("preview did not update in %v", region)
		}
	}
	old := a.Images[core.A]
	if err := a.assignMouth(core.A, core.Asset{Name: "bad.jpg", Data: []byte("invalid")}); err == nil {
		t.Fatal("invalid image accepted")
	}
	if a.Images[core.A] != old || a.Doc.Mouths[core.A].Name != asset.Name {
		t.Fatal("failed import changed asset")
	}
}

func TestSavedProjectState(t *testing.T) {
	a := &App{Doc: core.NewDocument(), Dirty: true, Edited: true, LastError: "old failure"}
	path := filepath.Join(t.TempDir(), "我的工程.amsproj")
	a.savedProject(path)
	if a.Doc.Name != "我的工程" || a.ProjectPath != path || a.Dirty || a.Edited || a.LastError != "" {
		t.Fatalf("incorrect saved state: %+v", a)
	}
	a.Dirty = true
	a.savedProject(path)
	if a.ProjectPath != path || a.Dirty {
		t.Fatal("save lost current project")
	}
}

func TestProjectFileExtension(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"工程", "工程.amsproj"},
		{"工程.amsproj", "工程.amsproj"},
		{"工程.AMSPROJ", "工程.AMSPROJ"},
		{"工程.amsproj.amsproj", "工程.amsproj.amsproj"},
		{"工程.v2", "工程.v2.amsproj"},
	} {
		path := filepath.Join(t.TempDir(), tc.input)
		want := filepath.Join(filepath.Dir(path), tc.want)
		if got := projectFilePath(path); got != want {
			t.Fatalf("%s: got %s want %s", path, got, want)
		}
		if got := projectFilePath(want); got != want {
			t.Fatalf("filename changed on repeat: %s", got)
		}
	}
}
