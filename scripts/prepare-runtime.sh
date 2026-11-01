#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if [ "$#" -ne 1 ]; then
  echo 'Usage: scripts/prepare-runtime.sh /path/to/sherpa-model-directory' >&2
  exit 2
fi
model_dir="$1"
test -f "$model_dir/model.int8.onnx"
test -f "$model_dir/tokens.txt"
mkdir -p resources/runtime/model resources/runtime/darwin resources/runtime/windows
cp "$model_dir/model.int8.onnx" "$model_dir/tokens.txt" resources/runtime/model/
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM
curl -fL --retry 2 --connect-timeout 15 --max-time 180 https://github.com/corvo007/cpp-ctc-aligner/releases/download/v0.2.2/cpp-ort-aligner-macos-universal2.tar.gz -o "$work_dir/mac.tar.gz"
curl -fL --retry 2 --connect-timeout 15 --max-time 180 https://github.com/corvo007/cpp-ctc-aligner/releases/download/v0.2.2/cpp-ort-aligner-windows-x64.zip -o "$work_dir/windows.zip"
printf '80ea38065d252fb27051c35b379072f395544c4dc4c4b78ae4dc6b589a1e465f  %s\n' "$work_dir/mac.tar.gz" | shasum -a 256 -c -
printf '1d11b83b5086073524a3ebcb081ab6dca52753f6d751c950ddac2b4eee25a946  %s\n' "$work_dir/windows.zip" | shasum -a 256 -c -
tar -xzf "$work_dir/mac.tar.gz" -C "$work_dir"
unzip -q "$work_dir/windows.zip" -d "$work_dir"
cp -R "$work_dir/cpp-ort-aligner-macos-universal2/." resources/runtime/darwin/
cp -R "$work_dir/cpp-ort-aligner-windows-x64/." resources/runtime/windows/
python3 scripts/runtime-manifest.py
