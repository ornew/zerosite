package sitegen

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ignoreRevsFile lists commits that lastmod skips, in the format read by
// git blame --ignore-revs-file.
const ignoreRevsFile = ".git-blame-ignore-revs"

var objectName = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)

// gitLastModified returns the committer date of the latest commit that changed
// each source, keyed by repository-relative source path. Commits listed in
// .git-blame-ignore-revs are skipped unless every commit that changed a source
// is listed. Sources without a commit are absent from the result.
func gitLastModified(repo string, sources []string) (map[string]string, error) {
	if len(sources) == 0 {
		return map[string]string{}, nil
	}
	top, err := git(repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	shallow, err := git(repo, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return nil, err
	}
	if shallow == "true" {
		return nil, fmt.Errorf("sitemap.lastmod git requires full history, but %s is a shallow clone", repo)
	}
	ignored, err := readIgnoreRevs(filepath.Join(top, ignoreRevsFile))
	if err != nil {
		return nil, err
	}
	wanted := map[string]bool{}
	for _, s := range sources {
		wanted[s] = true
	}
	// -z keeps file names unquoted and NUL-terminates every field. A commit is
	// a \x01 header field followed by its file names; merges list none.
	cmd := exec.Command("git", "log", "-z", "--relative", "--no-renames", "--name-only", "--format=\x01%H %cI")
	cmd.Dir = repo
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("sitemap.lastmod git: %w", err)
	}
	dates, fallback := map[string]string{}, map[string]string{}
	var date string
	var skip bool
	fields := bufio.NewScanner(stdout)
	fields.Buffer(make([]byte, 64*1024), 1024*1024)
	fields.Split(splitNUL)
	for len(dates) < len(wanted) && fields.Scan() {
		field := fields.Text()
		if header, ok := strings.CutPrefix(field, "\x01"); ok {
			hash, when, _ := strings.Cut(header, " ")
			date, skip = when, ignored[hash]
			continue
		}
		// The first file name of a commit follows a newline.
		name := strings.TrimPrefix(field, "\n")
		if !wanted[name] || dates[name] != "" {
			continue
		}
		if !skip {
			dates[name] = date
		} else if fallback[name] == "" {
			fallback[name] = date
		}
	}
	scanErr := fields.Err()
	if len(dates) == len(wanted) || scanErr != nil {
		// Older history cannot change the result, and an unread pipe would
		// block git, so stop it instead of waiting for it to finish.
		cmd.Process.Kill()
		cmd.Wait()
	} else if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("sitemap.lastmod git log: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	if scanErr != nil {
		return nil, fmt.Errorf("sitemap.lastmod git log: %w", scanErr)
	}
	for s, d := range fallback {
		if dates[s] == "" {
			dates[s] = d
		}
	}
	return dates, nil
}

func splitNUL(data []byte, atEOF bool) (int, []byte, error) {
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("sitemap.lastmod git requires a git repository: git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

// readIgnoreRevs accepts one full object name per line; # starts a comment.
func readIgnoreRevs(file string) (map[string]bool, error) {
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return map[string]bool{}, nil
	}
	if err != nil {
		return nil, err
	}
	revs := map[string]bool{}
	for i, line := range strings.Split(string(data), "\n") {
		line, _, _ = strings.Cut(line, "#")
		line = strings.ToLower(strings.TrimSpace(line))
		if line == "" {
			continue
		}
		if !objectName.MatchString(line) {
			return nil, fmt.Errorf("%s:%d: expected a full commit object name, got %q", ignoreRevsFile, i+1, line)
		}
		revs[line] = true
	}
	return revs, nil
}
