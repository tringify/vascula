// vascula-docs builds the documentation site from docs/*.md without network
// access. With -serve it also serves the result locally, the way the host does:
// /name serves name.html.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"vascula.dev/vascula/internal/docsite"
)

func main() {
	root := flag.String("root", ".", "repository root containing docs/, CHANGELOG.md and module-version.txt")
	out := flag.String("out", "dist/docs", "new output directory")
	serve := flag.String("serve", "", "after building, serve the site on this address (for example :8080)")
	flag.Parse()
	if err := os.MkdirAll(filepath.Dir(*out), 0755); err != nil {
		fail(err)
	}
	if err := docsite.Build(*root, *out); err != nil {
		fail(err)
	}
	fmt.Println(filepath.Join(*out, "index.html"))
	if *serve == "" {
		return
	}
	files := http.FileServer(http.Dir(*out))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path != "/" && !strings.Contains(filepath.Base(path), ".") {
			if _, err := os.Stat(filepath.Join(*out, path+".html")); err == nil {
				r.URL.Path = path + ".html"
			}
		}
		files.ServeHTTP(w, r)
	})
	fmt.Println("serving on", *serve)
	fail(http.ListenAndServe(*serve, handler))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
