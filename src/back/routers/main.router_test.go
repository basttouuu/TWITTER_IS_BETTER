package routers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"twitter/src/back/templates"
)

func TestRouterServesEmbeddedPagesAndAssets(t *testing.T) {
	if err := templates.Load(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("mysql", "user:password@tcp(localhost:3306)/twitter")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	handler, err := MainRouter(db)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		path       string
		statusCode int
		contains   string
	}{
		{path: "/register", statusCode: http.StatusOK, contains: "Créer un compte"},
		{path: "/static/css/style.css", statusCode: http.StatusOK, contains: "--blue:"},
		{path: "/health", statusCode: http.StatusOK},
		{path: "/healthz", statusCode: http.StatusNoContent},
	} {
		t.Run(test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
			if response.Code != test.statusCode {
				t.Fatalf("status = %d, want %d", response.Code, test.statusCode)
			}
			if test.contains != "" && !strings.Contains(response.Body.String(), test.contains) {
				t.Fatalf("response does not contain %q", test.contains)
			}
		})
	}
}

func TestEscapeLikeTreatsWildcardsLiterally(t *testing.T) {
	if got, want := escapeLike("100%_ready!"), "100!%!_ready!!"; got != want {
		t.Fatalf("escapeLike() = %q, want %q", got, want)
	}
}
