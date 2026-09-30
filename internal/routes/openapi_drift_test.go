package routes

// Drift check entre el router y docs/openapi.yaml (fase 4 del harness
// documental, docs/_process/estandar/plan-harness-2026-09-29.md).
//
// El OpenAPI de este repo se escribe a mano, así que nada impide que una ruta
// nueva quede sin documentar. Este test arma el router real con dependencias
// falsas (JWKS de mentira, Postgres perezoso, Mongo inalcanzable → modo
// degradado, sin Redis), lista las rutas que Gin registró y las compara con
// los paths+métodos del contrato. Falla si hay rutas sin documentar o paths
// documentados que el router no registra.
//
// No necesita DATABASE_URL, MONGO_URI ni REDIS_URL: pgxpool no conecta hasta
// el primer query, y connectMeasurementsRepo degrada (no falla) si Mongo no
// responde.

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"gopkg.in/yaml.v3"

	"github.com/tu-org/embolsadora-api/internal/config"
)

// undocumentedRoutes son rutas que el router registra y que, por decisión, no
// forman parte del contrato en docs/openapi.yaml. Cada entrada lleva el motivo.
// Agregar algo acá es una decisión explícita, no una forma de silenciar el test.
var undocumentedRoutes = map[string]string{
	"GET /ping":    "liveness para el smoke test del deploy; operación, no contrato",
	"GET /health":  "health check operativo (docs/operations.md); operación, no contrato",
	"GET /metrics": "métricas Prometheus; operación, no contrato",
}

// knownDrift son rutas que deberían estar documentadas y todavía no lo están.
// Es deuda, no una excepción: la lista tiene que achicarse hasta quedar vacía.
// Una entrada que ya está documentada hace fallar el test, para que se borre.
var knownDrift = map[string]string{}

var ginParam = regexp.MustCompile(`[:*][A-Za-z0-9_]+`)
var oasParam = regexp.MustCompile(`\{[^}]+\}`)

// normalize reemplaza los nombres de parámetros por "{}" para que
// /tenants/:tenantId y /tenants/{id} se consideren el mismo path.
func normalize(path string) string {
	path = ginParam.ReplaceAllString(path, "{}")
	return oasParam.ReplaceAllString(path, "{}")
}

func buildRouterForDrift(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"keys":[]}`)
	}))
	t.Cleanup(jwks.Close)

	// Pool perezoso: pgxpool.New no abre conexiones hasta el primer query.
	db, err := pgxpool.New(context.Background(), "postgres://drift:drift@127.0.0.1:1/drift?sslmode=disable")
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(db.Close)

	cfg := &config.Config{
		Mongo: config.MongoConfig{
			URI:      "mongodb://127.0.0.1:1",
			Database: "drift",
			Timeout:  50 * time.Millisecond,
		},
		Ingest: config.IngestConfig{
			MaxBodyBytes: 4194304, MaxEvents: 1000, RateLimitRPS: 200, RateLimitBurst: 1000,
			APIKeyCacheTTL: time.Minute,
		},
		Supabase: config.SupabaseConfig{
			JWKSUrl:     jwks.URL,
			JWTIssuer:   "drift",
			JWTAudience: "authenticated",
			URL:         "http://127.0.0.1:1",
			AppBaseURL:  "http://localhost:3000",
		},
		Dashboards: config.DashboardsConfig{
			MetricsRateLimitRPM: 15, MetricsRateLimitBurst: 5, MetricsMaxSpecs: 10,
			MetricsMaxBuckets: 1000, MetricsMaxRawPoints: 5000, MetricsMaxGroups: 200,
			MetricsMaxBatchQueries: 50, MetricsMaxTimeMS: 5000,
		},
	}

	r := gin.New()
	RegisterURLMappings(r, db, cfg, nil)
	return r
}

func openAPIOperations(t *testing.T) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "openapi.yaml"))
	if err != nil {
		t.Fatalf("leer docs/openapi.yaml: %v", err)
	}
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parsear docs/openapi.yaml: %v", err)
	}
	methods := map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true}
	ops := map[string]bool{}
	for path, item := range doc.Paths {
		for method := range item {
			if methods[method] {
				ops[strings.ToUpper(method)+" "+normalize(path)] = true
			}
		}
	}
	return ops
}

func TestOpenAPIMatchesRouter(t *testing.T) {
	documented := openAPIOperations(t)
	registered := map[string]string{} // clave normalizada -> ruta tal como la registró Gin

	for _, rt := range buildRouterForDrift(t).Routes() {
		registered[rt.Method+" "+normalize(rt.Path)] = rt.Method + " " + rt.Path
	}

	var missing, stale []string
	for key, original := range registered {
		if _, ok := undocumentedRoutes[original]; ok {
			continue
		}
		if documented[key] {
			if _, ok := knownDrift[original]; ok {
				stale = append(stale, original+" ya está documentada: borrala de knownDrift")
			}
			continue
		}
		if _, ok := knownDrift[original]; ok {
			continue
		}
		missing = append(missing, original)
	}

	var phantom []string
	for key := range documented {
		if _, ok := registered[key]; !ok {
			phantom = append(phantom, key)
		}
	}

	sort.Strings(missing)
	sort.Strings(phantom)
	sort.Strings(stale)
	for _, m := range missing {
		t.Errorf("ruta registrada sin documentar en docs/openapi.yaml: %s", m)
	}
	for _, p := range phantom {
		t.Errorf("path documentado en docs/openapi.yaml que el router no registra: %s", p)
	}
	for _, s := range stale {
		t.Errorf("%s", s)
	}
}
