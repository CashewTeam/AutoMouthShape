#!/usr/bin/env python3
"""Record the exact offline runtime files included in each build."""
import hashlib
import json
from pathlib import Path
root = Path(__file__).resolve().parents[1]
runtime = root / 'resources/runtime'
files = {}
for path in sorted(runtime.rglob('*')):
    if path.is_file() and not path.name.startswith('.'):
        h = hashlib.sha256()
        with path.open('rb') as stream:
            for block in iter(lambda: stream.read(1024 * 1024), b''):
                h.update(block)
        files[path.relative_to(runtime).as_posix()] = {'size': path.stat().st_size, 'sha256': h.hexdigest()}
manifest = {
    'app_version': '0.0.1',
    'mygo': 'github.com/egoist/mygo v0.7.3',
    'aligner': {'version': 'v0.2.2', 'source': 'https://github.com/corvo007/cpp-ctc-aligner/releases/tag/v0.2.2'},
    'onnxruntime': '1.23.2',
    'model': {'name': 'sherpa-onnx-omnilingual-asr-1600-languages-300M-ctc-int8-2025-11-12', 'source': 'user-provided local SubtitleAgent model'},
    'traditional_dictionary': 'https://github.com/BYVoid/OpenCC/blob/ver.1.1.9/data/dictionary/TSCharacters.txt',
    'files': files,
}
(root / 'third_party/runtime-manifest.json').write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
print(f'Recorded {len(files)} offline runtime files')
