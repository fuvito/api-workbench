package doccheck

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// The version the app reports (the newest entry in help/changelog.md, which the About dialog shows) and the version in
// the user guide's header are the same, so a release cannot ship with one of them forgotten.
func TestVersionsAgree(t *testing.T) {
	root := filepath.Join("..", "..", "..", "apps", "desktop")
	about, err := os.ReadFile(filepath.Join(root, "wails", "help", "changelog.md"))
	if err != nil {
		t.Fatal(err)
	}
	guide, err := os.ReadFile(filepath.Join(root, "wails", "help", "user-guide.md"))
	if err != nil {
		t.Fatal(err)
	}
	app := regexp.MustCompile(`(?m)^## (\d+\.\d+\.\d+) — `).FindSubmatch(about)
	doc := regexp.MustCompile(`(?m)^\*\*Version:\*\* (\d+\.\d+\.\d+)`).FindSubmatch(guide)
	if app == nil || doc == nil {
		t.Fatalf("the version was not found: changelog %v, user guide %v", app != nil, doc != nil)
	}
	if string(app[1]) != string(doc[1]) {
		t.Errorf("help/changelog.md says %s and the user guide says %s: bump both for a release", app[1], doc[1])
	}
}
