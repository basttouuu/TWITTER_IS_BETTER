package templates

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type templateUser struct {
	Username string
}

type templatePost struct {
	Username  string
	Content   string
	CreatedAt time.Time
}

type templatePage struct {
	User  *templateUser
	Posts []templatePost
	Error string
	Query string
}

func TestLoadAndRenderPages(t *testing.T) {
	if err := Load(); err != nil {
		t.Fatal(err)
	}

	data := templatePage{
		User:  &templateUser{Username: "alex"},
		Posts: []templatePost{{Username: "alex", Content: "<script>alert(1)</script>", CreatedAt: time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)}},
	}
	for _, name := range []string{"home.html", "login.html", "register.html", "error.html"} {
		t.Run(name, func(t *testing.T) {
			response := httptest.NewRecorder()
			if err := RenderTemplate(response, name, data); err != nil {
				t.Fatal(err)
			}
			if response.Code != 200 || !strings.Contains(response.Header().Get("Content-Type"), "text/html") {
				t.Fatalf("unexpected response: status=%d content-type=%q", response.Code, response.Header().Get("Content-Type"))
			}
			if name == "home.html" && !strings.Contains(response.Body.String(), "&lt;script&gt;") {
				t.Fatal("post content was not HTML-escaped")
			}
		})
	}
}
