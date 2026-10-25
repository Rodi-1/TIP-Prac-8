package notes

import (
	"context"
	"os"
	"testing"
	"time"

	"example.com/pz8-mongo/internal/db"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestCreateAndGet(t *testing.T) {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		t.Skip("set MONGO_URI for MongoDB integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	deps, err := db.ConnectMongo(ctx, uri, "pz8_test_"+primitive.NewObjectID().Hex())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		if err := deps.Database.Drop(cleanupCtx); err != nil {
			t.Error(err)
		}
		deps.Client.Disconnect(cleanupCtx)
	})
	r, err := NewRepo(deps.Database)
	if err != nil {
		t.Fatal(err)
	}
	created, err := r.Create(ctx, "T1", "C1", nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.ByID(ctx, created.ID.Hex())
	if err != nil || got.ID != created.ID || got.Title != "T1" || got.Content != "C1" {
		t.Fatalf("Create/ByID mismatch: %+v, %v", got, err)
	}
	if _, err := r.Create(ctx, "T1", "duplicate", nil); !mongo.IsDuplicateKeyError(err) {
		t.Fatalf("unique index: %v", err)
	}
}
