package database

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

func Open() (*sql.DB, error) {
	rawURL := os.Getenv("SCALINGO_MYSQL_URL")
	if rawURL == "" {
		rawURL = os.Getenv("MYSQL_URL")
	}
	if rawURL == "" {
		rawURL = os.Getenv("DATABASE_URL")
	}
	if rawURL == "" {
		return nil, fmt.Errorf("SCALINGO_MYSQL_URL (or MYSQL_URL / DATABASE_URL) is required")
	}

	dsn, err := toDSN(rawURL)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open MySQL connection: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(3 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect to MySQL: %w", err)
	}
	return db, nil
}

func Migrate(ctx context.Context, db *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
			username VARCHAR(30) NOT NULL UNIQUE,
			email VARCHAR(254) NOT NULL UNIQUE,
			password_hash VARCHAR(255) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS sessions (
			token_hash CHAR(64) NOT NULL PRIMARY KEY,
			user_id BIGINT UNSIGNED NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_sessions_expiration (expires_at),
			CONSTRAINT fk_sessions_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
		`CREATE TABLE IF NOT EXISTS posts (
			id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
			user_id BIGINT UNSIGNED NOT NULL,
			content VARCHAR(280) NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_posts_created_at (created_at),
			CONSTRAINT fk_posts_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("create application tables: %w", err)
		}
	}
	return nil
}

func toDSN(raw string) (string, error) {
	if !strings.Contains(raw, "://") {
		config, err := mysql.ParseDSN(raw)
		if err != nil {
			return "", fmt.Errorf("invalid MySQL connection string")
		}
		config.ParseTime = true
		return config.FormatDSN(), nil
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "mysql" {
		return "", fmt.Errorf("invalid MySQL connection URL")
	}
	if parsed.User == nil || parsed.Hostname() == "" || strings.Trim(parsed.Path, "/") == "" {
		return "", fmt.Errorf("MySQL URL must include a username, host, and database name")
	}

	port := parsed.Port()
	if port == "" {
		port = "3306"
	}
	config := mysql.NewConfig()
	config.User = parsed.User.Username()
	config.Passwd, _ = parsed.User.Password()
	config.Net = "tcp"
	config.Addr = net.JoinHostPort(parsed.Hostname(), port)
	config.DBName = strings.Trim(parsed.Path, "/")
	config.ParseTime = true
	config.Loc = time.UTC
	config.Params = map[string]string{"charset": "utf8mb4"}

	query := parsed.Query()
	if mode := query.Get("ssl-mode"); mode != "" {
		switch strings.ToUpper(mode) {
		case "DISABLED":
			query.Set("tls", "false")
		case "PREFERRED":
			query.Set("tls", "preferred")
		case "REQUIRED", "VERIFY_CA", "VERIFY_IDENTITY":
			query.Set("tls", "true")
		default:
			return "", fmt.Errorf("unsupported MySQL ssl-mode %q", mode)
		}
		query.Del("ssl-mode")
	}
	for key, values := range query {
		switch strings.ToLower(key) {
		case "tls", "usessl":
			if len(values) != 1 {
				return "", fmt.Errorf("unsupported MySQL tls setting")
			}
			switch strings.ToLower(values[0]) {
			case "true", "required":
				config.TLSConfig = "true"
			case "false", "disabled":
				config.TLSConfig = "false"
			case "preferred":
				config.TLSConfig = "preferred"
			default:
				return "", fmt.Errorf("unsupported MySQL tls setting")
			}
		default:
			if len(values) != 1 {
				return "", fmt.Errorf("invalid MySQL URL parameter %q", key)
			}
			config.Params[key] = values[0]
		}
	}
	return config.FormatDSN(), nil
}
