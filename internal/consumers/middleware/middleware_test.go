package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	apimw "github.com/tu-org/embolsadora-api/internal/api/middleware"
	"github.com/tu-org/embolsadora-api/internal/consumers"
	consumermw "github.com/tu-org/embolsadora-api/internal/consumers/middleware"
	domainapikeys "github.com/tu-org/embolsadora-api/internal/domain/apikeys"
	"github.com/tu-org/embolsadora-api/internal/security"
)

// fakeAuthenticator implementa security.Authenticator con un desenlace fijo.
// Lo que se prueba aca es como APIKeyAuth TRADUCE cada desenlace de
// Authenticate a un status HTTP — no la logica de autenticacion en si, que ya
// cubre internal/security/apikeys_authenticator_test.go contra un repo falso
// propio. Este paquete no tenia NINGUN test antes de este archivo, pese a
// sostener tres comportamientos que el plan entero llama no negociables.
type fakeAuthenticator struct {
	identity *domainapikeys.DeviceIdentity
	err      error
}

func (f *fakeAuthenticator) Authenticate(context.Context, string) (*domainapikeys.DeviceIdentity, error) {
	return f.identity, f.err
}

// newRouter monta la cadena real APIKeyAuth -> RateLimit -> handler final, tal
// como la arma internal/routes/url_mappings.go. `seen` opcionalmente captura
// la identidad que llego al handler, para verificar que APIKeyAuth la dejo en
// el contexto del request.
func newRouter(auth security.Authenticator, limiter *consumers.RateLimiter, seen *[]*domainapikeys.DeviceIdentity) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(consumermw.APIKeyAuth(auth, zap.NewNop()))
	r.Use(consumermw.RateLimit(limiter))
	r.POST("/events", func(c *gin.Context) {
		if seen != nil {
			*seen = append(*seen, security.DeviceIdentityFrom(c.Request.Context()))
		}
		c.Status(http.StatusOK)
	})
	return r
}

