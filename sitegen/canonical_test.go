package sitegen

import (
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	canonicalRe = regexp.MustCompile(`<link rel="canonical" href="([^"]*)">`)
	ogURLRe     = regexp.MustCompile(`<meta property="og:url" content="([^"]*)">`)
	hrefRe      = regexp.MustCompile(`<a [^>]*href="([^"]*)"`)
	locRe       = regexp.MustCompile(`<loc>([^<]*)</loc>`)
)

// TestCanonicalSitemapAndLinksAgree builds a site and checks that every
// indexable page has exactly one URL: the canonical tag, og:url and sitemap
// entry are equal, and every internal page link resolves to such a URL.
func TestCanonicalSitemapAndLinksAgree(t *testing.T) {
	repo, out, c := fixture(t)
	c.SiteURL = "https://example.com"
	c.BasePath = "/project/"
	put(t, repo, "README.md", "# Home\n[Guide](docs/guide/README.md)\n[First](docs/guide/first.md#top)\n[Self](#home)\n")
	put(t, repo, "docs/guide/README.md", "# Guide\n[Home](../../README.md)\n[First](first.md)\n")
	put(t, repo, "docs/guide/first.md", "---\naliases: [/old/]\n---\n# First\n## Top\n[Guide](README.md)\n")
	put(t, repo, "docs/guide/draft.md", "---\nnoIndex: true\n---\n# Hidden\n")
	run(t, repo, out, c)

	sitemap := map[string]bool{}
	for _, m := range locRe.FindAllStringSubmatch(read(t, out, "sitemap.xml"), -1) {
		sitemap[m[1]] = true
	}
	if len(sitemap) != 3 {
		t.Fatalf("sitemap lists %v, want home, guide and first", sitemap)
	}
	pages := 0
	err := filepath.WalkDir(out, func(file string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(file, ".html") {
			return err
		}
		rel, _ := filepath.Rel(out, file)
		rel = filepath.ToSlash(rel)
		body := read(t, out, rel)
		if !strings.Contains(body, "<link rel=\"canonical\"") {
			return nil // alias redirect stub or other non-page output
		}
		pages++
		page := "https://example.com/project/" + strings.TrimSuffix(strings.TrimSuffix(rel, "index.html"), "/")
		if strings.HasSuffix(rel, "index.html") && rel != "index.html" {
			page += "/"
		}
		canonical := canonicalRe.FindStringSubmatch(body)[1]
		if canonical != page {
			t.Errorf("%s: canonical %q, want %q", rel, canonical, page)
		}
		if og := ogURLRe.FindStringSubmatch(body); og == nil || og[1] != canonical {
			t.Errorf("%s: og:url %v differs from canonical %q", rel, og, canonical)
		}
		if indexable := !strings.Contains(body, `content="noindex"`); indexable != sitemap[canonical] {
			t.Errorf("%s: indexable=%v but sitemap has it=%v", rel, indexable, sitemap[canonical])
		}
		base, _ := url.Parse(canonical)
		for _, m := range hrefRe.FindAllStringSubmatch(body, -1) {
			ref, err := url.Parse(m[1])
			if err != nil || ref.IsAbs() || ref.Path == "" || strings.HasPrefix(ref.Path, "../assets/") || strings.Contains(ref.Path, "assets/") {
				continue
			}
			target := base.ResolveReference(ref)
			target.Fragment = ""
			if !sitemap[target.String()] && target.String() != "https://example.com/project/docs/guide/draft/" {
				t.Errorf("%s: link %q resolves to %s, which is not a canonical page URL", rel, m[1], target)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if pages != 4 {
		t.Fatalf("checked %d pages, want 4", pages)
	}

	// The file form of each directory URL is redirected, not left to duplicate.
	redirects := read(t, out, "_redirects")
	for _, want := range []string{
		"/project/old/ /project/docs/guide/first/ 301\n",
		"/project/index.html /project/ 301!\n",
		"/project/docs/guide/index.html /project/docs/guide/ 301!\n",
	} {
		if !strings.Contains(redirects, want) {
			t.Errorf("_redirects lacks %q:\n%s", want, redirects)
		}
	}
}

func TestDirectoryLinksDefaultFollowsSiteURL(t *testing.T) {
	repo, out, c := fixture(t)
	put(t, repo, "README.md", "# Home\n[Guide](docs/guide/README.md)\n")
	put(t, repo, "docs/guide/README.md", "# Guide\n")
	run(t, repo, out, c)
	if !strings.Contains(read(t, out, "index.html"), `href="docs/guide/index.html"`) {
		t.Error("without siteURL, links keep the file form")
	}
	if _, err := os.Stat(filepath.Join(out, "_redirects")); err == nil {
		t.Error("without siteURL and redirects, no _redirects is written")
	}
	c.SiteURL = "https://example.com"
	run(t, repo, out, c)
	home := read(t, out, "index.html")
	for _, want := range []string{`href="docs/guide/"`, `class="brand" href="./"`} {
		if !strings.Contains(home, want) {
			t.Errorf("with siteURL, home lacks %s", want)
		}
	}
	if strings.Contains(home, "index.html") {
		t.Error("with siteURL, no link names index.html")
	}
	if !strings.Contains(read(t, out, "docs/guide/index.html"), `class="brand" href="../../"`) {
		t.Error("brand link of a nested page is not the directory URL")
	}
}
