// Package docsite builds the Vascula documentation site from docs/*.md.
//
// Output, for hosting at the site root:
//
//	index.html, <slug>.html   pages (served at /, /<slug>)
//	<slug>.md, index.md       each page's Markdown source, for agents
//	llms.txt, llms-full.txt   an index for language models, and the whole site
//	search.json               the client-side search index
//	sitemap.xml, robots.txt, 404.html, assets/
//
// The build fails on a broken internal link or anchor, so the site can never
// ship a dead reference.
package docsite

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed site.css
var siteCSS string

//go:embed site.js
var siteJS string

// The original Vein artwork and its social card, kept for Vascula.
//
//go:embed assets/vascula-greenhouse.webp
var greenhouseWebP string

//go:embed assets/vascula-og.png
var socialPNG string

// Origin is where the site is published.
const Origin = "https://vascula.dev"

// sections orders the navigation groups.
var sections = []string{"Start", "Language", "Go", "Guides", "Reference"}

// Page is one documentation page.
type Page struct {
	Slug        string
	Title       string
	Description string
	Section     string
	Order       int
	Source      string // Markdown body, without front matter
	out         rendered
}

// URL is the page's address on the site.
func (p *Page) URL() string {
	if p.Slug == "index" {
		return "/"
	}
	return "/" + p.Slug
}

// Load reads docs/*.md and CHANGELOG.md from the repository root.
func Load(root string) ([]*Page, string, error) {
	files, err := filepath.Glob(filepath.Join(root, "docs", "*.md"))
	if err != nil {
		return nil, "", err
	}
	var pages []*Page
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, "", err
		}
		p, err := parsePage(strings.TrimSuffix(filepath.Base(file), ".md"), string(raw))
		if err != nil {
			return nil, "", fmt.Errorf("%s: %w", file, err)
		}
		pages = append(pages, p)
	}
	changelog, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		return nil, "", err
	}
	pages = append(pages, &Page{
		Slug: "changelog", Title: "Changelog", Section: "Reference", Order: 52,
		Description: "Every change in every release, and what to check when you upgrade.",
		Source:      strings.TrimSpace(string(changelog)),
	})
	version, err := os.ReadFile(filepath.Join(root, "module-version.txt"))
	if err != nil {
		return nil, "", err
	}
	sort.SliceStable(pages, func(i, j int) bool {
		si, sj := sectionIndex(pages[i].Section), sectionIndex(pages[j].Section)
		if si != sj {
			return si < sj
		}
		return pages[i].Order < pages[j].Order
	})
	return pages, strings.TrimSpace(string(version)), nil
}

func sectionIndex(s string) int {
	for i, name := range sections {
		if name == s {
			return i
		}
	}
	return len(sections)
}

func parsePage(slug, raw string) (*Page, error) {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	if !strings.HasPrefix(raw, "---\n") {
		return nil, fmt.Errorf("missing front matter")
	}
	end := strings.Index(raw[4:], "\n---\n")
	if end < 0 {
		return nil, fmt.Errorf("unterminated front matter")
	}
	p := &Page{Slug: slug, Source: strings.TrimSpace(raw[4+end+5:])}
	for _, line := range strings.Split(raw[4:4+end], "\n") {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			return nil, fmt.Errorf("bad front matter line %q", line)
		}
		value = strings.TrimSpace(value)
		switch strings.TrimSpace(key) {
		case "title":
			p.Title = value
		case "description":
			p.Description = value
		case "section":
			p.Section = value
		case "order":
			n, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("order: %w", err)
			}
			p.Order = n
		default:
			return nil, fmt.Errorf("unknown front matter key %q", key)
		}
	}
	if p.Title == "" || p.Description == "" || sectionIndex(p.Section) == len(sections) {
		return nil, fmt.Errorf("front matter needs title, description and a known section")
	}
	return p, nil
}