func doRequest(r *gin.Engine, apiKey string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/events", nil)
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// noRateLimit es un limiter sin Redis: fail-open, nunca deniega. Se usa en los
// tests de APIKeyAuth para que el limiter no interfiera con lo que se esta
// probando (la traduccion de Authenticate a status HTTP).
func noRateLimit() *consumers.RateLimiter { return consumers.NewRateLimiter(nil, 0, 0) }

// Sin header X-Api-Key (o vacio) es 403, no 401: el Edge trata 403 como
// "reintentar, probablemente una rotacion de key en curso" (ver comentario en
// middleware.go). Un 401 no esta en su tabla de decision y no dispararia el
// reintento que se necesita.
func TestAPIKeyAuthMissingKeyReturns403(t *testing.T) {
	r := newRouter(&fakeAuthenticator{}, noRateLimit(), nil)
	w := doRequest(r, "")
	assert.Equal(t, http.StatusForbidden, w.Code, "sin X-Api-Key debe ser 403, nunca 401")
}

// Una credencial que Authenticate rechaza con ErrAPIKeyInvalid tambien es 403
// — mismo codigo que "falta el header", deliberado: el handler no puede dejar
// que el cliente distinga "no mande key" de "mande una mala".
func TestAPIKeyAuthInvalidKeyReturns403(t *testing.T) {
	auth := &fakeAuthenticator{err: security.ErrAPIKeyInvalid}
	r := newRouter(auth, noRateLimit(), nil)
	w := doRequest(r, "emb_000000000000_secreto-cualquiera")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// El caso que hace a I-1 aplicable tambien en la capa de auth: un error de
// Authenticate que NO es ErrAPIKeyInvalid es un fallo de infraestructura (la
// consulta a Postgres broto, un timeout, lo que sea) y no una credencial mala.
// Devolver 403 aca haria que una caida de base se vea, desde el Edge, IDENTICA
// a una key revocada — y el Edge no reintenta un 403. Tiene que ser 500.
func TestAPIKeyAuthRepositoryErrorReturns500(t *testing.T) {
	auth := &fakeAuthenticator{err: errors.New("apikeys: no se pudo consultar Postgres")}
	r := newRouter(auth, noRateLimit(), nil)
	w := doRequest(r, "emb_000000000000_secreto-cualquiera")
	assert.Equal(t, http.StatusInternalServerError, w.Code,
		"un error de infraestructura en Authenticate NO puede traducirse a 403 (I-1 en la capa de auth)")
}

// Camino feliz: una key valida deja la identidad en el contexto del request y
// la cadena sigue hasta el handler final.
func TestAPIKeyAuthValidKeyPopulatesContextAndProceeds(t *testing.T) {
	identity := &domainapikeys.DeviceIdentity{
		TenantID:  uuid.New(),
		DeviceID:  uuid.New(),
		MachineID: "EMB-DEV-001",
		KeyPK:     uuid.New(),
		KeyID:     "abc123def456",
	}
	auth := &fakeAuthenticator{identity: identity}
	var seen []*domainapikeys.DeviceIdentity
	r := newRouter(auth, noRateLimit(), &seen)

	w := doRequest(r, "emb_abc123def456_secreto-valido")

	assert.Equal(t, http.StatusOK, w.Code)
	require.Len(t, seen, 1, "el handler final debe haberse ejecutado exactamente una vez")
	require.NotNil(t, seen[0], "APIKeyAuth debe dejar la identidad en el contexto")
	assert.Equal(t, identity.TenantID, seen[0].TenantID)
	assert.Equal(t, identity.DeviceID, seen[0].DeviceID)
	assert.Equal(t, identity.MachineID, seen[0].MachineID)
	assert.Equal(t, identity.KeyID, seen[0].KeyID)
}

// SC-006, verificado en la capa correcta: HTTP de punta a punta, no
// RateLimiter.Allow() por si solo (asi es como estaba mal en la primera
// version de este plan — probaba una tupla (bool, int), nunca un status code
// ni un header). Requiere Redis real: el fail-open sin Redis se prueba aparte,
// en internal/consumers/ingest_integration_test.go
// (TestRateLimiterFailsOpenWithoutRedis), que guarda una politica distinta y
// deliberada.
func TestSC006RateLimitReturns429WithRetryAfter(t *testing.T) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL no seteada; se omite el test de rate limit")
	}
	opt, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	t.Cleanup(func() { _ = rdb.Close() })

	identity := &domainapikeys.DeviceIdentity{
		TenantID:  uuid.New(),
		DeviceID:  uuid.New(),
		MachineID: "EMB-DEV-001",
		KeyPK:     uuid.New(),
		KeyID:     "mw-test-" + time.Now().Format("150405.000000"),
	}
	// burst 1: un unico token en el bucket. Se lo consume ANTES del request
	// HTTP para que la denegacion sea determinista y no dependa de timing.
	limiter := consumers.NewRateLimiter(rdb, 1, 1)
	allowed, _, err := limiter.Allow(context.Background(), identity.KeyID)
	require.NoError(t, err)
	require.True(t, allowed, "el primer consumo debe pasar (burst=1)")

	r := newRouter(&fakeAuthenticator{identity: identity}, limiter, nil)
	w := doRequest(r, "emb_000000000000_lo-que-sea") // fakeAuthenticator ignora el valor

	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// El Retry-After no es decorativo: el Edge lo lee y espera exactamente ese
	// tiempo. Un 0 le diria "reintenta ya", que es lo opuesto de un backoff —
	// justamente el bug que una ronda de fixes anterior en el limiter corrigio.
	retryAfterHeader := w.Header().Get("Retry-After")
	require.NotEmpty(t, retryAfterHeader, "la respuesta 429 debe traer el header Retry-After")
	retryAfter, err := strconv.Atoi(retryAfterHeader)
	require.NoError(t, err, "Retry-After debe ser un entero")
	assert.GreaterOrEqual(t, retryAfter, 1, "Retry-After debe ser >= 1 segundo, nunca 0")
}

// El bug del item 4 de la review final: apimw.CORS() esta registrado con
// r.Use() a nivel del engine en cmd/api/main.go, asi que corre ANTES que
// cualquier middleware de un grupo especifico -incluido NoCORS()- para TODAS
// las rutas, /api/v1/consumers incluido. Antes de este fix, NoCORS() no hacia
// nada para un request normal (no-OPTIONS): la respuesta salia con
// Access-Control-Allow-Origin: * puesto por el middleware global, exactamente
// como si NoCORS() no existiera.
//
// Este test arma la cadena real -apimw.CORS() primero, NoCORS() despues- y
// verifica que los headers CORS NO lleguen a la respuesta final.
func TestNoCORSStripsHeadersLeftByGlobalCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(apimw.CORS()) // el middleware global real tal como main.go lo registra
	r.Use(consumermw.NoCORS())
	r.GET("/events", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"),
		"NoCORS debe borrar el header que el CORS global ya dejo puesto")
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Methods"))
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Headers"))
}

// Un preflight (OPTIONS) contra una ruta que pasa por NoCORS() se rechaza con
// 405 y nunca llega al handler final. En la cadena real de produccion,
// apimw.CORS() (registrado antes) ya intercepta el OPTIONS con un 204 propio
// -eso queda fuera de este test, que verifica el comportamiento de NoCORS()
// en si mismo, no el orden completo de middlewares del engine.
func TestNoCORSRefusesPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(consumermw.NoCORS())
	reached := false
	r.OPTIONS("/events", func(c *gin.Context) {
		reached = true
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodOptions, "/events", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
	assert.False(t, reached, "NoCORS debe abortar el preflight antes de llegar al handler final")
}
