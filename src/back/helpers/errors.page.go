package helpers

import (
	"net/http"
	"twitter/src/back/templates"
)

func NotFoundHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		if err := templates.RenderTemplate(w, "error.html", struct{ User any }{}); err != nil {
			http.Error(w, "Page introuvable", http.StatusNotFound)
		}
	}
}
