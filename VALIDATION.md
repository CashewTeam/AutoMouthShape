# 验证记录

## 0.0.6：Windows 目录内包含独立 macOS App（2026-11-01）

- ZIP 根目录放 Windows 程序、Windows 运行库及完整 `AutoMouthShape.app`；模型仅存于 App 内，Windows 通过相对路径读取。
- App 内不含 Windows 运行库，亦不配置外部模型路径；独立移到中文、含空格目录后，签名检查、界面渲染及真实中文逐字对齐集成测试通过。
- `go test ./...`、`go vet ./...`、macOS Universal 与 Windows x64 构建通过。
- ZIP 为 310.2 MiB，CRC、资源 SHA-256 及 Windows 模型路径检查通过；Windows 未进行实机运行验收。

## 7z 快速压缩参数（2026-11-01）

- 使用 LZMA2 级别 1、HC4 匹配器、32 个 fast bytes、线程数按 `os.cpu_count()` 检测本机逻辑处理器数量；保留固实压缩及 512 MiB 字典以利用重复模型。
- 小样本压缩及解压字节一致性检查通过；随后本机以 8 线程参数完成一次完整打包，7z 为 602.4 MiB，仍耗时数分钟且仅比 ZIP 小幅缩小。
- 7z 完整性、解压后逐文件 SHA-256、macOS 执行权限及签名检查通过。仅调整打包流程，版本保持 `0.0.5`。

## 合并离线打包（2026-11-01）

- 恢复各平台完整独立资源的布局，仅将 ZIP 压缩级别设为 9；撤销共享模型支持，应用保持 `0.0.5`。
- 最高压缩级别的合并包为 607.0 MiB（原 607.3 MiB）；ZIP CRC、逐文件 SHA-256 及解压后的 macOS 签名检查通过。

- 仅调整打包流程，应用版本保持 `0.0.5`；日常构建仍不自动生成 ZIP。
- `python3 scripts/package.py` 生成一个 ZIP，包含 `macOS/AutoMouthShape.app` 与完整 `Windows/` 便携目录。
- ZIP CRC 检查通过；macOS 21 个文件、Windows 17 个文件均与构建目录的 SHA-256 一致。
- 解压后 macOS 应用的执行权限与 `codesign --verify --deep --strict` 验证通过。

## 0.0.5（2026-11-01）

- 顶栏直接显示打包应用使用的 `resources/icon.png`，替换原手绘标记。
- 原生渲染预览、`go test ./internal/desktop` 和 `go vet ./internal/desktop`：通过。

## 0.0.4（2026-11-01）

- 播放按钮及时间码固定宽度，时间码采用等宽数字；跨小数、分钟及小时的控件位置回归检查通过。
- 普通滚轮/触控板平移、Ctrl/Cmd＋滚轮缩放、捏合缩放、+/− 和 Shift＋方向键检查通过。
- 鼠标位于视图 20%、50%、80% 位置时，缩放前后对应时间保持不变；首尾滚动边界及 100%–6400% 缩放范围检查通过。
- `go test ./internal/desktop` 与 `go vet ./internal/desktop`：通过。

## 0.0.3（2026-11-01）

- 顶栏合并为单行，保留所有导入、分析、工程和导出操作。
- 1440×940 与 1180×760 原生渲染检查通过，长工程名省略显示且不挤占按钮。
- `go test ./internal/desktop` 与 `go vet ./internal/desktop`：通过。
- 日常构建不再调用 ZIP 打包；发布 ZIP 需单独运行 `scripts/package.py`。

## 0.0.2（2026-11-01）

- `go test ./internal/desktop` 与 `go vet ./internal/desktop`：通过。
- 原生 UI 测试覆盖「前一音 / 后一音」按汉字起点跳转、起点与发音中途定位、首尾边界、无对齐数据，以及左右方向键逐帧定位。
- 既有口型替换、边界拖动及撤销操作检查通过。

## 0.0.1

验证日期：2026-11-01。构建机为 macOS Apple Silicon，Go 1.27.3，MyGo v0.7.3。

## 自动验证

- `CGO_ENABLED=0 go test ./...`：通过。
- `AMS_INTEGRATION=1 CGO_ENABLED=0 go test ./internal/aligner`：通过，调用实际发布二进制和项目内模型，未模拟推理。
- `go vet ./...`：通过。
- macOS arm64、amd64 与 Windows amd64：均通过纯 Go 编译。

覆盖内容：

- UTF-8/BOM/CRLF、多行 SRT 与非法时间、重叠字幕、空文件。
- WAV、MP3 解码和 PCM 时长；80 ms 静音最短长度及闭口区间切分。
- 七张 PNG 缺失与画布检查。
- 边界拖动、起止时间约束、拆分合并、撤销重做，以及原生界面的逐帧定位和口型替换。
- 便携工程保存、搬移到中文路径后重新打开，素材字节与时间轴一致。
- 24/25/30/60 fps 的完整序列帧数，导出 PNG 与预览采样结果逐帧逐字节一致；取消导出保留未完成标记。
- 真实模型：简体、繁体、重复字、快慢语速、跨 30 秒批次的 140 字音频、整体错配字幕、全静音、取消和子进程失败。

原生界面已完成 1440×940 渲染检查。macOS 实机已验证：启动打包应用、播放进度与字幕同步、暂停、调用包内模型重新分析、原生保存对话框写入便携工程，以及原生导出对话框生成 157 张 512×512 透明 PNG。测试工程包含 20 个汉字及 21 个口型区间。

## 平台验证边界

Windows x64 已完成编译、资源和离线 ZIP 完整性检查；当前机器没有 Windows 运行环境，因此未声称完成 Windows 实机播放、文件对话框或中文安装路径验收。

macOS 使用本地 ad-hoc 签名，未做 Developer ID 签名或 Apple 公证。发布包不含安装器和自动更新功能。

## 模型来源核对

用户指定模型已通过 `cp` 复制到 `resources/runtime/model/`。源文件与项目副本 SHA-256 一致：

`e7c4e54ee4c4c47829cc6667d5d00ed8ea7bef1dcfeef0fce766f77752a2726c`

两平台对齐器归档亦已核对发布文件 SHA-256。逐文件校验记录位于 `third_party/runtime-manifest.json`。
