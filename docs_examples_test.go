package vascula

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The documentation in docs/*.md is a test suite. Every ```vascula block must
// compile. A block followed by a ```json data block and/or an ```output block
// is rendered and must produce exactly that output; a block followed by an
// ```error block must fail (at compilation or rendering) with that text.
// ```vascula fragment blocks show partial syntax and are not compiled.

type docBlock struct {
	lang  string
	flags []string
	body  string
	line  int
}

func (b docBlock) has(flag string) bool {
	for _, f := range b.flags {
		if f == flag {
			return true
		}
	}
	return false
}

// docBlocks returns the fenced code blocks of a Markdown file, and for each
// block whether only blank lines separate it from the previous one.
func docBlocks(src string) ([]docBlock, []bool) {
	var blocks []docBlock
	var adjacent []bool
	lines := strings.Split(src, "\n")
	onlyBlank := false
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "```") {
			if trimmed != "" {
				onlyBlank = false
			}
			continue
		}
		info := strings.Fields(strings.TrimPrefix(trimmed, "```"))
		b := docBlock{line: i + 1}
		if len(info) > 0 {
			b.lang, b.flags = info[0], info[1:]
		}
		var body []string
		for i++; i < len(lines) && strings.TrimSpace(lines[i]) != "```"; i++ {
			body = append(body, lines[i])
		}
		b.body = strings.Join(body, "\n")
		adjacent = append(adjacent, onlyBlank)
		blocks = append(blocks, b)
		onlyBlank = true
	}
	return blocks, adjacent
}

type docExampleData struct {
	Settings map[string]interface{} `json:"settings"`
	Data     map[string]interface{} `json:"data"`
	Allow    []string               `json:"allow"`
	Globals  map[string]interface{} `json:"globals"`
	Children map[string]struct {
		Source   string                 `json:"source"`
		Settings map[string]interface{} `json:"settings"`
		Allow    []string               `json:"allow"`
	} `json:"children"`
	Reserved []string `json:"reserved"`
}

func TestDocumentationExamples(t *testing.T) {
	files, err := filepath.Glob("docs/*.md")
	if err != nil || len(files) == 0 {
		t.Fatalf("no documentation found: %v", err)
	}
	rendered := 0
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		blocks, adjacent := docBlocks(string(raw))
		for i := 0; i < len(blocks); i++ {
			b := blocks[i]
			if b.lang != "vascula" || b.has("fragment") {
				continue
			}
			name := fmt.Sprintf("%s:%d", file, b.line)
			var data *docBlock
			var expect *docBlock
			j := i + 1
			if j < len(blocks) && adjacent[j] && blocks[j].lang == "json" && blocks[j].has("data") {
				data = &blocks[j]
				j++
			}
			if j < len(blocks) && adjacent[j] && (blocks[j].lang == "output" || blocks[j].lang == "error") {
				expect = &blocks[j]
			}
			t.Run(name, func(t *testing.T) {
				var input docExampleData
				if data != nil {
					if err := json.Unmarshal([]byte(data.body), &input); err != nil {
						t.Fatalf("data block at line %d: %v", data.line, err)
					}
				}
				opts := CompileOptions{Reserved: input.Reserved}
				tmpl, err := CompileWithOptions(b.body, opts)
				if err != nil {
					if expect != nil && expect.lang == "error" && strings.Contains(err.Error(), strings.TrimSpace(expect.body)) {
						return
					}
					t.Fatalf("compile: %v", err)
				}
				if expect == nil {
					return
				}
				resolve := func(child string) (Child, error) {
					c, ok := input.Children[child]
					if !ok {
						return Child{}, fmt.Errorf("no child %q in the example", child)
					}
					ct, err := CompileWithOptions(c.Source, opts)
					if err != nil {
						return Child{}, err
					}
					return Child{Template: ct, Settings: c.Settings, Allow: c.Allow}, nil
				}
				out, err := tmpl.Render(Options{Data: input.Data, Allow: input.Allow, Settings: input.Settings, Globals: input.Globals, Resolve: resolve})
				if expect.lang == "error" {
					if err == nil || !strings.Contains(err.Error(), strings.TrimSpace(expect.body)) {
						t.Fatalf("want error containing %q, got %v (output %q)", strings.TrimSpace(expect.body), err, out)
					}
					return
				}
				if err != nil {
					t.Fatalf("render: %v", err)
				}
				// A Markdown block cannot show a final line break, so trailing
				// line breaks are not compared; everything else is exact.
				if strings.TrimRight(out, "\n") != strings.TrimRight(expect.body, "\n") {
					t.Fatalf("output\ngot:  %q\nwant: %q", out, expect.body)
				}
			})
			rendered++
		}
	}
	if rendered < 130 {
		t.Errorf("only %d documented examples; every feature needs one", rendered)
	}
}

// Every built-in filter has a section in docs/filters.md.
func TestEveryFilterIsDocumented(t *testing.T) {
	raw, err := os.ReadFile("docs/filters.md")
	if err != nil {
		t.Fatal(err)
	}
	documented := map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(line, "### ") {
			for _, name := range strings.Split(strings.TrimPrefix(line, "### "), ",") {
				documented[strings.TrimSpace(name)] = true
			}
		}
	}
	for name := range builtinFilters() {
		if !documented[name] {
			t.Errorf("filter %s has no section in docs/filters.md", name)
		}
	}
}
