package desktop

import (
	"automouthshape/internal/aligner"
	"automouthshape/internal/core"
	"automouthshape/internal/demo"
	"context"
	"errors"
	"fmt"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/audio"
	"github.com/egoist/mygo/ui"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type App struct {
	Doc            *core.Document
	PCM            *core.PCM
	Engine         *aligner.Engine
	Window         *mygo.Window
	Images         map[core.Mouth]*ui.Bitmap
	Player         *audio.Player
	History        core.History
	Selected       int
	Position       float64
	Zoom           float64
	Pan            float64
	Loop           bool
	Busy           bool
	Progress       float64
	Status         string
	LastError      string
	Dirty          bool
	Edited         bool
	audioImported  bool
	srtImported    bool
	ProjectPath    string
	cancel         context.CancelFunc
	drag           int
	dragIndex      int
	dragBefore     []core.Segment
	timelineHandle ui.Handle
	closing        bool
}

func New(root string) (*App, error) {
	d, p, e := demo.LoadDemo()
	if e != nil {
		return nil, e
	}
	a := &App{Doc: d, PCM: p, Selected: -1, Zoom: 1, Status: "示例已就绪 · 点击播放，或导入自己的素材", dragIndex: -1}
	a.install(d, p)
	a.Engine, e = aligner.New(root)
	if e != nil {
		a.LastError = e.Error()
		a.Status = "离线分析资源缺失；可继续预览和编辑示例"
	}
	return a, nil
}
func (a *App) install(d *core.Document, p *core.PCM) {
	if a.Player != nil {
		a.Player.Close()
	}
	a.Doc = d
	a.PCM = p
	a.Player = audio.NewSound(core.SampleRate, 1, p.Samples).NewPlayer()
	a.Images = map[core.Mouth]*ui.Bitmap{}
	for _, m := range core.Mouths {
		im, e := ui.DecodeBitmap(d.Mouths[m].Data)
		if e == nil {
			a.Images[m] = im
		}
	}
	a.History = core.History{}
	a.Selected = -1
	a.Position = 0
	a.Pan = 0
	a.Dirty = false
	a.Edited = false
	a.audioImported = false
	a.srtImported = false
	a.ProjectPath = ""
}
func (a *App) Run() error {
	mygo.App.WhenReady(func() {
		a.Window = mygo.NewWindow(mygo.WindowOptions{Title: "AutoMouthShape · 0.0.1", Width: 1440, Height: 940, MinWidth: 1180, MinHeight: 760, Content: ui.View(a.View)})
		a.Window.OnClose(func(e *mygo.CloseEvent) {
			if a.closing {
				return
			}
			if a.Busy {
				e.PreventDefault()
				a.Status = "请先取消或等待当前任务"
				return
			}
			if a.Dirty {
				e.PreventDefault()
				a.guard(func() { a.closing = true; a.Window.Close() })
			}
		})
		a.Window.OnClosed(func() {
			if a.cancel != nil {
				a.cancel()
			}
			a.Player.Close()
		})
	})
	return mygo.App.Run()
}
func (a *App) update(fn func()) { a.Window.Update(fn) }
func (a *App) fail(e error)     { a.LastError = e.Error(); a.Status = "操作失败 · 点击查看日志" }
func (a *App) seek(t float64) {
	a.Position = max(0, min(a.Doc.Duration, t))
	a.Player.Seek(time.Duration(a.Position * float64(time.Second)))
}
func (a *App) toggle() {
	if a.Player.Playing() {
		a.Player.Pause()
	} else {
		if len(a.Doc.Timeline) == 0 {
			a.Status = "请先完成分析"
			return
		}
		if a.Loop && a.Selected >= 0 && a.Selected < len(a.Doc.Timeline) {
			v := a.Doc.Timeline[a.Selected]
			a.Player.SetLoopRange(time.Duration(v.Start*1e9), time.Duration(v.End*1e9))
			if a.Position < v.Start || a.Position >= v.End {
				a.seek(v.Start)
			}
			a.Player.SetLoop(true)
		} else {
			a.Player.SetLoop(false)
		}
		a.Player.Play()
	}
}
func (a *App) selected() bool { return a.Selected >= 0 && a.Selected < len(a.Doc.Timeline) }
func (a *App) commit(next []core.Segment) {
	if e := a.History.Commit(a.Doc, next); e != nil {
		a.fail(e)
		return
	}
	a.Dirty = true
	a.Edited = true
	a.Status = "口型修改已应用"
	if a.Selected >= len(a.Doc.Timeline) {
		a.Selected = len(a.Doc.Timeline) - 1
	}
}
func (a *App) replace(m core.Mouth) {
	if !a.selected() || a.Busy {
		return
	}
	next := slices.Clone(a.Doc.Timeline)
	next[a.Selected].Mouth = m
	a.commit(next)
}
func (a *App) split() {
	s, e := core.Split(a.Doc, a.Selected, a.Position)
	if e != nil {
		a.fail(e)
		return
	}
	a.commit(s)
}
func (a *App) merge() {
	s, e := core.Merge(a.Doc, a.Selected)
	if e != nil {
		a.fail(e)
		return
	}
	a.commit(s)
}
func (a *App) undo() {
	if a.History.Undo(a.Doc) {
		a.Selected = -1
		a.Dirty = true
		a.Edited = true
	}
}
func (a *App) redo() {
	if a.History.Redo(a.Doc) {
		a.Selected = -1
		a.Dirty = true
		a.Edited = true
	}
}
func (a *App) choose(title string, ext []string, dir bool, apply func(string)) {
	a.Busy = true
	a.Player.Pause()
	go func() {
		opts := mygo.OpenDialogOptions{Parent: a.Window, Title: title, Directory: dir, CreateDirectories: dir}
		if len(ext) > 0 {
			opts.Filters = []mygo.FileFilter{{Name: title, Extensions: ext}}
		}
		paths, e := mygo.Dialog.Open(opts)
		a.update(func() {
			a.Busy = false
			if e != nil {
				a.fail(e)
				return
			}
			if len(paths) > 0 {
				apply(paths[0])
			}
		})
	}()
}
func (a *App) saveTo(path string, done func()) {
	a.Busy = true
	a.Player.Pause()
	d := a.Doc
	go func() {
		var e error
		if path == "" {
			path, e = mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.Window, Title: "保存便携工程", DefaultPath: d.Name + ".amsproj", Filters: []mygo.FileFilter{{Name: "AutoMouthShape 工程", Extensions: []string{"amsproj"}}}})
		}
		if e == nil && path != "" {
			if !strings.EqualFold(filepath.Ext(path), ".amsproj") {
				path += ".amsproj"
			}
			e = core.SaveProject(path, d)
		}
		a.update(func() {
			a.Busy = false
			if e != nil {
				a.fail(e)
				return
			}
			if path == "" {
				return
			}
			a.Dirty = false
			a.Edited = false
			a.ProjectPath = path
			a.Status = "工程已保存 · " + filepath.Base(path)
			if done != nil {
				done()
			}
		})
	}()
}
func (a *App) guard(action func()) {
	if !a.Dirty {
		action()
		return
	}
	a.Busy = true
	a.Player.Pause()
	go func() {
		r, e := mygo.Dialog.Message(mygo.MessageOptions{Parent: a.Window, Type: mygo.MessageQuestion, Title: "保留当前修改？", Message: "当前工程有尚未保存的修改", Detail: "可以先保存便携工程，再继续操作。", Buttons: []string{"保存并继续", "放弃修改", "取消"}, DefaultButton: 0, CancelButton: 2})
		a.update(func() {
			a.Busy = false
			if e != nil {
				a.fail(e)
				return
			}
			switch r.Button {
			case 0:
				a.saveTo(a.ProjectPath, action)
			case 1:
				action()
			}
		})
	}()
}
func (a *App) guardEdits(action func()) {
	if a.Edited {
		a.guard(action)
	} else {
		action()
	}
}
func (a *App) autoAnalyze() {
	if a.audioImported && a.srtImported {
		a.analyze()
	} else if a.audioImported {
		a.Status = "音频已导入，请导入对应 SRT 字幕；也可手动分析当前组合"
	} else {
		a.Status = "字幕已导入，请导入对应音频；也可手动分析当前组合"
	}
}
func (a *App) importAudio() {
	a.guardEdits(func() {
		a.choose("导入音频", []string{"wav", "mp3"}, false, func(path string) {
			a.Busy = true
			go func() {
				data, e := os.ReadFile(path)
				var pcm *core.PCM
				if e == nil {
					pcm, e = core.Decode(data)
				}
				a.update(func() {
					a.Busy = false
					if e != nil {
						a.fail(e)
						return
					}
					a.Player.Close()
					a.PCM = pcm
					a.Player = audio.NewSound(core.SampleRate, 1, pcm.Samples).NewPlayer()
					a.Doc.Audio = core.Asset{Name: filepath.Base(path), Data: data}
					a.Doc.Duration = pcm.Duration()
					a.invalidate()
					a.audioImported = true
					a.autoAnalyze()
				})
			}()
		})
	})
}
func (a *App) importSRT() {
	a.guardEdits(func() {
		a.choose("导入 SRT 字幕", []string{"srt"}, false, func(path string) {
			data, e := os.ReadFile(path)
			if e != nil {
				a.fail(e)
				return
			}
			cues, e := core.ParseSRT(data)
			if e != nil {
				a.fail(e)
				return
			}
			a.Doc.Subtitle = core.Asset{Name: filepath.Base(path), Data: data}
			a.Doc.Cues = cues
			a.invalidate()
			a.srtImported = true
			a.autoAnalyze()
		})
	})
}
func (a *App) invalidate() {
	a.Player.Stop()
	a.Position = 0
	a.Doc.Timeline = nil
	a.Doc.Characters = nil
	a.History = core.History{}
	a.Selected = -1
	a.Dirty = true
	a.Edited = false
}
func (a *App) importMouths() {
	a.choose("选择七张口型 PNG 所在文件夹", nil, true, func(path string) {
		m, e := core.LoadMouthFolder(path)
		if e != nil {
			a.fail(e)
			return
		}
		a.Doc.Mouths = m
		a.reloadImages()
		a.Dirty = true
		a.Status = "口型素材已替换，保留当前时间轴"
	})
}
func (a *App) importMouth(m core.Mouth) {
	a.choose("指定 "+m.Label()+" 口型 PNG", []string{"png"}, false, func(path string) {
		b, e := os.ReadFile(path)
		if e != nil {
			a.fail(e)
			return
		}
		next := make(map[core.Mouth]core.Asset)
		for k, v := range a.Doc.Mouths {
			next[k] = v
		}
		next[m] = core.Asset{Name: filepath.Base(path), Data: b}
		if _, _, e = core.ValidateMouths(next); e != nil {
			a.fail(e)
			return
		}
		a.Doc.Mouths = next
		a.reloadImages()
		a.Dirty = true
	})
}
func (a *App) reloadImages() {
	for _, m := range core.Mouths {
		im, e := ui.DecodeBitmap(a.Doc.Mouths[m].Data)
		if e == nil {
			a.Images[m] = im
		}
	}
}
func (a *App) open() {
	a.guard(func() {
		a.choose("打开便携工程", []string{"amsproj"}, false, func(path string) {
			a.Busy = true
			go func() {
				d, p, e := core.OpenProject(path)
				a.update(func() {
					a.Busy = false
					if e != nil {
						a.fail(e)
						return
					}
					a.install(d, p)
					a.audioImported, a.srtImported = true, true
					a.ProjectPath = path
					a.Status = "工程已打开，无需重新分析"
				})
			}()
		})
	})
}
func (a *App) restore() {
	a.guard(func() {
		d, p, e := demo.LoadDemo()
		if e != nil {
			a.fail(e)
			return
		}
		a.install(d, p)
		a.Status = "已恢复内置示例"
	})
}
func (a *App) analyze() {
	if a.Busy {
		return
	}
	if a.Engine == nil {
		a.fail(fmt.Errorf("离线分析资源不完整，请检查 resources/runtime"))
		return
	}
	if len(a.Doc.Audio.Data) == 0 || len(a.Doc.Cues) == 0 {
		return
	}
	a.Player.Pause()
	a.Busy = true
	a.LastError = ""
	a.Progress = 0
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	d, p := a.Doc, a.PCM
	go func() {
		chars, timeline, e := a.Engine.Analyze(ctx, d, p, func(v float64, s string) { a.update(func() { a.Progress = v; a.Status = s }) })
		cancel()
		a.update(func() {
			a.Busy = false
			a.cancel = nil
			if e != nil {
				if errors.Is(e, context.Canceled) {
					a.Status = "分析已取消；保留原时间轴"
				} else {
					a.fail(e)
				}
				return
			}
			a.Doc.Characters = chars
			a.Doc.Timeline = timeline
			a.History = core.History{}
			a.Selected = -1
			a.Dirty = true
			a.Edited = false
			a.Progress = 1
			a.Status = fmt.Sprintf("分析完成 · %d 个汉字 · %d 个口型区间", len(chars), len(timeline))
			low := 0
			for _, ch := range chars {
				if ch.Score < .2 {
					low++
				}
			}
			if low > 0 {
				a.Status += fmt.Sprintf(" · %d 个低分字请手动复核", low)
			}
		})
	}()
}
func (a *App) reanalyze() { a.guardEdits(func() { a.analyze() }) }
func (a *App) export() {
	a.choose("选择 PNG 序列的导出位置", nil, true, func(path string) {
		a.Busy = true
		a.Progress = 0
		a.LastError = ""
		ctx, cancel := context.WithCancel(context.Background())
		a.cancel = cancel
		d := a.Doc
		go func() {
			dir, e := core.Export(ctx, path, d, func(n, total int) {
				a.update(func() {
					a.Progress = float64(n) / float64(total)
					a.Status = fmt.Sprintf("导出 PNG · %d / %d", n, total)
				})
			})
			cancel()
			a.update(func() {
				a.Busy = false
				a.cancel = nil
				if e != nil {
					if errors.Is(e, context.Canceled) {
						a.Status = "导出已取消，未完成目录：" + dir
					} else {
						a.fail(e)
					}
					return
				}
				a.Progress = 1
				a.Status = "导出完成 · " + dir
			})
		}()
	})
}
func (a *App) log() {
	content := a.LastError
	if content == "" {
		return
	}
	go mygo.Dialog.Message(mygo.MessageOptions{Parent: a.Window, Type: mygo.MessageError, Title: "操作日志", Message: "操作未能完成", Detail: content, Buttons: []string{"关闭"}})
}
