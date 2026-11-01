# AutoMouthShape 0.0.5

使用 MyGo native UI 的中文口型工作台。纯 Go 界面，不使用 HTML、JavaScript 或 WebView。音频、字幕、口型分析和工程内容均在本地处理，运行时不联网。

## 使用

1. 打开应用即可播放内置中文示例。
2. 点击「口型文件夹」载入七张 PNG，也可在左侧分别指定文件。
3. 分别导入 WAV/MP3 和 UTF-8 SRT。两项导入完成后自动分析。若只想替换示例中的一项，可点击「重新分析」分析当前组合。
4. 在时间轴点击波形定位，点击口型区间选中，拖动区间边界调整时间；右侧可替换口型、修改起止时间、在播放头拆分或合并至右侧。
5. 保存 `.amsproj` 便携工程，或选择输出位置导出透明 PNG 序列。

口型文件名（大小写不敏感）：

| 文件 | 口型 |
| --- | --- |
| `a.png` | A |
| `o.png` | O |
| `e.png` | E |
| `i.png` | I |
| `u.png` | U |
| `v.png` | Ü |
| `closed.png` | 闭口 |

七张 PNG 须有相同画布尺寸与口型位置。导出保留原始尺寸和透明度，默认 30 fps，可选 24/25/60 fps，从音频起点开始，文件为 `frame_000001.png` 等。每次创建新的输出子目录，取消或失败时保留 `INCOMPLETE.txt`。

「前一音 / 后一音」跳到播放头前后最近的已对齐汉字发音起点，跳转后暂停；句间停顿会被跳过。音频边界或没有对齐结果时，对应按钮禁用。空格播放或暂停，左右方向键逐帧定位；Cmd/Ctrl+S 保存，Cmd/Ctrl+O 打开，Cmd/Ctrl+Z 撤销，Cmd/Ctrl+Shift+Z 重做。勾选「循环选中区间」后点击播放即可循环。

时间轴交互：

- 普通滚轮或触控板双指：左右平移时间轴。
- Ctrl/Cmd＋滚轮、触控板捏合：以鼠标位置为锚点缩放。
- 鼠标位于时间轴上，或时间轴获得焦点时，`+` / `-` 缩放，`Shift＋←/→` 左右平移当前可见范围的 20%。
- 缩放按钮与键盘缩放使用鼠标最后停留的时间轴位置，尚未移入时间轴时使用视图中心；缩放到音频边界时限制在有效范围内。

## 对齐方式与边界

使用 `cpp-ctc-aligner v0.2.2` 官方二进制及本地的 `sherpa-onnx-omnilingual-asr-1600-languages-300M-ctc-int8-2025-11-12` 模型。该模型使用 `tokens.txt` 原生中文字表，调用时使用 `--language cmn`，不启用 MMS 拼音罗马化。

SRT 按句界组成约 30 秒批次，附加前后 0.5 秒上下文，输出逐字真实 CTC 时间点。上游输出为包含结束帧的时间，单帧字符的起止可相同；应用将其解释为 20 ms 帧，并从当前字起点保持口型至下一字。原始分数保存在工程中。

繁体字在送入模型前按 OpenCC 字表转换为简体，原字幕文字仍保留。当前支持普通话汉字及标点；字幕中的阿拉伯数字、英文和读音标记需先改成中文读法。多音字使用字表默认读音，可以直接在时间轴修正口型。字幕须与音频相符，且句级时间大致正确；不提供自动转写。

静音检测为 10 ms RMS 窗口，默认低于 −40 dBFS 且持续至少 80 ms 时闭口。调节灵敏度后需重新分析。音频带持续背景音乐时，静音闭口检测效果会受到影响。

对齐失败、输出非法或整批平均分低于 0.35 会显示错误日志，不自动改用均分时间。个别低分字保留真实时间并提示人工复核，避免把单个不确定字符当作整段失败。替换口型素材保留编辑；替换音频、字幕或重新分析前，如有手动口型编辑，会提示保存。

## 开发与构建

需要 Go 1.27.3 或更新版本，MyGo 固定为 v0.7.3。应用运行无需 Python 或 FFmpeg；Python 3 只用于开发时记录依赖和包装 ZIP。

```sh
go tool mygo dev
```

也可直接 `go run .`。开发截图：

```sh
go run . --screenshot /tmp/automouthshape.png
```

离线运行资源位于 `resources/runtime/`，包含：

```text
model/model.int8.onnx
model/tokens.txt
darwin/cpp-ort-aligner
darwin/libonnxruntime.dylib
darwin/Chinese_to_Pinyin.txt
windows/cpp-ort-aligner.exe
windows/onnxruntime.dll
windows/Chinese_to_Pinyin.txt
```

模型已从你指定的 SubtitleAgent 目录复制进项目。大文件和构建产物不纳入源码版本管理。重新准备时执行 `scripts/prepare-runtime.sh /本地模型文件夹`，仅在开发准备阶段下载两平台的对齐器。

```sh
go test ./...
AMS_INTEGRATION=1 go test ./internal/aligner
go vet ./...
./scripts/build.sh
```

构建结果：

- `build/darwin-universal/AutoMouthShape.app`
- `build/windows-amd64/AutoMouthShape.exe`（保留同目录所有资源）

日常构建不生成 ZIP。需要发布离线包时，再单独执行：

```sh
python3 scripts/package.py
```

这会生成当前版本的 macOS 和 Windows 离线 ZIP。

macOS 包使用本地 ad-hoc 签名，未做 Apple 公证。Windows 提供完整便携包。运行测试请参见 [VALIDATION.md](VALIDATION.md)。运行资源的来源、版本和 SHA-256 记录在 `third_party/runtime-manifest.json`。

## 源码结构

- `internal/core`：音频解码、SRT、时间轴编辑、工程与导出。
- `internal/aligner`：独立进程对齐、字符映射、CTC 时间适配。
- `internal/desktop`：MyGo 原生界面、播放和后台任务。
- `internal/demo`：内嵌示例及真实模型预分析结果。

内置口型为程序绘制的原创几何示例，音频由本机中文系统语音合成。`go run ./cmd/prepare-demo` 可以重新生成几何素材和预分析结果。应用从 `0.0.1` 起步，后续按补丁版本递增。
