# Markdown math

This chapter defines the formulas that `markdown.math` enables: their syntax,
the HTML they produce, the assets a published site carries, and the pages that
load them. The option is off by default; with it off, a dollar sign is text and
no math asset is published.

## Scope and terminology

A formula is TeX source between dollar signs. An inline formula is part of a
paragraph; a display formula is a block of its own. The rules follow the
delimiters of GitHub's Markdown renderer, so a formula written for GitHub reads
the same on the site, with the deviations listed under Contract.

## Contract

Set `markdown.math` to `true` to enable the syntax below. The option is a
boolean and defaults to `false`.

Inline math:

- `$…$` within one line. The opening dollar MUST NOT be followed by a space,
  tab, or the end of the line. The closing dollar MUST NOT be preceded by a
  space or tab and MUST NOT be followed by an ASCII digit.
- `$$…$$` within one line is inline math as well.
- The formula MUST NOT be empty and MUST NOT cross a line break.
- A `|` inside inline math splits a table cell, as on GitHub, so the formula
  does not span cells.

Display math:

- A paragraph that begins with `$$` and ends with `$$` is a display formula. The
  usual form puts each delimiter alone on its line. The content, without its
  surrounding white space, MUST NOT be empty and MUST NOT contain `$$`, and it
  MUST NOT begin or end with an unescaped dollar sign. The closing `$$` MUST NOT
  be escaped: `$$\$$` is text, not a formula.
- A fenced code block whose info string is `math` is a display formula. Its
  content is the formula.
- A display formula is a block: a `$$` paragraph that is preceded by other text
  in the same paragraph is not display math.

Dollar signs that are not delimiters:

- A dollar sign preceded by an odd number of backslashes is text; `\$` writes a
  literal dollar sign in prose.
- A run of three or more dollar signs is text.
- A dollar sign inside a code span is never a delimiter. The scan for a closing
  delimiter skips code spans, and a code span that starts before an opening
  delimiter keeps its contents.
- Every other dollar sign that does not satisfy these rules is text.

Formula content:

- The content is TeX and reaches the browser verbatim. Emphasis, links, HTML
  and other Markdown syntax do not apply inside a formula.
- Within a formula, a backslash escapes the next character only for finding the
  delimiters: `\$` is a TeX dollar and does not close the formula, and `\\` does
  not escape the delimiter that follows it.
- Leading indentation of the continuation lines of a paragraph, which belongs
  to the container syntax, is not part of the content.

Deviations from GitHub, which renders some formulas differently: inside a
formula, GitHub treats `\$` as the closing delimiter and processes Markdown
escapes and emphasis first; Zerosite does neither. GitHub also unescapes `\$`
before it looks for delimiters, so `\$x\$` renders as a formula there. Zerosite
keeps `\$x\$` as text.

## Output

Each inline formula is written as

```html
<span class="math math-inline">TeX</span>
```

and each display formula as

```html
<div class="math math-display">TeX</div>
```

The TeX is HTML-escaped, so `<`, `&` and `"` are text in the source and the
browser reads them back verbatim. A heading keeps the TeX of its formulas as the
text used for its anchor and for search.

## Assets and page loading

When `markdown.math` is `true`, the build publishes the following files, whether
or not a page uses them:

| Output path | Content |
|:--|:--|
| `assets/math/math.js` | Loader script that renders the marked elements |
| `assets/math/math.css` | Layout of display formulas |
| `assets/math/katex.min.js` | KaTeX 0.19.1 minified build |
| `assets/math/katex.min.css` | KaTeX 0.19.1 stylesheet, with its WOFF and TrueType sources removed |
| `assets/math/fonts/*.woff2` | The WOFF2 fonts the stylesheet names |
| `assets/math/LICENSE` | The MIT license of KaTeX |

Every `url()` in `katex.min.css` names one of the published fonts. The
stylesheet was edited from the upstream package as described in the vendor note
of the Zerosite source tree, `sitegen/math/VENDOR.md`.

The files are published even when `bundledAssets` is `false`, because they
belong to the option, not to the default theme.

A page loads `katex.min.css`, `math.css` and `math.js` if and only if its
rendered body contains a formula element. A page without a formula links none of
them, and its HTML and its `assets/site.css` are the same with the option on or
off. The stylesheets and `math.js` are linked as ordinary `styles` and `scripts`
entries of the page; `math.js` is loaded with `defer`.

`math.js` renders each `span.math-inline` and `div.math-display` element with
KaTeX, reading its TeX from the text of the element. A formula that KaTeX
rejects keeps its TeX source as text and logs the error to the console; the
build does not fail.

## Validation and errors

Syntax is never an error: a dollar sign that does not form a formula stays
text. A page with a formula does not fail the build when KaTeX rejects it at
render time. Enabling the option fails only if the published asset names collide
with a page or asset of the site, as for any generated asset.

## Compatibility and support

Enabling the option changes only the pages that contain a formula, and the
search index text of those pages. The search index keeps the TeX source of each
formula, in the body and in headings, so a search finds the symbols as written. A custom `template` replaces the layout, so
it must output the page's `Styles` and `Scripts`; otherwise the formulas remain
their TeX source.

The assets are same-origin, so the content security policy needs no new origin.
KaTeX's HTML output sets its styles through the CSS object model. A few
MathML-only attributes set a `style` attribute; a strict `style-src` blocks
those, with no visible effect on the HTML rendering of the formulas.

## Examples

With `markdown.math` set to `true`, this paragraph:

```markdown
The sum of the first $n$ terms is $S_n$.

$$
S_n = \sum_{i=1}^{n} i
$$
```

produces one inline element and one display element; the page links the
stylesheet and `math.js`. The paragraph below stays text, because the dollar
sign before `5` closes nothing:

```markdown
It costs $5 and $6 today.
```

## Conformance

The `render` package tests the delimiter rules and the HTML of formulas
(`TestMathInline`, `TestMathDisplay`, `TestMathHeadingText`,
`TestMathSourceUnchanged`, `TestMathDisplayEnds`, `TestMathCRLF`,
`TestMathTableCell`). The `sitegen` package tests page loading, asset
publication, the published URLs of the KaTeX stylesheet, the search text of
formulas, the option being off, and unchanged output of pages without a formula
(`TestMathLoadsOnlyOnPagesWithFormulas`, `TestMathAssetsPublished`,
`TestMathAssetURLs`, `TestMathSearchText`, `TestMathOptionOffKeepsDollars`,
`TestMathOptionKeepsOtherPagesIdentical`, `TestMathOptionKeepsSiteCSS`).

## Implementation status

Implemented in `render` and `sitegen` for every page built from Markdown,
including design records and record indexes. Not implemented: math in
templates written outside Markdown, and TeX macro packages beyond what KaTeX
0.19.1 provides.
