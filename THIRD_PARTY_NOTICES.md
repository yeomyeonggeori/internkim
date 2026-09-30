# Third-party notices

Code and assets in this repository that come from elsewhere, with the license
each carries. Dependencies installed from a package manager are licensed by
their own packages and are listed in the lockfiles.

## Fonts

The font files under `assets/fonts/files/` are distributed under the SIL Open
Font License 1.1 (https://openfontlicense.org). Each family keeps its
upstream copyright notice; the fonts are not sold by themselves.

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

`assets/fonts/files/MANIFEST.txt` records the exact source and version of each
file.

## Vendored code

### Avatar gradient engine

`web/src/lib/avatar-gradient/` is copied from `@outpacelabs/avatars` 0.6.0
(https://avatars.outpacestudios.com/). MIT License, Copyright (c) 2026 Outpace
Studios. The full text is in `web/src/lib/avatar-gradient/LICENSE`.

### noble-curves and noble-hashes

`internal/admind/site_pb_hooks/passkey-lib.js` is a bundle that contains
`@noble/curves` 1.6.0 and `@noble/hashes` 1.5.0. MIT License, Copyright (c)
2022 Paul Miller (https://paulmillr.com). The bundle keeps the upstream
notices, and its sources are in `internal/admind/site_pb_hooks/source/`.

### Site scaffold

`internal/admind/site_scaffold_dist/react-vite-ts/dist/` is a production build
of a Vite and React starter with Tailwind CSS. React, React DOM and Tailwind
CSS are MIT licensed.
