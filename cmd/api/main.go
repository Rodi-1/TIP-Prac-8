package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"example.com/pz8-mongo/internal/db"
	"example.com/pz8-mongo/internal/notes"
	"github.com/go-chi/chi/v5"
)

func main() {
	uri := getenv("MONGO_URI", "mongodb://root:secret@localhost:27017/?authSource=admin")
	dbName := getenv("MONGO_DB", "pz8")
	addr := getenv("HTTP_ADDR", ":8080")
	deps, err := db.ConnectMongo(context.Background(), uri, dbName)
	if err != nil {
		log.Fatal("mongo connect: ", err)
	}
	defer deps.Client.Disconnect(context.Background())
	repo, err := notes.NewRepo(deps.Database)
	if err != nil {
		log.Fatal("notes repo: ", err)
	}
	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Mount("/api/v1/notes", notes.NewHandler(repo).Routes())
	log.Println("listening on", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
