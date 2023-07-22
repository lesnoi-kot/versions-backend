package mongostore

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Store struct {
	*mongo.Client
}

func ConnectStore(ctx context.Context, mongoURI string) (*Store, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(timeoutCtx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		return nil, err
	}

	if err := client.Ping(timeoutCtx, readpref.Primary()); err != nil {
		return nil, err
	}

	return &Store{client}, nil
}

type GetDocumentsOptions[T any] struct {
	Collection  string
	Filter      bson.D
	FindOptions *options.FindOptions
}

func GetDocuments[T any](ctx context.Context, store *Store, opts GetDocumentsOptions[T]) ([]*T, error) {
	cur, err := store.
		Database(DatabaseName).
		Collection(opts.Collection).
		Find(ctx, opts.Filter, opts.FindOptions)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	documents := make([]*T, 0)

	for cur.Next(ctx) {
		document := new(T)
		if err := cur.Decode(document); err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}

	if err := cur.Err(); err != nil {
		return nil, err
	}

	return documents, nil
}

func (store *Store) GetDocumentsCount(ctx context.Context, collection string) (int64, error) {
	count, err := store.
		Database(DatabaseName).
		Collection(collection).
		CountDocuments(ctx, bson.D{})
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (store *Store) GetSourceBy(ctx context.Context, filter bson.D, opts *options.FindOneOptions) (*Source, error) {
	source := new(Source)
	err := store.
		Database(DatabaseName).
		Collection(SourcesCollectionName).
		FindOne(ctx, filter, opts).
		Decode(source)
	if err != nil {
		return nil, err
	}

	return source, nil
}
