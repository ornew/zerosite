package sitegen

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ornew/zerosite/render"
)

func gitRun(t *testing.T, dir string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1", "GIT_AUTHOR_NAME=a", "GIT_AUTHOR_EMAIL=a@example.com", "GIT_COMMITTER_NAME=a", "GIT_COMMITTER_EMAIL=a@example.com")
	cmd.Env = append(cmd.Env, env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func commitAt(t *testing.T, repo, date, message string) string {
	t.Helper()
	gitRun(t, repo, nil, "add", "-A")
	env := []string{"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date}
	gitRun(t, repo, env, "commit", "-q", "-m", message)
	return gitRun(t, repo, nil, "rev-parse", "HEAD")
}

func lastmodFixture(t *testing.T) (string, string, Config, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	repo, out, c := fixture(t)
	c.SiteURL = "https://example.com"
	c.Sitemap.Lastmod = "git"
	c.RecordIndexes = []RecordIndexConfig{{Source: "docs/guide/records/README.md", Directory: "docs/guide/records", Title: "Records", Columns: []render.Column{{Title: "Title", Field: "title"}}}}
	gitRun(t, repo, nil, "init", "-q")
	put(t, repo, "README.md", "# Home\n")
	put(t, repo, "docs/guide/old.md", "# Old\n")
	put(t, repo, "docs/guide/records/001-first.md", "# First\n")
	commitAt(t, repo, "2026-01-02T03:04:05+00:00", "first")
	put(t, repo, "docs/guide/old.md", "# Old\nEdited.\n")
	put(t, repo, "docs/guide/bulk.md", "# Bulk\n")
	put(t, repo, "README.md", "# Home\nEdited.\n")
	bulk := commitAt(t, repo, "2026-02-03T04:05:06+00:00", "bulk")
	put(t, repo, "README.md", "# Home\nEdited again.\n")
	commitAt(t, repo, "2026-03-04T05:06:07+09:00", "home")
	return repo, out, c, bulk
}

func TestSitemapLastmodFromGit(t *testing.T) {
	repo, out, c, bulk := lastmodFixture(t)
	put(t, repo, ".git-blame-ignore-revs", "# bulk rename\n"+strings.ToUpper(bulk)+" # trailing comment\n\n")
	put(t, repo, "docs/guide/new.md", "# Uncommitted\n")
	run(t, repo, out, c)
	sitemap := read(t, out, "sitemap.xml")
	for _, want := range []string{
		"<url><loc>https://example.com/</loc><lastmod>2026-03-04T05:06:07+09:00</lastmod></url>",
		// The ignored commit is skipped in favor of an earlier one.
		"<url><loc>https://example.com/docs/guide/old/</loc><lastmod>2026-01-02T03:04:05Z</lastmod></url>",
		// A source changed only by ignored commits keeps the latest of them.
		"<url><loc>https://example.com/docs/guide/bulk/</loc><lastmod>2026-02-03T04:05:06Z</lastmod></url>",
		"<url><loc>https://example.com/docs/guide/new/</loc></url>",
		"<url><loc>https://example.com/docs/guide/records/</loc></url>",
	} {
		if !strings.Contains(sitemap, want) {
			t.Fatalf("sitemap lacks %s:\n%s", want, sitemap)
		}
	}
}

func TestSitemapLastmodWithoutIgnoreRevs(t *testing.T) {
	repo, out, c, _ := lastmodFixture(t)
	run(t, repo, out, c)
	if !strings.Contains(read(t, out, "sitemap.xml"), "<url><loc>https://example.com/docs/guide/old/</loc><lastmod>2026-02-03T04:05:06Z</lastmod></url>") {
		t.Fatal("latest commit not used without ignored revisions")
	}
}

func TestSitemapLastmodSkipsNoIndexPages(t *testing.T) {
	repo, out, c, _ := lastmodFixture(t)
	put(t, repo, "docs/guide/old.md", "---\nnoIndex: true\n---\n# Old\n")
	commitAt(t, repo, "2026-04-05T06:07:08+00:00", "hide")
	run(t, repo, out, c)
	if strings.Contains(read(t, out, "sitemap.xml"), "/docs/guide/old/") {
		t.Fatal("noindex page in sitemap")
	}
}

func TestSitemapLastmodFailures(t *testing.T) {
	t.Run("value", func(t *testing.T) {
		repo, out, c := fixture(t)
		c.SiteURL = "https://example.com"
		c.Sitemap.Lastmod = "mtime"
		put(t, repo, "README.md", "# Home\n")
		expectError(t, repo, out, c, "sitemap.lastmod must be")
	})
	t.Run("siteURL", func(t *testing.T) {
		repo, out, c := fixture(t)
		c.Sitemap.Lastmod = "git"
		put(t, repo, "README.md", "# Home\n")
		expectError(t, repo, out, c, "requires siteURL")
	})
	t.Run("not a repository", func(t *testing.T) {
		if _, err := exec.LookPath("git"); err != nil {
			t.Skip("git is not installed")
		}
		repo, out, c := fixture(t)
		c.SiteURL = "https://example.com"
		c.Sitemap.Lastmod = "git"
		put(t, repo, "README.md", "# Home\n")
		t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(repo))
		expectError(t, repo, out, c, "requires a git repository")
	})
	t.Run("ignore revs", func(t *testing.T) {
		repo, out, c, bulk := lastmodFixture(t)
		put(t, repo, ".git-blame-ignore-revs", bulk[:12]+"\n")
		expectError(t, repo, out, c, "full commit object name")
	})
	t.Run("shallow", func(t *testing.T) {
		source, _, c, _ := lastmodFixture(t)
		root, err := canonicalPath(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		clone := filepath.Join(root, "clone")
		gitRun(t, root, nil, "clone", "-q", "--depth", "1", "file://"+source, clone)
		expectError(t, clone, filepath.Join(root, "output"), c, "shallow clone")
	})
}

func TestSitemapLastmodNestedRepositoryAndSpecialNames(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	top, err := canonicalPath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(top, "site source")
	gitRun(t, top, nil, "init", "-q")
	put(t, repo, "README.md", "# Home\n")
	put(t, repo, "docs/guide/a \"quoted\"\tname.md", "# Quoted\n")
	put(t, repo, "docs/guide/日本語.md", "# Japanese\n")
	put(t, top, "outside.md", "# Outside\n")
	commitAt(t, top, "2026-05-06T07:08:09+00:00", "first")
	put(t, top, "outside.md", "# Outside\nEdited.\n")
	commitAt(t, top, "2026-06-07T08:09:10+00:00", "outside")
	gitRun(t, top, nil, "checkout", "-q", "-b", "side")
	put(t, top, "side.md", "# Side\n")
	commitAt(t, top, "2026-06-08T00:00:00+00:00", "side")
	gitRun(t, top, nil, "checkout", "-q", "-")
	merge := []string{"GIT_AUTHOR_DATE=2026-06-09T00:00:00+00:00", "GIT_COMMITTER_DATE=2026-06-09T00:00:00+00:00"}
	gitRun(t, top, merge, "merge", "-q", "--no-ff", "-m", "merge", "side")
	c := defaultConfig()
	c.Sections = []Section{{Title: "Overview", Match: []string{"README.md"}}, {Title: "Guides", Match: []string{"docs/guide/**"}}}
	c.SiteURL = "https://example.com"
	c.Sitemap.Lastmod = "git"
	out := filepath.Join(top, "output")
	run(t, repo, out, c)
	sitemap := read(t, out, "sitemap.xml")
	if n := strings.Count(sitemap, "<lastmod>2026-05-06T07:08:09Z</lastmod>"); n != 3 {
		t.Fatalf("got %d lastmod dates from the nested repository, want 3:\n%s", n, sitemap)
	}
}
