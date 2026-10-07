# Third-party notices

Code and assets in this repository that come from elsewhere, with the license
each carries. Dependencies installed from a package manager are licensed by
their own packages and are listed in the lockfiles.

## Fonts

The font files under `assets/fonts/files/` and `web/src/lib/fonts/` are
distributed under the SIL Open Font License 1.1 (https://openfontlicense.org).
Each family's directory holds an `OFL.txt` with its copyright notice and the
license text. The fonts are not sold by themselves.

| Family | Directory | Upstream |
| --- | --- | --- |
| A2Z (에이투지체) | `a2z` | https://github.com/Freesentation/A2Z |
| D2Coding | `d2coding` | https://github.com/naver/d2codingfont |
| Galmuri | `galmuri` | https://github.com/quiple/galmuri |
| Gowun Batang | `gowun-batang` | https://github.com/yangheeryu/Gowun-Batang |
| Gowun Dodum | `gowun-dodum` | https://github.com/yangheeryu/Gowun-Dodum |
| MaruBuri (마루부리) | `maruburi` | https://hangeul.naver.com/font/maru |
| Paperlogy | `paperlogy` | https://github.com/Freesentation/paperlogy |
| Pretendard | `pretendard` | https://github.com/orioncactus/pretendard |
| Freesentation (프리젠테이션) | `web/src/lib/fonts` | https://github.com/Freesentation/freesentation |

`assets/fonts/files/MANIFEST.txt` records the exact source and version of each
file.

## Vendored code

### Avatar gradient engine

`web/src/lib/avatar-gradient/` is copied from `@outpacelabs/avatars` 0.6.0
(https://avatars.outpacestudios.com/). MIT License, Copyright (c) 2026 Outpace
Studios. The full text is in `web/src/lib/avatar-gradient/LICENSE`.


## Embedding server and model

The company host package carries a pinned llama.cpp release (MIT License,
Copyright (c) 2023-2026 The ggml authors, https://github.com/ggml-org/llama.cpp)
whose `LICENSE` file stays beside its programs, and the Q8_0 GGUF conversion of
Google's EmbeddingGemma 2 published at
https://huggingface.co/ggml-org/embeddinggemma-2-GGUF, which its model card
licenses under Apache-2.0. `internal/runtime/blueclaw/host_payload_downloads.go` pins both by
version and checksum.
