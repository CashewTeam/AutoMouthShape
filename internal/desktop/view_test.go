package desktop

import (
	"automouthshape/internal/core"
	"github.com/egoist/mygo/ui"
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
	if e = tt.Click("后一帧"); e != nil {
		t.Fatal(e)
	}
	if a.Position != 1.0/30 {
		t.Fatal("frame step", a.Position)
	}
	a.Selected = 1
	tt.Key(0, ui.KeyRight)
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
