package desktop

import (
	"automouthshape/internal/core"
	"fmt"
	"github.com/egoist/mygo/audio"
	"github.com/egoist/mygo/ui"
	"math"
	"slices"
	"strconv"
)

var palette = map[core.Mouth]ui.Color{core.A: ui.RGB(221, 135, 96), core.O: ui.RGB(211, 170, 84), core.E: ui.RGB(118, 172, 128), core.I: ui.RGB(92, 167, 180), core.U: ui.RGB(126, 149, 209), core.V: ui.RGB(178, 135, 197), core.Closed: ui.RGB(105, 115, 130)}

func (a *App) View(c *ui.Context) {
	if a.Player.Playing() {
		a.Position = a.Player.Position().Seconds()
		c.AnimationFrame()
	}
	if e := audio.Err(); e != nil && a.LastError != e.Error() {
		a.fail(e)
	}
	if !a.Busy {
		if c.Shortcut(0, ui.KeySpace) {
			a.toggle()
		}
		if c.Shortcut(0, ui.KeyLeft) {
			a.Player.Pause()
			a.seek(core.Snap(a.Position, a.Doc.FPS) - 1/float64(a.Doc.FPS))
		}
		if c.Shortcut(0, ui.KeyRight) {
			a.Player.Pause()
			a.seek(core.Snap(a.Position, a.Doc.FPS) + 1/float64(a.Doc.FPS))
		}
		if c.Shortcut(ui.Cmd, ui.KeyS) {
			a.saveTo(a.ProjectPath, nil)
		}
		if c.Shortcut(ui.Cmd, ui.KeyZ) {
			a.undo()
		}
		if c.Shortcut(ui.Cmd|ui.Shift, ui.KeyZ) {
			a.redo()
		}
		if c.Shortcut(ui.Cmd, ui.KeyO) {
			a.open()
		}
	}
	t := c.Theme()
	ui.Column(c).Fill().Gap(0).Children(func() {
		ui.Row(c).FillWidth().Height(64).Padding(16, 22).Gap(14).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(30, 30).Radius(10).Background(t.Accent).Draw(func(p *ui.Painter, r ui.Rect) {
				p.Line(r.X+7, r.Y+15, r.X+12, r.Y+15, 3, t.AccentText)
				p.Line(r.X+12, r.Y+15, r.X+16, r.Y+22, 3, t.AccentText)
				p.Line(r.X+16, r.Y+22, r.X+23, r.Y+9, 3, t.AccentText)
			})
			ui.Column(c).Gap(1).Children(func() {
				ui.Text(c, "AutoMouthShape").FontSize(19).Bold()
				ui.Text(c, "中文口型工作台").FontSize(11).TextColor(t.TextMuted)
			})
			ui.Box(c).Grow(1)
			label := a.Doc.Name
			if a.Dirty {
				label += " · 未保存"
			}
			ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
			if ui.Button(c.Key("open"), "打开工程").Disabled(a.Busy).Clicked() {
				a.open()
			}
			if ui.Button(c.Key("save"), "保存工程").Disabled(a.Busy).Clicked() {
				a.saveTo(a.ProjectPath, nil)
			}
			if ui.PrimaryButton(c.Key("export"), "导出 PNG 序列").Disabled(a.Busy || len(a.Doc.Timeline) == 0).Clicked() {
				a.export()
			}
		})
		ui.Box(c).FillWidth().Height(1).Background(t.Border)
		ui.Row(c).FillWidth().Height(50).PaddingX(20).Gap(8).AlignItems(ui.Center).Children(func() {
			if ui.Button(c.Key("mouthfolder"), "口型文件夹").Disabled(a.Busy).Clicked() {
				a.importMouths()
			}
			if ui.Button(c.Key("audio"), "导入音频").Disabled(a.Busy).Clicked() {
				a.importAudio()
			}
			if ui.Button(c.Key("srt"), "导入字幕").Disabled(a.Busy).Clicked() {
				a.importSRT()
			}
			ui.Box(c).Width(10)
			if ui.Button(c.Key("analyze"), "重新分析").Disabled(a.Busy || a.Engine == nil).Clicked() {
				a.reanalyze()
			}
			if ui.Button(c.Key("restore"), "恢复示例").Disabled(a.Busy).Clicked() {
				a.restore()
			}
			ui.Box(c).Grow(1)
			ui.Text(c, "离线分析 · Omnilingual CTC").FontSize(11).TextColor(t.TextMuted)
		})
		ui.Box(c).FillWidth().Height(1).Background(t.Border)
		ui.Row(c).FillWidth().Grow(1).MinHeight(240).Children(func() {
			a.sidebar(c)
			ui.Box(c).FillHeight().Width(1).Background(t.Border)
			ui.Column(c).Grow(1).FillHeight().Padding(18).Gap(12).Children(func() {
				ui.Row(c).FillWidth().Children(func() {
					ui.Text(c, "口型预览").FontSize(12).Bold()
					ui.Box(c).Grow(1)
					ui.Text(c, "透明图层 · 原始画布").FontSize(11).TextColor(t.TextMuted)
				})
				ui.Box(c.Key("preview")).FillWidth().Grow(1).MinHeight(150).Radius(12).Clip().Draw(func(p *ui.Painter, r ui.Rect) {
					p.Fill(r, ui.RGB(225, 227, 231), 0)
					const tile = float32(20)
					for y := float32(0); y < r.H; y += tile {
						for x := float32(0); x < r.W; x += tile {
							if (int(x/tile)+int(y/tile))%2 == 0 {
								p.Fill(ui.Rect{X: r.X + x, Y: r.Y + y, W: min(tile, r.W-x), H: min(tile, r.H-y)}, ui.RGB(241, 242, 244), 0)
							}
						}
					}
					im := a.Images[core.MouthAt(a.Doc.Timeline, a.Position)]
					if im != nil {
						size := min(r.W-32, r.H-32)
						p.Image(im, ui.Rect{X: r.X + (r.W-size)/2, Y: r.Y + (r.H-size)/2, W: size, H: size}, ui.Contain)
					}
				})
				current := ""
				for _, cue := range a.Doc.Cues {
					if a.Position >= cue.Start && a.Position < cue.End {
						current = cue.Text
						break
					}
				}
				if current == "" {
					current = "—"
				}
				ui.Text(c, current).FontSize(18).TextAlign(ui.Center).FillWidth().Height(26)
				ui.Row(c).Gap(8).AlignItems(ui.Center).Justify(ui.Center).FillWidth().Children(func() {
					if ui.Button(c.Key("previous"), "前一帧").Disabled(a.Busy).Clicked() {
						a.Player.Pause()
						a.seek(core.Snap(a.Position, a.Doc.FPS) - 1/float64(a.Doc.FPS))
					}
					play := "播放"
					if a.Player.Playing() {
						play = "暂停"
					}
					if ui.PrimaryButton(c.Key("play"), play).Width(88).Disabled(a.Busy || len(a.Doc.Timeline) == 0).Clicked() {
						a.toggle()
					}
					if ui.Button(c.Key("next"), "后一帧").Disabled(a.Busy).Clicked() {
						a.Player.Pause()
						a.seek(core.Snap(a.Position, a.Doc.FPS) + 1/float64(a.Doc.FPS))
					}
					ui.Text(c, clock(a.Position)+" / "+clock(a.Doc.Duration)).FontSize(12).TextColor(t.TextMuted)
				})
			})
			ui.Box(c).FillHeight().Width(1).Background(t.Border)
			a.inspector(c)
		})
		ui.Box(c).FillWidth().Height(1).Background(t.Border)
		a.timeline(c)
		ui.Box(c).FillWidth().Height(1).Background(t.Border)
		ui.Row(c).FillWidth().Height(36).PaddingX(20).Gap(10).AlignItems(ui.Center).Children(func() {
			status := a.Status
			if a.Busy {
				status = fmt.Sprintf("%s · %.0f%%", status, a.Progress*100)
			}
			ui.Text(c, status).FontSize(11).TextColor(t.TextMuted).Grow(1).SingleLine()
			if a.Busy && a.cancel != nil {
				if ui.Button(c.Key("cancel"), "取消任务").Clicked() {
					a.cancel()
				}
			}
			if a.LastError != "" {
				if ui.Button(c.Key("logs"), "查看日志").Clicked() {
					a.log()
				}
			}
			ui.Text(c, "v"+core.Version).FontSize(10).TextColor(t.TextMuted)
		})
	})
}
func clock(s float64) string { m := int(s) / 60; return fmt.Sprintf("%02d:%06.3f", m, s-float64(m*60)) }
func (a *App) sidebar(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Width(228).FillHeight().Children(func() {
		ui.Column(c).FillWidth().Padding(18).Gap(13).Children(func() {
			ui.Text(c, "输入资源").FontSize(12).Bold()
			ui.Text(c, "音频").FontSize(10).TextColor(t.TextMuted)
			ui.Text(c, a.Doc.Audio.Name).FontSize(12).Wrap()
			ui.Text(c, fmt.Sprintf("%.2f 秒 · WAV / MP3", a.Doc.Duration)).FontSize(10).TextColor(t.TextMuted)
			ui.Text(c, "字幕").FontSize(10).TextColor(t.TextMuted)
			ui.Text(c, a.Doc.Subtitle.Name).FontSize(12).Wrap()
			ui.Text(c, fmt.Sprintf("%d 条字幕 · %d 个已对齐汉字", len(a.Doc.Cues), len(a.Doc.Characters))).FontSize(10).TextColor(t.TextMuted)
			ui.Box(c).FillWidth().Height(1).Background(t.Border)
			ui.Text(c, "口型素材").FontSize(12).Bold()
			ui.Text(c, "点击右侧按钮指定 PNG").FontSize(10).TextColor(t.TextMuted)
			for _, m := range core.Mouths {
				ui.Row(c.Key("asset-" + string(m))).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(46, 38).Radius(5).Background(ui.RGB(238, 239, 242)).Draw(func(p *ui.Painter, r ui.Rect) {
						if im := a.Images[m]; im != nil {
							p.Image(im, r, ui.Contain)
						}
					})
					ui.Text(c, m.Label()).FontSize(12).Bold().Grow(1)
					ui.Box(c).Size(6, 6).Radius(3).Background(palette[m])
					if ui.Button(c.Key("assign-"+string(m)), "指定").FontSize(10).Disabled(a.Busy).Clicked() {
						a.importMouth(m)
					}
				})
			}
			ui.Box(c).Height(6)
			ui.Text(c, "七张 PNG 使用相同画布尺寸与口型位置，可避免切换时跳动。").FontSize(10).TextColor(t.TextMuted).Wrap()
		})
	})
}
func (a *App) inspector(c *ui.Context) {
	t := c.Theme()
	ui.Scroll(c).Width(248).FillHeight().Children(func() {
		ui.Column(c).FillWidth().Padding(18).Gap(12).Children(func() {
			ui.Text(c, "区间编辑").FontSize(12).Bold()
			if !a.selected() {
				ui.Text(c, "在下方时间轴选择一个口型区间").FontSize(12).TextColor(t.TextMuted).Wrap()
			} else {
				v := a.Doc.Timeline[a.Selected]
				ui.Text(c, fmt.Sprintf("#%03d · %s", a.Selected+1, v.Mouth.Label())).FontSize(20).Bold().TextColor(palette[v.Mouth])
				shown := 0
				for _, ch := range a.Doc.Characters {
					if ch.Start >= v.Start && ch.Start < v.End && shown < 3 {
						color := t.TextMuted
						if ch.Score < .2 {
							color = t.Warning
						}
						ui.Text(c, fmt.Sprintf("%s · %s · 分数 %.3f", ch.Text, ch.Pinyin, ch.Score)).FontSize(10).TextColor(color)
						shown++
					}
				}
				ui.Text(c, "起点 / 终点（秒）").FontSize(10).TextColor(t.TextMuted)
				start, end := v.Start, v.End
				if ui.NumberInput(c.Key("start"), &start, 0, a.Doc.Duration, 1/float64(a.Doc.FPS)).FillWidth().Disabled(a.Busy).Changed() {
					a.commit(core.Boundary(a.Doc, a.Selected, true, start))
				}
				if ui.NumberInput(c.Key("end"), &end, 0, a.Doc.Duration, 1/float64(a.Doc.FPS)).FillWidth().Disabled(a.Busy).Changed() {
					a.commit(core.Boundary(a.Doc, a.Selected, false, end))
				}
				ui.Text(c, "替换选中口型").FontSize(10).TextColor(t.TextMuted)
				ui.Row(c).Gap(4).Wrap().FillWidth().Children(func() {
					for _, m := range core.Mouths {
						b := ui.Button(c.Key("replace-"+string(m)), m.Label()).Label("替换为 " + m.Label()).MinWidth(40).Disabled(a.Busy)
						if v.Mouth == m {
							b.Border(1, palette[m])
						}
						if b.Clicked() {
							a.replace(m)
						}
					}
				})
				ui.Row(c).Gap(6).Children(func() {
					if ui.Button(c.Key("split"), "播放头拆分").Disabled(a.Busy).Clicked() {
						a.split()
					}
					if ui.Button(c.Key("merge"), "合并至右").Disabled(a.Busy || a.Selected+1 >= len(a.Doc.Timeline)).Clicked() {
						a.merge()
					}
				})
				if ui.Checkbox(c.Key("loop"), &a.Loop, "循环选中区间").Disabled(a.Busy).Changed() {
					a.Player.SetLoop(false)
				}
			}
			ui.Box(c).FillWidth().Height(1).Background(t.Border)
			ui.Text(c, "输出设置").FontSize(12).Bold()
			fps := strconv.Itoa(a.Doc.FPS)
			if ui.Select(c.Key("fps"), &fps, []string{"24", "25", "30", "60"}).FillWidth().Disabled(a.Busy).Changed() {
				a.Doc.FPS, _ = strconv.Atoi(fps)
				a.Dirty = true
			}
			ui.Text(c, "透明 PNG · 素材原尺寸").FontSize(10).TextColor(t.TextMuted)
			ui.Text(c, "闭口灵敏度").FontSize(12).Bold()
			db := a.Doc.SilenceDB
			if ui.Slider(c.Key("silence"), &db, -65, -20).FillWidth().Disabled(a.Busy).Changed() {
				a.Doc.SilenceDB = math.Round(db)
				a.Dirty = true
			}
			ui.Text(c, fmt.Sprintf("静音阈值 %.0f dBFS · 重新分析后应用", a.Doc.SilenceDB)).FontSize(10).TextColor(t.TextMuted).Wrap()
			ui.Box(c).FillWidth().Height(1).Background(t.Border)
			ui.Text(c, "快捷键").FontSize(12).Bold()
			ui.Text(c, "空格   播放 / 暂停\n← →   逐帧定位\n⌘ / Ctrl + Z   撤销\n⌘ / Ctrl + S   保存工程").FontSize(11).TextColor(t.TextMuted)
		})
	})
}
func (a *App) span() float64 { return max(.1, a.Doc.Duration/a.Zoom) }
func (a *App) timeline(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).FillWidth().Height(258).Gap(0).Children(func() {
		ui.Row(c).FillWidth().Height(40).PaddingX(18).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "口型时间轴").FontSize(12).Bold()
			ui.Text(c, "拖动区间边界调整 · 点击波形定位").FontSize(10).TextColor(t.TextMuted).Grow(1)
			if ui.Button(c.Key("undo"), "撤销").Disabled(a.Busy).Clicked() {
				a.undo()
			}
			if ui.Button(c.Key("redo"), "重做").Disabled(a.Busy).Clicked() {
				a.redo()
			}
			if ui.Button(c.Key("zoomout"), "−").Disabled(a.Zoom <= 1).Clicked() {
				a.Zoom = max(1, a.Zoom/1.5)
				a.Pan = min(a.Pan, max(0, a.Doc.Duration-a.span()))
			}
			ui.Text(c, fmt.Sprintf("%.0f%%", a.Zoom*100)).FontSize(10)
			if ui.Button(c.Key("zoomin"), "+").Disabled(a.Zoom >= 64).Clicked() {
				a.Zoom = min(64, a.Zoom*1.5)
				a.Pan = max(0, min(a.Position-a.span()/2, a.Doc.Duration-a.span()))
			}
		})
		ui.Row(c).FillWidth().Height(190).Children(func() {
			ui.Column(c).Width(74).FillHeight().Padding(12, 10).Gap(0).Children(func() {
				ui.Text(c, "时间").FontSize(10).Height(30).TextColor(t.TextMuted)
				ui.Text(c, "音频").FontSize(10).Height(60).TextColor(t.TextMuted)
				ui.Text(c, "字幕").FontSize(10).Height(44).TextColor(t.TextMuted)
				ui.Text(c, "口型").FontSize(10).TextColor(t.TextMuted)
			})
			ui.Box(c.Key("timeline")).Label("口型编辑时间轴").Bind(&a.timelineHandle).Grow(1).FillHeight().Clip().Focusable().HandleInput(func(ev ui.InputEvent) bool { return a.timelineInput(c, ev) }).Draw(func(p *ui.Painter, r ui.Rect) { a.drawTimeline(p, r, t) })
		})
		pan := a.Pan
		if ui.Slider(c.Key("pan"), &pan, 0, max(.0001, a.Doc.Duration-a.span())).FillWidth().Height(24).Disabled(a.Zoom <= 1).Changed() {
			a.Pan = pan
		}
	})
}
func (a *App) timelineInput(c *ui.Context, e ui.InputEvent) bool {
	if a.Busy {
		return false
	}
	w := a.timelineHandle.Bounds(c).W
	if w <= 0 {
		return false
	}
	t := max(0, min(a.Doc.Duration, a.Pan+float64(e.X/w)*a.span()))
	switch e.Kind {
	case ui.InputPointerDown:
		if e.Button != 0 {
			return false
		}
		a.Player.Pause()
		a.drag = 0
		if e.Y >= 132 && e.Y <= 184 {
			for i, v := range a.Doc.Timeline {
				left := float32((v.Start-a.Pan)/a.span()) * w
				right := float32((v.End-a.Pan)/a.span()) * w
				if math.Abs(float64(e.X-left)) < 5 || math.Abs(float64(e.X-right)) < 5 {
					a.Selected = i
					a.drag = 1
					if math.Abs(float64(e.X-right)) < math.Abs(float64(e.X-left)) {
						a.drag = 2
					}
					a.dragIndex = i
					a.dragBefore = slices.Clone(a.Doc.Timeline)
					return true
				}
			}
			for i, v := range a.Doc.Timeline {
				if t >= v.Start && t < v.End {
					a.Selected = i
					break
				}
			}
		}
		a.seek(core.Snap(t, a.Doc.FPS))
		return true
	case ui.InputPointerMove:
		if e.Button != 0 {
			return false
		}
		if a.drag > 0 {
			a.Doc.Timeline = a.dragBefore
			a.Doc.Timeline = core.Boundary(a.Doc, a.dragIndex, a.drag == 1, t)
		} else {
			a.seek(core.Snap(t, a.Doc.FPS))
		}
		return true
	case ui.InputPointerUp:
		if a.drag > 0 {
			next := a.Doc.Timeline
			a.Doc.Timeline = a.dragBefore
			a.commit(next)
			a.drag = 0
			a.dragBefore = nil
		}
		return true
	case ui.InputScroll:
		a.Pan = max(0, min(a.Doc.Duration-a.span(), a.Pan+float64(e.DX+e.DY)*a.span()/float64(w)))
		return true
	case ui.InputZoom:
		a.Zoom = max(1, min(64, a.Zoom*float64(e.Scale)))
		a.Pan = max(0, min(a.Pan, a.Doc.Duration-a.span()))
		return true
	}
	return false
}
func (a *App) drawTimeline(p *ui.Painter, r ui.Rect, t *ui.Theme) {
	p.Fill(r, t.Background, 0)
	span := a.span()
	x := func(s float64) float32 { return r.X + float32((s-a.Pan)/span)*r.W }
	step := 1.0
	if span > 20 {
		step = 5
	} else if span < 3 {
		step = .25
	}
	for sec := math.Ceil(a.Pan/step) * step; sec <= a.Pan+span; sec += step {
		px := x(sec)
		p.Line(px, r.Y+22, px, r.Y+r.H, 1, t.Border)
		p.Text(px+4, r.Y+4, fmt.Sprintf("%.2fs", sec), 10, t.TextMuted)
	}
	p.Fill(ui.Rect{X: r.X, Y: r.Y + 31, W: r.W, H: 48}, t.Surface, 4)
	for px := float32(0); px < r.W; px += 2 {
		from := int((a.Pan + float64(px/r.W)*span) * core.SampleRate)
		to := min(len(a.PCM.Samples), int((a.Pan+float64((px+2)/r.W)*span)*core.SampleRate))
		amp := float32(0)
		for j := max(0, from); j < to; j++ {
			amp = max(amp, float32(math.Abs(float64(a.PCM.Samples[j]))))
		}
		height := max(1, amp*21)
		p.Line(r.X+px, r.Y+55-height, r.X+px, r.Y+55+height, 1.5, t.Accent)
	}
	for _, cue := range a.Doc.Cues {
		l, rr := x(cue.Start), x(cue.End)
		if rr < r.X || l > r.X+r.W {
			continue
		}
		p.Fill(ui.Rect{X: l, Y: r.Y + 92, W: max(1, rr-l-2), H: 30}, t.Surface, 4)
		if rr-l > 32 {
			p.Clip(ui.Rect{X: l + 4, Y: r.Y + 93, W: rr - l - 8, H: 28}, 0, func() { p.Text(l+7, r.Y+99, cue.Text, 11, t.TextMuted) })
		}
	}
	for i, v := range a.Doc.Timeline {
		l, rr := x(v.Start), x(v.End)
		if rr < r.X || l > r.X+r.W {
			continue
		}
		box := ui.Rect{X: l + 1, Y: r.Y + 137, W: max(1, rr-l-2), H: 38}
		p.Fill(box, palette[v.Mouth], 4)
		if i == a.Selected {
			p.Stroke(box, t.Text, 4, 2)
		}
		if box.W > 16 {
			p.Clip(box, 0, func() { p.Text(l+6, r.Y+148, v.Mouth.Label(), 11, ui.RGB(255, 255, 255)) })
		}
	}
	head := x(a.Position)
	if head >= r.X && head <= r.X+r.W {
		p.Line(head, r.Y+21, head, r.Y+184, 2, t.Accent)
		p.Fill(ui.Rect{X: head - 4, Y: r.Y + 18, W: 8, H: 8}, t.Accent, 2)
	}
}
