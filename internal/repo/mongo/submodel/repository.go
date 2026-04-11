package submodel

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

const collectionName = "submodels"

// MongoSubmodelRepository implements aas.SubmodelRepository using MongoDB.
type MongoSubmodelRepository struct {
	col *mongo.Collection
}

// New creates a MongoSubmodelRepository and ensures the required indexes exist.
func New(db *mongo.Database) (*MongoSubmodelRepository, error) {
	col := db.Collection(collectionName)
	r := &MongoSubmodelRepository{col: col}
	if err := r.ensureIndexes(context.Background()); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *MongoSubmodelRepository) ensureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "tenantId", Value: 1}, {Key: "shellId", Value: 1}, {Key: "idShort", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "tenantId", Value: 1}, {Key: "shellId", Value: 1}},
		},
	}
	_, err := r.col.Indexes().CreateMany(ctx, indexes)
	return err
}

func (r *MongoSubmodelRepository) Create(ctx context.Context, sm *aas.Submodel) (*aas.Submodel, error) {
	start := time.Now()
	sm.UpdatedAt = start.UTC()

	_, err := r.col.InsertOne(ctx, sm)
	if err != nil {
		telemetry.MongoOperationErrors.WithLabelValues(collectionName, "create").Inc()
		return nil, mapError(err)
	}
	telemetry.MongoOperationDuration.WithLabelValues(collectionName, "create").Observe(time.Since(start).Seconds())
	return sm, nil
}

func (r *MongoSubmodelRepository) GetByID(ctx context.Context, tenantID uuid.UUID, submodelID string) (*aas.Submodel, error) {
	start := time.Now()
	filter := bson.D{{Key: "_id", Value: submodelID}, {Key: "tenantId", Value: tenantID}}

	var sm aas.Submodel
	err := r.col.FindOne(ctx, filter).Decode(&sm)
	if err != nil {
		telemetry.MongoOperationErrors.WithLabelValues(collectionName, "get").Inc()
		return nil, mapError(err)
	}
	telemetry.MongoOperationDuration.WithLabelValues(collectionName, "get").Observe(time.Since(start).Seconds())
	return &sm, nil
}

func (r *MongoSubmodelRepository) ListByShell(ctx context.Context, tenantID uuid.UUID, shellID string, limit, offset int) ([]*aas.Submodel, int64, error) {
	start := time.Now()
	if limit <= 0 {
		limit = 100
	}
	filter := bson.D{{Key: "tenantId", Value: tenantID}, {Key: "shellId", Value: shellID}}

	total, err := r.col.CountDocuments(ctx, filter)
	if err != nil {
		telemetry.MongoOperationErrors.WithLabelValues(collectionName, "list").Inc()
		return nil, 0, mapError(err)
	}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSkip(int64(offset))

	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		telemetry.MongoOperationErrors.WithLabelValues(collectionName, "list").Inc()
		return nil, 0, mapError(err)
	}
	defer cursor.Close(ctx)

	var submodels []*aas.Submodel
	if err := cursor.All(ctx, &submodels); err != nil {
		telemetry.MongoOperationErrors.WithLabelValues(collectionName, "list").Inc()
		return nil, 0, mapError(err)
	}
	telemetry.MongoOperationDuration.WithLabelValues(collectionName, "list").Observe(time.Since(start).Seconds())
	return submodels, total, nil
}

// UpsertElement inserts or replaces the SubmodelElement identified by its IDShort.
// It reads the current submodel, updates the matching element in memory, and writes back atomically.
func (r *MongoSubmodelRepository) UpsertElement(ctx context.Context, tenantID uuid.UUID, submodelID string, element aas.SubmodelElement) error {
	start := time.Now()

	sm, err := r.GetByID(ctx, tenantID, submodelID)
	if err != nil {
		return err
	}

	found := false
	for i, el := range sm.SubmodelElements {
		if el.IDShort == element.IDShort {
			sm.SubmodelElements[i] = element
			found = true
			break
		}
	}
	if !found {
		sm.SubmodelElements = append(sm.SubmodelElements, element)
	}

	filter := bson.D{{Key: "_id", Value: submodelID}, {Key: "tenantId", Value: tenantID}}
	update := bson.D{{Key: "$set", Value: bson.D{
		{Key: "submodelElements", Value: sm.SubmodelElements},
		{Key: "updatedAt", Value: time.Now().UTC()},
	}}}

	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		telemetry.MongoOperationErrors.WithLabelValues(collectionName, "upsert_element").Inc()
		return mapError(err)
	}
	if res.MatchedCount == 0 {
		return domain.ErrNotFound
	}
	telemetry.MongoOperationDuration.WithLabelValues(collectionName, "upsert_element").Observe(time.Since(start).Seconds())
	return nil
}

func (r *MongoSubmodelRepository) Delete(ctx context.Context, tenantID uuid.UUID, submodelID string) error {
	start := time.Now()
	filter := bson.D{{Key: "_id", Value: submodelID}, {Key: "tenantId", Value: tenantID}}

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
