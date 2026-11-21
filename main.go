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
		log.Fatalf("chargement des modèles : %v", err)
	}

	db, err := database.Open()
	if err != nil {
		log.Fatalf("connexion à la base de données : %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, db); err != nil {
		log.Fatalf("initialisation de la base de données : %v", err)
	}
	handler, err := routers.MainRouter(db)
	if err != nil {
		log.Fatalf("initialisation du serveur HTTP : %v", err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("Serveur en écoute sur %s", addr)
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("erreur du serveur HTTP : %v", err)
	}
}
