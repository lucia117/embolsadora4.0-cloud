package apikeys

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	domainapikeys "github.com/tu-org/embolsadora-api/internal/domain/apikeys"
)

// Repository implementa domainapikeys.Repository sobre Postgres.
type Repository struct {
	db *pgxpool.Pool
}

// var _ domainapikeys.Repository = (*Repository)(nil) fuerza en tiempo de
// compilacion que Repository siga implementando la interfaz completa. Sin
// esto, un cambio de firma en domainapikeys.Repository que este paquete no
// siguiera solo se notaria en el call site que lo usa (routes/url_mappings.go
// via NewRepository), potencialmente lejos de aca.
var _ domainapikeys.Repository = (*Repository)(nil)

// NewRepository construye el repositorio de API keys.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// GetByKeyID resuelve el key_id publico a una credencial completa. El JOIN con
// edge_devices trae machine_id y status en la misma ida: el camino caliente de
// la ingesta hace un solo roundtrip.
func (r *Repository) GetByKeyID(ctx context.Context, keyID string) (*domainapikeys.Credential, error) {
	const q = `
		SELECT k.id, k.tenant_id, k.device_id, k.key_id, k.key_hash,
		       k.expires_at, k.revoked_at,
		       d.machine_id, d.status
		  FROM edge_device_api_keys k
		  JOIN edge_devices d ON d.id = k.device_id
		 WHERE k.key_id = $1`

	var c domainapikeys.Credential
	err := r.db.QueryRow(ctx, q, keyID).Scan(
		&c.KeyPK, &c.TenantID, &c.DeviceID, &c.KeyID, &c.KeyHash,
		&c.ExpiresAt, &c.RevokedAt,
		&c.MachineID, &c.DeviceStatus,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domainapikeys.ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Create persiste una key nueva.
func (r *Repository) Create(ctx context.Context, k *domainapikeys.APIKey) error {
	const q = `
		INSERT INTO edge_device_api_keys
		       (id, tenant_id, device_id, key_id, key_hash, name, created_at, created_by, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err := r.db.Exec(ctx, q,
		k.ID, k.TenantID, k.DeviceID, k.KeyID, k.KeyHash,
		k.Name, k.CreatedAt, k.CreatedBy, k.ExpiresAt,
	)
	return err
}

// ListByDevice devuelve todas las keys del device, nuevas primero.
func (r *Repository) ListByDevice(ctx context.Context, tenantID, deviceID uuid.UUID) ([]*domainapikeys.APIKey, error) {
	const q = `
		SELECT id, tenant_id, device_id, key_id, key_hash, name,
		       created_at, created_by, expires_at, revoked_at, last_used_at
		  FROM edge_device_api_keys
		 WHERE tenant_id = $1 AND device_id = $2
		 ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, tenantID, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domainapikeys.APIKey
	for rows.Next() {
		var k domainapikeys.APIKey
		if err := rows.Scan(
			&k.ID, &k.TenantID, &k.DeviceID, &k.KeyID, &k.KeyHash, &k.Name,
			&k.CreatedAt, &k.CreatedBy, &k.ExpiresAt, &k.RevokedAt, &k.LastUsedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, &k)
	}
	return out, rows.Err()
}

// Revoke marca la key como revocada. El `AND revoked_at IS NULL` la hace
// idempotente: una segunda revocacion no pisa el timestamp original.
func (r *Repository) Revoke(ctx context.Context, tenantID, keyPK uuid.UUID) error {
	const q = `
		UPDATE edge_device_api_keys
		   SET revoked_at = CURRENT_TIMESTAMP
		 WHERE id = $1 AND tenant_id = $2 AND revoked_at IS NULL`

	tag, err := r.db.Exec(ctx, q, keyPK, tenantID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		// O no existe, o no es del tenant, o ya estaba revocada. Distinguirlas
		// obliga a un SELECT extra sin ganancia: chequeamos existencia y listo.
		return r.assertExists(ctx, tenantID, keyPK)
	}
	return nil
}

func (r *Repository) assertExists(ctx context.Context, tenantID, keyPK uuid.UUID) error {
	const q = `SELECT 1 FROM edge_device_api_keys WHERE id = $1 AND tenant_id = $2`
	var one int
	err := r.db.QueryRow(ctx, q, keyPK, tenantID).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return domainapikeys.ErrKeyNotFound
	}
	return err
}

// TouchLastUsed actualiza last_used_at. El llamador se encarga de no invocarlo
// mas de una vez por minuto por key (ver el throttle en security.Authenticator).
func (r *Repository) TouchLastUsed(ctx context.Context, keyPK uuid.UUID) error {
	const q = `UPDATE edge_device_api_keys SET last_used_at = CURRENT_TIMESTAMP WHERE id = $1`
	_, err := r.db.Exec(ctx, q, keyPK)
	return err
}
