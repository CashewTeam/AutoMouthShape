package desktop

import (
	"github.com/egoist/mygo/ui"
	"math"
	"path/filepath"
	"testing"
)

func navigationApp(t *testing.T) *App {
	t.Helper()
	root, err := filepath.Abs("../../resources/runtime")
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Player.Close)
	return a
}

func TestTransportGeometryStaysFixed(t *testing.T) {
	a := navigationApp(t)
	a.Doc.Duration = 3661.999
	tt := ui.NewTester(a.View, 1440, 940)
	labels := []string{"前一音", "播放", "后一音", "播放时间码"}
	initial := map[string]ui.Rect{}
	for _, label := range labels {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatal("missing", label)
		}
		initial[label] = r
	}
	for _, position := range []float64{.111, .888, 59.999, 60, 3599.999, 3600} {
		a.Position = position
		tt.Frame()
		for _, label := range labels {
			r, ok := tt.Find(label)
			if !ok || r != initial[label] {
				t.Fatalf("%s moved at %.3f: %+v, want %+v", label, position, r, initial[label])
			}
		}
	}
}

func TestTimelineWheelPinchAndKeyboard(t *testing.T) {
	a := navigationApp(t)
	a.Position = .1
	var viewContext *ui.Context
	tt := ui.NewTester(func(c *ui.Context) { viewContext = c; a.View(c) }, 1440, 940)
	box, ok := tt.Find("口型编辑时间轴")
	if !ok {
		t.Fatal("missing timeline")
	}
	y := box.Y + 55
	anchorAt := func(x float32) float64 { return a.Pan + float64((x-box.X)/box.W)*a.span() }
	assertAnchor := func(x float32, want float64) {
		t.Helper()
		if got := anchorAt(x); math.Abs(got-want) > 1e-5 {
			t.Fatalf("cursor time moved: got %.9f, want %.9f", got, want)
		}
	}
	for _, fraction := range []float32{.2, .5, .8} {
		a.Zoom = 1
		a.Pan = 0
		tt.Frame()
		x := box.X + fraction*box.W
		tt.Move(x, y)
		anchor := anchorAt(x)
		tt.Pinch(x, y, 2)
		if a.Zoom != 2 {
			t.Fatal("pinch did not zoom", a.Zoom)
		}
		assertAnchor(x, anchor)
		// MyGo's Tester.Wheel omits modifiers even after HoldModifiers.
		// Deliver the complete wheel event through the registered app handler.
		a.timelineInput(viewContext, ui.InputEvent{Kind: ui.InputScroll, X: x - box.X, Y: 55, DY: -40, Mods: ui.Ctrl})
		tt.Frame()
		if a.Zoom <= 2 {
			t.Fatal("Ctrl+wheel did not zoom", a.Zoom)
		}
		assertAnchor(x, anchor)
		a.timelineInput(viewContext, ui.InputEvent{Kind: ui.InputScroll, X: x - box.X, Y: 55, DY: 40, Mods: ui.Cmd})
		tt.Frame()
		assertAnchor(x, anchor)
		if a.Position != .1 {
			t.Fatal("zoom moved playhead", a.Position)
		}
	}
	a.Zoom = 4
	a.Pan = a.Doc.Duration * .3
	tt.Frame()
	x := box.X + .6*box.W
	tt.Move(x, y)
	pan := a.Pan
	tt.Wheel(x, y, 0, 40)
	if a.Pan <= pan {
		t.Fatal("wheel did not pan")
	}
	tt.Wheel(x, y, 0, -40)
	if math.Abs(a.Pan-pan) > 1e-8 {
		t.Fatal("wheel reverse did not return", a.Pan, pan)
	}
	tt.Scroll(x, y, 60, 0)
	if a.Pan <= pan {
		t.Fatal("touchpad did not pan")
	}
	anchor := anchorAt(x)
	if err := tt.Click("+"); err != nil {
		t.Fatal(err)
	}
	assertAnchor(x, anchor)
	tt.Move(x, y)
	anchor = anchorAt(x)
	zoom := a.Zoom
	tt.Key(0, ui.KeyEqual)
	if a.Zoom <= zoom {
		t.Fatal("keyboard zoom in")
	}
	assertAnchor(x, anchor)
	tt.Key(0, ui.KeyMinus)
	assertAnchor(x, anchor)
	pan = a.Pan
	tt.Key(ui.Shift, ui.KeyRight)
	if a.Pan <= pan {
		t.Fatal("Shift+right did not pan")
	}
	tt.Key(ui.Shift, ui.KeyLeft)
	if math.Abs(a.Pan-pan) > 1e-8 {
		t.Fatal("Shift+left did not return")
	}
	if a.Position != .1 {
		t.Fatal("scroll shortcut moved playhead", a.Position)
	}
	tt.Scroll(x, y, 100000, 0)
	if a.Pan != a.Doc.Duration-a.span() {
		t.Fatal("right boundary", a.Pan)
	}
	tt.Scroll(x, y, -100000, 0)
	if a.Pan != 0 {
		t.Fatal("left boundary", a.Pan)
	}
	tt.Pinch(x, y, 1000)
	if a.Zoom != 64 {
		t.Fatal("maximum zoom", a.Zoom)
	}
	tt.Pinch(x, y, .000001)
	if a.Zoom != 1 || a.Pan != 0 {
		t.Fatal("zoom to fit", a.Zoom, a.Pan)
	}
}
