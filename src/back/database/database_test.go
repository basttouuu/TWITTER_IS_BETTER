package database

import (
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
)

func TestToDSNFromScalingoURL(t *testing.T) {
	dsn, err := toDSN("mysql://app:secret@db.example.com:3307/twitter?ssl-mode=REQUIRED")
	if err != nil {
		t.Fatal(err)
	}
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	if config.User != "app" || config.Passwd != "secret" || config.Addr != "db.example.com:3307" {
		t.Fatalf("unexpected connection settings: %#v", config)
	}
	if config.DBName != "twitter" || !config.ParseTime || config.TLSConfig != "true" {
		t.Fatalf("unexpected database settings: %#v", config)
	}
}

func TestToDSNRejectsInvalidURLs(t *testing.T) {
	for _, input := range []string{
		"postgres://app:secret@db.example.com/twitter",
		"mysql://app:secret@db.example.com/",
		"mysql://app:secret@db.example.com/twitter?ssl-mode=INVALID",
	} {
		if _, err := toDSN(input); err == nil {
			t.Errorf("toDSN(%q) succeeded, want an error", input)
		} else if strings.Contains(err.Error(), "secret") {
			t.Errorf("error exposed the password: %v", err)
		}
	}
}
