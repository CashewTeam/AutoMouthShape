#!/usr/bin/env python3
"""Wrap both already-built offline applications in one portable ZIP archive."""
import os
import shutil
import json
import subprocess
import tempfile
from pathlib import Path
root = Path(__file__).resolve().parents[1]
version = json.loads((root/'mygo.json').read_text())['version']
build = root/'build'
mac = build/'darwin-universal/AutoMouthShape.app'
win = build/'windows-amd64'
if not mac.is_dir() or not (win/'AutoMouthShape.exe').is_file():
    raise SystemExit('Build both platforms first: go tool mygo build -platform darwin/universal,windows/amd64 -skip-dmg')
mac_resources = mac/'Contents/Resources'
for base in (mac_resources,win):
    for name in ('runtime/model/model.int8.onnx','runtime/model/tokens.txt','third_party/runtime-manifest.json'):
        if not (base/name).is_file():
            raise SystemExit(f'Offline package is missing {base/name}')
output = build/f'AutoMouthShape-{version}-macOS-Windows.zip'
with tempfile.TemporaryDirectory(prefix='.package-', dir=build) as temporary:
    package = Path(temporary)/f'AutoMouthShape-{version}'
    subprocess.run(['ditto',str(win),str(package)],check=True)
    app = package/'AutoMouthShape.app'
    subprocess.run(['ditto',str(mac),str(app)],check=True)
    mac_base = app/'Contents/Resources'
    # Keep one model inside the self-contained macOS app.
    shutil.rmtree(package/'runtime/model')
    shutil.rmtree(package/'runtime/darwin')
    shutil.rmtree(mac_base/'runtime/windows')
    model_path = '../AutoMouthShape.app/Contents/Resources/runtime/model'
    (package/'runtime/model-location.json').write_text(json.dumps({'path': model_path})+'\n')
    for base, platform in ((package, 'windows'), (mac_base, 'darwin')):
        path = base/'third_party/runtime-manifest.json'
        manifest = json.loads(path.read_text())
        files = manifest['files']
        manifest['files'] = {name: info for name, info in files.items() if name.startswith(platform+'/') or (platform == 'darwin' and name.startswith('model/'))}
        if platform == 'windows':
            manifest['shared_model'] = {'path': model_path, 'files': {name[len('model/'):]: info for name, info in files.items() if name.startswith('model/')}}
        path.write_text(json.dumps(manifest,ensure_ascii=False,indent=2)+'\n')
    subprocess.run(['codesign','--force','--sign','-','--preserve-metadata=identifier,entitlements,flags',str(app)],check=True)
    subprocess.run(['codesign','--verify','--deep','--strict',str(app)],check=True)
    archive = Path(temporary)/output.name
    subprocess.run(['ditto','-c','-k','--zlibCompressionLevel','9','--keepParent',str(package),str(archive)],check=True)
    os.replace(archive, output)
print(f'Created combined offline ZIP: {output}')
