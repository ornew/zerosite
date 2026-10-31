package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// renderMath renders Markdown with the math extension alone.
func renderMath(t *testing.T, src string) string {
	t.Helper()
	md := goldmark.New(goldmark.WithExtensions(mathExtension{}))
	var b bytes.Buffer
	if err := md.Convert([]byte(src), &b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func inline(content string) string {
	return `<span class="math math-inline">` + content + `</span>`
}

func display(content string) string {
	return `<div class="math math-display">` + content + "</div>\n"
}

// TestMathInline checks the delimiter rules of $…$ and $$…$$ within a line, both where a formula is
// found and where the dollars stay text.
func TestMathInline(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"simple", "a $x$ b", "<p>a " + inline("x") + " b</p>\n"},
		{"two formulas", "$a$ and $b$", "<p>" + inline("a") + " and " + inline("b") + "</p>\n"},
		{"punctuation after the closing dollar", "$x$.", "<p>" + inline("x") + ".</p>\n"},
		{"opener followed by a digit", "$5 and 6$", "<p>" + inline("5 and 6") + "</p>\n"},
		{"TeX is verbatim", "$a_b*c$", "<p>" + inline("a_b*c") + "</p>\n"},
		{"backslashes and HTML are verbatim", `$\alpha <b> & "q"$`, "<p>" + inline(`\alpha &lt;b&gt; &amp; &quot;q&quot;`) + "</p>\n"},
		{"TeX dollar inside math", `$x\$y$`, "<p>" + inline(`x\$y`) + "</p>\n"},
		{"even backslashes do not escape the closer", `$x\\$ z`, "<p>" + inline(`x\\`) + " z</p>\n"},
		{"emphasis outside the formula", "*a $x$ b*", "<p><em>a " + inline("x") + " b</em></p>\n"},
		{"double dollar on one line is inline", "a $$b$$ c", "<p>a " + inline("b") + " c</p>\n"},
		{"two double-dollar formulas", "$$x$$ and $$y$$", "<p>" + inline("x") + " and " + inline("y") + "</p>\n"},
		{"two dollars inside a formula are not a closer", "$x$$y$", "<p>" + inline("x$$y") + "</p>\n"},
		{"backslash-escaped opener is text", `\$x$`, "<p>$x$</p>\n"},
		{"escaped dollar in prose", `\$5 costs`, "<p>$5 costs</p>\n"},
		{"escaped dollar before a formula", `\$5 and $x$`, "<p>$5 and " + inline("x") + "</p>\n"},
		{"space after the opener", "$ x $", "<p>$ x $</p>\n"},
		{"space before the closer", "$x $", "<p>$x $</p>\n"},
		{"digit after the closer", "$x$5", "<p>$x$5</p>\n"},
		{"closer preceded by a space", "costs $5 and $6", "<p>costs $5 and $6</p>\n"},
		{"unclosed opener", "a $ b", "<p>a $ b</p>\n"},
		{"formula does not cross a line", "$a\nb$", "<p>$a\nb$</p>\n"},
		{"formula on each line", "a $x$ and\n$y$ next", "<p>a " + inline("x") + " and\n" + inline("y") + " next</p>\n"},
		{"run of three is text", "$$$x$$$", "<p>$$$x$$$</p>\n"},
		{"dollar inside a code span is not a closer", "$a `b$` c", "<p>$a <code>b$</code> c</p>\n"},
		{"code span opened first keeps its dollars", "a `$x` b$", "<p>a <code>$x</code> b$</p>\n"},
		{"code span is not math", "`$x$`", "<p><code>$x$</code></p>\n"},
		{"text without dollars is unchanged", "a *b* `c` [d](e)", "<p>a <em>b</em> <code>c</code> <a href=\"e\">d</a></p>\n"},
		{"end-of-input marker in prose is text", "the rule ends at $$ and $$ marks the end", "<p>the rule ends at $$ and $$ marks the end</p>\n"},
		{"end-of-input marker in a code span is code", "a `$$` marker and `$name`", "<p>a <code>$$</code> marker and <code>$name</code></p>\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := renderMath(t, c.src); got != c.want {
				t.Errorf("renderMath(%q)\n got %q\nwant %q", c.src, got, c.want)
			}
		})
	}
}

