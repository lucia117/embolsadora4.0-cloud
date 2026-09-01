// Package mongo encapsula el ciclo de vida de la conexion a MongoDB.
// Es, junto con internal/repo/mongo/measurements, el unico lugar del codigo que
// sabe que MongoDB existe: el resto del sistema habla con interfaces de dominio.
package mongo

import (
	"context"
	"fmt"

	mongodriver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"github.com/tu-org/embolsadora-api/internal/config"
)

// Client envuelve la conexion y la base de datos ya seleccionada.
type Client struct {
	client *mongodriver.Client
	db     *mongodriver.Database
}

// Connect abre la conexion y verifica que el servidor responda, devolviendo
// error si Mongo no esta disponible. Connect NO decide fatal vs degradado —
// eso es responsabilidad del llamador: routes/url_mappings.go arranca la API
// igual ante este error y deja la ingesta degradada (responde 500) en vez de
// tumbar el resto de los endpoints, que no dependen de Mongo.
func Connect(ctx context.Context, cfg config.MongoConfig) (*Client, error) {
	pingCtx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	opts := options.Client().
		ApplyURI(cfg.URI).
		SetConnectTimeout(cfg.Timeout).
		SetServerSelectionTimeout(cfg.Timeout)

	// OJO: en el driver v2, Connect NO recibe context — a diferencia de la v1.
	// Solo crea el cliente; la conexion real se establece de forma perezosa, y
	// por eso el Ping de abajo es lo que de verdad valida que Mongo responde.
	cli, err := mongodriver.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("mongo: no se pudo conectar: %w", err)
	}
	if err := cli.Ping(pingCtx, readpref.Primary()); err != nil {
		_ = cli.Disconnect(context.Background())
		return nil, fmt.Errorf("mongo: ping fallido: %w", err)
	}
	return &Client{client: cli, db: cli.Database(cfg.Database)}, nil
}

// Database devuelve la base ya seleccionada.
func (c *Client) Database() *mongodriver.Database { return c.db }

// Ping verifica que el primario responda. Lo usa el health check.
func (c *Client) Ping(ctx context.Context) error {
	return c.client.Ping(ctx, readpref.Primary())
}

// Close cierra la conexion.
func (c *Client) Close(ctx context.Context) error { return c.client.Disconnect(ctx) }
