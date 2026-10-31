package render

import (
	"bytes"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Math in Markdown, enabled by Options.Math. The delimiters follow GitHub's, so a formula reads the
// same on both:
//
//   - Inline math is $…$ within one line. The opening $ must not be followed by a space or the end of
//     the line; the closing $ must not be preceded by a space nor followed by a digit. $$…$$ on one line
//     is inline math too. A run of three or more dollars is literal text.
//   - Display math is a paragraph that begins with $$ and ends with $$; the usual form puts each
//     delimiter alone on its line. The content must not contain $$ (otherwise the paragraph is inline
//     math, as on GitHub). A ```math fence is display math as well.
//   - A dollar escaped by an odd number of backslashes is literal: \$ in prose.
//   - Inside math, the content is TeX and reaches KaTeX verbatim: no Markdown emphasis, escapes, links
//     or HTML apply. A backslash escapes the next character for the purpose of finding the delimiters
//     only, so \$ inside math is a TeX dollar and does not close the formula, and \\ does not escape the
//     delimiter that follows it.
//   - A dollar inside a code span is never a delimiter: the scan for the closing delimiter skips code
//     spans.
//
// The renderer writes <span class="math math-inline"> and <div class="math math-display">, with the
// TeX content escaped as HTML text. The published page loads the vendored KaTeX, which renders these
// elements; without scripts the TeX source stays visible.

// kindInlineMath and kindDisplayMath are the kinds of the math nodes.
var (
	kindInlineMath  = ast.NewNodeKind("MathInline")
	kindDisplayMath = ast.NewNodeKind("MathDisplay")
)

// inlineMath is a formula within a line of a paragraph.
type inlineMath struct {
	ast.BaseInline
	content []byte
}

func (n *inlineMath) Kind() ast.NodeKind { return kindInlineMath }

func (n *inlineMath) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"Content": string(n.content)}, nil)
}

// Text returns the TeX source, so that a heading with a formula keeps its words for the anchor.
func (n *inlineMath) Text(source []byte) []byte { return n.content }

// displayMath is a formula of its own block.
type displayMath struct {
	ast.BaseBlock
	content []byte
}

func (n *displayMath) Kind() ast.NodeKind { return kindDisplayMath }

func (n *displayMath) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{"Content": string(n.content)}, nil)
}

// mathExtension adds inline and display math to a goldmark Markdown instance.
type mathExtension struct{}

func (mathExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithInlineParsers(util.Prioritized(mathInlineParser{}, 500)),
		parser.WithASTTransformers(util.Prioritized(mathBlockTransformer{}, 90)),
	)
	m.Renderer().AddOptions(renderer.WithNodeRenderers(util.Prioritized(mathRenderer{}, 500)))
}

// mathInlineParser parses $…$ and $$…$$ on one line. It is triggered by each $ and returns nil, which
// leaves the dollar as text, unless the delimiters match.
type mathInlineParser struct{}

func (mathInlineParser) Trigger() []byte { return []byte{'$'} }

func (mathInlineParser) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, seg := block.PeekLine()
	src := block.Source()
	at := seg.Start
	if len(line) == 0 || line[0] != '$' || escaped(src, at) {
		return nil
	}
	// The second and later dollars of a run are not openers; the run is decided at its first dollar.
	if at > 0 && src[at-1] == '$' && !escaped(src, at-1) {
		return nil
	}
	n := runLength(line, 0, '$')
	if n > 2 || n >= len(line) || isSpace(line[n]) {
		return nil
	}
	for j := n; j < len(line); {
		switch c := line[j]; c {
		case '`':
			m := runLength(line, j, '`')
			if end := codeSpanEnd(line, j+m, m); end >= 0 {
				j = end
			} else {
				j += m
			}
		case '\\':
			// Skip the escaped character, so that \$ is not a delimiter and \\ leaves its successor
			// free to be one.
			j += 2
		case '$':
			k := runLength(line, j, '$')
			if k == n && j > n && !isSpace(line[j-1]) && !(j+k < len(line) && isDigit(line[j+k])) {
				block.Advance(j + k)
				return &inlineMath{content: bytes.Clone(line[n:j])}
			}
			j += k
		default:
			j++
		}
	}
	return nil
}

// escaped reports whether the character at i is preceded by an odd number of backslashes.
func escaped(src []byte, i int) bool {
	n := 0
	for j := i - 1; j >= 0 && src[j] == '\\'; j-- {
		n++
	}
	return n%2 == 1
}

// runLength returns the number of consecutive bytes equal to c starting at i.
func runLength(b []byte, i int, c byte) int {
	n := 0
	for i+n < len(b) && b[i+n] == c {
		n++
	}
	return n
}

