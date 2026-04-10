package main

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"
)

//go:embed all:templates
var templateFS embed.FS

var funcMap = template.FuncMap{
	"deref": func(s *string) string {
		if s == nil {
			return ""
		}
		return *s
	},
	"derefFloat": func(f *float64) float64 {
		if f == nil {
			return 0
		}
		return *f
	},
	"derefBool": func(b *bool) bool {
		if b == nil {
			return false
		}
		return *b
	},
	"add": func(a, b int) int { return a + b },
	"sub": func(a, b int) int { return a - b },
	"formatSI": func(f *float64, unit *string) string {
		if f == nil {
			return "—"
		}
		u := ""
		if unit != nil {
			u = *unit
		}
		return FormatSI(*f, u)
	},
	"formatSIInput": func(f *float64, unit *string) string {
		if f == nil {
			return ""
		}
		u := ""
		if unit != nil {
			u = *unit
		}
		return FormatSIInput(*f, u)
	},
}

type Renderer struct {
	pages     map[string]*template.Template
	fragments map[string]*template.Template
}

func NewRenderer() *Renderer {
	r := &Renderer{
		pages:     make(map[string]*template.Template),
		fragments: make(map[string]*template.Template),
	}

	layoutBytes, err := templateFS.ReadFile("templates/layout.html")
	if err != nil {
		log.Fatalf("failed to read layout template: %v", err)
	}
	layoutStr := string(layoutBytes)

	err = fs.WalkDir(templateFS, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".html") {
			return err
		}
		if path == "templates/layout.html" {
			return nil
		}

		name := strings.TrimPrefix(path, "templates/")
		name = strings.TrimSuffix(name, ".html")
		base := filepath.Base(name)

		contentBytes, err := templateFS.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", path, err)
		}

		if strings.HasPrefix(base, "_") {
			t := template.Must(template.New(name).Funcs(funcMap).Parse(string(contentBytes)))
			r.fragments[name] = t
		} else {
			t := template.Must(template.New("layout").Funcs(funcMap).Parse(layoutStr))
			template.Must(t.Parse(string(contentBytes)))
			r.pages[name] = t
		}

		return nil
	})
	if err != nil {
		log.Fatalf("failed to load templates: %v", err)
	}

	log.Printf("loaded %d page templates, %d fragment templates", len(r.pages), len(r.fragments))
	return r
}

func (r *Renderer) RenderPage(w http.ResponseWriter, name string, data any) {
	t, ok := r.pages[name]
	if !ok {
		http.Error(w, fmt.Sprintf("template %q not found", name), http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", data); err != nil {
		log.Printf("error rendering page %q: %v", name, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

func (r *Renderer) RenderFragment(w http.ResponseWriter, name string, data any) {
	t, ok := r.fragments[name]
	if !ok {
		http.Error(w, fmt.Sprintf("fragment %q not found", name), http.StatusInternalServerError)
		return
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		log.Printf("error rendering fragment %q: %v", name, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}
