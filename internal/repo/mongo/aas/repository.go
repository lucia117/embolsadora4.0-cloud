package aas

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/domain/aas"
	"github.com/tu-org/embolsadora-api/internal/telemetry"
)

const collectionName = "asset_administration_shells"

// MongoShellRepository implements aas.ShellRepository using MongoDB.
type MongoShellRepository struct {
	col *mongo.Collection
}

// New creates a MongoShellRepository and ensures the required indexes exist.
func New(db *mongo.Database) (*MongoShellRepository, error) {
	col := db.Collection(collectionName)
	r := &MongoShellRepository{col: col}
	if err := r.ensureIndexes(context.Background()); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *MongoShellRepository) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "tenantId", Value: 1}, {Key: "globalAssetId", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "updatedAt", Value: -1}},
		},
	}
	_, err := r.col.Indexes().CreateMany(ctx, indexes)
	return err
}

func (r *MongoShellRepository) Create(ctx context.Context, shell *aas.AssetAdministrationShell) (*aas.AssetAdministrationShell, error) {
	start := time.Now()
	shell.CreatedAt = start.UTC()
	shell.UpdatedAt = start.UTC()

	_, err := r.col.InsertOne(ctx, shell)
	if err != nil {
		telemetry.MongoOperationErrors.WithLabelValues(collectionName, "create").Inc()
		return nil, mapError(err)
	}
	telemetry.MongoOperationDuration.WithLabelValues(collectionName, "create").Observe(time.Since(start).Seconds())
	return shell, nil
}

func (r *MongoShellRepository) GetByID(ctx context.Context, tenantID uuid.UUID, shellID string) (*aas.AssetAdministrationShell, error) {
	start := time.Now()
	filter := bson.D{{Key: "_id", Value: shellID}, {Key: "tenantId", Value: tenantID}}

	var shell aas.AssetAdministrationShell
	err := r.col.FindOne(ctx, filter).Decode(&shell)
	if err != nil {
		telemetry.MongoOperationErrors.WithLabelValues(collectionName, "get").Inc()
		return nil, mapError(err)
	}
	telemetry.MongoOperationDuration.WithLabelValues(collectionName, "get").Observe(time.Since(start).Seconds())
	return &shell, nil
}

func (r *MongoShellRepository) Update(ctx context.Context, tenantID uuid.UUID, shellID string, upd *aas.ShellUpdate) (*aas.AssetAdministrationShell, error) {
	start := time.Now()
	filter := bson.D{{Key: "_id", Value: shellID}, {Key: "tenantId", Value: tenantID}}

	set := bson.D{{Key: "updatedAt", Value: time.Now().UTC()}}
	if upd.Description != nil {
		set = append(set, bson.E{Key: "description", Value: upd.Description})
	}
	if upd.Administration != nil {
		set = append(set, bson.E{Key: "administration", Value: upd.Administration})
	}
	if upd.AssetKind != nil {
		set = append(set, bson.E{Key: "assetKind", Value: *upd.AssetKind})
	}
	if upd.AssetType != nil {
		set = append(set, bson.E{Key: "assetType", Value: *upd.AssetType})
	}
	if upd.SubmodelRefs != nil {
		set = append(set, bson.E{Key: "submodelRefs", Value: upd.SubmodelRefs})
	}

	after := options.After
	opt := options.FindOneAndUpdate().SetReturnDocument(after)
	var updated aas.AssetAdministrationShell
	err := r.col.FindOneAndUpdate(ctx, filter, bson.D{{Key: "$set", Value: set}}, opt).Decode(&updated)
	if err != nil {
		telemetry.MongoOperationErrors.WithLabelValues(collectionName, "update").Inc()
		return nil, mapError(err)
	}
	telemetry.MongoOperationDuration.WithLabelValues(collectionName, "update").Observe(time.Since(start).Seconds())
	return &updated, nil
}

func (r *MongoShellRepository) Delete(ctx context.Context, tenantID uuid.UUID, shellID string) error {
	start := time.Now()
	filter := bson.D{{Key: "_id", Value: shellID}, {Key: "tenantId", Value: tenantID}}

	res, err := r.col.DeleteOne(ctx, filter)
	if err != nil {
		telemetry.MongoOperationErrors.WithLabelValues(collectionName, "delete").Inc()
		return mapError(err)
	}
	if res.DeletedCount == 0 {
		return domain.ErrNotFound
	}
	telemetry.MongoOperationDuration.WithLabelValues(collectionName, "delete").Observe(time.Since(start).Seconds())
	return nil
}

func (r *MongoShellRepository) ListByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int) ([]*aas.AssetAdministrationShell, int64, error) {
	start := time.Now()
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	filter := bson.D{{Key: "tenantId", Value: tenantID}}

	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		telemetry.MongoOperationErrors.WithLabelValues(collectionName, "list").Inc()
		return nil, 0, mapError(err)
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "updatedAt", Value: -1}}).
		SetLimit(int64(limit)).
		SetSkip(int64(offset))

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		telemetry.MongoOperationErrors.WithLabelValues(collectionName, "list").Inc()
		return nil, 0, mapError(err)
	}
	defer cursor.Close(ctx)

	var shells []*aas.AssetAdministrationShell
	if err := cursor.All(ctx, &shells); err != nil {
		telemetry.MongoOperationErrors.WithLabelValues(collectionName, "list").Inc()
		return nil, 0, mapError(err)
	}
	telemetry.MongoOperationDuration.WithLabelValues(collectionName, "list").Observe(time.Since(start).Seconds())
	return shells, total, nil
}

// mapError translates MongoDB driver errors to domain errors.
func mapError(err error) error {
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.ErrNotFound
	}
	if mongo.IsDuplicateKeyError(err) {
		return domain.ErrConflict
	}
	return err
}
