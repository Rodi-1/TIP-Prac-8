package notes

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var ErrNotFound = errors.New("note not found")

type Repo struct{ col *mongo.Collection }

func NewRepo(db *mongo.Database) (*Repo, error) {
	col := db.Collection("notes")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "title", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "title", Value: "text"}}},
		{Keys: bson.D{{Key: "expiresAt", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	})
	if err != nil {
		return nil, err
	}
	return &Repo{col: col}, nil
}

func (r *Repo) Create(ctx context.Context, title, content string, expiresAt *time.Time) (Note, error) {
	now := time.Now().UTC().Truncate(time.Millisecond)
	n := Note{Title: title, Content: content, CreatedAt: now, UpdatedAt: now, ExpiresAt: expiresAt}
	res, err := r.col.InsertOne(ctx, n)
	if err != nil {
		return Note{}, err
	}
	n.ID = res.InsertedID.(primitive.ObjectID)
	return n, nil
}

func (r *Repo) ByID(ctx context.Context, idHex string) (Note, error) {
	oid, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return Note{}, ErrNotFound
	}
	var n Note
	if err := r.col.FindOne(ctx, bson.M{"_id": oid}).Decode(&n); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return Note{}, ErrNotFound
		}
		return Note{}, err
	}
	return n, nil
}

func (r *Repo) List(ctx context.Context, q string, limit, skip int64, after string) ([]Note, error) {
	filter := bson.M{}
	if q != "" {
		filter["$text"] = bson.M{"$search": q}
	}
	opts := options.Find().SetLimit(limit).SetSkip(skip).SetSort(bson.D{{Key: "_id", Value: -1}})
	if after != "" {
		oid, err := primitive.ObjectIDFromHex(after)
		if err != nil {
			return nil, err
		}
		filter["_id"] = bson.M{"$lt": oid}
	}
	cur, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)
	out := make([]Note, 0)
	for cur.Next(ctx) {
		var n Note
		if err := cur.Decode(&n); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, cur.Err()
}

func (r *Repo) Update(ctx context.Context, idHex string, title, content *string, expiresAt *time.Time) (Note, error) {
	oid, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return Note{}, ErrNotFound
	}
	set := bson.M{"updatedAt": time.Now().UTC().Truncate(time.Millisecond)}
	if title != nil {
		set["title"] = *title
	}
	if content != nil {
		set["content"] = *content
	}
	if expiresAt != nil {
		set["expiresAt"] = *expiresAt
	}
	var n Note
	err = r.col.FindOneAndUpdate(ctx, bson.M{"_id": oid}, bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&n)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Note{}, ErrNotFound
	}
	return n, err
}

func (r *Repo) Delete(ctx context.Context, idHex string) error {
	oid, err := primitive.ObjectIDFromHex(idHex)
	if err != nil {
		return ErrNotFound
	}
	res, err := r.col.DeleteOne(ctx, bson.M{"_id": oid})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repo) Statistics(ctx context.Context) (Stats, error) {
	cur, err := r.col.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id": nil, "count": bson.M{"$sum": 1},
			"averageContentLength": bson.M{"$avg": bson.M{"$strLenCP": "$content"}},
		}}},
	})
	if err != nil {
		return Stats{}, err
	}
	defer cur.Close(ctx)
	var result Stats
	if cur.Next(ctx) {
		if err := cur.Decode(&result); err != nil {
			return Stats{}, err
		}
	}
	return result, cur.Err()
}
