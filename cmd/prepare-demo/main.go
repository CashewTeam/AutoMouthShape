// prepare-demo regenerates the geometric sample mouths and real model alignment.
package main

import (
	"automouthshape/internal/aligner"
	"automouthshape/internal/core"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

func main() {
	icon := image.NewNRGBA(image.Rect(0, 0, 1024, 1024))
	for y := 0; y < 1024; y++ {
		for x := 0; x < 1024; x++ {
			dx := math.Max(math.Abs(float64(x)-512)-300, 0)
			dy := math.Max(math.Abs(float64(y)-512)-300, 0)
			if dx*dx+dy*dy < 150*150 {
				icon.SetNRGBA(x, y, color.NRGBA{R: 43, G: 91, B: 221, A: 255})
			}
		}
	}
	for i, h := range []int{110, 210, 300, 180, 95} {
		for x := 282 + i*96; x < 330+i*96; x++ {
			for y := 512 - h/2; y < 512+h/2; y++ {
				icon.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
	}
	iconFile, err := os.Create("resources/icon.png")
	must(err)
	must(png.Encode(iconFile, icon))
	must(iconFile.Close())
	root, e := filepath.Abs("resources/runtime")
	must(e)
	engine, e := aligner.New(root)
	must(e)
	for n, m := range core.Mouths {
		img := image.NewNRGBA(image.Rect(0, 0, 512, 512))
		rx, ry := float64(94), float64(68)
		switch m {
		case core.O:
			rx, ry = 57, 77
		case core.E:
			rx, ry = 105, 37
		case core.I:
			rx, ry = 111, 23
		case core.U:
			rx, ry = 43, 46
		case core.V:
			rx, ry = 49, 57
		case core.Closed:
			rx, ry = 101, 5
		}
		for y := range 512 {
			for x := range 512 {
				dx, dy := float64(x)-256, float64(y)-268
				d := dx*dx/(rx*rx) + dy*dy/(ry*ry)
				if d < 1.15 {
					c := color.NRGBA{R: 100, G: 42, B: 55, A: 255}
					if d < .82 {
						c = color.NRGBA{R: 49, G: 25, B: 42, A: 255}
						if m != core.Closed && dy > ry*.33 && math.Abs(dx) < rx*.65 {
							c = color.NRGBA{R: 222, G: 111, B: 132, A: 255}
						}
						if (m == core.I || m == core.E) && dy < -ry*.2 && dy > -ry*.68 {
							c = color.NRGBA{R: 255, G: 243, B: 226, A: 255}
						}
					}
					img.SetNRGBA(x, y, c)
				}
			}
		}
		f, e := os.Create("internal/demo/data/mouths/" + string(m) + ".png")
		must(e)
		must(png.Encode(f, img))
		must(f.Close())
		log.Printf("mouth %d: %s", n, m)
	}
	d := core.NewDocument()
	d.Audio = read("internal/demo/data/example.wav")
	d.Subtitle = read("internal/demo/data/example.srt")
	d.Cues, e = core.ParseSRT(d.Subtitle.Data)
	must(e)
	pcm, e := core.Decode(d.Audio.Data)
	must(e)
	d.Duration = pcm.Duration()
	chars, timeline, e := engine.Analyze(context.Background(), d, pcm, func(p float64, s string) { log.Print(s) })
	must(e)
	b, e := json.MarshalIndent(struct {
		Characters []core.Character `json:"characters"`
		Timeline   []core.Segment   `json:"timeline"`
	}{chars, timeline}, "", "  ")
	must(e)
	must(os.WriteFile("internal/demo/data/analysis.json", b, 0644))
	log.Printf("%d characters, %d mouth segments, %.3fs", len(chars), len(timeline), d.Duration)
}
func must(e error) {
	if e != nil {
		log.Fatal(e)
	}
}
func read(p string) core.Asset {
	b, e := os.ReadFile(p)
	must(e)
	return core.Asset{Name: filepath.Base(p), Data: b}
}
