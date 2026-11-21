package templates

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"unicode/utf8"

	"twitter/src/front"
)

var listTemp *template.Template

func Load() error {
	parsed, err := template.New("").Funcs(template.FuncMap{
		"initial": func(value string) string {
			r, _ := utf8.DecodeRuneInString(value)
			return strings.ToUpper(string(r))
		},
	}).ParseFS(front.Templates(), "templates/*.html")
	if err != nil {
		return fmt.Errorf("parse page templates: %w", err)
	}
	parsed, err = parsed.ParseFS(front.Templates(), "templates/component/*.html")
	if err != nil {
		return fmt.Errorf("parse component templates: %w", err)
	}
	listTemp = parsed
	return nil
}

func RenderTemplate(w http.ResponseWriter, name string, data any) error {
	if listTemp == nil {
		return fmt.Errorf("templates have not been loaded")
	}

	var buffer bytes.Buffer
	if err := listTemp.ExecuteTemplate(&buffer, name, data); err != nil {
		return fmt.Errorf("execute %q template: %w", name, err)
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := buffer.WriteTo(w)
	return err
}
