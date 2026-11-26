package doccheck

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// currentDocs calls fn for every Markdown document that tells a reader what to do today. Reports,
// plans, proposals, the archive, the both changelogs and release note drafts are history.
func currentDocs(t *testing.T, fn func(rel, text string)) {
	t.Helper()
	root := filepath.Join("..", "..", "..")
	history := []string{"docs/reports/", "docs/planning/", "docs/enhancements/", "docs/archive/",
		"docs/process/engineering-changelog.md", "docs/process/release-notes-",
		"apps/desktop/wails/help/changelog.md"}
	count := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "dist", "build", "wailsjs", "test-results", "playwright-report":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(path), ".md") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		for _, h := range history {
			if strings.HasPrefix(rel, h) {
				return nil
			}
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		count++
		fn(rel, string(data))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count < 10 {
		t.Fatalf("only %d documents were read; the walk is not finding them", count)
	}
}

var reActionRef = regexp.MustCompile(`fuvito/api-workbench(?:/clrsig-action)?@([A-Za-z0-9._-]+)`)

// Every Action reference a reader can copy names the release this tree is. A reference such as @v1,
// for which no tag exists, fails the workflow before it starts. This compares with the version in
// the changelog, not with git tags, because CI checks out without tags.
func TestDocumentedActionRefsNameThisVersion(t *testing.T) {
	about, err := os.ReadFile(filepath.Join("..", "..", "..", "apps", "desktop", "wails", "help", "changelog.md"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^## (\d+\.\d+\.\d+) — `).FindSubmatch(about)
	if m == nil {
		t.Fatal("the version was not found in help/changelog.md")
	}
	want := "v" + string(m[1])
	found := 0
	currentDocs(t, func(rel, text string) {
		for i, line := range strings.Split(text, "\n") {
			for _, ref := range reActionRef.FindAllStringSubmatch(line, -1) {
				found++
				if got := strings.TrimRight(ref[1], "."); got != want {
					t.Errorf("%s:%d uses the Action at @%s; write @%s", rel, i+1, got, want)
				}
			}
		}
	})
	if found == 0 {
		t.Fatal("no Action reference was found, so nothing was checked")
	}
}

var reCommonAction = regexp.MustCompile(`actions/(checkout|upload-artifact|setup-go)@(v\d+)`)

// The workflows we show and generate use the same major versions as the ones we run ourselves.
func TestShownWorkflowsUseTheSameActionVersionsAsCI(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	ours := map[string]string{}
	for _, name := range []string{"ci.yml", "release.yml"} {
		data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", name))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range reCommonAction.FindAllStringSubmatch(string(data), -1) {
			if v, ok := ours[m[1]]; ok && v != m[2] {
				t.Errorf("our workflows use actions/%s at both %s and %s", m[1], v, m[2])
			}
			ours[m[1]] = m[2]
		}
	}
	check := func(rel, text string) {
		for _, m := range reCommonAction.FindAllStringSubmatch(text, -1) {
			if want := ours[m[1]]; want != m[2] {
				t.Errorf("%s shows actions/%s@%s, but our workflows use %s", rel, m[1], m[2], want)
			}
		}
	}
	currentDocs(t, check)
	generator, err := os.ReadFile(filepath.Join(root, "packages", "core", "scaffold", "workflow.go"))
	if err != nil {
		t.Fatal(err)
	}
	check("packages/core/scaffold/workflow.go", string(generator))
}
