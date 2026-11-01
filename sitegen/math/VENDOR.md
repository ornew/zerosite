# Vendored KaTeX

The files in `sitegen/static/math/` come from the npm package `katex` version
0.19.1, licensed under the MIT license (`LICENSE`). ZeroSite publishes them under
`assets/math/`. This note records how the vendored copy was made, so that it can
be made again. It lives outside `static/` so that the build does not publish it.

## Upstream

| File | Source in the package | sha512 |
|:--|:--|:--|
| `katex-0.19.1.tgz` (npm tarball, not vendored) | `npm pack katex@0.19.1` | `8c6f2e2a7b63484726d84a503a1d17b6eae93eeb82f6bdd2ab91cb1ddb05756653dbb13aef6485d7d4048bea1fd3f61e3b2d7994381184abebe7ad78b91ac47c` |
| `katex.min.js` | `package/dist/katex.min.js`, unchanged | `a9c7b48febf765ee1612da31b31efd41afb21680ff17d2d66e4c29c934e5dbebdd09c097d7c277ca8ce94fef92e4fccdedf8545316637be424faed4b453f1160` |
| `LICENSE` | `package/LICENSE`, unchanged | `6e3904fa8e9c5ddc349c5f6a1646ca4ec4bbfc820a215dd2da6f8bf9197002e2f1863679b12d5994df08bd28829c4c56c244fa75bd54517cd26912222e7fa60e` |
| `fonts/*.woff2` (20 files) | `package/dist/fonts/*.woff2`, unchanged | per file, identical to the package |
| `katex.min.css` (original) | `package/dist/katex.min.css` | `718d354b411053006f5fea8536cefb51072678f01a4e7b132a88a919880e4145f009daad41c5be6e7c60e6bf6216716031412819aa575ce594c3c5a54a821d67` |

## Edit to katex.min.css

The package names three sources for each font: WOFF2, WOFF and TrueType. ZeroSite
ships only the WOFF2 fonts, so the WOFF and TrueType sources are removed. Every
`@font-face` rule keeps its WOFF2 source, and no other byte of the stylesheet
changes. The edit is one `sed` command, run on the original file:

```sh
sed -E 's/,url\(fonts\/[^)]*\.(woff|ttf)\) format\("(woff|truetype)"\)//g' katex.min.css > katex.min.css.new
```

The result has 20 `url(fonts/…woff2)` references, and its sha512 is:

```
84c3984d0ca0c39466f5a4ba4d0808c027d2e4d7c391abad83db3a27ec746588e7b9b2d886109641c4e64c20cb5ca1a372e33f2b41049ad418167784e25ea897
```

## ZeroSite-only stylesheet

`math.css` is not from KaTeX. It holds the layout rule for display formulas and
is linked from the pages that contain a formula, next to `katex.min.css`.