// codeSpanEnd returns the index after the first run of exactly m backticks at or after from, or -1.
func codeSpanEnd(b []byte, from, m int) int {
	for j := from; j < len(b); {
		if b[j] != '`' {
			j++
			continue
		}
		k := runLength(b, j, '`')
		if k == m {
			return j + k
		}
		j += k
	}
	return -1
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

// mathBlockTransformer turns display formulas into displayMath blocks. It works on the raw source of
// the paragraph, which is taken before any inline parsing changes it.
type mathBlockTransformer struct{}

func (mathBlockTransformer) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	type replacement struct {
		old ast.Node
		new *displayMath
	}
	var repl []replacement
	_ = ast.Walk(doc, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		if !enter {
			return ast.WalkContinue, nil
		}
		switch b := n.(type) {
		case *ast.Paragraph:
			if content, ok := displayContent(segmentsSource(b.Lines(), src)); ok {
				repl = append(repl, replacement{b, &displayMath{content: content}})
			}
		case *ast.TextBlock:
			// A paragraph of a tight list is a text block.
			if content, ok := displayContent(segmentsSource(b.Lines(), src)); ok {
				repl = append(repl, replacement{b, &displayMath{content: content}})
			}
		case *ast.FencedCodeBlock:
			if string(b.Language(src)) == "math" {
				repl = append(repl, replacement{b, &displayMath{content: codeLines(b, src)}})
			}
		}
		return ast.WalkContinue, nil
	})
	for _, r := range repl {
		parent := r.old.Parent()
		parent.ReplaceChild(parent, r.old, r.new)
	}
}

// displayContent returns the formula of a paragraph that begins and ends with $$. The rules apply to
// the content without its surrounding white space: it is not empty, it contains no $$, and it neither
// begins nor ends with an unescaped dollar, which would make the delimiters ambiguous. The closing
// $$ must not be escaped, so $$\$$ is text. The returned content keeps its white space.
func displayContent(t []byte) ([]byte, bool) {
	if len(t) < 4 || !bytes.HasPrefix(t, []byte("$$")) || !bytes.HasSuffix(t, []byte("$$")) || escaped(t, len(t)-2) {
		return nil, false
	}
	content := t[2 : len(t)-2]
	trimmed := bytes.TrimSpace(content)
	last := len(trimmed) - 1
	if len(trimmed) == 0 || bytes.Contains(trimmed, []byte("$$")) ||
		trimmed[0] == '$' || (trimmed[last] == '$' && !escaped(trimmed, last)) {
		return nil, false
	}
	return bytes.Clone(content), true
}

// segmentsSource returns the raw text of the lines of a paragraph, without the spaces around it and
// with CRLF line endings read as LF.
func segmentsSource(lines *text.Segments, src []byte) []byte {
	var b bytes.Buffer
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.Write(seg.Value(src))
	}
	return bytes.TrimSpace(bytes.ReplaceAll(b.Bytes(), []byte("\r\n"), []byte("\n")))
}

// codeLines returns the verbatim lines of a fenced code block, with CRLF line endings read as LF and
// without the final line break.
func codeLines(b *ast.FencedCodeBlock, src []byte) []byte {
	var out bytes.Buffer
	lines := b.Lines()
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		out.Write(seg.Value(src))
	}
	return bytes.TrimSuffix(bytes.ReplaceAll(out.Bytes(), []byte("\r\n"), []byte("\n")), []byte("\n"))
}

// MathSource returns the TeX source of a formula node, and false for any other node. Consumers that
// index the text of a page use it to keep formulas.
func MathSource(n ast.Node) ([]byte, bool) {
	switch m := n.(type) {
	case *inlineMath:
		return m.content, true
	case *displayMath:
		return m.content, true
	}
	return nil, false
}

// mathRenderer writes the math nodes with their TeX content escaped as HTML text.
type mathRenderer struct{}

func (mathRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindInlineMath, renderInlineMath)
	reg.Register(kindDisplayMath, renderDisplayMath)
}

func renderInlineMath(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<span class="math math-inline">`)
		_, _ = w.Write(util.EscapeHTML(n.(*inlineMath).content))
		_, _ = w.WriteString(`</span>`)
	}
	return ast.WalkSkipChildren, nil
}

func renderDisplayMath(w util.BufWriter, _ []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString(`<div class="math math-display">`)
		_, _ = w.Write(util.EscapeHTML(n.(*displayMath).content))
		_, _ = w.WriteString("</div>\n")
	}
	return ast.WalkSkipChildren, nil
}

// HasMath reports whether rendered HTML contains a formula that the math extension wrote. A page
// with no formula loads none of the math assets.
func HasMath(html string) bool { return strings.Contains(html, `class="math math-`) }
