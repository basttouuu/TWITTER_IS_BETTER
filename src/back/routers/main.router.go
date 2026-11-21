package routers

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"twitter/src/back/templates"
	"twitter/src/front"
)

const sessionCookieName = "twitter_session"

type contextKey int

const userContextKey contextKey = iota

type User struct {
	ID       int64
	Username string
}

type Post struct {
	Username  string
	Content   string
	CreatedAt time.Time
}

type pageData struct {
	User  *User
	Posts []Post
	Error string
}

type Server struct {
	db *sql.DB
}

func MainRouter(db *sql.DB) (http.Handler, error) {
	server := &Server{db: db}
	router := http.NewServeMux()

	assets, err := front.Assets()
	if err != nil {
		return nil, fmt.Errorf("load static assets: %w", err)
	}
	router.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(assets))))

	router.HandleFunc("GET /{$}", server.withUser(server.home))
	router.HandleFunc("GET /register", server.withUser(server.registerPage))
	router.HandleFunc("POST /register", server.register)
	router.HandleFunc("GET /login", server.withUser(server.loginPage))
	router.HandleFunc("POST /login", server.login)
	router.HandleFunc("POST /logout", server.withUser(server.logout))
	router.HandleFunc("POST /posts", server.withUser(server.createPost))
	router.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	router.HandleFunc("/", server.notFound)

	return securityHeaders(router), nil
}

func (s *Server) withUser(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, err := s.userFromRequest(r)
		if err != nil {
			log.Printf("load user session: %v", err)
			http.Error(w, "Une erreur est survenue.", http.StatusInternalServerError)
			return
		}
		if user == nil {
			if _, err := r.Cookie(sessionCookieName); err == nil {
				s.clearSessionCookie(w)
			}
		}
		ctx := context.WithValue(r.Context(), userContextKey, user)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		s.notFound(w, r)
		return
	}
	data := pageData{User: userFromContext(r)}
	rows, err := s.db.QueryContext(r.Context(), `
		SELECT users.username, posts.content, posts.created_at
		FROM posts JOIN users ON users.id = posts.user_id
		ORDER BY posts.created_at DESC, posts.id DESC
		LIMIT 50`)
	if err != nil {
		log.Printf("load feed: %v", err)
		http.Error(w, "Impossible de charger le fil.", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	for rows.Next() {
		var post Post
		if err := rows.Scan(&post.Username, &post.Content, &post.CreatedAt); err != nil {
			log.Printf("read feed item: %v", err)
			http.Error(w, "Impossible de charger le fil.", http.StatusInternalServerError)
			return
		}
		data.Posts = append(data.Posts, post)
	}
	if err := rows.Err(); err != nil {
		log.Printf("read feed: %v", err)
		http.Error(w, "Impossible de charger le fil.", http.StatusInternalServerError)
		return
	}
	s.render(w, "home.html", data)
}

func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "Requête non autorisée.", http.StatusForbidden)
		return
	}
	user := userFromContext(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requête invalide.", http.StatusBadRequest)
		return
	}
	content := strings.TrimSpace(r.FormValue("content"))
	if length := utf8.RuneCountInString(content); length == 0 || length > 280 {
		s.render(w, "home.html", pageData{
			User:  user,
			Error: "Une publication doit contenir entre 1 et 280 caractères.",
		})
		return
	}
	if _, err := s.db.ExecContext(r.Context(),
		"INSERT INTO posts (user_id, content) VALUES (?, ?)", user.ID, content); err != nil {
		log.Printf("create post for user %d: %v", user.ID, err)
		http.Error(w, "Impossible de publier pour le moment.", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		w.WriteHeader(http.StatusNotFound)
		s.render(w, "error.html", pageData{User: userFromContext(r)})
		return
	}
	http.NotFound(w, r)
}

func (s *Server) render(w http.ResponseWriter, name string, data pageData) {
	if err := templates.RenderTemplate(w, name, data); err != nil {
		log.Printf("render %s: %v", name, err)
		http.Error(w, "Impossible d'afficher cette page.", http.StatusInternalServerError)
	}
}

func userFromContext(r *http.Request) *User {
	user, _ := r.Context().Value(userContextKey).(*User)
	return user
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'self'")
		next.ServeHTTP(w, r)
	})
}
