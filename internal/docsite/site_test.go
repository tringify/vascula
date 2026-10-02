package docsite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildPublishesEveryPage(t *testing.T) {
	out := filepath.Join(t.TempDir(), "site")
	if err := Build("../..", out); err != nil {
		t.Fatal(err)
	}
	pages, _, err := Load("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pages {
		for _, name := range []string{p.Slug + ".html", p.Slug + ".md"} {
			if _, err := os.Stat(filepath.Join(out, name)); err != nil {
				t.Errorf("missing %s", name)
			}
		}
	}
	for _, name := range []string{"llms.txt", "llms-full.txt", "search.json", "sitemap.xml", "robots.txt", "404.html", "assets/site.css", "assets/site.js", "favicon.svg"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("missing %s", name)
		}
	}
	llms, _ := os.ReadFile(filepath.Join(out, "llms.txt"))
	if !strings.Contains(string(llms), Origin+"/filters.md") {
		t.Errorf("llms.txt does not link the Markdown pages:\n%s", llms)
	}
	if err := Build("../..", out); err == nil {
		t.Error("Build overwrote an existing directory")
	}
}

func TestBrokenLinksFailTheBuild(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "docs"), 0755)
	os.WriteFile(filepath.Join(root, "module-version.txt"), []byte("v0.0.0\n"), 0644)
	os.WriteFile(filepath.Join(root, "CHANGELOG.md"), []byte("# Changelog\n"), 0644)
	page := "---\ntitle: Home\ndescription: d\nsection: Start\norder: 1\n---\n\n# Home\n\nSee [missing](/nowhere) and [anchor](/index#nope).\n"
	os.WriteFile(filepath.Join(root, "docs", "index.md"), []byte(page), 0644)
	err := Build(root, filepath.Join(root, "out"))
	if err == nil || !strings.Contains(err.Error(), "nowhere") {
		t.Fatalf("err = %v, want a broken-link error naming /nowhere", err)
	}
}