// Build renders the site from the repository at root into out, which must
// not exist yet.
func Build(root, out string) error {
	if _, err := os.Stat(out); err == nil {
		return fmt.Errorf("%s already exists; choose a new output directory", out)
	}
	pages, version, err := Load(root)
	if err != nil {
		return err
	}
	bySlug := map[string]*Page{}
	for _, p := range pages {
		r, err := renderMarkdown(p.Source)
		if err != nil {
			return fmt.Errorf("%s: %w", p.Slug, err)
		}
		p.out = r
		bySlug[p.Slug] = p
	}
	if err := checkLinks(pages, bySlug); err != nil {
		return err
	}

	files := map[string]string{
		"assets/site.css":                siteCSS,
		"assets/site.js":                 siteJS,
		"favicon.svg":                    faviconSVG,
		"assets/vascula-greenhouse.webp": greenhouseWebP,
		// vascula.dev/vascula resolves to the repository for go get.
		"vascula.html":          goImportPage,
		"assets/vascula-og.png": socialPNG,
		"robots.txt":            "User-agent: *\nAllow: /\nSitemap: " + Origin + "/sitemap.xml\n",
		"404.html":              layout(&Page{Slug: "404", Title: "Not found", Description: "This page does not exist."}, pages, version, "<h1>Page not found</h1>\n<p>That page does not exist. Try the search, or start at <a href=\"/\">the home page</a>.</p>\n", nil, nil),
	}
	var search []map[string]interface{}
	var sitemap strings.Builder
	sitemap.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n" + `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")
	var llms, full strings.Builder
	llms.WriteString("# Vascula\n\n> Vascula is a template language for HTML, embedded in Go: output is always escaped, templates read only the data the application allows, and rendering is bounded. Version " + version + ".\n\n")
	llms.WriteString("Start with [Vascula for agents](" + Origin + "/agents.md): the whole language, rules and common mistakes on one page. [llms-full.txt](" + Origin + "/llms-full.txt) has every page.\n")
	full.WriteString("# Vascula documentation (" + version + ")\n\nSource: " + Origin + "\n")
	section := ""
	for i, p := range pages {
		var prev, next *Page
		if i > 0 {
			prev = pages[i-1]
		}
		if i+1 < len(pages) {
			next = pages[i+1]
		}
		name := p.Slug + ".html"
		files[name] = layout(p, pages, version, p.out.HTML, prev, next)
		files[p.Slug+".md"] = "# " + p.Title + "\n\n> " + p.Description + "\n\n" + stripTitle(p.Source) + "\n"
		search = append(search, map[string]interface{}{
			"t": p.Title, "u": p.URL(), "s": p.Section, "d": p.Description,
			"h": headingIndex(p), "x": compact(p.out.Text, 4000),
		})
		fmt.Fprintf(&sitemap, "  <url><loc>%s%s</loc></url>\n", Origin, p.URL())
		if p.Section != section {
			section = p.Section
			fmt.Fprintf(&llms, "\n## %s\n\n", section)
		}
		fmt.Fprintf(&llms, "- [%s](%s/%s.md): %s\n", p.Title, Origin, p.Slug, p.Description)
		fmt.Fprintf(&full, "\n\n---\n\n# %s\n\nURL: %s%s\n\n%s\n", p.Title, Origin, p.URL(), stripTitle(p.Source))
	}
	sitemap.WriteString("</urlset>\n")
	files["sitemap.xml"] = sitemap.String()
	files["llms.txt"] = llms.String()
	files["llms-full.txt"] = full.String()
	searchJSON, err := json.Marshal(search)
	if err != nil {
		return err
	}
	files["search.json"] = string(searchJSON)

	for name, content := range files {
		path := filepath.Join(out, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// stripTitle drops the leading "# Title" line; the Markdown copy adds its own.
func stripTitle(src string) string {
	if strings.HasPrefix(src, "# ") {
		if i := strings.IndexByte(src, '\n'); i >= 0 {
			return strings.TrimSpace(src[i+1:])
		}
		return ""
	}
	return src
}

func headingIndex(p *Page) []map[string]string {
	var out []map[string]string
	for _, h := range p.out.Headings {
		out = append(out, map[string]string{"t": h.Text, "a": h.ID})
	}
	return out
}

func compact(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		s = s[:max]
	}
	return s
}

func checkLinks(pages []*Page, bySlug map[string]*Page) error {
	var problems []string
	for _, p := range pages {
		for _, link := range p.out.Links {
			path, anchor, _ := strings.Cut(link, "#")
			target := p
			switch {
			case path == "":
			case path == "/":
				target = bySlug["index"]
			case strings.HasPrefix(path, "/") && (strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".txt")):
				slug := strings.TrimSuffix(strings.TrimPrefix(path, "/"), ".md")
				if strings.HasSuffix(path, ".txt") {
					if path != "/llms.txt" && path != "/llms-full.txt" {
						problems = append(problems, p.Slug+": unknown file "+link)
					}
					continue
				}
				if bySlug[slug] == nil {
					problems = append(problems, p.Slug+": broken link "+link)
				}
				continue
			case strings.HasPrefix(path, "/"):
				target = bySlug[strings.TrimPrefix(path, "/")]
			default:
				problems = append(problems, p.Slug+": links must start with / ("+link+")")
				continue
			}
			if target == nil {
				problems = append(problems, p.Slug+": broken link "+link)
				continue
			}
			if anchor != "" && !hasAnchor(target, anchor) {
				problems = append(problems, p.Slug+": missing anchor "+link)
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("documentation links:\n  %s", strings.Join(problems, "\n  "))
	}
	return nil
}

func hasAnchor(p *Page, anchor string) bool {
	return strings.Contains(p.out.HTML, `id="`+anchor+`"`)
}

const faviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="-4 -4 32 32" shape-rendering="crispEdges"><rect x="-4" y="-4" width="32" height="32" rx="6" fill="#0b0b0c"/><path fill="#a6f87b" fill-rule="evenodd" d="M10 1h5v5h5v5h-5v5h-5v7H5v-5H0v-5h5V8h5z M10 8h5v5h-5z"/></svg>`

// goImportPage answers go get for the module path vascula.dev/vascula.
const goImportPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="go-import" content="vascula.dev/vascula git https://github.com/tringify/vascula">
<meta name="go-source" content="vascula.dev/vascula https://github.com/tringify/vascula https://github.com/tringify/vascula/tree/main{/dir} https://github.com/tringify/vascula/blob/main{/dir}/{file}#L{line}">
<meta http-equiv="refresh" content="0; url=/"><title>vascula.dev/vascula</title></head>
<body><a href="/">Vascula documentation</a> · <a href="https://github.com/tringify/vascula">Source</a></body></html>
`

// The Vascula symbol: the pixel mark first drawn for Vein.
const logoSVG = `<svg class="logo" viewBox="0 0 24 24" shape-rendering="crispEdges" aria-hidden="true"><path fill="currentColor" fill-rule="evenodd" d="M10 1h5v5h5v5h-5v5h-5v7H5v-5H0v-5h5V8h5z M10 8h5v5h-5z"/></svg>`

func layout(p *Page, pages []*Page, version, body string, prev, next *Page) string {
	e := html.EscapeString
	var nav strings.Builder
	section := ""
	for _, other := range pages {
		if other.Section != section {
			if section != "" {
				nav.WriteString("</ul>\n")
			}
			section = other.Section
			fmt.Fprintf(&nav, "<p class=\"nav-section\">%s</p>\n<ul>\n", e(section))
		}
		current := ""
		if other.Slug == p.Slug {
			current = ` aria-current="page"`
		}
		fmt.Fprintf(&nav, `<li><a href="%s"%s>%s</a></li>`+"\n", other.URL(), current, e(other.Title))
	}
	nav.WriteString("</ul>\n")

	var toc strings.Builder
	if len(p.out.Headings) > 1 {
		toc.WriteString(`<nav class="toc" aria-label="On this page"><p>On this page</p><ul>`)
		for _, h := range p.out.Headings {
			fmt.Fprintf(&toc, `<li class="l%d"><a href="#%s">%s</a></li>`, h.Level, h.ID, e(h.Text))
		}
		toc.WriteString("</ul></nav>")
	}

	var pager strings.Builder
	if prev != nil || next != nil {
		pager.WriteString(`<nav class="pager" aria-label="Previous and next">`)
		if prev != nil {
			fmt.Fprintf(&pager, `<a class="prev" href="%s"><span>Previous</span>%s</a>`, prev.URL(), e(prev.Title))
		}
		if next != nil {
			fmt.Fprintf(&pager, `<a class="next" href="%s"><span>Next</span>%s</a>`, next.URL(), e(next.Title))
		}
		pager.WriteString("</nav>")
	}

	title := p.Title + " · Vascula"
	if p.Slug == "index" {
		title = "Vascula: safe, fast HTML templates for Go"
	}
	hero := ""
	if p.Slug == "index" {
		hero = `<figure class="hero-art"><div class="scene" data-animated="false"><img class="art" src="/assets/vascula-greenhouse.webp" width="1078" height="311" alt="Pixel-art greenhouse at night: glowing green vines connect a developer at a desk to products, a shop front, carts and parcels."><svg class="traces" viewBox="0 0 1078 309" aria-hidden="true" focusable="false"><path d="M383 225 L383 243 C420 251 492 248 511 239 C527 230 528 211 523 201 C521 193 535 186 532 178 C526 164 510 161 510 145 L510 120"/><path d="M415 85 C411 103 423 103 436 112 L442 120 M416 49 L416 32 C420 25 428 25 432 17"/><path d="M437 66 C446 65 455 65 469 65 M538 72 C551 75 550 88 563 89 C579 92 597 97 609 85 C625 73 641 69 661 71"/><path d="M599 107 C597 124 586 133 591 148 C592 162 612 169 611 184 C610 200 582 204 575 219 C568 231 572 244 568 251 C582 261 617 261 643 261 L679 261"/><path d="M634 236 C634 253 647 262 667 260 M764 254 C788 258 813 255 824 247 C834 239 824 224 835 212 C847 199 850 168 852 146 C853 129 861 119 859 111"/><path d="M773 103 C788 100 803 95 814 85 C826 73 841 66 857 69 C870 71 887 63 891 49"/><path d="M958 53 C959 71 952 80 936 82 C922 88 927 100 940 106 C956 111 972 122 969 138 C968 151 953 156 955 171 L955 184 M970 143 C987 154 998 170 997 193"/><path d="M721 109 C738 110 750 111 764 107 M443 30 C462 12 501 17 527 21 C550 21 565 28 574 36"/></svg><canvas class="sparks" width="1078" height="309" aria-hidden="true"></canvas><div class="halo" aria-hidden="true"></div></div></figure>`
	}
	mdLink := ""
	canonical := ""
	if p.Slug != "404" {
		mdPath := "/" + p.Slug + ".md"
		mdLink = `<a class="md-link" href="` + mdPath + `">View as Markdown</a>`
		canonical = `<link rel="canonical" href="` + Origin + p.URL() + `"><link rel="alternate" type="text/markdown" href="` + mdPath + `">`
	}
	return `<!doctype html>
<html lang="en" data-theme="dark">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + e(title) + `</title>
<meta name="description" content="` + e(p.Description) + `">
<meta property="og:title" content="` + e(title) + `">
<meta property="og:description" content="` + e(p.Description) + `">
<meta property="og:type" content="website">
<meta property="og:image" content="` + Origin + `/assets/vascula-og.png">
<meta property="og:image:width" content="1200">
<meta property="og:image:height" content="630">
<meta name="twitter:card" content="summary_large_image">
<meta name="theme-color" content="#0b0b0c">
` + canonical + `
<link rel="icon" href="/favicon.svg" type="image/svg+xml">
<link rel="stylesheet" href="/assets/site.css">
<script>try{var t=localStorage.getItem("theme");if(t)document.documentElement.dataset.theme=t;else if(matchMedia("(prefers-color-scheme: light)").matches)document.documentElement.dataset.theme="light"}catch(e){}</script>
</head>
<body>
<a class="skip" href="#content">Skip to content</a>
<header class="top">
  <button class="menu" type="button" aria-label="Menu" aria-expanded="false">☰</button>
  <a class="brand" href="/">` + logoSVG + `<span>Vascula</span></a>
  <span class="version">` + e(version) + `</span>
  <div class="search">
    <input type="search" placeholder="Search the docs" aria-label="Search the docs" autocomplete="off">
    <kbd>/</kbd>
    <div class="results" role="listbox" hidden></div>
  </div>
  <button class="theme" type="button" aria-label="Switch light and dark theme">◐</button>
</header>
<div class="shell">
  <nav class="sidebar" aria-label="Documentation">` + nav.String() + `</nav>
  <main id="content">
    <article>` + hero + body + `</article>
    ` + pager.String() + `
    <footer><span>MIT licence · <a href="https://github.com/tringify/vascula">GitHub</a> · Made by <a href="https://tringify.com">Tringify</a> · <a href="https://dev-docs.tringify.com/themes/">Build Tringify themes</a></span>` + mdLink + `</footer>
  </main>
  ` + toc.String() + `
</div>
<script src="/assets/site.js" defer></script>
</body>
</html>
`
}
