package sitegen

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const mathLoaderLink = `assets/math/math.js"`
const mathStyleLink = `assets/math/katex.min.css"`
const mathRuleLink = `assets/math/math.css"`

// TestMathLoadsOnlyOnPagesWithFormulas checks that math-enabled sites link the KaTeX loader and
// stylesheets from the pages that contain a formula, and only those pages.
func TestMathLoadsOnlyOnPagesWithFormulas(t *testing.T) {
	repo, out, c := fixture(t)
	c.Markdown.Math = true
	put(t, repo, "README.md", "# Home\n\nNo formula here.\n")
	put(t, repo, "docs/proof.md", "# Proof\n\nFor $x_1 \\le n$:\n\n$$\n\\sum_{i=1}^{n} i\n$$\n")
	put(t, repo, "docs/fence.md", "# Fence\n\n```math\na^2 + b^2 = c^2\n```\n")
	run(t, repo, out, c)

	for page, want := range map[string]bool{"index.html": false, "docs/proof/index.html": true, "docs/fence/index.html": true} {
		html := read(t, out, page)
		for _, link := range []string{mathLoaderLink, mathStyleLink, mathRuleLink} {
			if got := strings.Contains(html, link); got != want {
				t.Errorf("%s: %s linked %v, want %v", page, link, got, want)
			}
		}
	}
	if !strings.Contains(read(t, out, "docs/proof/index.html"), `class="math math-display"`) {
		t.Error("display formula not rendered as a math element")
	}
	if !strings.Contains(read(t, out, "docs/fence/index.html"), `class="math math-display"`) {
		t.Error("math fence not rendered as a math element")
	}
}

// TestMathAssetsPublished checks that the vendored KaTeX build and its fonts are published with the
// math option, and that they are not published without it.
func TestMathAssetsPublished(t *testing.T) {
	repo, out, c := fixture(t)
	put(t, repo, "README.md", "# Home\n")
	c.Markdown.Math = true
	run(t, repo, out, c)
	for _, name := range []string{"assets/math/math.js", "assets/math/math.css", "assets/math/katex.min.js", "assets/math/katex.min.css", "assets/math/LICENSE", "assets/math/fonts/KaTeX_Main-Regular.woff2"} {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(name))); err != nil {
			t.Errorf("%s not published: %v", name, err)
		}
	}

	repo, out, c = fixture(t)
	put(t, repo, "README.md", "# Home\n")
	run(t, repo, out, c)
	if _, err := os.Stat(filepath.Join(out, "assets", "math")); !os.IsNotExist(err) {
		t.Errorf("math assets published without the math option: %v", err)
	}
}

// TestMathOptionOffKeepsDollars checks that without the option a dollar sign stays text, with no
// math assets linked.
func TestMathOptionOffKeepsDollars(t *testing.T) {
	repo, out, c := fixture(t)
	put(t, repo, "README.md", "# Home\n")
	put(t, repo, "docs/prices.md", "# Prices\n\nThe cost is $x_1$ and $y_2$.\n")
	run(t, repo, out, c)
	page := read(t, out, "docs/prices/index.html")
	if !strings.Contains(page, "$x_1$") || strings.Contains(page, "math-inline") || strings.Contains(page, "math.js") {
		t.Fatalf("dollars changed without the math option: %s", page)
	}
}

// TestMathOptionKeepsOtherPagesIdentical checks that enabling math changes only the pages with a
// formula: a page without one is byte-identical with and without the option.
func TestMathOptionKeepsOtherPagesIdentical(t *testing.T) {
	repo, out, c := fixture(t)
	put(t, repo, "README.md", "# Home\n\nText with `$x$` in code, \\$5 and *emphasis*.\n")
	put(t, repo, "docs/proof.md", "# Proof\n\nFor $x$.\n")
	run(t, repo, out, c)
	without := read(t, out, "index.html")

	c.Markdown.Math = true
	run(t, repo, out, c)
	if got := read(t, out, "index.html"); got != without {
		t.Fatalf("math option changed a page without a formula:\n%s\n---\n%s", without, got)
	}
	if !bytes.Contains([]byte(read(t, out, "docs/proof/index.html")), []byte(`class="math math-inline"`)) {
		t.Fatal("formula page not rendered with the option")
	}
}

