package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"
	"twitter/src/back/database"
	"twitter/src/back/routers"
	"twitter/src/back/templates"
)

func main() {
	if err := templates.Load(); err != nil {
		log.Fatalf("load templates: %v", err)
	}

	db, err := database.Open()
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, db); err != nil {
		log.Fatalf("initialize database: %v", err)
	}
	handler, err := routers.MainRouter(db)
	if err != nil {
		log.Fatalf("initialize HTTP server: %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("Server listening on %s", addr)
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
