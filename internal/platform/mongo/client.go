package mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/tu-org/embolsadora-api/internal/config"
)

// Connect establishes a MongoDB connection, validates it with a ping, and returns the client.
// The caller is responsible for calling client.Disconnect when done.
func Connect(ctx context.Context, cfg config.MongoConfig) (*mongo.Client, error) {
	opts := options.Client().ApplyURI(cfg.URI)

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("mongo: connect: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(pingCtx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("mongo: ping failed: %w", err)
	}

	return client, nil
}