// TestMathOptionKeepsSiteCSS checks that the site stylesheet is the embedded base with the option
// off and on: the formula rule lives in assets/math/math.css, which only formula pages link.
func TestMathOptionKeepsSiteCSS(t *testing.T) {
	base, err := embedded.ReadFile("static/site.css")
	if err != nil {
		t.Fatal(err)
	}
	repo, out, c := fixture(t)
	put(t, repo, "README.md", "# Home\n")
	run(t, repo, out, c)
	if got := read(t, out, "assets/site.css"); got != string(base) {
		t.Fatal("site stylesheet differs from the embedded base with the option off")
	}

	c.Markdown.Math = true
	put(t, repo, "docs/proof.md", "# Proof\n\nFor $x$.\n")
	run(t, repo, out, c)
	if got := read(t, out, "assets/site.css"); got != string(base) {
		t.Fatal("site stylesheet differs from the embedded base with the option on")
	}
	if !strings.Contains(read(t, out, "assets/math/math.css"), "div.math-display") {
		t.Fatal("math stylesheet lacks the display rule")
	}
}

// TestMathAssetURLs checks that every url() in the published KaTeX stylesheet names a file that the
// build publishes, and that only WOFF2 fonts are named.
func TestMathAssetURLs(t *testing.T) {
	repo, out, c := fixture(t)
	put(t, repo, "README.md", "# Home\n")
	c.Markdown.Math = true
	run(t, repo, out, c)
	css := read(t, out, "assets/math/katex.min.css")
	urls := regexp.MustCompile(`url\(([^)]+)\)`).FindAllStringSubmatch(css, -1)
	if len(urls) == 0 {
		t.Fatal("the KaTeX stylesheet names no resource")
	}
	for _, u := range urls {
		target := path.Join("assets/math", u[1])
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(target))); err != nil {
			t.Errorf("url(%s) does not resolve to a published file: %v", u[1], err)
		}
		if !strings.HasSuffix(u[1], ".woff2") {
			t.Errorf("url(%s): only WOFF2 fonts are published", u[1])
		}
	}
}

// TestMathSearchText checks that a formula's TeX source is in the search index of its page, in the
// body and in a heading, for both the plain-text and the HTML index.
func TestMathSearchText(t *testing.T) {
	for _, mode := range []string{"", "html"} {
		repo, out, c := fixture(t)
		c.SearchIndex.Text = mode
		c.Markdown.Math = true
		put(t, repo, "README.md", "# Home\n")
		put(t, repo, "docs/search.md", "# The $HeadTok_1$ rule\n\nBody $\\alpha+\\beta$ here.\n\n$$\n\\sum_i s_i\n$$\n")
		run(t, repo, out, c)
		raw := strings.TrimSuffix(strings.TrimPrefix(read(t, out, "assets/search-index.js"), "globalThis.ZEROSITE_SEARCH = "), ";\n")
		var entries []map[string]any
		if err := json.Unmarshal([]byte(raw), &entries); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, e := range entries {
			if !strings.Contains(e["title"].(string), "rule") {
				continue
			}
			found = true
			all := ""
			for _, v := range e {
				all += strings.TrimSpace(strings.Join(strings.Fields(toText(v)), " ")) + " "
			}
			for _, want := range []string{"HeadTok_1", `\alpha+\beta`, `\sum_i s_i`} {
				if !strings.Contains(all, want) {
					t.Errorf("search mode %q: entry lacks %q: %s", mode, want, all)
				}
			}
		}
		if !found {
			t.Fatalf("search mode %q: entry for the formula page is missing in %s", mode, raw)
		}
	}
}

// toText flattens a search entry field, which may be a string or a list of strings.
func toText(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		var b strings.Builder
		for _, y := range x {
			b.WriteString(toText(y))
			b.WriteByte(' ')
		}
		return b.String()
	}
	return ""
}
