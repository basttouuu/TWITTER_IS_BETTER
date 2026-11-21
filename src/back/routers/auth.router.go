package routers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"
)

const sessionDuration = 30 * 24 * time.Hour

var usernamePattern = regexp.MustCompile(`^[a-z0-9_]{3,30}$`)

func (s *Server) registerPage(w http.ResponseWriter, r *http.Request) {
	if userFromContext(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, "register.html", pageData{})
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "Requête non autorisée.", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requête invalide.", http.StatusBadRequest)
		return
	}
	username := strings.ToLower(strings.TrimSpace(r.FormValue("username")))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")
	if !usernamePattern.MatchString(username) {
		s.render(w, "register.html", pageData{Error: "Le pseudo doit contenir de 3 à 30 lettres, chiffres ou tirets bas."})
		return
	}
	address, err := mail.ParseAddress(email)
	if err != nil || address.Address != email || len(email) > 254 {
		s.render(w, "register.html", pageData{Error: "Saisissez une adresse e-mail valide."})
		return
	}
	if len(password) < 8 || len(password) > 72 {
		s.render(w, "register.html", pageData{Error: "Le mot de passe doit contenir entre 8 et 72 caractères."})
		return
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("hash account password: %v", err)
		http.Error(w, "Impossible de créer le compte pour le moment.", http.StatusInternalServerError)
		return
	}

	tx, err := s.db.BeginTx(r.Context(), nil)
	if err != nil {
		log.Printf("begin account registration: %v", err)
		http.Error(w, "Impossible de créer le compte pour le moment.", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(r.Context(),
		"INSERT INTO users (username, email, password_hash) VALUES (?, ?, ?)",
		username, email, string(passwordHash))
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			s.render(w, "register.html", pageData{Error: "Ce pseudo ou cette adresse e-mail est déjà utilisé."})
			return
		}
		log.Printf("create user: %v", err)
		http.Error(w, "Impossible de créer le compte pour le moment.", http.StatusInternalServerError)
		return
	}
	userID, err := result.LastInsertId()
	if err != nil {
		log.Printf("read new user id: %v", err)
		http.Error(w, "Impossible de créer le compte pour le moment.", http.StatusInternalServerError)
		return
	}
	token, err := insertSession(r.Context(), tx, userID)
	if err != nil {
		log.Printf("create account session: %v", err)
		http.Error(w, "Impossible de créer le compte pour le moment.", http.StatusInternalServerError)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("save account: %v", err)
		http.Error(w, "Impossible de créer le compte pour le moment.", http.StatusInternalServerError)
		return
	}
	s.setSessionCookie(w, token)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) loginPage(w http.ResponseWriter, r *http.Request) {
	if userFromContext(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.render(w, "login.html", pageData{})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "Requête non autorisée.", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Requête invalide.", http.StatusBadRequest)
		return
	}
	identifier := strings.ToLower(strings.TrimSpace(r.FormValue("identifier")))
	password := r.FormValue("password")
	var user User
	var passwordHash string
	err := s.db.QueryRowContext(r.Context(),
		"SELECT id, username, password_hash FROM users WHERE username = ? OR email = ? LIMIT 1",
		identifier, identifier).Scan(&user.ID, &user.Username, &passwordHash)
	if errors.Is(err, sql.ErrNoRows) {
		s.render(w, "login.html", pageData{Error: "Identifiants incorrects."})
		return
	}
	if err != nil {
		log.Printf("find login account: %v", err)
		http.Error(w, "Impossible de se connecter pour le moment.", http.StatusInternalServerError)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		s.render(w, "login.html", pageData{Error: "Identifiants incorrects."})
		return
	}
	token, err := insertSession(r.Context(), s.db, user.ID)
	if err != nil {
		log.Printf("create login session: %v", err)
		http.Error(w, "Impossible de se connecter pour le moment.", http.StatusInternalServerError)
		return
	}
	s.setSessionCookie(w, token)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "Requête non autorisée.", http.StatusForbidden)
		return
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		if _, err := s.db.ExecContext(r.Context(), "DELETE FROM sessions WHERE token_hash = ?", hashToken(cookie.Value)); err != nil {
			log.Printf("delete session: %v", err)
			http.Error(w, "Impossible de fermer la session pour le moment.", http.StatusInternalServerError)
			return
		}
	}
	s.clearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) userFromRequest(r *http.Request) (*User, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return nil, nil
		}
		return nil, fmt.Errorf("read session cookie: %w", err)
	}

	var user User
	err = s.db.QueryRowContext(r.Context(), `
		SELECT users.id, users.username
		FROM sessions JOIN users ON users.id = sessions.user_id
		WHERE sessions.token_hash = ? AND sessions.expires_at > UTC_TIMESTAMP()`,
		hashToken(cookie.Value)).Scan(&user.ID, &user.Username)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}
	return &user, nil
}

type sessionWriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertSession(ctx context.Context, writer sessionWriter, userID int64) (string, error) {
	var tokenBytes [32]byte
	if _, err := rand.Read(tokenBytes[:]); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes[:])
	_, err := writer.ExecContext(ctx,
		"INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)",
		hashToken(token), userID, time.Now().UTC().Add(sessionDuration))
	if err != nil {
		return "", fmt.Errorf("store session: %w", err)
	}
	if _, err := writer.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at < UTC_TIMESTAMP()"); err != nil {
		return "", fmt.Errorf("remove expired sessions: %w", err)
	}
	return token, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionDuration.Seconds()),
		HttpOnly: true,
		Secure:   cookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cookieSecure(),
		SameSite: http.SameSiteLaxMode,
	})
}

func cookieSecure() bool {
	return strings.EqualFold(os.Getenv("COOKIE_SECURE"), "true") ||
		(strings.EqualFold(os.Getenv("APP_ENV"), "production") &&
			!strings.EqualFold(os.Getenv("COOKIE_SECURE"), "false"))
}

func sameOrigin(r *http.Request) bool {
	source := r.Header.Get("Origin")
	if source == "" {
		source = r.Header.Get("Referer")
	}
	if source == "" {
		return true
	}
	parsed, err := url.Parse(source)
	return err == nil && parsed.Host != "" && strings.EqualFold(parsed.Host, r.Host)
}