// TestMathDisplay checks the paragraphs and fences that are display formulas.
func TestMathDisplay(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"delimiters on their own lines", "$$\nx = 1\n$$", display("\nx = 1\n")},
		{"one-line paragraph", "$$x$$", display("x")},
		{"spaces inside the delimiters", "$$ x $$", display(" x ")},
		{"verbatim TeX", "$$\n\\alpha <b> & _x_\n$$", display("\n\\alpha &lt;b&gt; &amp; _x_\n")},
		{"escaped dollars are TeX", "$$\\$x\\$$$", display(`\$x\$`)},
		{"math fence", "```math\nx^2_1 <\n```", display("x^2_1 &lt;")},
		{"math fence keeps a blank line", "```math\na\n\nb\n```", display("a\n\nb")},
		{"math fence with dollars", "```math\n$x$\n```", display("$x$")},
		{"delimiters inside a sentence are inline", "$$x$$ tail", "<p>" + inline("x") + " tail</p>\n"},
		{"trailing text after the closing delimiter", "$$\nx\n$$ tail", "<p>$$\nx\n$$ tail</p>\n"},
		{"text before the opening delimiter", "text\n$$\nx\n$$", "<p>text\n$$\nx\n$$</p>\n"},
		{"empty formula", "$$$$", "<p>$$$$</p>\n"},
		{"two display-like formulas in one paragraph", "$$x$$y$$", "<p>" + inline("x") + "y$$</p>\n"},
		{"escaped opener is text", `\$\$x\$\$`, "<p>$$x$$</p>\n"},
		{"other fence languages stay code", "```text\n$x$\n```", "<pre><code class=\"language-text\">$x$\n</code></pre>\n"},
		{"display formula in a list item", "- $$\n  x\n  $$", "<ul>\n<li>\n" + display("\nx\n") + "</li>\n</ul>\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := renderMath(t, c.src); got != c.want {
				t.Errorf("renderMath(%q)\n got %q\nwant %q", c.src, got, c.want)
			}
		})
	}
}

// TestMathHeadingText checks that a heading keeps the TeX source of a formula in its text, so that
// its anchor and search text carry the words of the heading.
func TestMathHeadingText(t *testing.T) {
	md := goldmark.New(goldmark.WithExtensions(mathExtension{}))
	doc := md.Parser().Parse(text.NewReader([]byte("## The $x_1$ lemma\n")))
	var heading *ast.Heading
	_ = ast.Walk(doc, func(n ast.Node, enter bool) (ast.WalkStatus, error) {
		if h, ok := n.(*ast.Heading); ok && enter {
			heading = h
		}
		return ast.WalkContinue, nil
	})
	if heading == nil {
		t.Fatal("no heading")
	}
	if got, want := string(heading.Text([]byte("## The $x_1$ lemma\n"))), "The x_1 lemma"; got != want {
		t.Errorf("heading text = %q, want %q", got, want)
	}
}

// TestMathSourceUnchanged checks that the extension leaves Markdown without dollars rendering exactly
// as goldmark renders it, for the constructs of the documentation.
func TestMathSourceUnchanged(t *testing.T) {
	src := strings.Join([]string{
		"# Title",
		"",
		"Text with *emphasis*, `code`, [a link](https://example.com) and a\\_escaped underscore.",
		"",
		"```go",
		"x := 1",
		"```",
		"",
		"```pego",
		"list <- $item:item ($sep:\",\" $item:item)* $$",
		"```",
		"",
		"A rule such as `list <- $item:item $$` ends with the end-of-input marker.",
		"",
		"| a | b |",
		"|---|---|",
		"| 1 | 2 |",
	}, "\n")
	plain := goldmark.New()
	var want bytes.Buffer
	if err := plain.Convert([]byte(src), &want); err != nil {
		t.Fatal(err)
	}
	if got := renderMath(t, src); got != want.String() {
		t.Errorf("math extension changed Markdown without dollars:\n got %q\nwant %q", got, want.String())
	}
}

// TestMathDisplayEnds checks that the display rules apply to the content without its surrounding
// white space, and that an escaped closing $$ leaves the paragraph as text.
func TestMathDisplayEnds(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"trailing dollar after a line break", "$$\nx $\n$$", "<p>$$\nx $\n$$</p>\n"},
		{"leading dollar after a line break", "$$\n$x\n$$", "<p>$$\n$x\n$$</p>\n"},
		{"escaped closing delimiter", `$$\$$`, "<p>$$$$</p>\n"},
		{"even backslashes before the closer", `$$x\\$$`, display(`x\\`)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := renderMath(t, c.src); got != c.want {
				t.Errorf("renderMath(%q)\n got %q\nwant %q", c.src, got, c.want)
			}
		})
	}
}

// TestMathCRLF checks that CRLF line endings in a fence or a display paragraph do not reach the
// formula as carriage returns.
func TestMathCRLF(t *testing.T) {
	if got, want := renderMath(t, "```math\r\na\r\nb\r\n```\r\n"), display("a\nb"); got != want {
		t.Errorf("CRLF fence\n got %q\nwant %q", got, want)
	}
	if got, want := renderMath(t, "$$\r\nx\r\n$$\r\n"), display("\nx\n"); got != want {
		t.Errorf("CRLF display paragraph\n got %q\nwant %q", got, want)
	}
}

// TestMathTableCell checks that a pipe inside inline math splits a table cell, as on GitHub, so the
// formula is not recognized across the cells.
func TestMathTableCell(t *testing.T) {
	md := goldmark.New(goldmark.WithExtensions(extension.Table, mathExtension{}))
	var b bytes.Buffer
	if err := md.Convert([]byte("| a | b |\n| --- | --- |\n| $x|y$ | c |\n"), &b); err != nil {
		t.Fatal(err)
	}
	got := b.String()
	if strings.Contains(got, `class="math`) || !strings.Contains(got, "<td>$x</td>") || !strings.Contains(got, "<td>y$</td>") {
		t.Errorf("pipe inside math did not split the cell:\n%s", got)
	}
}
