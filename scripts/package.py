#!/usr/bin/env python3
"""Wrap already-built, complete offline applications in portable ZIP archives."""
import json
import subprocess
import zipfile
from pathlib import Path
root = Path(__file__).resolve().parents[1]
version = json.loads((root/'mygo.json').read_text())['version']
build = root/'build'
mac = build/'darwin-universal/AutoMouthShape.app'
win = build/'windows-amd64'
if not mac.is_dir() or not (win/'AutoMouthShape.exe').is_file():
    raise SystemExit('Build both platforms first: go tool mygo build -platform darwin/universal,windows/amd64 -skip-dmg')
for base in (mac/'Contents/Resources',win):
    for name in ('runtime/model/model.int8.onnx','runtime/model/tokens.txt','third_party/runtime-manifest.json'):
        if not (base/name).is_file():
            raise SystemExit(f'Offline package is missing {base/name}')
subprocess.run(['ditto','-c','-k','--keepParent',str(mac),str(build/f'AutoMouthShape-{version}-macOS-Universal.zip')],check=True)
with zipfile.ZipFile(build/f'AutoMouthShape-{version}-Windows-x64.zip','w',compression=zipfile.ZIP_DEFLATED,compresslevel=6) as archive:
    for path in sorted(win.rglob('*')):
        if path.is_file():
            archive.write(path,Path('AutoMouthShape')/path.relative_to(win))
print('Created macOS Universal and Windows x64 offline ZIP packages')
