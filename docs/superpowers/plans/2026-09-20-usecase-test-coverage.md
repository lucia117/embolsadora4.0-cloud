# Cobertura de tests en la capa de casos de uso — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Agregar los tests faltantes en `internal/app/*` y `internal/api/usecases/*` identificados en el relevamiento (paquetes sin ningún test, y paquetes con test parcial que dejan métodos enteros sin cubrir).

**Architecture:** Cada tarea es un paquete Go independiente (archivo de test propio, sin estado compartido con otras tareas) — se puede ejecutar en paralelo, un subagente por tarea, sin conflictos de merge. La convención real del repo (confirmada por grep exhaustivo) es **fakes manuscritos por archivo de test**, nunca `uber/mock` pese a estar en `go.mod` como dependencia indirecta. `testify` (`require`/`assert`) es el único framework de aserciones.

**Tech Stack:** Go 1.24, testify, zap (`zap.NewNop()` en tests), uuid, go-redis/v8 (solo en las 2 tareas gateadas por `REDIS_URL`).

**Spec:** No hay spec previa — este plan nace del relevamiento hecho en la conversación (ver research embebido en cada tarea). No existe un `docs/superpowers/specs/*` asociado.

## Global Constraints

- **Nunca usar `uber/mock` / `mockgen`.** Cero archivos `mock_*.go` en todo el repo; introducirlo rompería la convención. Todo doble de prueba es un struct manuscrito en el propio `_test.go` que implementa la interfaz completa (los métodos que el usecase no usa devuelven zero-value).
- **`testify`** (`require` para asserts que deben abortar el test, `assert` para el resto) — sin excepciones.
- **Cada tarea es dueña de su propio archivo.** Aunque dos tareas toquen paquetes hermanos, ninguna edita un archivo que otra tarea también edita — así se pueden ejecutar todas en paralelo sin pisarse. Cuando una tarea extiende un fake ya existente en un archivo compartido (`edge_devices/service_test.go`, `roles/service_test.go`), es la única tarea que toca ese archivo.
- **No es TDD clásico.** El código de producción ya existe (esto es backfill de cobertura, no feature nueva), así que el ciclo RED→GREEN no aplica igual: el paso "escribir el test" seguido de "correrlo" debería dar GREEN directo si el test está bien escrito contra el comportamiento real. Si da RED, es señal de un bug real en el código de producción o un error en el test — no "falta implementar nada".
- **Comando de verificación por tarea:** `go test ./<paquete>/... -v` (ver comando exacto en cada tarea). Verificación global al final: `go build ./...` y `go test ./...`.
- **Nombres de test:** seguir el estilo ya presente en cada paquete (algunos usan español descriptivo tipo `TestUpdateRoleRolGlobalOcultoDevuelveNotFoundNoSystemRole`, otros inglés `TestService_Query_Scalar`). Este plan usa nombres concretos por task; son sugerencias, no camisa de fuerza — lo que importa es que cubran el escenario descrito.
- **Tests gateados por infraestructura real** (`REDIS_URL`, `DATABASE_URL`) siguen el patrón ya usado en `internal/api/middleware/dashboard_ratelimit_test.go` / `internal/api/usecases/password_usecase_test.go`: `if os.Getenv("X") == "" { t.Skip(...) }`. Solo dos tareas de este plan (19) lo necesitan; todo lo demás es 100% fake, sin infraestructura.

## File Structure

| # | Paquete | Acción | Archivo de test |
|---|---|---|---|
| 1 | `internal/app/edge_devices` | Modificar | `service_test.go` (extender fakes existentes) |
| 2 | `internal/app/roles` | Modificar | `service_test.go` (extender `fakeRolesRepo`) |
| 3 | `internal/app/users` | Crear | `service_unit_test.go` (nuevo, separado del integration test existente) |
| 4 | `internal/app/alarm_rules` | Crear | `service_test.go` |
| 5 | `internal/app/dashboard_layouts` | Crear | `service_test.go` |
| 6 | `internal/app/logs` | Crear | `service_test.go` |
| 7 | `internal/app/notifications` | Crear | `service_test.go` |
| 8 | `internal/app/permissions` | Crear | `service_test.go` |
| 9 | `internal/api/usecases/tenants/create_tenant` | Crear | `usecase_test.go` |
| 10 | `internal/api/usecases/tenants/delete_tenant` | Crear | `usecase_test.go` |
| 11 | `internal/api/usecases/tenants/get_tenant` | Crear | `get_tenant_test.go` |
| 12 | `internal/api/usecases/tenants/update_tenant` | Crear | `usecase_test.go` |
| 13 | `internal/api/usecases/user_roles/bulk_assign_user_roles` | Crear | `usecase_test.go` |
| 14 | `internal/api/usecases/user_roles/get_user_roles` | Crear | `usecase_test.go` |
| 15 | `internal/api/usecases/user_roles/list_user_roles` | Crear | `usecase_test.go` |
| 16 | `internal/api/usecases/user_roles/revoke_user_role` | Crear | `usecase_test.go` |
| 17 | `internal/api/usecases/user_roles/update_user_role` | Crear | `usecase_test.go` |
| 18 | `internal/api/usecases` (auth_usecase.go) | Crear | `auth_usecase_test.go` |
| 19 | `internal/api/usecases` (invitation_usecase.go: Resend/Revoke/List/checkRateLimit) | Crear | `invitation_resend_revoke_list_test.go` + `invitation_ratelimit_integration_test.go` |
| 20 | `internal/api/usecases` (password_usecase.go: ClearPasswordChangeRequired) | Crear | `password_usecase_clear_test.go` |

---

## Task 1: `internal/app/edge_devices` — 12 métodos sin test

**Files:**
- Modify: `internal/app/edge_devices/service_test.go`

**Interfaces:**
- Consumes: `edge_devices.Repository` (7 métodos), `edgeclient.EdgeDeviceClient` (3 métodos), `domainapikeys.Repository` (5 métodos) — todas en `internal/app/edge_devices/service.go`.
- Produces: n/a (archivo de test hoja, nada depende de él).

El archivo ya tiene `fakeRepo` (implementa `domain.Repository`, usado por los 2 tests existentes de `UpdateDevice` — no tocar esos 2 tests). Hay que **extender** `fakeRepo` con campos de error/resultado configurables (manteniendo compatibilidad: zero-value = comportamiento actual) y **agregar** `fakeClient` (`edgeclient.EdgeDeviceClient`) y `fakeAPIKeysRepo` (`domainapikeys.Repository`), que no existen todavía.

- [ ] **Step 1: Reemplazar el bloque de `fakeRepo` y agregar los fakes nuevos**

Reemplazar el `type fakeRepo struct { ... }` y sus métodos (líneas 16-47 del archivo actual) por:

```go
type fakeRepo struct {
	device      *domain.EdgeDevice
	updateCalls []*domain.EdgeDevice

	getErr error // distinto de ErrDeviceNotFound, para probar el branch de error de infra

	listResult []*domain.EdgeDevice
	listErr    error

	createErr error

	setStatusResult *domain.EdgeDevice
	setStatusErr    error

	updateErr error

	updateHealthErr   error
	updateHealthCalls []healthCall

	saveEventErr   error
	saveEventCalls []*domain.DeviceEvent

	listEventsResult []*domain.DeviceEvent
	listEventsErr    error
}

type healthCall struct {
	status  string
	summary string
}

func (f *fakeRepo) List(context.Context, uuid.UUID) ([]*domain.EdgeDevice, error) {
	return f.listResult, f.listErr
}
func (f *fakeRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.EdgeDevice, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.device == nil {
		return nil, domain.ErrDeviceNotFound
	}
	cp := *f.device
	return &cp, nil
}
func (f *fakeRepo) Create(context.Context, *domain.EdgeDevice) error { return f.createErr }
func (f *fakeRepo) Update(_ context.Context, d *domain.EdgeDevice) error {
	f.updateCalls = append(f.updateCalls, d)
	if f.updateErr != nil {
		return f.updateErr
	}
	f.device = d
	return nil
}
func (f *fakeRepo) SetStatus(context.Context, uuid.UUID, uuid.UUID, string) (*domain.EdgeDevice, error) {
	return f.setStatusResult, f.setStatusErr
}
func (f *fakeRepo) UpdateHealthState(_ context.Context, _ uuid.UUID, _ uuid.UUID, status, summary string) error {
	f.updateHealthCalls = append(f.updateHealthCalls, healthCall{status, summary})
	return f.updateHealthErr
}
func (f *fakeRepo) SaveEvent(_ context.Context, e *domain.DeviceEvent) error {
	f.saveEventCalls = append(f.saveEventCalls, e)
	return f.saveEventErr
}
func (f *fakeRepo) ListEvents(context.Context, uuid.UUID, uuid.UUID) ([]*domain.DeviceEvent, error) {
	return f.listEventsResult, f.listEventsErr
}

// fakeClient implementa edgeclient.EdgeDeviceClient (satisfacción estructural,
// no hace falta importar el paquete edgeclient).
type fakeClient struct {
	statusResult *domain.CheckResult
	statusErr    error
	healthResult *domain.CheckResult
	healthErr    error
	telemetryResult *domain.TelemetrySnapshot
	telemetryErr    error
}

func (f *fakeClient) StatusCheck(ctx context.Context, baseURL string) (*domain.CheckResult, error) {
	return f.statusResult, f.statusErr
}
func (f *fakeClient) HealthCheck(ctx context.Context, baseURL string) (*domain.CheckResult, error) {
	return f.healthResult, f.healthErr
}
func (f *fakeClient) GetTelemetry(ctx context.Context, baseURL string) (*domain.TelemetrySnapshot, error) {
	return f.telemetryResult, f.telemetryErr
}

// fakeAPIKeysRepo implementa domainapikeys.Repository.
type fakeAPIKeysRepo struct {
	createErr   error
	createCalls []*apikeys.APIKey

	listResult []*apikeys.APIKey
	listErr    error

	revokeErr   error
	revokeCalls []uuid.UUID
}

func (f *fakeAPIKeysRepo) GetByKeyID(context.Context, string) (*apikeys.Credential, error) {
	return nil, nil
}
func (f *fakeAPIKeysRepo) Create(_ context.Context, k *apikeys.APIKey) error {
	f.createCalls = append(f.createCalls, k)
	return f.createErr
}
func (f *fakeAPIKeysRepo) ListByDevice(context.Context, uuid.UUID, uuid.UUID) ([]*apikeys.APIKey, error) {
	return f.listResult, f.listErr
}
func (f *fakeAPIKeysRepo) Revoke(_ context.Context, _ uuid.UUID, keyPK uuid.UUID) error {
	f.revokeCalls = append(f.revokeCalls, keyPK)
	return f.revokeErr
}
func (f *fakeAPIKeysRepo) TouchLastUsed(context.Context, uuid.UUID) error { return nil }
```

Agregar el import `apikeys "github.com/tu-org/embolsadora-api/internal/domain/apikeys"` y `"errors"` al bloque `import` existente.

- [ ] **Step 2: `ListDevices` — feliz + error**

```go
func TestListDevices(t *testing.T) {
	tests := []struct {
		name    string
		result  []*domain.EdgeDevice
		err     error
	}{
		{"feliz", []*domain.EdgeDevice{{ID: uuid.New()}}, nil},
		{"error de repo", nil, errors.New("db down")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{listResult: tt.result, listErr: tt.err}
			svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
			got, err := svc.ListDevices(context.Background(), uuid.New())
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.result, got)
		})
	}
}
```

- [ ] **Step 3: `GetDevice` — feliz, `ErrDeviceNotFound`, error de infra**

```go
func TestGetDevice(t *testing.T) {
	tenantID, deviceID := uuid.New(), uuid.New()
	infraErr := errors.New("conexión perdida")

	t.Run("feliz", func(t *testing.T) {
		repo := &fakeRepo{device: &domain.EdgeDevice{ID: deviceID, TenantID: tenantID, Name: "Edge"}}
		svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
		got, err := svc.GetDevice(context.Background(), tenantID, deviceID)
		require.NoError(t, err)
		require.Equal(t, "Edge", got.Name)
	})
	t.Run("no encontrado", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
		_, err := svc.GetDevice(context.Background(), tenantID, deviceID)
		require.ErrorIs(t, err, domain.ErrDeviceNotFound)
	})
	t.Run("error de infra propaga sin mapear", func(t *testing.T) {
		repo := &fakeRepo{getErr: infraErr}
		svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
		_, err := svc.GetDevice(context.Background(), tenantID, deviceID)
		require.ErrorIs(t, err, infraErr)
		require.NotErrorIs(t, err, domain.ErrDeviceNotFound)
	})
}
```

- [ ] **Step 4: `CreateDevice` — feliz con defaults hardcodeados + error**

```go
func TestCreateDevice(t *testing.T) {
	tenantID := uuid.New()
	t.Run("feliz setea status y health por defecto", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
		got, err := svc.CreateDevice(context.Background(), tenantID, domain.CreateDeviceCommand{
			Name: "Nuevo", MachineID: "m1", EdgeType: "RASPBERRY_PLC", RaspberryBaseURL: "http://x.local",
		})
		require.NoError(t, err)
		require.Equal(t, "ACTIVE", got.Status)
		require.Equal(t, "UNKNOWN", got.LastHealthStatus)
		require.Equal(t, tenantID, got.TenantID)
	})
	t.Run("error de repo", func(t *testing.T) {
		repo := &fakeRepo{createErr: errors.New("dup")}
		svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
		_, err := svc.CreateDevice(context.Background(), tenantID, domain.CreateDeviceCommand{})
		require.Error(t, err)
	})
}
```

- [ ] **Step 5: `EnableDevice` / `DisableDevice` — feliz, not-found, error de infra**

```go
func TestEnableDisableDevice(t *testing.T) {
	tenantID, deviceID := uuid.New(), uuid.New()
	infraErr := errors.New("timeout")

	for _, tc := range []struct {
		name     string
		call     func(*app.Service) (*domain.EdgeDevice, error)
		wantStat string
	}{
		{"enable", func(s *app.Service) (*domain.EdgeDevice, error) {
			return s.EnableDevice(context.Background(), tenantID, deviceID)
		}, "ACTIVE"},
		{"disable", func(s *app.Service) (*domain.EdgeDevice, error) {
			return s.DisableDevice(context.Background(), tenantID, deviceID)
		}, "DISABLED"},
	} {
		t.Run(tc.name+"/feliz", func(t *testing.T) {
			repo := &fakeRepo{setStatusResult: &domain.EdgeDevice{ID: deviceID, Status: tc.wantStat}}
			svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
			got, err := tc.call(svc)
			require.NoError(t, err)
			require.Equal(t, tc.wantStat, got.Status)
		})
		t.Run(tc.name+"/not_found", func(t *testing.T) {
			repo := &fakeRepo{setStatusErr: domain.ErrDeviceNotFound}
			svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
			_, err := tc.call(svc)
			require.ErrorIs(t, err, domain.ErrDeviceNotFound)
		})
		t.Run(tc.name+"/error_infra", func(t *testing.T) {
			repo := &fakeRepo{setStatusErr: infraErr}
			svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
			_, err := tc.call(svc)
			require.ErrorIs(t, err, infraErr)
		})
	}
}
```

- [ ] **Step 6: `StatusCheck` / `HealthCheck` — not-found, disabled, client error sintetiza resultado, `SaveEvent` error propaga, `UpdateHealthState` error es solo warning**

```go
func TestStatusAndHealthCheck(t *testing.T) {
	tenantID, deviceID, userID := uuid.New(), uuid.New(), uuid.New()
	activeDevice := &domain.EdgeDevice{ID: deviceID, TenantID: tenantID, Status: "ACTIVE", RaspberryBaseURL: "http://x.local"}
	disabledDevice := &domain.EdgeDevice{ID: deviceID, TenantID: tenantID, Status: "DISABLED"}

	calls := []struct {
		name string
		call func(*app.Service, context.Context) (*domain.CheckResult, error)
		setClientErr func(*fakeClient, error)
		setClientResult func(*fakeClient, *domain.CheckResult)
	}{
		{"StatusCheck", func(s *app.Service, ctx context.Context) (*domain.CheckResult, error) {
			return s.StatusCheck(ctx, tenantID, deviceID, userID, "user@x.com")
		}, func(c *fakeClient, e error) { c.statusErr = e }, func(c *fakeClient, r *domain.CheckResult) { c.statusResult = r }},
		{"HealthCheck", func(s *app.Service, ctx context.Context) (*domain.CheckResult, error) {
			return s.HealthCheck(ctx, tenantID, deviceID, userID, "user@x.com")
		}, func(c *fakeClient, e error) { c.healthErr = e }, func(c *fakeClient, r *domain.CheckResult) { c.healthResult = r }},
	}

	for _, tc := range calls {
		t.Run(tc.name+"/not_found", func(t *testing.T) {
			repo := &fakeRepo{}
			svc := app.NewService(repo, &fakeClient{}, zap.NewNop(), nil, nil)
			_, err := tc.call(svc, context.Background())
			require.ErrorIs(t, err, domain.ErrDeviceNotFound)
		})
		t.Run(tc.name+"/disabled", func(t *testing.T) {
			repo := &fakeRepo{device: disabledDevice}
			svc := app.NewService(repo, &fakeClient{}, zap.NewNop(), nil, nil)
			_, err := tc.call(svc, context.Background())
			require.ErrorIs(t, err, domain.ErrDeviceDisabled)
		})
		t.Run(tc.name+"/client_error_sintetiza_resultado_ERROR", func(t *testing.T) {
			repo := &fakeRepo{device: activeDevice}
			client := &fakeClient{}
			tc.setClientErr(client, errors.New("unreachable"))
			svc := app.NewService(repo, client, zap.NewNop(), nil, nil)
			got, err := tc.call(svc, context.Background())
			require.NoError(t, err, "un client error no falla el request, sintetiza un resultado ERROR")
			require.Equal(t, "ERROR", got.OverallStatus)
			require.Len(t, repo.saveEventCalls, 1, "el resultado sintético también se audita")
		})
		t.Run(tc.name+"/save_event_error_propaga", func(t *testing.T) {
			repo := &fakeRepo{device: activeDevice, saveEventErr: errors.New("insert failed")}
			client := &fakeClient{}
			tc.setClientResult(client, &domain.CheckResult{OverallStatus: "OK"})
			svc := app.NewService(repo, client, zap.NewNop(), nil, nil)
			_, err := tc.call(svc, context.Background())
			require.Error(t, err)
		})
		t.Run(tc.name+"/update_health_state_error_es_solo_warning", func(t *testing.T) {
			repo := &fakeRepo{device: activeDevice, updateHealthErr: errors.New("cache write failed")}
			client := &fakeClient{}
			tc.setClientResult(client, &domain.CheckResult{OverallStatus: "OK"})
			svc := app.NewService(repo, client, zap.NewNop(), nil, nil)
			got, err := tc.call(svc, context.Background())
			require.NoError(t, err, "UpdateHealthState fallando no debe fallar el request")
			require.Equal(t, "OK", got.OverallStatus)
		})
	}
}
```

- [ ] **Step 7: `GetTelemetry` — not-found, disabled, error propaga (a diferencia de los checks, NO sintetiza)**

```go
func TestGetTelemetry(t *testing.T) {
	tenantID, deviceID := uuid.New(), uuid.New()
	activeDevice := &domain.EdgeDevice{ID: deviceID, TenantID: tenantID, Status: "ACTIVE", RaspberryBaseURL: "http://x.local"}

	t.Run("not_found", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.NewService(repo, &fakeClient{}, zap.NewNop(), nil, nil)
		_, err := svc.GetTelemetry(context.Background(), tenantID, deviceID)
		require.ErrorIs(t, err, domain.ErrDeviceNotFound)
	})
	t.Run("disabled", func(t *testing.T) {
		repo := &fakeRepo{device: &domain.EdgeDevice{ID: deviceID, Status: "DISABLED"}}
		svc := app.NewService(repo, &fakeClient{}, zap.NewNop(), nil, nil)
		_, err := svc.GetTelemetry(context.Background(), tenantID, deviceID)
		require.ErrorIs(t, err, domain.ErrDeviceDisabled)
	})
	t.Run("error de client propaga sin sintetizar", func(t *testing.T) {
		repo := &fakeRepo{device: activeDevice}
		client := &fakeClient{telemetryErr: errors.New("unreachable")}
		svc := app.NewService(repo, client, zap.NewNop(), nil, nil)
		got, err := svc.GetTelemetry(context.Background(), tenantID, deviceID)
		require.Error(t, err)
		require.Nil(t, got)
	})
	t.Run("feliz", func(t *testing.T) {
		repo := &fakeRepo{device: activeDevice}
		client := &fakeClient{telemetryResult: &domain.TelemetrySnapshot{}}
		svc := app.NewService(repo, client, zap.NewNop(), nil, nil)
		got, err := svc.GetTelemetry(context.Background(), tenantID, deviceID)
		require.NoError(t, err)
		require.NotNil(t, got)
	})
}
```

- [ ] **Step 8: `ListEvents` — not_found, error, feliz**

```go
func TestListEvents(t *testing.T) {
	tenantID, deviceID := uuid.New(), uuid.New()
	t.Run("not_found", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
		_, err := svc.ListEvents(context.Background(), tenantID, deviceID)
		require.ErrorIs(t, err, domain.ErrDeviceNotFound)
	})
	t.Run("error de repo", func(t *testing.T) {
		repo := &fakeRepo{device: &domain.EdgeDevice{ID: deviceID}, listEventsErr: errors.New("db down")}
		svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
		_, err := svc.ListEvents(context.Background(), tenantID, deviceID)
		require.Error(t, err)
	})
	t.Run("feliz", func(t *testing.T) {
		want := []*domain.DeviceEvent{{ID: uuid.New()}}
		repo := &fakeRepo{device: &domain.EdgeDevice{ID: deviceID}, listEventsResult: want}
		svc := app.NewService(repo, nil, zap.NewNop(), nil, nil)
		got, err := svc.ListEvents(context.Background(), tenantID, deviceID)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
}
```

- [ ] **Step 9: `CreateAPIKey` / `ListAPIKeys` / `RevokeAPIKey`**

```go
func TestCreateAPIKey(t *testing.T) {
	tenantID, deviceID := uuid.New(), uuid.New()
	t.Run("device no existe propaga sin mapear", func(t *testing.T) {
		repo := &fakeRepo{getErr: errors.New("no existe")}
		svc := app.NewService(repo, nil, zap.NewNop(), &fakeAPIKeysRepo{}, nil)
		_, _, _, err := svc.CreateAPIKey(context.Background(), tenantID, deviceID, "k1", nil, nil)
		require.Error(t, err)
	})
	t.Run("error de apiKeys.Create", func(t *testing.T) {
		repo := &fakeRepo{device: &domain.EdgeDevice{ID: deviceID, Status: "ACTIVE"}}
		apiRepo := &fakeAPIKeysRepo{createErr: errors.New("insert failed")}
		svc := app.NewService(repo, nil, zap.NewNop(), apiRepo, nil)
		_, _, _, err := svc.CreateAPIKey(context.Background(), tenantID, deviceID, "k1", nil, nil)
		require.Error(t, err)
	})
	t.Run("feliz devuelve plaintext y el status ACTUAL del device", func(t *testing.T) {
		repo := &fakeRepo{device: &domain.EdgeDevice{ID: deviceID, Status: "ACTIVE"}}
		apiRepo := &fakeAPIKeysRepo{}
		svc := app.NewService(repo, nil, zap.NewNop(), apiRepo, nil)
		key, plaintext, status, err := svc.CreateAPIKey(context.Background(), tenantID, deviceID, "k1", nil, nil)
		require.NoError(t, err)
		require.NotEmpty(t, plaintext)
		require.NotNil(t, key)
		require.Equal(t, "ACTIVE", status)
		require.Len(t, apiRepo.createCalls, 1)
	})
}

func TestListAPIKeys(t *testing.T) {
	tenantID, deviceID := uuid.New(), uuid.New()
	t.Run("error de GetByID", func(t *testing.T) {
		repo := &fakeRepo{getErr: errors.New("boom")}
		svc := app.NewService(repo, nil, zap.NewNop(), &fakeAPIKeysRepo{}, nil)
		_, _, err := svc.ListAPIKeys(context.Background(), tenantID, deviceID)
		require.Error(t, err)
	})
	t.Run("error de ListByDevice", func(t *testing.T) {
		repo := &fakeRepo{device: &domain.EdgeDevice{ID: deviceID, Status: "ACTIVE"}}
		apiRepo := &fakeAPIKeysRepo{listErr: errors.New("db down")}
		svc := app.NewService(repo, nil, zap.NewNop(), apiRepo, nil)
		_, _, err := svc.ListAPIKeys(context.Background(), tenantID, deviceID)
		require.Error(t, err)
	})
	t.Run("feliz devuelve status del device", func(t *testing.T) {
		want := []*apikeys.APIKey{{ID: uuid.New()}}
		repo := &fakeRepo{device: &domain.EdgeDevice{ID: deviceID, Status: "DISABLED"}}
		apiRepo := &fakeAPIKeysRepo{listResult: want}
		svc := app.NewService(repo, nil, zap.NewNop(), apiRepo, nil)
		keys, status, err := svc.ListAPIKeys(context.Background(), tenantID, deviceID)
		require.NoError(t, err)
		require.Equal(t, want, keys)
		require.Equal(t, "DISABLED", status)
	})
}

func TestRevokeAPIKey(t *testing.T) {
	tenantID, deviceID, keyPK := uuid.New(), uuid.New(), uuid.New()
	t.Run("key no encontrada en el device", func(t *testing.T) {
		apiRepo := &fakeAPIKeysRepo{listResult: []*apikeys.APIKey{{ID: uuid.New()}}} // otro id
		svc := app.NewService(&fakeRepo{}, nil, zap.NewNop(), apiRepo, nil)
		err := svc.RevokeAPIKey(context.Background(), tenantID, deviceID, keyPK)
		require.ErrorIs(t, err, apikeys.ErrKeyNotFound)
	})
	t.Run("error de ListByDevice", func(t *testing.T) {
		apiRepo := &fakeAPIKeysRepo{listErr: errors.New("db down")}
		svc := app.NewService(&fakeRepo{}, nil, zap.NewNop(), apiRepo, nil)
		err := svc.RevokeAPIKey(context.Background(), tenantID, deviceID, keyPK)
		require.Error(t, err)
	})
	t.Run("error de Revoke", func(t *testing.T) {
		apiRepo := &fakeAPIKeysRepo{listResult: []*apikeys.APIKey{{ID: keyPK, KeyID: "kid1"}}, revokeErr: errors.New("db down")}
		svc := app.NewService(&fakeRepo{}, nil, zap.NewNop(), apiRepo, nil)
		err := svc.RevokeAPIKey(context.Background(), tenantID, deviceID, keyPK)
		require.Error(t, err)
	})
	t.Run("feliz con redis nil no falla (nil-safety)", func(t *testing.T) {
		apiRepo := &fakeAPIKeysRepo{listResult: []*apikeys.APIKey{{ID: keyPK, KeyID: "kid1"}}}
		svc := app.NewService(&fakeRepo{}, nil, zap.NewNop(), apiRepo, nil)
		err := svc.RevokeAPIKey(context.Background(), tenantID, deviceID, keyPK)
		require.NoError(t, err)
		require.Len(t, apiRepo.revokeCalls, 1)
		require.Equal(t, keyPK, apiRepo.revokeCalls[0])
	})
}
```

- [ ] **Step 10: Correr los tests del paquete**

Run: `go test ./internal/app/edge_devices/... -v`
Expected: PASS — todos los tests nuevos más los 2 existentes de `UpdateDevice`.

- [ ] **Step 11: Commit**

```bash
git add internal/app/edge_devices/service_test.go
git commit -m "test(edge_devices): cubrir los 12 métodos del service sin test"
```

---

## Task 2: `internal/app/roles` — `ListRoles`, `GetRole`, `CreateRole`, `CountActiveAssignments` + huecos en `UpdateRole`/`DeleteRole`

**Files:**
- Modify: `internal/app/roles/service_test.go`

**Interfaces:**
- Consumes: `rolesRepo.Repository` (7 métodos: `List`, `GetByIDForTenant`, `CountCustomByTenant`, `Create`, `Update`, `SoftDelete`, `CountActiveAssignments`).
- Produces: n/a.

El archivo ya tiene `fakeRolesRepo{role *domain.Role; hidden bool}` (package `roles_test`). Hay que extenderlo con campos de error/resultado, sin romper los 4 tests existentes (que solo usan `role`/`hidden`).

- [ ] **Step 1: Extender `fakeRolesRepo`**

Reemplazar el bloque `type fakeRolesRepo struct { ... }` y sus métodos (líneas 23-51 del archivo actual) por:

```go
type fakeRolesRepo struct {
	role   *domain.Role
	hidden bool

	listResult []*domain.Role
	listErr    error

	countCustomResult int
	countCustomErr     error

	createErr   error
	createCalls []*domain.Role

	updateErr error

	softDeleteErr   error
	softDeleteCalls []string

	countActiveResult int
	countActiveErr    error
}

func (f *fakeRolesRepo) List(ctx context.Context, tenantID uuid.UUID, includeGlobal bool) ([]*domain.Role, error) {
	return f.listResult, f.listErr
}

func (f *fakeRolesRepo) GetByIDForTenant(ctx context.Context, id string, tenantID uuid.UUID, includeGlobal bool) (*domain.Role, error) {
	if f.hidden {
		return nil, domain.ErrRoleNotFound
	}
	return f.role, nil
}

func (f *fakeRolesRepo) CountCustomByTenant(ctx context.Context, tenantID uuid.UUID) (int, error) {
	return f.countCustomResult, f.countCustomErr
}

func (f *fakeRolesRepo) Create(ctx context.Context, role *domain.Role) error {
	f.createCalls = append(f.createCalls, role)
	return f.createErr
}

func (f *fakeRolesRepo) Update(ctx context.Context, role *domain.Role) error { return f.updateErr }

func (f *fakeRolesRepo) SoftDelete(ctx context.Context, id string) error {
	f.softDeleteCalls = append(f.softDeleteCalls, id)
	return f.softDeleteErr
}

func (f *fakeRolesRepo) CountActiveAssignments(ctx context.Context, roleID string) (int, error) {
	return f.countActiveResult, f.countActiveErr
}
```

Agregar `"errors"` al bloque `import` del archivo (los tests de los Steps 2-6 lo necesitan).

- [ ] **Step 2: `ListRoles` — feliz + error**

```go
func TestListRoles(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := []*domain.Role{{ID: "admin"}}
		repo := &fakeRolesRepo{listResult: want}
		svc := appRoles.NewService(repo, zap.NewNop())
		got, err := svc.ListRoles(context.Background(), uuid.New(), false)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("error de repo", func(t *testing.T) {
		repo := &fakeRolesRepo{listErr: errors.New("db down")}
		svc := appRoles.NewService(repo, zap.NewNop())
		_, err := svc.ListRoles(context.Background(), uuid.New(), false)
		require.Error(t, err)
	})
}
```

- [ ] **Step 3: `GetRole` — feliz, `ErrRoleNotFound`, otro error**

```go
func TestGetRole(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		repo := &fakeRolesRepo{role: &domain.Role{ID: "admin"}}
		svc := appRoles.NewService(repo, zap.NewNop())
		got, err := svc.GetRole(context.Background(), "admin", uuid.New(), false)
		require.NoError(t, err)
		require.Equal(t, "admin", got.ID)
	})
	t.Run("no encontrado (oculto)", func(t *testing.T) {
		repo := &fakeRolesRepo{hidden: true}
		svc := appRoles.NewService(repo, zap.NewNop())
		_, err := svc.GetRole(context.Background(), "super_admin", uuid.New(), false)
		require.ErrorIs(t, err, domain.ErrRoleNotFound)
	})
}
```

- [ ] **Step 4: `CreateRole` — límite de roles custom, dedup de permisos, duplicate name, error genérico**

```go
func TestCreateRole(t *testing.T) {
	tenantID := uuid.New()

	t.Run("limite de roles custom alcanzado", func(t *testing.T) {
		repo := &fakeRolesRepo{countCustomResult: domain.MaxCustomRolesPerTenant}
		svc := appRoles.NewService(repo, zap.NewNop())
		_, err := svc.CreateRole(context.Background(), tenantID, "Nuevo", "desc", nil)
		require.ErrorIs(t, err, domain.ErrRoleLimitReached)
		require.Empty(t, repo.createCalls)
	})
	t.Run("error contando roles custom", func(t *testing.T) {
		repo := &fakeRolesRepo{countCustomErr: errors.New("db down")}
		svc := appRoles.NewService(repo, zap.NewNop())
		_, err := svc.CreateRole(context.Background(), tenantID, "Nuevo", "desc", nil)
		require.Error(t, err)
	})
	t.Run("dedup y sort de permisos", func(t *testing.T) {
		repo := &fakeRolesRepo{}
		svc := appRoles.NewService(repo, zap.NewNop())
		got, err := svc.CreateRole(context.Background(), tenantID, "Nuevo", "desc",
			[]string{"perm_b", " perm_a ", "perm_a", "", "perm_b"})
		require.NoError(t, err)
		require.Equal(t, []string{"perm_a", "perm_b"}, got.Permissions)
		require.Equal(t, tenantID, *got.TenantID)
	})
	t.Run("nombre duplicado propaga sin loguear como error", func(t *testing.T) {
		repo := &fakeRolesRepo{createErr: domain.ErrRoleDuplicateName}
		svc := appRoles.NewService(repo, zap.NewNop())
		_, err := svc.CreateRole(context.Background(), tenantID, "Dup", "desc", nil)
		require.ErrorIs(t, err, domain.ErrRoleDuplicateName)
	})
}
```

- [ ] **Step 5: `CountActiveAssignments` — passthrough**

```go
func TestCountActiveAssignments(t *testing.T) {
	repo := &fakeRolesRepo{countActiveResult: 3}
	svc := appRoles.NewService(repo, zap.NewNop())
	got, err := svc.CountActiveAssignments(context.Background(), "admin")
	require.NoError(t, err)
	require.Equal(t, 3, got)
}
```

- [ ] **Step 6: Completar huecos de `UpdateRole`/`DeleteRole` — happy path, `ErrRoleDuplicateName`, `ErrRoleHasAssignments`**

```go
func TestUpdateRoleHappyPathAplicaDedupDePermisos(t *testing.T) {
	repo := &fakeRolesRepo{role: &domain.Role{ID: "custom_abc", IsSystemRole: false}}
	svc := appRoles.NewService(repo, zap.NewNop())
	got, err := svc.UpdateRole(context.Background(), "custom_abc", uuid.New(), false, "Nuevo nombre", "desc", []string{"p2", "p1", "p1"})
	require.NoError(t, err)
	require.Equal(t, "Nuevo nombre", got.Name)
	require.Equal(t, []string{"p1", "p2"}, got.Permissions)
}

func TestUpdateRoleNombreDuplicado(t *testing.T) {
	repo := &fakeRolesRepo{role: &domain.Role{ID: "custom_abc"}, updateErr: domain.ErrRoleDuplicateName}
	svc := appRoles.NewService(repo, zap.NewNop())
	_, err := svc.UpdateRole(context.Background(), "custom_abc", uuid.New(), false, "x", "y", nil)
	require.ErrorIs(t, err, domain.ErrRoleDuplicateName)
}

func TestDeleteRoleHappyPath(t *testing.T) {
	repo := &fakeRolesRepo{role: &domain.Role{ID: "custom_abc", IsSystemRole: false}}
	svc := appRoles.NewService(repo, zap.NewNop())
	err := svc.DeleteRole(context.Background(), "custom_abc", uuid.New(), false)
	require.NoError(t, err)
	require.Equal(t, []string{"custom_abc"}, repo.softDeleteCalls)
}

func TestDeleteRoleConAsignacionesActivasFalla(t *testing.T) {
	repo := &fakeRolesRepo{role: &domain.Role{ID: "custom_abc", IsSystemRole: false}, countActiveResult: 2}
	svc := appRoles.NewService(repo, zap.NewNop())
	err := svc.DeleteRole(context.Background(), "custom_abc", uuid.New(), false)
	require.ErrorIs(t, err, domain.ErrRoleHasAssignments)
	require.Empty(t, repo.softDeleteCalls, "no debe borrar si hay asignaciones activas")
}

func TestDeleteRoleErrorContandoAsignaciones(t *testing.T) {
	repo := &fakeRolesRepo{role: &domain.Role{ID: "custom_abc"}, countActiveErr: errors.New("db down")}
	svc := appRoles.NewService(repo, zap.NewNop())
	err := svc.DeleteRole(context.Background(), "custom_abc", uuid.New(), false)
	require.Error(t, err)
}
```

- [ ] **Step 7: Correr los tests del paquete**

Run: `go test ./internal/app/roles/... -v`
Expected: PASS — los 4 tests existentes más todos los nuevos.

- [ ] **Step 8: Commit**

```bash
git add internal/app/roles/service_test.go
git commit -m "test(roles): cubrir ListRoles, GetRole, CreateRole, CountActiveAssignments y huecos de Update/DeleteRole"
```

---

## Task 3: `internal/app/users` — `ListUsers`, `GetUser`, `GetUserWithRoles`, `DeleteUser`, `ListPendingUsers`, `UpdateUserStatus`

**Files:**
- Create: `internal/app/users/service_unit_test.go` (nuevo — el `service_test.go` existente es de integración contra Postgres real vía `DATABASE_URL` y cubre solo `CreateUser`/`UpdateUser`; no tocarlo. Este archivo nuevo es 100% fakes, sin infraestructura, para los 6 métodos restantes).

**Interfaces:**
- Consumes: `usersRepo.Repository` (`internal/repo/pg/users/repository.go`: `ListByTenant`, `GetByID`, `GetByIDWithRoles`, `ListPendingByTenant`, `Create`, `CreateWithRole`, `Update`, `Delete`), `userRolesRepo.UserRoleRepository` (7 métodos, solo se usa `UpdateStatus`), `rolesRepo.Repository` (7 métodos, solo se usa `GetByIDForTenant` vía `appRoles.EnsureAssignable` — no se ejercita en estos 6 métodos salvo indirectamente, así que el fake puede devolver siempre éxito).
- Produces: n/a.

- [ ] **Step 1: Crear el archivo con los 3 fakes**

```go
package users_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/users"
	"github.com/tu-org/embolsadora-api/internal/domain"
	domainUsers "github.com/tu-org/embolsadora-api/internal/domain/users"
)

// fakeUsersRepo implementa usersRepo.Repository.
type fakeUsersRepo struct {
	listResult []*domainUsers.User
	listTotal  int64
	listErr    error

	getResult *domainUsers.User
	getErr    error

	getWithRolesResult *domainUsers.UserWithRoles
	getWithRolesErr    error

	listPendingResult []*domainUsers.User
	listPendingErr    error

	deleteErr   error
	deleteCalls []struct{ tenantID, userID string }

	updateResult *domainUsers.User
	updateErr    error
}

func (f *fakeUsersRepo) ListByTenant(ctx context.Context, tenantID string, limit, offset int, includeGlobal bool) ([]*domainUsers.User, int64, error) {
	return f.listResult, f.listTotal, f.listErr
}
func (f *fakeUsersRepo) GetByID(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool) (*domainUsers.User, error) {
	return f.getResult, f.getErr
}
func (f *fakeUsersRepo) GetByIDWithRoles(ctx context.Context, tenantID, userID string, crossTenant, includeGlobal bool) (*domainUsers.UserWithRoles, error) {
	return f.getWithRolesResult, f.getWithRolesErr
}
func (f *fakeUsersRepo) ListPendingByTenant(ctx context.Context, tenantID string, includeGlobal bool) ([]*domainUsers.User, error) {
	return f.listPendingResult, f.listPendingErr
}
func (f *fakeUsersRepo) Create(ctx context.Context, user *domainUsers.User) (*domainUsers.User, error) {
	return user, nil
}
func (f *fakeUsersRepo) CreateWithRole(ctx context.Context, user *domainUsers.User, utr *domain.UserTenantRole) (*domainUsers.User, error) {
	return user, nil
}
func (f *fakeUsersRepo) Update(ctx context.Context, user *domainUsers.User) (*domainUsers.User, error) {
	return f.updateResult, f.updateErr
}
func (f *fakeUsersRepo) Delete(ctx context.Context, tenantID, userID string) error {
	f.deleteCalls = append(f.deleteCalls, struct{ tenantID, userID string }{tenantID, userID})
	return f.deleteErr
}

// fakeUserRoleRepo implementa userRolesRepo.UserRoleRepository (solo UpdateStatus importa acá).
type fakeUserRoleRepo struct {
	updateStatusResult *domain.UserTenantRole
	updateStatusErr    error
	updateStatusCalls  []struct {
		userID, tenantID uuid.UUID
		status           domain.UserRoleStatus
	}
}

func (f *fakeUserRoleRepo) FindByTenant(context.Context, uuid.UUID, *string, bool) ([]domain.UserTenantRoleDetail, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) FindByID(context.Context, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) Create(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) Update(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) Revoke(context.Context, uuid.UUID, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) BulkCreate(context.Context, []domain.UserTenantRole, bool) ([]domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) FindByUser(context.Context, uuid.UUID, uuid.UUID, bool, bool) ([]domain.UserRoleWithContext, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) UpdateStatus(ctx context.Context, userID, tenantID uuid.UUID, status domain.UserRoleStatus, includeGlobal bool) (*domain.UserTenantRole, error) {
	f.updateStatusCalls = append(f.updateStatusCalls, struct {
		userID, tenantID uuid.UUID
		status           domain.UserRoleStatus
	}{userID, tenantID, status})
	return f.updateStatusResult, f.updateStatusErr
}

// fakeRolesRepoForUsers implementa rolesRepo.Repository (no se ejercita en estos 6
// métodos — ninguno llama EnsureAssignable — así que basta con satisfacer la interfaz).
type fakeRolesRepoForUsers struct{}

func (f *fakeRolesRepoForUsers) List(context.Context, uuid.UUID, bool) ([]*domain.Role, error) {
	return nil, nil
}
func (f *fakeRolesRepoForUsers) GetByIDForTenant(context.Context, string, uuid.UUID, bool) (*domain.Role, error) {
	return &domain.Role{}, nil
}
func (f *fakeRolesRepoForUsers) CountCustomByTenant(context.Context, uuid.UUID) (int, error) {
	return 0, nil
}
func (f *fakeRolesRepoForUsers) Create(context.Context, *domain.Role) error { return nil }
func (f *fakeRolesRepoForUsers) Update(context.Context, *domain.Role) error { return nil }
func (f *fakeRolesRepoForUsers) SoftDelete(context.Context, string) error   { return nil }
func (f *fakeRolesRepoForUsers) CountActiveAssignments(context.Context, string) (int, error) {
	return 0, nil
}

func newTestService(repo *fakeUsersRepo, urRepo *fakeUserRoleRepo) *app.Service {
	return app.NewService(repo, urRepo, &fakeRolesRepoForUsers{}, zap.NewNop())
}
```

- [ ] **Step 2: `ListUsers` / `ListPendingUsers` — feliz + error (mismo patrón, tests separados)**

```go
func TestListUsers(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := []*domainUsers.User{{ID: "u1"}}
		repo := &fakeUsersRepo{listResult: want, listTotal: 1}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		got, total, err := svc.ListUsers(context.Background(), "tenant1", 10, 0, false)
		require.NoError(t, err)
		require.Equal(t, want, got)
		require.EqualValues(t, 1, total)
	})
	t.Run("error de repo", func(t *testing.T) {
		repo := &fakeUsersRepo{listErr: errors.New("db down")}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		_, _, err := svc.ListUsers(context.Background(), "tenant1", 10, 0, false)
		require.Error(t, err)
	})
}

func TestListPendingUsers(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := []*domainUsers.User{{ID: "u1"}}
		repo := &fakeUsersRepo{listPendingResult: want}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		got, err := svc.ListPendingUsers(context.Background(), "tenant1", false)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("error de repo", func(t *testing.T) {
		repo := &fakeUsersRepo{listPendingErr: errors.New("db down")}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		_, err := svc.ListPendingUsers(context.Background(), "tenant1", false)
		require.Error(t, err)
	})
}
```

- [ ] **Step 3: `GetUser` / `GetUserWithRoles` — feliz, `ErrNotFound`, otro error**

```go
func TestGetUser(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		repo := &fakeUsersRepo{getResult: &domainUsers.User{ID: "u1"}}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		got, err := svc.GetUser(context.Background(), "tenant1", "u1", false, false)
		require.NoError(t, err)
		require.Equal(t, "u1", got.ID)
	})
	t.Run("no encontrado", func(t *testing.T) {
		repo := &fakeUsersRepo{getErr: domainUsers.ErrNotFound}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		_, err := svc.GetUser(context.Background(), "tenant1", "u1", false, false)
		require.ErrorIs(t, err, domainUsers.ErrNotFound)
	})
	t.Run("otro error", func(t *testing.T) {
		repo := &fakeUsersRepo{getErr: errors.New("db down")}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		_, err := svc.GetUser(context.Background(), "tenant1", "u1", false, false)
		require.Error(t, err)
		require.NotErrorIs(t, err, domainUsers.ErrNotFound)
	})
}

func TestGetUserWithRoles(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := &domainUsers.UserWithRoles{User: domainUsers.User{ID: "u1"}, Roles: []domainUsers.AssignedRole{{ID: "admin"}}}
		repo := &fakeUsersRepo{getWithRolesResult: want}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		got, err := svc.GetUserWithRoles(context.Background(), "tenant1", "u1", false, false)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("no encontrado", func(t *testing.T) {
		repo := &fakeUsersRepo{getWithRolesErr: domainUsers.ErrNotFound}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		_, err := svc.GetUserWithRoles(context.Background(), "tenant1", "u1", false, false)
		require.ErrorIs(t, err, domainUsers.ErrNotFound)
	})
}
```

- [ ] **Step 4: `DeleteUser` — `ErrNotFound` en el precheck no llama `Delete`, `ErrNotFound` en `Delete`, feliz usa `current.TenantID`**

```go
func TestDeleteUser(t *testing.T) {
	t.Run("usuario no encontrado en el precheck nunca llama Delete", func(t *testing.T) {
		repo := &fakeUsersRepo{getErr: domainUsers.ErrNotFound}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		err := svc.DeleteUser(context.Background(), "tenant-request", "u1", false, false)
		require.ErrorIs(t, err, domainUsers.ErrNotFound)
		require.Empty(t, repo.deleteCalls, "un usuario invisible en el precheck no debe llegar a Delete")
	})
	t.Run("error de Delete", func(t *testing.T) {
		repo := &fakeUsersRepo{getResult: &domainUsers.User{ID: "u1", TenantID: "tenant-real"}, deleteErr: domainUsers.ErrNotFound}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		err := svc.DeleteUser(context.Background(), "tenant-request", "u1", false, false)
		require.ErrorIs(t, err, domainUsers.ErrNotFound)
	})
	t.Run("feliz borra contra el tenant REAL del target, no el de la request", func(t *testing.T) {
		repo := &fakeUsersRepo{getResult: &domainUsers.User{ID: "u1", TenantID: "tenant-real"}}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		err := svc.DeleteUser(context.Background(), "tenant-request", "u1", true, false)
		require.NoError(t, err)
		require.Len(t, repo.deleteCalls, 1)
		require.Equal(t, "tenant-real", repo.deleteCalls[0].tenantID, "debe usar current.TenantID, no el tenantID de la request")
	})
}
```

- [ ] **Step 5: `UpdateUserStatus` — guard de auto-desactivación, status inválido, not-found, `ErrNoActiveAssignment`, feliz devuelve el snapshot pre-mutación**

```go
func TestUpdateUserStatus(t *testing.T) {
	t.Run("no puede desactivarse a si mismo", func(t *testing.T) {
		svc := newTestService(&fakeUsersRepo{}, &fakeUserRoleRepo{})
		_, err := svc.UpdateUserStatus(context.Background(), "tenant1", "u1", "u1", "inactive", false, false)
		require.ErrorIs(t, err, domainUsers.ErrCannotDeactivateSelf)
	})
	t.Run("status invalido", func(t *testing.T) {
		svc := newTestService(&fakeUsersRepo{}, &fakeUserRoleRepo{})
		_, err := svc.UpdateUserStatus(context.Background(), "tenant1", "u1", "caller1", "on-vacation", false, false)
		require.ErrorIs(t, err, domainUsers.ErrInvalidStatus)
	})
	t.Run("usuario no encontrado en el precheck", func(t *testing.T) {
		repo := &fakeUsersRepo{getErr: domainUsers.ErrNotFound}
		svc := newTestService(repo, &fakeUserRoleRepo{})
		_, err := svc.UpdateUserStatus(context.Background(), "tenant1", "u1", "caller1", "active", false, false)
		require.ErrorIs(t, err, domainUsers.ErrNotFound)
	})
	t.Run("sin asignacion activa", func(t *testing.T) {
		tenantUUID := uuid.New()
		repo := &fakeUsersRepo{getResult: &domainUsers.User{ID: "u1", TenantID: tenantUUID.String()}}
		urRepo := &fakeUserRoleRepo{updateStatusErr: domain.ErrNoActiveAssignment}
		svc := newTestService(repo, urRepo)
		_, err := svc.UpdateUserStatus(context.Background(), "tenant1", "u1", "caller1", "suspended", false, false)
		require.ErrorIs(t, err, domain.ErrNoActiveAssignment)
	})
	t.Run("feliz devuelve el snapshot pre-mutacion, no re-consulta", func(t *testing.T) {
		tenantUUID := uuid.New()
		userUUID := uuid.New()
		current := &domainUsers.User{ID: userUUID.String(), TenantID: tenantUUID.String(), FirstName: "Ana"}
		repo := &fakeUsersRepo{getResult: current}
		urRepo := &fakeUserRoleRepo{updateStatusResult: &domain.UserTenantRole{}}
		svc := newTestService(repo, urRepo)
		got, err := svc.UpdateUserStatus(context.Background(), "tenant1", userUUID.String(), "caller1", "active", false, false)
		require.NoError(t, err)
		require.Same(t, current, got, "debe devolver el mismo puntero resuelto en el precheck, sin un segundo GetByID")
		require.Len(t, urRepo.updateStatusCalls, 1)
		require.Equal(t, domain.UserRoleStatusActive, urRepo.updateStatusCalls[0].status)
		require.Equal(t, tenantUUID, urRepo.updateStatusCalls[0].tenantID, "debe usar el tenant REAL del target")
	})
}
```

- [ ] **Step 6: Correr los tests del paquete (solo los nuevos, sin tocar el archivo de integración)**

Run: `go test ./internal/app/users/... -run 'TestListUsers|TestListPendingUsers|TestGetUser|TestGetUserWithRoles|TestDeleteUser|TestUpdateUserStatus' -v`
Expected: PASS. (El resto del paquete —`TestCreateUserCon...`, `TestUpdateUserNoPuedePintarRolGlobal`— sigue gateado por `DATABASE_URL` y se skipea si no está seteada; no debe romperse.)

- [ ] **Step 7: Commit**

```bash
git add internal/app/users/service_unit_test.go
git commit -m "test(users): cubrir ListUsers, GetUser, GetUserWithRoles, DeleteUser, ListPendingUsers, UpdateUserStatus con fakes"
```

---

## Task 4: `internal/app/alarm_rules` — sin ningún test

**Files:**
- Create: `internal/app/alarm_rules/service_test.go`

**Interfaces:**
- Consumes: `alarmRepo.Repository` (`internal/repo/pg/alarm_rules/repository.go`: `List`, `GetByID`, `Create`, `Update`, `Delete`).
- Produces: n/a.

Todas las validaciones (`Name`, `Metric`, `Operator`, `Severity`) son puras — no golpean el repo — así que varios tests de `CreateAlarmRule`/`UpdateAlarmRule` pueden usar un fake completamente vacío.

- [ ] **Step 1: Crear el archivo con el fake y `ListAlarmRules`/`GetAlarmRule`**

```go
package alarm_rules_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/alarm_rules"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	listResult []*domain.AlarmRule
	listErr    error

	getResult *domain.AlarmRule
	getErr    error

	createErr   error
	createCalls []*domain.AlarmRule

	updateErr   error
	updateCalls []*domain.AlarmRule

	deleteErr error
}

func (f *fakeRepo) List(context.Context, uuid.UUID) ([]*domain.AlarmRule, error) {
	return f.listResult, f.listErr
}
func (f *fakeRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.AlarmRule, error) {
	return f.getResult, f.getErr
}
func (f *fakeRepo) Create(_ context.Context, r *domain.AlarmRule) error {
	f.createCalls = append(f.createCalls, r)
	return f.createErr
}
func (f *fakeRepo) Update(_ context.Context, r *domain.AlarmRule) error {
	f.updateCalls = append(f.updateCalls, r)
	return f.updateErr
}
func (f *fakeRepo) Delete(context.Context, uuid.UUID, uuid.UUID) error { return f.deleteErr }

func validInput() app.CreateAlarmRuleInput {
	return app.CreateAlarmRuleInput{
		Name: "Temp alta", Metric: "temperature", Operator: "gt", Threshold: 80, Severity: "critical", Enabled: true,
	}
}

func TestListAlarmRules(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := []*domain.AlarmRule{{ID: uuid.New()}}
		svc := app.NewService(&fakeRepo{listResult: want}, zap.NewNop())
		got, err := svc.ListAlarmRules(context.Background(), uuid.New())
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("error de repo", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{listErr: errors.New("db down")}, zap.NewNop())
		_, err := svc.ListAlarmRules(context.Background(), uuid.New())
		require.Error(t, err)
	})
}

func TestGetAlarmRule(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := &domain.AlarmRule{ID: uuid.New()}
		svc := app.NewService(&fakeRepo{getResult: want}, zap.NewNop())
		got, err := svc.GetAlarmRule(context.Background(), want.ID, uuid.New())
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("no encontrada", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{getErr: domain.ErrAlarmRuleNotFound}, zap.NewNop())
		_, err := svc.GetAlarmRule(context.Background(), uuid.New(), uuid.New())
		require.ErrorIs(t, err, domain.ErrAlarmRuleNotFound)
	})
}
```

- [ ] **Step 2: `CreateAlarmRule` — las 4 validaciones + feliz + error de repo (table-driven)**

```go
func TestCreateAlarmRule(t *testing.T) {
	tenantID := uuid.New()

	tests := []struct {
		name    string
		mutate  func(*app.CreateAlarmRuleInput)
		wantErr error
	}{
		{"nombre vacio", func(i *app.CreateAlarmRuleInput) { i.Name = "" }, app.ErrNameRequired},
		{"metrica vacia", func(i *app.CreateAlarmRuleInput) { i.Metric = "" }, app.ErrMetricRequired},
		{"operador invalido", func(i *app.CreateAlarmRuleInput) { i.Operator = "contains" }, app.ErrInvalidOperator},
		{"severidad invalida", func(i *app.CreateAlarmRuleInput) { i.Severity = "urgent" }, app.ErrInvalidSeverity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := validInput()
			tt.mutate(&input)
			repo := &fakeRepo{}
			svc := app.NewService(repo, zap.NewNop())
			_, err := svc.CreateAlarmRule(context.Background(), tenantID, input)
			require.ErrorIs(t, err, tt.wantErr)
			require.Empty(t, repo.createCalls, "una validacion fallida no debe llegar al repo")
		})
	}

	t.Run("feliz", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.NewService(repo, zap.NewNop())
		got, err := svc.CreateAlarmRule(context.Background(), tenantID, validInput())
		require.NoError(t, err)
		require.Equal(t, tenantID, got.TenantID)
		require.Equal(t, "gt", got.Operator)
		require.Len(t, repo.createCalls, 1)
	})

	t.Run("error de repo", func(t *testing.T) {
		repo := &fakeRepo{createErr: errors.New("db down")}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.CreateAlarmRule(context.Background(), tenantID, validInput())
		require.Error(t, err)
	})
}
```

- [ ] **Step 3: `UpdateAlarmRule` — not-found en `GetByID` no llama `Update`, campos opcionales, validaciones sobre campos seteados, feliz**

```go
func strp(s string) *string { return &s }
func f64p(f float64) *float64 { return &f }
func boolp(b bool) *bool { return &b }

func TestUpdateAlarmRule(t *testing.T) {
	tenantID, ruleID := uuid.New(), uuid.New()
	existing := &domain.AlarmRule{ID: ruleID, TenantID: tenantID, Name: "Vieja", Metric: "temp", Operator: "gt", Severity: "info", Threshold: 10, Enabled: false}

	t.Run("regla no encontrada no llama Update", func(t *testing.T) {
		repo := &fakeRepo{getErr: domain.ErrAlarmRuleNotFound}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.UpdateAlarmRule(context.Background(), ruleID, tenantID, app.UpdateAlarmRuleInput{})
		require.ErrorIs(t, err, domain.ErrAlarmRuleNotFound)
		require.Empty(t, repo.updateCalls)
	})

	t.Run("solo threshold y enabled, sin validacion", func(t *testing.T) {
		repo := &fakeRepo{getResult: existing}
		svc := app.NewService(repo, zap.NewNop())
		got, err := svc.UpdateAlarmRule(context.Background(), ruleID, tenantID, app.UpdateAlarmRuleInput{
			Threshold: f64p(999), Enabled: boolp(true),
		})
		require.NoError(t, err)
		require.Equal(t, 999.0, got.Threshold)
		require.True(t, got.Enabled)
		require.Equal(t, "Vieja", got.Name, "campos no enviados no cambian")
	})

	t.Run("nombre vacio explicito falla", func(t *testing.T) {
		repo := &fakeRepo{getResult: existing}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.UpdateAlarmRule(context.Background(), ruleID, tenantID, app.UpdateAlarmRuleInput{Name: strp("")})
		require.ErrorIs(t, err, app.ErrNameRequired)
	})

	t.Run("operador invalido explicito falla", func(t *testing.T) {
		repo := &fakeRepo{getResult: existing}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.UpdateAlarmRule(context.Background(), ruleID, tenantID, app.UpdateAlarmRuleInput{Operator: strp("contains")})
		require.ErrorIs(t, err, app.ErrInvalidOperator)
	})

	t.Run("feliz actualiza todos los campos enviados", func(t *testing.T) {
		repo := &fakeRepo{getResult: existing}
		svc := app.NewService(repo, zap.NewNop())
		got, err := svc.UpdateAlarmRule(context.Background(), ruleID, tenantID, app.UpdateAlarmRuleInput{
			Name: strp("Nueva"), Operator: strp("lte"), Severity: strp("warning"),
		})
		require.NoError(t, err)
		require.Equal(t, "Nueva", got.Name)
		require.Equal(t, "lte", got.Operator)
		require.Equal(t, "warning", got.Severity)
		require.Len(t, repo.updateCalls, 1)
	})
}
```

- [ ] **Step 4: `DeleteAlarmRule` — not-found, otro error, feliz**

```go
func TestDeleteAlarmRule(t *testing.T) {
	t.Run("no encontrada", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{deleteErr: domain.ErrAlarmRuleNotFound}, zap.NewNop())
		err := svc.DeleteAlarmRule(context.Background(), uuid.New(), uuid.New())
		require.ErrorIs(t, err, domain.ErrAlarmRuleNotFound)
	})
	t.Run("otro error", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{deleteErr: errors.New("db down")}, zap.NewNop())
		err := svc.DeleteAlarmRule(context.Background(), uuid.New(), uuid.New())
		require.Error(t, err)
	})
	t.Run("feliz", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{}, zap.NewNop())
		err := svc.DeleteAlarmRule(context.Background(), uuid.New(), uuid.New())
		require.NoError(t, err)
	})
}
```

- [ ] **Step 5: Correr los tests del paquete**

Run: `go test ./internal/app/alarm_rules/... -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/alarm_rules/service_test.go
git commit -m "test(alarm_rules): cubrir el service completo (paquete no tenía ningún test)"
```

---

## Task 5: `internal/app/dashboard_layouts` — sin ningún test

**Files:**
- Create: `internal/app/dashboard_layouts/service_test.go`

**Interfaces:**
- Consumes: `domain.Repository` (`internal/domain/dashboard_layouts/repository.go`: `List`, `GetByID`, `CountByTenantUser`, `Create`, `Update`, `SoftDelete`). El límite (3 layouts) y la unicidad de nombre se resuelven atómicamente en el repo real — el fake solo necesita devolver `ErrLimitReached`/`ErrDuplicateName` cuando se lo pidan, no reimplementar el conteo.
- Produces: n/a.

- [ ] **Step 1: Crear el archivo con el fake**

```go
package dashboard_layouts_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/dashboard_layouts"
	domain "github.com/tu-org/embolsadora-api/internal/domain/dashboard_layouts"
)

type fakeRepo struct {
	listResult []*domain.DashboardLayout
	listErr    error

	getResult *domain.DashboardLayout
	getErr    error

	createErr   error
	createCalls []*domain.DashboardLayout

	updateErr   error
	updateCalls []*domain.DashboardLayout

	softDeleteErr error
}

func (f *fakeRepo) List(context.Context, uuid.UUID, uuid.UUID) ([]*domain.DashboardLayout, error) {
	return f.listResult, f.listErr
}
func (f *fakeRepo) GetByID(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) (*domain.DashboardLayout, error) {
	return f.getResult, f.getErr
}
func (f *fakeRepo) CountByTenantUser(context.Context, uuid.UUID, uuid.UUID) (int, error) {
	return 0, nil
}
func (f *fakeRepo) Create(_ context.Context, l *domain.DashboardLayout) error {
	f.createCalls = append(f.createCalls, l)
	return f.createErr
}
func (f *fakeRepo) Update(_ context.Context, l *domain.DashboardLayout) error {
	f.updateCalls = append(f.updateCalls, l)
	return f.updateErr
}
func (f *fakeRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID, uuid.UUID) error {
	return f.softDeleteErr
}
```

- [ ] **Step 2: `ListLayouts` / `GetLayout`**

```go
func TestListLayouts(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := []*domain.DashboardLayout{{ID: uuid.New()}}
		svc := app.NewService(&fakeRepo{listResult: want}, zap.NewNop())
		got, err := svc.ListLayouts(context.Background(), uuid.New(), uuid.New())
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("error de repo", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{listErr: errors.New("db down")}, zap.NewNop())
		_, err := svc.ListLayouts(context.Background(), uuid.New(), uuid.New())
		require.Error(t, err)
	})
}

func TestGetLayout(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := &domain.DashboardLayout{ID: uuid.New()}
		svc := app.NewService(&fakeRepo{getResult: want}, zap.NewNop())
		got, err := svc.GetLayout(context.Background(), uuid.New(), uuid.New(), want.ID)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("no encontrado", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{getErr: domain.ErrLayoutNotFound}, zap.NewNop())
		_, err := svc.GetLayout(context.Background(), uuid.New(), uuid.New(), uuid.New())
		require.ErrorIs(t, err, domain.ErrLayoutNotFound)
	})
}
```

- [ ] **Step 3: `CreateLayout` — normaliza `Widgets` nil, límite, nombre duplicado, error genérico**

```go
func TestCreateLayout(t *testing.T) {
	tenantID, userID := uuid.New(), uuid.New()

	t.Run("widgets nil se normaliza a slice vacio, no nil", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.NewService(repo, zap.NewNop())
		got, err := svc.CreateLayout(context.Background(), tenantID, userID, domain.CreateLayoutCommand{Name: "Mi layout"})
		require.NoError(t, err)
		require.NotNil(t, got.Widgets)
		require.Empty(t, got.Widgets)
	})
	t.Run("limite alcanzado propaga sin loguear como error", func(t *testing.T) {
		repo := &fakeRepo{createErr: domain.ErrLimitReached}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.CreateLayout(context.Background(), tenantID, userID, domain.CreateLayoutCommand{Name: "x"})
		require.ErrorIs(t, err, domain.ErrLimitReached)
	})
	t.Run("nombre duplicado", func(t *testing.T) {
		repo := &fakeRepo{createErr: domain.ErrDuplicateName}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.CreateLayout(context.Background(), tenantID, userID, domain.CreateLayoutCommand{Name: "x"})
		require.ErrorIs(t, err, domain.ErrDuplicateName)
	})
	t.Run("feliz preserva los widgets enviados", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.NewService(repo, zap.NewNop())
		widgets := []domain.Widget{{ID: "w1", Type: "chart"}}
		got, err := svc.CreateLayout(context.Background(), tenantID, userID, domain.CreateLayoutCommand{Name: "x", Widgets: widgets})
		require.NoError(t, err)
		require.Equal(t, widgets, got.Widgets)
		require.Equal(t, tenantID, got.TenantID)
		require.Equal(t, userID, got.UserID)
	})
}
```

- [ ] **Step 4: `UpdateLayout` — not-found inicial, normaliza widgets, not-found/duplicate en `Update`, feliz**

```go
func TestUpdateLayout(t *testing.T) {
	tenantID, userID, layoutID := uuid.New(), uuid.New(), uuid.New()
	existing := &domain.DashboardLayout{ID: layoutID, TenantID: tenantID, UserID: userID, Name: "Vieja"}

	t.Run("no encontrado en el GetByID inicial", func(t *testing.T) {
		repo := &fakeRepo{getErr: domain.ErrLayoutNotFound}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.UpdateLayout(context.Background(), tenantID, userID, layoutID, domain.UpdateLayoutCommand{})
		require.ErrorIs(t, err, domain.ErrLayoutNotFound)
	})
	t.Run("nombre duplicado en el Update", func(t *testing.T) {
		repo := &fakeRepo{getResult: existing, updateErr: domain.ErrDuplicateName}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.UpdateLayout(context.Background(), tenantID, userID, layoutID, domain.UpdateLayoutCommand{Name: "Otra"})
		require.ErrorIs(t, err, domain.ErrDuplicateName)
	})
	t.Run("feliz normaliza widgets nil", func(t *testing.T) {
		repo := &fakeRepo{getResult: existing}
		svc := app.NewService(repo, zap.NewNop())
		got, err := svc.UpdateLayout(context.Background(), tenantID, userID, layoutID, domain.UpdateLayoutCommand{Name: "Nueva"})
		require.NoError(t, err)
		require.Equal(t, "Nueva", got.Name)
		require.NotNil(t, got.Widgets)
	})
}
```

- [ ] **Step 5: `DeleteLayout` — not-found, `ErrCannotDeleteLastLayout`, otro error, feliz**

```go
func TestDeleteLayout(t *testing.T) {
	t.Run("no encontrado", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{softDeleteErr: domain.ErrLayoutNotFound}, zap.NewNop())
		err := svc.DeleteLayout(context.Background(), uuid.New(), uuid.New(), uuid.New())
		require.ErrorIs(t, err, domain.ErrLayoutNotFound)
	})
	t.Run("no se puede borrar el ultimo layout", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{softDeleteErr: domain.ErrCannotDeleteLastLayout}, zap.NewNop())
		err := svc.DeleteLayout(context.Background(), uuid.New(), uuid.New(), uuid.New())
		require.ErrorIs(t, err, domain.ErrCannotDeleteLastLayout)
	})
	t.Run("feliz", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{}, zap.NewNop())
		err := svc.DeleteLayout(context.Background(), uuid.New(), uuid.New(), uuid.New())
		require.NoError(t, err)
	})
}
```

- [ ] **Step 6: Correr los tests del paquete**

Run: `go test ./internal/app/dashboard_layouts/... -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/app/dashboard_layouts/service_test.go
git commit -m "test(dashboard_layouts): cubrir el service completo (paquete no tenía ningún test)"
```

---

## Task 6: `internal/app/logs` — sin ningún test

**Files:**
- Create: `internal/app/logs/service_test.go`

**Interfaces:**
- Consumes: `logsRepo.Repository` (`internal/repo/pg/logs/repository.go`: `Write`, `List`, `Get`, `GetContext`, `Export`, `GetRetention`, `UpsertRetention`).
- Produces: n/a.

**Importante:** el `Service` tiene un mapa interno (`subs`) que se inicializa en `New(...)` — siempre construir el service con `app.New(repo, zap.NewNop())`, nunca con `&app.Service{}` a mano (no es posible desde fuera del paquete de todos modos, pero vale la aclaración para quien lea el código).

- [ ] **Step 1: Crear el archivo con el fake**

```go
package logs_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/logs"
	"github.com/tu-org/embolsadora-api/internal/domain"
	logsRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/logs"
)

type fakeRepo struct {
	mu sync.Mutex

	listResult []domain.LogEntry
	listTotal  int
	listErr    error
	lastListParams logsRepo.ListParams

	getResult *domain.LogEntry
	getErr    error

	getContextBefore []domain.LogEntry
	getContextAnchor *domain.LogEntry
	getContextAfter  []domain.LogEntry
	getContextErr    error
	lastWindowSize   int

	exportResult []domain.LogEntry
	exportTotal  int
	exportErr    error
	lastExportParams logsRepo.ExportParams

	retentionResult *domain.RetentionPolicy
	retentionErr    error

	upsertRetentionResult *domain.RetentionPolicy
	upsertRetentionErr    error

	writeResult *domain.LogEntry
	writeErr    error
}

func (f *fakeRepo) Write(_ context.Context, entry *domain.LogEntry) (*domain.LogEntry, error) {
	if f.writeErr != nil {
		return nil, f.writeErr
	}
	if f.writeResult != nil {
		return f.writeResult, nil
	}
	return entry, nil
}
func (f *fakeRepo) List(_ context.Context, params logsRepo.ListParams) ([]domain.LogEntry, int, error) {
	f.lastListParams = params
	return f.listResult, f.listTotal, f.listErr
}
func (f *fakeRepo) Get(context.Context, uuid.UUID, uuid.UUID) (*domain.LogEntry, error) {
	return f.getResult, f.getErr
}
func (f *fakeRepo) GetContext(_ context.Context, _ uuid.UUID, _ uuid.UUID, windowSize int) ([]domain.LogEntry, *domain.LogEntry, []domain.LogEntry, error) {
	f.lastWindowSize = windowSize
	return f.getContextBefore, f.getContextAnchor, f.getContextAfter, f.getContextErr
}
func (f *fakeRepo) Export(_ context.Context, params logsRepo.ExportParams) ([]domain.LogEntry, int, error) {
	f.lastExportParams = params
	return f.exportResult, f.exportTotal, f.exportErr
}
func (f *fakeRepo) GetRetention(context.Context, uuid.UUID) (*domain.RetentionPolicy, error) {
	return f.retentionResult, f.retentionErr
}
func (f *fakeRepo) UpsertRetention(context.Context, *domain.RetentionPolicy) (*domain.RetentionPolicy, error) {
	return f.upsertRetentionResult, f.upsertRetentionErr
}
```

- [ ] **Step 2: `List` — clamp de `Limit`, `NextCursor` presente/ausente, error**

```go
func TestList(t *testing.T) {
	t.Run("limit menor o igual a 0 clampea a 50", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.New(repo, zap.NewNop())
		_, err := svc.List(context.Background(), logsRepo.ListParams{Limit: 0})
		require.NoError(t, err)
		require.Equal(t, 50, repo.lastListParams.Limit)
	})
	t.Run("limit mayor a 100 clampea a 50", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.New(repo, zap.NewNop())
		_, err := svc.List(context.Background(), logsRepo.ListParams{Limit: 500})
		require.NoError(t, err)
		require.Equal(t, 50, repo.lastListParams.Limit)
	})
	t.Run("trae exactamente el limit: hay NextCursor", func(t *testing.T) {
		entries := make([]domain.LogEntry, 10)
		for i := range entries {
			entries[i] = domain.LogEntry{ID: uuid.New(), CreatedAt: time.Now()}
		}
		repo := &fakeRepo{listResult: entries, listTotal: 100}
		svc := app.New(repo, zap.NewNop())
		got, err := svc.List(context.Background(), logsRepo.ListParams{Limit: 10})
		require.NoError(t, err)
		require.NotNil(t, got.NextCursor)
		require.Equal(t, 100, got.Total)
	})
	t.Run("trae menos que el limit: no hay NextCursor", func(t *testing.T) {
		repo := &fakeRepo{listResult: []domain.LogEntry{{ID: uuid.New(), CreatedAt: time.Now()}}}
		svc := app.New(repo, zap.NewNop())
		got, err := svc.List(context.Background(), logsRepo.ListParams{Limit: 50})
		require.NoError(t, err)
		require.Nil(t, got.NextCursor)
	})
	t.Run("error de repo", func(t *testing.T) {
		svc := app.New(&fakeRepo{listErr: errors.New("db down")}, zap.NewNop())
		_, err := svc.List(context.Background(), logsRepo.ListParams{})
		require.Error(t, err)
	})
}
```

- [ ] **Step 3: `Get` / `GetContext` — passthrough y clamp de `windowSize`**

```go
func TestGet(t *testing.T) {
	want := &domain.LogEntry{ID: uuid.New()}
	svc := app.New(&fakeRepo{getResult: want}, zap.NewNop())
	got, err := svc.Get(context.Background(), uuid.New(), want.ID)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestGetContext(t *testing.T) {
	tests := []struct {
		name  string
		input int
		want  int
	}{
		{"cero clampea a 10", 0, 10},
		{"negativo clampea a 10", -5, 10},
		{"mayor a 50 clampea a 10", 51, 10},
		{"dentro de rango se respeta", 20, 20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{}
			svc := app.New(repo, zap.NewNop())
			_, _, _, err := svc.GetContext(context.Background(), uuid.New(), uuid.New(), tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.want, repo.lastWindowSize)
		})
	}
}
```

- [ ] **Step 4: `Export` — fuerza `MaxRows`, trunca y marca `Truncated`, error**

```go
func TestExport(t *testing.T) {
	t.Run("fuerza MaxRows a 50001 sin importar el input", func(t *testing.T) {
		repo := &fakeRepo{}
		svc := app.New(repo, zap.NewNop())
		_, err := svc.Export(context.Background(), logsRepo.ExportParams{MaxRows: 10})
		require.NoError(t, err)
		require.Equal(t, 50001, repo.lastExportParams.MaxRows)
	})
	t.Run("por debajo del limite no trunca", func(t *testing.T) {
		entries := make([]domain.LogEntry, 100)
		repo := &fakeRepo{exportResult: entries, exportTotal: 100}
		svc := app.New(repo, zap.NewNop())
		got, err := svc.Export(context.Background(), logsRepo.ExportParams{})
		require.NoError(t, err)
		require.False(t, got.Truncated)
		require.Len(t, got.Entries, 100)
	})
	t.Run("por encima de 50000 trunca y marca Truncated", func(t *testing.T) {
		entries := make([]domain.LogEntry, 50001)
		repo := &fakeRepo{exportResult: entries, exportTotal: 60000}
		svc := app.New(repo, zap.NewNop())
		got, err := svc.Export(context.Background(), logsRepo.ExportParams{})
		require.NoError(t, err)
		require.True(t, got.Truncated)
		require.Len(t, got.Entries, 50000)
		require.Equal(t, 60000, got.TotalAvailable)
	})
	t.Run("error de repo", func(t *testing.T) {
		svc := app.New(&fakeRepo{exportErr: errors.New("db down")}, zap.NewNop())
		_, err := svc.Export(context.Background(), logsRepo.ExportParams{})
		require.Error(t, err)
	})
}
```

- [ ] **Step 5: `GetRetention` — default sintético sin persistir, error real, otro comportamiento**

```go
func TestGetRetention(t *testing.T) {
	tenantID := uuid.New()
	t.Run("sin politica devuelve default 90 dias sin persistir", func(t *testing.T) {
		repo := &fakeRepo{retentionErr: domain.ErrRetentionNotFound}
		svc := app.New(repo, zap.NewNop())
		got, err := svc.GetRetention(context.Background(), tenantID)
		require.NoError(t, err)
		require.Equal(t, 90, got.RetentionDays)
		require.Equal(t, tenantID, got.TenantID)
	})
	t.Run("politica existente pasa tal cual", func(t *testing.T) {
		want := &domain.RetentionPolicy{TenantID: tenantID, RetentionDays: 30}
		repo := &fakeRepo{retentionResult: want}
		svc := app.New(repo, zap.NewNop())
		got, err := svc.GetRetention(context.Background(), tenantID)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("otro error propaga", func(t *testing.T) {
		repo := &fakeRepo{retentionErr: errors.New("db down")}
		svc := app.New(repo, zap.NewNop())
		_, err := svc.GetRetention(context.Background(), tenantID)
		require.Error(t, err)
	})
}
```

- [ ] **Step 6: `UpdateRetention` — validación de rango, feliz**

```go
func TestUpdateRetention(t *testing.T) {
	tenantID := uuid.New()
	t.Run("menor a 1 falla", func(t *testing.T) {
		svc := app.New(&fakeRepo{}, zap.NewNop())
		_, err := svc.UpdateRetention(context.Background(), tenantID, 0)
		require.ErrorIs(t, err, domain.ErrInvalidRetentionDays)
	})
	t.Run("mayor a 3650 falla", func(t *testing.T) {
		svc := app.New(&fakeRepo{}, zap.NewNop())
		_, err := svc.UpdateRetention(context.Background(), tenantID, 3651)
		require.ErrorIs(t, err, domain.ErrInvalidRetentionDays)
	})
	t.Run("feliz", func(t *testing.T) {
		want := &domain.RetentionPolicy{TenantID: tenantID, RetentionDays: 60}
		repo := &fakeRepo{upsertRetentionResult: want}
		svc := app.New(repo, zap.NewNop())
		got, err := svc.UpdateRetention(context.Background(), tenantID, 60)
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
}
```

- [ ] **Step 7: `Subscribe`/`Unsubscribe`/`Publish`/`Write` — pub/sub en memoria (correr con `-race`)**

```go
func TestSubscribePublishUnsubscribe(t *testing.T) {
	tenantID := uuid.New()
	svc := app.New(&fakeRepo{}, zap.NewNop())

	ch := svc.Subscribe(tenantID)
	entry := &domain.LogEntry{ID: uuid.New(), TenantID: tenantID}
	svc.Publish(tenantID, entry)

	select {
	case got := <-ch:
		require.Equal(t, entry, got)
	case <-time.After(time.Second):
		t.Fatal("no se recibió el entry publicado")
	}

	svc.Unsubscribe(tenantID, ch)
	svc.Publish(tenantID, &domain.LogEntry{ID: uuid.New(), TenantID: tenantID})
	select {
	case _, ok := <-ch:
		require.False(t, ok, "el channel debe estar cerrado tras Unsubscribe")
	case <-time.After(100 * time.Millisecond):
		t.Fatal("el channel debería estar cerrado, no bloqueado")
	}

	require.NotPanics(t, func() { svc.Unsubscribe(tenantID, ch) }, "un segundo Unsubscribe del mismo channel no debe panickear")
}

func TestPublishConSubscriberLentoNoBloquea(t *testing.T) {
	tenantID := uuid.New()
	svc := app.New(&fakeRepo{}, zap.NewNop())
	svc.Subscribe(tenantID) // nadie lee de este channel

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			svc.Publish(tenantID, &domain.LogEntry{ID: uuid.New(), TenantID: tenantID})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish no debe bloquear cuando el subscriber está lleno/lento")
	}
}

func TestWritePublicaAlTenantDelEntryPersistido(t *testing.T) {
	tenantID := uuid.New()
	persisted := &domain.LogEntry{ID: uuid.New(), TenantID: tenantID}
	repo := &fakeRepo{writeResult: persisted}
	svc := app.New(repo, zap.NewNop())

	ch := svc.Subscribe(tenantID)
	err := svc.Write(context.Background(), &domain.LogEntry{TenantID: tenantID})
	require.NoError(t, err)

	select {
	case got := <-ch:
		require.Equal(t, persisted, got)
	case <-time.After(time.Second):
		t.Fatal("Write debe publicar el entry persistido a los suscriptores")
	}
}

func TestWriteErrorDeRepoNoPublica(t *testing.T) {
	svc := app.New(&fakeRepo{writeErr: errors.New("db down")}, zap.NewNop())
	err := svc.Write(context.Background(), &domain.LogEntry{})
	require.Error(t, err)
}
```

- [ ] **Step 8: Correr los tests del paquete con `-race`**

Run: `go test ./internal/app/logs/... -race -v`
Expected: PASS, sin warnings de data race.

- [ ] **Step 9: Commit**

```bash
git add internal/app/logs/service_test.go
git commit -m "test(logs): cubrir el service completo incluido pub/sub (paquete no tenía ningún test)"
```

---

## Task 7: `internal/app/notifications` — sin ningún test

**Files:**
- Create: `internal/app/notifications/service_test.go`

**Interfaces:**
- Consumes: `notifRepo.Repository` (`internal/repo/pg/notifications/repository.go`: `List`, `CountUnread`, `GetByID`, `Ack`, `Close`).
- Produces: n/a.

- [ ] **Step 1: Crear el archivo con el fake**

```go
package notifications_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/notifications"
	"github.com/tu-org/embolsadora-api/internal/domain"
	notifRepo "github.com/tu-org/embolsadora-api/internal/repo/pg/notifications"
)

type fakeRepo struct {
	listResult []*domain.Notification
	listTotal  int
	listErr    error
	lastListParams notifRepo.ListParams

	countUnreadResult int
	countUnreadErr    error

	getResult *domain.Notification
	getErr    error

	ackResult *domain.Notification
	ackErr    error

	closeResult *domain.Notification
	closeErr    error
}

func (f *fakeRepo) List(_ context.Context, _ uuid.UUID, params notifRepo.ListParams) ([]*domain.Notification, int, error) {
	f.lastListParams = params
	return f.listResult, f.listTotal, f.listErr
}
func (f *fakeRepo) CountUnread(context.Context, uuid.UUID) (int, error) {
	return f.countUnreadResult, f.countUnreadErr
}
func (f *fakeRepo) GetByID(context.Context, uuid.UUID, uuid.UUID) (*domain.Notification, error) {
	return f.getResult, f.getErr
}
func (f *fakeRepo) Ack(context.Context, uuid.UUID, uuid.UUID) (*domain.Notification, error) {
	return f.ackResult, f.ackErr
}
func (f *fakeRepo) Close(context.Context, uuid.UUID, uuid.UUID) (*domain.Notification, error) {
	return f.closeResult, f.closeErr
}
```

- [ ] **Step 2: `List` — clamp de `Limit`/`Offset` (table-driven) + error**

```go
func TestList(t *testing.T) {
	tests := []struct {
		name       string
		in         notifRepo.ListParams
		wantLimit  int
		wantOffset int
	}{
		{"limit <= 0 clampea a 20", notifRepo.ListParams{Limit: 0}, 20, 0},
		{"limit > 100 clampea a 100", notifRepo.ListParams{Limit: 500}, 100, 0},
		{"offset negativo clampea a 0", notifRepo.ListParams{Limit: 10, Offset: -5}, 10, 0},
		{"dentro de rango se respeta", notifRepo.ListParams{Limit: 30, Offset: 10}, 30, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepo{}
			svc := app.New(repo, zap.NewNop())
			_, _, err := svc.List(context.Background(), uuid.New(), tt.in)
			require.NoError(t, err)
			require.Equal(t, tt.wantLimit, repo.lastListParams.Limit)
			require.Equal(t, tt.wantOffset, repo.lastListParams.Offset)
		})
	}
	t.Run("error de repo", func(t *testing.T) {
		svc := app.New(&fakeRepo{listErr: errors.New("db down")}, zap.NewNop())
		_, _, err := svc.List(context.Background(), uuid.New(), notifRepo.ListParams{})
		require.Error(t, err)
	})
}
```

- [ ] **Step 3: `CountUnread` — feliz + error**

```go
func TestCountUnread(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		svc := app.New(&fakeRepo{countUnreadResult: 7}, zap.NewNop())
		got, err := svc.CountUnread(context.Background(), uuid.New())
		require.NoError(t, err)
		require.Equal(t, 7, got)
	})
	t.Run("error", func(t *testing.T) {
		svc := app.New(&fakeRepo{countUnreadErr: errors.New("db down")}, zap.NewNop())
		_, err := svc.CountUnread(context.Background(), uuid.New())
		require.Error(t, err)
	})
}
```

- [ ] **Step 4: `Get` / `Ack` / `Close` — feliz, `ErrNotificationNotFound`, otro error (table-driven sobre las 3 operaciones)**

```go
func TestGetAckClose(t *testing.T) {
	notif := &domain.Notification{ID: uuid.New(), Status: domain.StatusAcknowledged}

	cases := []struct {
		name string
		call func(*app.Service, *fakeRepo) (*domain.Notification, error)
		setResult func(*fakeRepo, *domain.Notification)
		setErr    func(*fakeRepo, error)
	}{
		{"Get", func(s *app.Service, r *fakeRepo) (*domain.Notification, error) {
			return s.Get(context.Background(), notif.ID, uuid.New())
		}, func(r *fakeRepo, n *domain.Notification) { r.getResult = n }, func(r *fakeRepo, e error) { r.getErr = e }},
		{"Ack", func(s *app.Service, r *fakeRepo) (*domain.Notification, error) {
			return s.Ack(context.Background(), notif.ID, uuid.New())
		}, func(r *fakeRepo, n *domain.Notification) { r.ackResult = n }, func(r *fakeRepo, e error) { r.ackErr = e }},
		{"Close", func(s *app.Service, r *fakeRepo) (*domain.Notification, error) {
			return s.Close(context.Background(), notif.ID, uuid.New())
		}, func(r *fakeRepo, n *domain.Notification) { r.closeResult = n }, func(r *fakeRepo, e error) { r.closeErr = e }},
	}

	for _, tc := range cases {
		t.Run(tc.name+"/feliz", func(t *testing.T) {
			repo := &fakeRepo{}
			tc.setResult(repo, notif)
			svc := app.New(repo, zap.NewNop())
			got, err := tc.call(svc, repo)
			require.NoError(t, err)
			require.Equal(t, notif, got)
		})
		t.Run(tc.name+"/no_encontrada", func(t *testing.T) {
			repo := &fakeRepo{}
			tc.setErr(repo, domain.ErrNotificationNotFound)
			svc := app.New(repo, zap.NewNop())
			_, err := tc.call(svc, repo)
			require.ErrorIs(t, err, domain.ErrNotificationNotFound)
		})
		t.Run(tc.name+"/otro_error", func(t *testing.T) {
			repo := &fakeRepo{}
			tc.setErr(repo, errors.New("db down"))
			svc := app.New(repo, zap.NewNop())
			_, err := tc.call(svc, repo)
			require.Error(t, err)
			require.NotErrorIs(t, err, domain.ErrNotificationNotFound)
		})
	}
}
```

- [ ] **Step 5: Correr los tests del paquete**

Run: `go test ./internal/app/notifications/... -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app/notifications/service_test.go
git commit -m "test(notifications): cubrir el service completo (paquete no tenía ningún test)"
```

---

## Task 8: `internal/app/permissions` — sin ningún test

**Files:**
- Create: `internal/app/permissions/service_test.go`

**Interfaces:**
- Consumes: `permissionsRepo.Repository` (`internal/repo/pg/permissions/repository.go`: `List`, `GetByID`, `Create`, `Update`, `Delete`).
- Produces: n/a.

**Importante:** `CreatePermission`/`UpdatePermission` hacen un **segundo roundtrip** con `GetByID` después de `Create`/`Update` para traer timestamps generados por la DB — el fake necesita poder devolver resultados/errores distintos para esa segunda llamada. Se resuelve con un contador de invocaciones.

- [ ] **Step 1: Crear el archivo con el fake**

```go
package permissions_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	app "github.com/tu-org/embolsadora-api/internal/app/permissions"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	listResult []*domain.Permission
	listErr    error

	// getByIDResults se consume en orden: la 1ra llamada devuelve
	// getByIDResults[0], la 2da (el roundtrip post-Create/Update) getByIDResults[1].
	// Si está vacío, devuelve getResult/getErr fijos para toda llamada.
	getByIDResults []*domain.Permission
	getByIDErrs    []error
	getByIDCalls   int

	getResult *domain.Permission
	getErr    error

	createErr error

	updateErr error

	deleteErr error
}

func (f *fakeRepo) List(context.Context, uuid.UUID) ([]*domain.Permission, error) {
	return f.listResult, f.listErr
}
func (f *fakeRepo) GetByID(context.Context, string, uuid.UUID) (*domain.Permission, error) {
	if len(f.getByIDResults) > 0 {
		i := f.getByIDCalls
		f.getByIDCalls++
		if i >= len(f.getByIDResults) {
			i = len(f.getByIDResults) - 1
		}
		return f.getByIDResults[i], f.getByIDErrs[i]
	}
	f.getByIDCalls++
	return f.getResult, f.getErr
}
func (f *fakeRepo) Create(context.Context, *domain.Permission) error { return f.createErr }
func (f *fakeRepo) Update(context.Context, *domain.Permission) error { return f.updateErr }
func (f *fakeRepo) Delete(context.Context, string, uuid.UUID) error  { return f.deleteErr }
```

- [ ] **Step 2: `ListPermissions` / `GetPermission`**

```go
func TestListPermissions(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := []*domain.Permission{{ID: "perm_x"}}
		svc := app.NewService(&fakeRepo{listResult: want}, zap.NewNop())
		got, err := svc.ListPermissions(context.Background(), uuid.New())
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("error de repo", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{listErr: errors.New("db down")}, zap.NewNop())
		_, err := svc.ListPermissions(context.Background(), uuid.New())
		require.Error(t, err)
	})
}

func TestGetPermission(t *testing.T) {
	t.Run("feliz", func(t *testing.T) {
		want := &domain.Permission{ID: "perm_x"}
		svc := app.NewService(&fakeRepo{getResult: want}, zap.NewNop())
		got, err := svc.GetPermission(context.Background(), "perm_x", uuid.New())
		require.NoError(t, err)
		require.Equal(t, want, got)
	})
	t.Run("no encontrado", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{getErr: domain.ErrPermissionNotFound}, zap.NewNop())
		_, err := svc.GetPermission(context.Background(), "perm_x", uuid.New())
		require.ErrorIs(t, err, domain.ErrPermissionNotFound)
	})
}
```

- [ ] **Step 3: `CreatePermission` — validación de campos (table-driven via `validatePermissionFields` indirectamente), error de `Create`, error del roundtrip de `GetByID`, feliz**

```go
func TestCreatePermission(t *testing.T) {
	tenantID := uuid.New()

	fieldTests := []struct {
		name                          string
		permName, section, desc string
	}{
		{"nombre muy corto", "ab", "sec", "desc"},
		{"section vacia", "nombre valido", "  ", "desc"},
		{"description vacia", "nombre valido", "sec", ""},
	}
	for _, tt := range fieldTests {
		t.Run(tt.name, func(t *testing.T) {
			svc := app.NewService(&fakeRepo{}, zap.NewNop())
			_, err := svc.CreatePermission(context.Background(), tenantID, tt.permName, tt.section, tt.desc)
			require.Error(t, err)
			var ve *app.ValidationError
			require.ErrorAs(t, err, &ve)
		})
	}

	t.Run("error de repo.Create", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{createErr: errors.New("db down")}, zap.NewNop())
		_, err := svc.CreatePermission(context.Background(), tenantID, "nombre valido", "sec", "desc")
		require.Error(t, err)
	})

	t.Run("error en el GetByID del roundtrip post-create", func(t *testing.T) {
		repo := &fakeRepo{getByIDResults: []*domain.Permission{nil}, getByIDErrs: []error{errors.New("read replica lag")}}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.CreatePermission(context.Background(), tenantID, "nombre valido", "sec", "desc")
		require.Error(t, err)
	})

	t.Run("feliz genera id, IsSystemPermission=false, y hace el roundtrip", func(t *testing.T) {
		created := &domain.Permission{ID: "generated", Name: "nombre valido"}
		repo := &fakeRepo{getByIDResults: []*domain.Permission{created}, getByIDErrs: []error{nil}}
		svc := app.NewService(repo, zap.NewNop())
		got, err := svc.CreatePermission(context.Background(), tenantID, "nombre valido", "sec", "desc")
		require.NoError(t, err)
		require.Equal(t, created, got)
		require.Equal(t, 1, repo.getByIDCalls)
	})
}
```

- [ ] **Step 4: `UpdatePermission` — not-found, `ErrPermissionIsSystem` ANTES de validar campos, validación, error de `Update`, feliz**

```go
func TestUpdatePermission(t *testing.T) {
	tenantID := uuid.New()

	t.Run("no encontrado", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{getErr: domain.ErrPermissionNotFound}, zap.NewNop())
		_, err := svc.UpdatePermission(context.Background(), "perm_x", tenantID, "nombre valido", "sec", "desc")
		require.ErrorIs(t, err, domain.ErrPermissionNotFound)
	})

	t.Run("permiso de sistema rechaza ANTES de validar campos (campos invalidos no importan)", func(t *testing.T) {
		repo := &fakeRepo{getResult: &domain.Permission{ID: "perm_x", IsSystemPermission: true}}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.UpdatePermission(context.Background(), "perm_x", tenantID, "", "", "")
		require.ErrorIs(t, err, domain.ErrPermissionIsSystem)
	})

	t.Run("campos invalidos en permiso no-sistema", func(t *testing.T) {
		repo := &fakeRepo{getResult: &domain.Permission{ID: "perm_x", IsSystemPermission: false}}
		svc := app.NewService(repo, zap.NewNop())
		_, err := svc.UpdatePermission(context.Background(), "perm_x", tenantID, "ab", "sec", "desc")
		var ve *app.ValidationError
		require.ErrorAs(t, err, &ve)
	})

	t.Run("feliz hace el roundtrip post-update", func(t *testing.T) {
		before := &domain.Permission{ID: "perm_x", IsSystemPermission: false}
		after := &domain.Permission{ID: "perm_x", Name: "nombre valido"}
		repo := &fakeRepo{getByIDResults: []*domain.Permission{before, after}, getByIDErrs: []error{nil, nil}}
		svc := app.NewService(repo, zap.NewNop())
		got, err := svc.UpdatePermission(context.Background(), "perm_x", tenantID, "nombre valido", "sec", "desc")
		require.NoError(t, err)
		require.Equal(t, after, got)
	})
}
```

- [ ] **Step 5: `DeletePermission` — not-found, `ErrPermissionIsSystem`, otro error, feliz**

```go
func TestDeletePermission(t *testing.T) {
	t.Run("no encontrado", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{deleteErr: domain.ErrPermissionNotFound}, zap.NewNop())
		err := svc.DeletePermission(context.Background(), "perm_x", uuid.New())
		require.ErrorIs(t, err, domain.ErrPermissionNotFound)
	})
	t.Run("es de sistema", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{deleteErr: domain.ErrPermissionIsSystem}, zap.NewNop())
		err := svc.DeletePermission(context.Background(), "perm_x", uuid.New())
		require.ErrorIs(t, err, domain.ErrPermissionIsSystem)
	})
	t.Run("feliz", func(t *testing.T) {
		svc := app.NewService(&fakeRepo{}, zap.NewNop())
		err := svc.DeletePermission(context.Background(), "perm_x", uuid.New())
		require.NoError(t, err)
	})
}
```

- [ ] **Step 6: Correr los tests del paquete**

Run: `go test ./internal/app/permissions/... -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/app/permissions/service_test.go
git commit -m "test(permissions): cubrir el service completo (paquete no tenía ningún test)"
```

---

## Task 9: `internal/api/usecases/tenants/create_tenant` — sin ningún test

**Files:**
- Create: `internal/api/usecases/tenants/create_tenant/usecase_test.go`

**Interfaces:**
- Consumes: `tenants.TenantRepository` (`internal/repo/pg/tenants/repository.go`: `Create`, `FindAll`, `FindByID`, `FindBySubdomain`, `Update`, `Delete`).

Seguir exactamente el patrón ya usado en el hermano testeado `get_all_tenants` (`fakeRepo` en el mismo paquete que el usecase, no `_test`; `testify/assert`).

- [ ] **Step 1: Crear el archivo completo**

```go
package create_tenant

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	createErr   error
	createCalls []*domain.Tenant
}

func (f *fakeRepo) Create(ctx context.Context, tenant *domain.Tenant) error {
	f.createCalls = append(f.createCalls, tenant)
	return f.createErr
}
func (f *fakeRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)             { return nil, nil }
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) { return nil, nil }
func (f *fakeRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return nil, nil
}
func (f *fakeRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error         { return nil }

func TestCreate_Feliz(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewUseCase(repo)

	tenant := &domain.Tenant{ID: uuid.New(), Name: "Acme"}
	err := uc.Create(context.Background(), tenant)

	assert.NoError(t, err)
	assert.Len(t, repo.createCalls, 1)
	assert.Same(t, tenant, repo.createCalls[0], "debe pasar el mismo puntero intacto al repo")
}

func TestCreate_ErrorDeRepo(t *testing.T) {
	repo := &fakeRepo{createErr: errors.New("subdomain duplicado")}
	uc := NewUseCase(repo)

	err := uc.Create(context.Background(), &domain.Tenant{})

	assert.Error(t, err)
}
```

- [ ] **Step 2: Correr los tests**

Run: `go test ./internal/api/usecases/tenants/create_tenant/... -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/api/usecases/tenants/create_tenant/usecase_test.go
git commit -m "test(create_tenant): cubrir el usecase (no tenía ningún test)"
```

---

## Task 10: `internal/api/usecases/tenants/delete_tenant` — sin ningún test

**Files:**
- Create: `internal/api/usecases/tenants/delete_tenant/usecase_test.go`

**Interfaces:**
- Consumes: `tenants.TenantRepository` (mismas 6 firmas que Task 9).

`Delete` mapea `domain.ErrNotFound` del repo a `delete_tenant.ErrTenantNotFound` (paquete-local, no el `domain.ErrNotFound`) — es el escenario más importante a cubrir.

- [ ] **Step 1: Crear el archivo completo**

```go
package delete_tenant

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	deleteErr   error
	deleteCalls []uuid.UUID
}

func (f *fakeRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)   { return nil, nil }
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return nil, nil
}
func (f *fakeRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return nil, nil
}
func (f *fakeRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error {
	f.deleteCalls = append(f.deleteCalls, id)
	return f.deleteErr
}

func TestDelete_Feliz(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewUseCase(repo)

	id := uuid.New()
	err := uc.Delete(context.Background(), id)

	assert.NoError(t, err)
	assert.Equal(t, []uuid.UUID{id}, repo.deleteCalls)
}

func TestDelete_NoEncontradoSeMapeaAErrTenantNotFoundLocal(t *testing.T) {
	repo := &fakeRepo{deleteErr: domain.ErrNotFound}
	uc := NewUseCase(repo)

	err := uc.Delete(context.Background(), uuid.New())

	assert.ErrorIs(t, err, ErrTenantNotFound)
	assert.NotErrorIs(t, err, domain.ErrNotFound, "el caller debe ver el error local del paquete, no domain.ErrNotFound directo")
}

func TestDelete_OtroErrorPasaTalCual(t *testing.T) {
	dbErr := errors.New("constraint violation")
	repo := &fakeRepo{deleteErr: dbErr}
	uc := NewUseCase(repo)

	err := uc.Delete(context.Background(), uuid.New())

	assert.ErrorIs(t, err, dbErr)
	assert.NotErrorIs(t, err, ErrTenantNotFound)
}
```

- [ ] **Step 2: Correr los tests**

Run: `go test ./internal/api/usecases/tenants/delete_tenant/... -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/api/usecases/tenants/delete_tenant/usecase_test.go
git commit -m "test(delete_tenant): cubrir el usecase, incluido el mapeo a ErrTenantNotFound local"
```

---

## Task 11: `internal/api/usecases/tenants/get_tenant` — sin ningún test

**Files:**
- Create: `internal/api/usecases/tenants/get_tenant/get_tenant_test.go`

**Interfaces:**
- Consumes: `tenants.TenantRepository` (mismas 6 firmas).

`Execute` trata `(nil, nil)` del repo (encontrado cero filas sin error SQL) como `ErrTenantNotFound` — ese es el branch distintivo a cubrir además del error de infra.

- [ ] **Step 1: Crear el archivo completo**

```go
package get_tenant

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	findByIDResult *domain.Tenant
	findByIDErr    error
}

func (f *fakeRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)   { return nil, nil }
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return f.findByIDResult, f.findByIDErr
}
func (f *fakeRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return nil, nil
}
func (f *fakeRepo) Update(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error         { return nil }

func TestExecute_Feliz(t *testing.T) {
	want := &domain.Tenant{ID: uuid.New(), Name: "Acme"}
	repo := &fakeRepo{findByIDResult: want}
	uc := NewUseCase(repo)

	got, err := uc.Execute(context.Background(), want.ID)

	assert.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestExecute_TenantNilSinErrorEsNotFound(t *testing.T) {
	repo := &fakeRepo{findByIDResult: nil, findByIDErr: nil}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New())

	assert.ErrorIs(t, err, ErrTenantNotFound)
}

func TestExecute_ErrorDeRepoPropaga(t *testing.T) {
	dbErr := errors.New("db down")
	repo := &fakeRepo{findByIDErr: dbErr}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New())

	assert.ErrorIs(t, err, dbErr)
}
```

- [ ] **Step 2: Correr los tests**

Run: `go test ./internal/api/usecases/tenants/get_tenant/... -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/api/usecases/tenants/get_tenant/get_tenant_test.go
git commit -m "test(get_tenant): cubrir el usecase, incluido el branch nil-sin-error como NotFound"
```

---

## Task 12: `internal/api/usecases/tenants/update_tenant` — sin ningún test (el más grande de los 4: 182 líneas de aplicación de campos opcionales)

**Files:**
- Create: `internal/api/usecases/tenants/update_tenant/usecase_test.go`

**Interfaces:**
- Consumes: `tenants.TenantRepository` (mismas 6 firmas).

- [ ] **Step 1: Crear el archivo con el fake y los casos base**

```go
package update_tenant

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	findByIDResult *domain.Tenant
	findByIDErr    error

	updateErr   error
	updateCalls []*domain.Tenant
}

func (f *fakeRepo) Create(ctx context.Context, tenant *domain.Tenant) error { return nil }
func (f *fakeRepo) FindAll(ctx context.Context) ([]domain.Tenant, error)   { return nil, nil }
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Tenant, error) {
	return f.findByIDResult, f.findByIDErr
}
func (f *fakeRepo) FindBySubdomain(ctx context.Context, subdomain string) (*domain.Tenant, error) {
	return nil, nil
}
func (f *fakeRepo) Update(_ context.Context, tenant *domain.Tenant) error {
	f.updateCalls = append(f.updateCalls, tenant)
	return f.updateErr
}
func (f *fakeRepo) Delete(ctx context.Context, id uuid.UUID) error { return nil }

func baseTenant() *domain.Tenant {
	return &domain.Tenant{
		ID: uuid.New(), Name: "Vieja", CompanyName: "Vieja SA", Subdomain: "vieja", Description: "d", IsActive: true,
		Theme:    domain.Theme{PrimaryColor: "#000000", LogoUrl: "http://old-logo"},
		Address:  domain.Address{Street: "Calle Vieja 123", City: "CABA"},
		Settings: domain.TenantSettings{ContactEmail: "old@x.com", Locale: "es-AR"},
	}
}

func strp(s string) *string { return &s }
func boolp(b bool) *bool    { return &b }

func TestUpdate_TenantNoEncontrado(t *testing.T) {
	repo := &fakeRepo{findByIDErr: nil, findByIDResult: nil}
	uc := NewUseCase(repo)

	_, err := uc.Update(context.Background(), uuid.New(), &UpdateTenantRequest{})

	assert.ErrorIs(t, err, ErrTenantNotFound)
	assert.Empty(t, repo.updateCalls)
}

func TestUpdate_ErrorDeFindByIDPropaga(t *testing.T) {
	dbErr := errors.New("db down")
	repo := &fakeRepo{findByIDErr: dbErr}
	uc := NewUseCase(repo)

	_, err := uc.Update(context.Background(), uuid.New(), &UpdateTenantRequest{})

	assert.ErrorIs(t, err, dbErr)
}

func TestUpdate_TodosLosCamposNilNoCambiaNadaExceptoUpdatedAt(t *testing.T) {
	original := baseTenant()
	before := *original // copia por valor para comparar después
	repo := &fakeRepo{findByIDResult: original}
	uc := NewUseCase(repo)

	got, err := uc.Update(context.Background(), original.ID, &UpdateTenantRequest{})

	require.NoError(t, err)
	assert.Equal(t, before.Name, got.Name)
	assert.Equal(t, before.CompanyName, got.CompanyName)
	assert.Equal(t, before.Theme, got.Theme)
	assert.Equal(t, before.Address, got.Address)
	assert.Equal(t, before.Settings, got.Settings)
	assert.True(t, got.UpdatedAt.After(before.UpdatedAt) || !got.UpdatedAt.IsZero(), "UpdatedAt siempre se refresca")
}

func TestUpdate_ErrorDeRepoUpdatePropaga(t *testing.T) {
	repo := &fakeRepo{findByIDResult: baseTenant(), updateErr: errors.New("constraint violation")}
	uc := NewUseCase(repo)

	_, err := uc.Update(context.Background(), uuid.New(), &UpdateTenantRequest{Name: strp("Nueva")})

	assert.Error(t, err)
}
```

- [ ] **Step 2: Subconjunto de campos top-level + `IsActive`**

```go
func TestUpdate_SubconjuntoDeCamposTopLevel(t *testing.T) {
	original := baseTenant()
	repo := &fakeRepo{findByIDResult: original}
	uc := NewUseCase(repo)

	got, err := uc.Update(context.Background(), original.ID, &UpdateTenantRequest{
		Name:     strp("Nueva"),
		IsActive: boolp(false),
	})

	require.NoError(t, err)
	assert.Equal(t, "Nueva", got.Name)
	assert.False(t, got.IsActive)
	assert.Equal(t, "Vieja SA", got.CompanyName, "campo no enviado no cambia")
	assert.Equal(t, "vieja", got.Subdomain, "campo no enviado no cambia")
}
```

- [ ] **Step 3: Campos anidados — `Theme`, `Address`, `Settings` (incluye `ContactEmail`/`CompanyWebsite` que escriben en `Settings`, no en un struct propio)**

```go
func TestUpdate_ThemeParcial(t *testing.T) {
	original := baseTenant()
	repo := &fakeRepo{findByIDResult: original}
	uc := NewUseCase(repo)

	got, err := uc.Update(context.Background(), original.ID, &UpdateTenantRequest{
		Theme: &ThemeUpdate{PrimaryColor: strp("#FFFFFF")},
	})

	require.NoError(t, err)
	assert.Equal(t, "#FFFFFF", got.Theme.PrimaryColor)
	assert.Equal(t, "http://old-logo", got.Theme.LogoUrl, "campo de Theme no enviado no cambia")
}

func TestUpdate_AddressParcial(t *testing.T) {
	original := baseTenant()
	repo := &fakeRepo{findByIDResult: original}
	uc := NewUseCase(repo)

	got, err := uc.Update(context.Background(), original.ID, &UpdateTenantRequest{
		Address: &AddressUpdate{City: strp("Rosario")},
	})

	require.NoError(t, err)
	assert.Equal(t, "Rosario", got.Address.City)
	assert.Equal(t, "Calle Vieja 123", got.Address.Street, "campo de Address no enviado no cambia")
}

func TestUpdate_ContactEmailYCompanyWebsiteEscribenEnSettings(t *testing.T) {
	original := baseTenant()
	repo := &fakeRepo{findByIDResult: original}
	uc := NewUseCase(repo)

	got, err := uc.Update(context.Background(), original.ID, &UpdateTenantRequest{
		ContactEmail:   strp("new@x.com"),
		CompanyWebsite: strp("https://new.example.com"),
	})

	require.NoError(t, err)
	assert.Equal(t, "new@x.com", got.Settings.ContactEmail)
	assert.Equal(t, "https://new.example.com", got.Settings.CompanyWebsite)
	assert.Equal(t, "es-AR", got.Settings.Locale, "el resto de Settings no cambia")
}

func TestUpdate_SettingsParcial(t *testing.T) {
	original := baseTenant()
	repo := &fakeRepo{findByIDResult: original}
	uc := NewUseCase(repo)

	got, err := uc.Update(context.Background(), original.ID, &UpdateTenantRequest{
		Settings: &SettingsUpdate{Locale: strp("en-US"), Currency: strp("USD")},
	})

	require.NoError(t, err)
	assert.Equal(t, "en-US", got.Settings.Locale)
	assert.Equal(t, "USD", got.Settings.Currency)
	assert.Equal(t, "old@x.com", got.Settings.ContactEmail, "campo de Settings no enviado no cambia")
}
```

- [ ] **Step 4: Correr los tests**

Run: `go test ./internal/api/usecases/tenants/update_tenant/... -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/usecases/tenants/update_tenant/usecase_test.go
git commit -m "test(update_tenant): cubrir el usecase completo, incluidos los campos anidados Theme/Address/Settings"
```

---

## Task 13: `internal/api/usecases/user_roles/bulk_assign_user_roles` — sin ningún test

**Files:**
- Create: `internal/api/usecases/user_roles/bulk_assign_user_roles/usecase_test.go`

**Interfaces:**
- Consumes: `userrolesrepo.UserRoleRepository` (7 métodos, solo se usa `BulkCreate`), `rolesRepo.Repository` (7 métodos, solo se usa `GetByIDForTenant` vía `appRoles.EnsureAssignable`).

- [ ] **Step 1: Crear el archivo con los 2 fakes**

```go
package bulk_assign_user_roles

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeUserRoleRepo struct {
	bulkCreateResult []domain.UserTenantRole
	bulkCreateErr    error
	bulkCreateCalls  [][]domain.UserTenantRole
}

func (f *fakeUserRoleRepo) FindByTenant(context.Context, uuid.UUID, *string, bool) ([]domain.UserTenantRoleDetail, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) FindByID(context.Context, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) Create(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) Update(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) Revoke(context.Context, uuid.UUID, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) BulkCreate(_ context.Context, utrs []domain.UserTenantRole, includeGlobal bool) ([]domain.UserTenantRole, error) {
	f.bulkCreateCalls = append(f.bulkCreateCalls, utrs)
	if f.bulkCreateErr != nil {
		return nil, f.bulkCreateErr
	}
	if f.bulkCreateResult != nil {
		return f.bulkCreateResult, nil
	}
	return utrs, nil
}
func (f *fakeUserRoleRepo) FindByUser(context.Context, uuid.UUID, uuid.UUID, bool, bool) ([]domain.UserRoleWithContext, error) {
	return nil, nil
}
func (f *fakeUserRoleRepo) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, domain.UserRoleStatus, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}

type fakeRolesRepo struct {
	getByIDForTenantErr error
}

func (f *fakeRolesRepo) List(context.Context, uuid.UUID, bool) ([]*domain.Role, error) { return nil, nil }
func (f *fakeRolesRepo) GetByIDForTenant(context.Context, string, uuid.UUID, bool) (*domain.Role, error) {
	if f.getByIDForTenantErr != nil {
		return nil, f.getByIDForTenantErr
	}
	return &domain.Role{ID: "operario"}, nil
}
func (f *fakeRolesRepo) CountCustomByTenant(context.Context, uuid.UUID) (int, error) { return 0, nil }
func (f *fakeRolesRepo) Create(context.Context, *domain.Role) error                  { return nil }
func (f *fakeRolesRepo) Update(context.Context, *domain.Role) error                  { return nil }
func (f *fakeRolesRepo) SoftDelete(context.Context, string) error                    { return nil }
func (f *fakeRolesRepo) CountActiveAssignments(context.Context, string) (int, error) { return 0, nil }
```

- [ ] **Step 2: Los 4 escenarios de `Execute`**

```go
func TestExecute_RolNoAsignableRechazaSinLlegarAlRepo(t *testing.T) {
	urRepo := &fakeUserRoleRepo{}
	roleRepo := &fakeRolesRepo{getByIDForTenantErr: domain.ErrRoleNotFound}
	uc := NewUseCase(urRepo, roleRepo)

	_, err := uc.Execute(context.Background(), BulkAssignRequest{
		UserIDs: []uuid.UUID{uuid.New()}, TenantID: uuid.New(), RoleID: "super_admin",
	})

	require.ErrorIs(t, err, domain.ErrInvalidRoleID)
	assert.Empty(t, urRepo.bulkCreateCalls, "una escalada bloqueada no debe llegar a BulkCreate")
}

func TestExecute_FelizArmaUnUTRPorUsuarioConElMismoRolYTenant(t *testing.T) {
	urRepo := &fakeUserRoleRepo{}
	roleRepo := &fakeRolesRepo{}
	uc := NewUseCase(urRepo, roleRepo)

	tenantID := uuid.New()
	userIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New()}

	got, err := uc.Execute(context.Background(), BulkAssignRequest{
		UserIDs: userIDs, TenantID: tenantID, RoleID: "operario", IncludeGlobal: false,
	})

	require.NoError(t, err)
	assert.Equal(t, 3, got.Assigned)
	require.Len(t, urRepo.bulkCreateCalls, 1)
	batch := urRepo.bulkCreateCalls[0]
	require.Len(t, batch, 3)
	for i, utr := range batch {
		assert.Equal(t, userIDs[i], utr.UserID)
		assert.Equal(t, tenantID, utr.TenantID)
		assert.Equal(t, "operario", *utr.RoleID)
		assert.Equal(t, domain.UserRoleStatusActive, utr.Status)
	}
	assert.Equal(t, batch[0].AssignedAt, batch[1].AssignedAt, "todo el batch comparte el mismo timestamp")
}

func TestExecute_ErrorDeBulkCreatePropaga(t *testing.T) {
	urRepo := &fakeUserRoleRepo{bulkCreateErr: errors.New("tx rollback")}
	roleRepo := &fakeRolesRepo{}
	uc := NewUseCase(urRepo, roleRepo)

	_, err := uc.Execute(context.Background(), BulkAssignRequest{
		UserIDs: []uuid.UUID{uuid.New()}, TenantID: uuid.New(), RoleID: "operario",
	})

	assert.Error(t, err)
}
```

- [ ] **Step 3: Correr los tests**

Run: `go test ./internal/api/usecases/user_roles/bulk_assign_user_roles/... -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/api/usecases/user_roles/bulk_assign_user_roles/usecase_test.go
git commit -m "test(bulk_assign_user_roles): cubrir el usecase (no tenía ningún test)"
```

---

## Task 14: `internal/api/usecases/user_roles/get_user_roles` — sin ningún test

**Files:**
- Create: `internal/api/usecases/user_roles/get_user_roles/usecase_test.go`

**Interfaces:**
- Consumes: `userrolesrepo.UserRoleRepository` (solo se usa `FindByUser`). Passthrough puro — 2 tests alcanzan.

- [ ] **Step 1: Crear el archivo completo**

```go
package get_user_roles

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	findByUserResult []domain.UserRoleWithContext
	findByUserErr    error
	lastCall         struct {
		userID, tenantID           uuid.UUID
		crossTenant, includeGlobal bool
	}
}

func (f *fakeRepo) FindByTenant(context.Context, uuid.UUID, *string, bool) ([]domain.UserTenantRoleDetail, error) {
	return nil, nil
}
func (f *fakeRepo) FindByID(context.Context, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) Create(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) Update(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) Revoke(context.Context, uuid.UUID, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) BulkCreate(context.Context, []domain.UserTenantRole, bool) ([]domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) FindByUser(_ context.Context, userID, tenantID uuid.UUID, crossTenant, includeGlobal bool) ([]domain.UserRoleWithContext, error) {
	f.lastCall.userID, f.lastCall.tenantID = userID, tenantID
	f.lastCall.crossTenant, f.lastCall.includeGlobal = crossTenant, includeGlobal
	return f.findByUserResult, f.findByUserErr
}
func (f *fakeRepo) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, domain.UserRoleStatus, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}

func TestExecute_FelizPasaLosCuatroParametros(t *testing.T) {
	want := []domain.UserRoleWithContext{{TenantName: "Acme"}}
	repo := &fakeRepo{findByUserResult: want}
	uc := NewUseCase(repo)

	userID, tenantID := uuid.New(), uuid.New()
	got, err := uc.Execute(context.Background(), Query{
		UserID: userID, TenantID: tenantID, CrossTenant: true, IncludeGlobal: true,
	})

	assert.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, userID, repo.lastCall.userID)
	assert.Equal(t, tenantID, repo.lastCall.tenantID)
	assert.True(t, repo.lastCall.crossTenant)
	assert.True(t, repo.lastCall.includeGlobal)
}

func TestExecute_ErrorDeRepoPropaga(t *testing.T) {
	repo := &fakeRepo{findByUserErr: errors.New("db down")}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), Query{UserID: uuid.New(), TenantID: uuid.New()})

	assert.Error(t, err)
}
```

- [ ] **Step 2: Correr los tests**

Run: `go test ./internal/api/usecases/user_roles/get_user_roles/... -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/api/usecases/user_roles/get_user_roles/usecase_test.go
git commit -m "test(get_user_roles): cubrir el usecase (no tenía ningún test)"
```

---

## Task 15: `internal/api/usecases/user_roles/list_user_roles` — sin ningún test

**Files:**
- Create: `internal/api/usecases/user_roles/list_user_roles/usecase_test.go`

**Interfaces:**
- Consumes: `userrolesrepo.UserRoleRepository` (solo se usa `FindByTenant`). Passthrough puro.

- [ ] **Step 1: Crear el archivo completo**

```go
package list_user_roles

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	findByTenantResult []domain.UserTenantRoleDetail
	findByTenantErr    error
	lastIncludeGlobal  bool
}

func (f *fakeRepo) FindByTenant(_ context.Context, _ uuid.UUID, _ *string, includeGlobal bool) ([]domain.UserTenantRoleDetail, error) {
	f.lastIncludeGlobal = includeGlobal
	return f.findByTenantResult, f.findByTenantErr
}
func (f *fakeRepo) FindByID(context.Context, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) Create(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) Update(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) Revoke(context.Context, uuid.UUID, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) BulkCreate(context.Context, []domain.UserTenantRole, bool) ([]domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) FindByUser(context.Context, uuid.UUID, uuid.UUID, bool, bool) ([]domain.UserRoleWithContext, error) {
	return nil, nil
}
func (f *fakeRepo) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, domain.UserRoleStatus, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}

func TestExecute_FelizPasaIncludeGlobal(t *testing.T) {
	want := []domain.UserTenantRoleDetail{{RoleName: "admin"}}
	repo := &fakeRepo{findByTenantResult: want}
	uc := NewUseCase(repo)

	got, err := uc.Execute(context.Background(), uuid.New(), nil, true)

	assert.NoError(t, err)
	assert.Equal(t, want, got)
	assert.True(t, repo.lastIncludeGlobal)
}

func TestExecute_ErrorDeRepoPropaga(t *testing.T) {
	repo := &fakeRepo{findByTenantErr: errors.New("db down")}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), nil, false)

	assert.Error(t, err)
}
```

- [ ] **Step 2: Correr los tests**

Run: `go test ./internal/api/usecases/user_roles/list_user_roles/... -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/api/usecases/user_roles/list_user_roles/usecase_test.go
git commit -m "test(list_user_roles): cubrir el usecase (no tenía ningún test)"
```

---

## Task 16: `internal/api/usecases/user_roles/revoke_user_role` — sin ningún test

**Files:**
- Create: `internal/api/usecases/user_roles/revoke_user_role/usecase_test.go`

**Interfaces:**
- Consumes: `userrolesrepo.UserRoleRepository` (`FindByID`, `Revoke`).

El cloaking es el escenario clave: "no existe" y "existe pero es de otro tenant" tienen que devolver el **mismo** `domain.ErrAssignmentNotFound`, y `Revoke` devolviendo `(nil, nil)` (el guard de identidad de plataforma en SQL) también se mapea al mismo error.

- [ ] **Step 1: Crear el archivo con el fake**

```go
package revoke_user_role

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	findByIDResult *domain.UserTenantRole
	findByIDErr    error

	revokeResult *domain.UserTenantRole
	revokeErr    error
}

func (f *fakeRepo) FindByTenant(context.Context, uuid.UUID, *string, bool) ([]domain.UserTenantRoleDetail, error) {
	return nil, nil
}
func (f *fakeRepo) FindByID(context.Context, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return f.findByIDResult, f.findByIDErr
}
func (f *fakeRepo) Create(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) Update(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) Revoke(context.Context, uuid.UUID, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return f.revokeResult, f.revokeErr
}
func (f *fakeRepo) BulkCreate(context.Context, []domain.UserTenantRole, bool) ([]domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) FindByUser(context.Context, uuid.UUID, uuid.UUID, bool, bool) ([]domain.UserRoleWithContext, error) {
	return nil, nil
}
func (f *fakeRepo) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, domain.UserRoleStatus, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
```

- [ ] **Step 2: Los 5 escenarios**

```go
func TestExecute_AsignacionInexistente(t *testing.T) {
	repo := &fakeRepo{findByIDResult: nil, findByIDErr: nil}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), false)

	assert.ErrorIs(t, err, domain.ErrAssignmentNotFound)
}

func TestExecute_AsignacionDeOtroTenantMismoErrorQueInexistente(t *testing.T) {
	tenantID, otroTenantID := uuid.New(), uuid.New()
	repo := &fakeRepo{findByIDResult: &domain.UserTenantRole{ID: uuid.New(), TenantID: otroTenantID}}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), tenantID, false)

	assert.ErrorIs(t, err, domain.ErrAssignmentNotFound, "misma respuesta que inexistente, para no revelar que la asignación existe en otro tenant")
}

func TestExecute_ErrorDeFindByIDPropaga(t *testing.T) {
	dbErr := errors.New("db down")
	repo := &fakeRepo{findByIDErr: dbErr}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), false)

	assert.ErrorIs(t, err, dbErr)
}

func TestExecute_ErrorDeRevokePropaga(t *testing.T) {
	tenantID := uuid.New()
	repo := &fakeRepo{
		findByIDResult: &domain.UserTenantRole{ID: uuid.New(), TenantID: tenantID},
		revokeErr:      errors.New("db down"),
	}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), tenantID, false)

	assert.Error(t, err)
}

func TestExecute_RevokeDevuelveNilSinErrorEsAssignmentNotFound(t *testing.T) {
	tenantID := uuid.New()
	repo := &fakeRepo{
		findByIDResult: &domain.UserTenantRole{ID: uuid.New(), TenantID: tenantID},
		revokeResult:   nil, revokeErr: nil,
	}
	uc := NewUseCase(repo)

	_, err := uc.Execute(context.Background(), uuid.New(), tenantID, false)

	assert.ErrorIs(t, err, domain.ErrAssignmentNotFound, "el guard de identidad de plataforma en SQL vuelve como (nil,nil), se mapea al mismo 404")
}

func TestExecute_Feliz(t *testing.T) {
	tenantID := uuid.New()
	want := &domain.UserTenantRole{ID: uuid.New(), TenantID: tenantID, Status: domain.UserRoleStatusRevoked}
	repo := &fakeRepo{
		findByIDResult: &domain.UserTenantRole{ID: want.ID, TenantID: tenantID},
		revokeResult:   want,
	}
	uc := NewUseCase(repo)

	got, err := uc.Execute(context.Background(), want.ID, tenantID, false)

	assert.NoError(t, err)
	assert.Equal(t, want, got)
}
```

- [ ] **Step 3: Correr los tests**

Run: `go test ./internal/api/usecases/user_roles/revoke_user_role/... -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/api/usecases/user_roles/revoke_user_role/usecase_test.go
git commit -m "test(revoke_user_role): cubrir el cloaking de assignment-not-found y el guard de identidad de plataforma"
```

---

## Task 17: `internal/api/usecases/user_roles/update_user_role` — sin ningún test

**Files:**
- Create: `internal/api/usecases/user_roles/update_user_role/usecase_test.go`

**Interfaces:**
- Consumes: `userrolesrepo.UserRoleRepository` (`FindByID`, `Update`), `rolesRepo.Repository` (vía `appRoles.EnsureAssignable`).

Mismo patrón de cloaking que Task 16, más el eje adicional de `EnsureAssignable` sobre el rol NUEVO.

- [ ] **Step 1: Crear el archivo con los 2 fakes**

```go
package update_user_role

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeRepo struct {
	findByIDResult *domain.UserTenantRole
	findByIDErr    error

	updateResult *domain.UserTenantRole
	updateErr    error
}

func (f *fakeRepo) FindByTenant(context.Context, uuid.UUID, *string, bool) ([]domain.UserTenantRoleDetail, error) {
	return nil, nil
}
func (f *fakeRepo) FindByID(context.Context, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return f.findByIDResult, f.findByIDErr
}
func (f *fakeRepo) Create(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) Update(context.Context, *domain.UserTenantRole, bool) (*domain.UserTenantRole, error) {
	return f.updateResult, f.updateErr
}
func (f *fakeRepo) Revoke(context.Context, uuid.UUID, uuid.UUID, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) BulkCreate(context.Context, []domain.UserTenantRole, bool) ([]domain.UserTenantRole, error) {
	return nil, nil
}
func (f *fakeRepo) FindByUser(context.Context, uuid.UUID, uuid.UUID, bool, bool) ([]domain.UserRoleWithContext, error) {
	return nil, nil
}
func (f *fakeRepo) UpdateStatus(context.Context, uuid.UUID, uuid.UUID, domain.UserRoleStatus, bool) (*domain.UserTenantRole, error) {
	return nil, nil
}

type fakeRolesRepo struct {
	getByIDForTenantErr error
}

func (f *fakeRolesRepo) List(context.Context, uuid.UUID, bool) ([]*domain.Role, error) { return nil, nil }
func (f *fakeRolesRepo) GetByIDForTenant(context.Context, string, uuid.UUID, bool) (*domain.Role, error) {
	if f.getByIDForTenantErr != nil {
		return nil, f.getByIDForTenantErr
	}
	return &domain.Role{ID: "operario"}, nil
}
func (f *fakeRolesRepo) CountCustomByTenant(context.Context, uuid.UUID) (int, error) { return 0, nil }
func (f *fakeRolesRepo) Create(context.Context, *domain.Role) error                  { return nil }
func (f *fakeRolesRepo) Update(context.Context, *domain.Role) error                  { return nil }
func (f *fakeRolesRepo) SoftDelete(context.Context, string) error                    { return nil }
func (f *fakeRolesRepo) CountActiveAssignments(context.Context, string) (int, error) { return 0, nil }
```

- [ ] **Step 2: Los 6 escenarios**

```go
func TestExecute_AsignacionInexistente(t *testing.T) {
	repo := &fakeRepo{}
	uc := NewUseCase(repo, &fakeRolesRepo{})

	_, err := uc.Execute(context.Background(), uuid.New(), uuid.New(), "operario", false)

	assert.ErrorIs(t, err, domain.ErrAssignmentNotFound)
}

func TestExecute_AsignacionDeOtroTenant(t *testing.T) {
	tenantID, otroTenantID := uuid.New(), uuid.New()
	repo := &fakeRepo{findByIDResult: &domain.UserTenantRole{ID: uuid.New(), TenantID: otroTenantID}}
	uc := NewUseCase(repo, &fakeRolesRepo{})

	_, err := uc.Execute(context.Background(), uuid.New(), tenantID, "operario", false)

	assert.ErrorIs(t, err, domain.ErrAssignmentNotFound)
}

func TestExecute_RolNuevoNoAsignableRechazaSinLlamarUpdate(t *testing.T) {
	tenantID := uuid.New()
	repo := &fakeRepo{findByIDResult: &domain.UserTenantRole{ID: uuid.New(), TenantID: tenantID}}
	roleRepo := &fakeRolesRepo{getByIDForTenantErr: domain.ErrRoleNotFound}
	uc := NewUseCase(repo, roleRepo)

	_, err := uc.Execute(context.Background(), uuid.New(), tenantID, "super_admin", false)

	assert.ErrorIs(t, err, domain.ErrInvalidRoleID)
}

func TestExecute_ErrorDeRepoUpdatePropaga(t *testing.T) {
	tenantID := uuid.New()
	repo := &fakeRepo{
		findByIDResult: &domain.UserTenantRole{ID: uuid.New(), TenantID: tenantID},
		updateErr:      errors.New("db down"),
	}
	uc := NewUseCase(repo, &fakeRolesRepo{})

	_, err := uc.Execute(context.Background(), uuid.New(), tenantID, "operario", false)

	assert.Error(t, err)
}

func TestExecute_UpdateDevuelveNilSinErrorEsAssignmentNotFound(t *testing.T) {
	tenantID := uuid.New()
	repo := &fakeRepo{
		findByIDResult: &domain.UserTenantRole{ID: uuid.New(), TenantID: tenantID},
		updateResult:   nil, updateErr: nil,
	}
	uc := NewUseCase(repo, &fakeRolesRepo{})

	_, err := uc.Execute(context.Background(), uuid.New(), tenantID, "operario", false)

	assert.ErrorIs(t, err, domain.ErrAssignmentNotFound, "el guard de identidad de plataforma en SQL vuelve como (nil,nil)")
}

func TestExecute_Feliz(t *testing.T) {
	tenantID := uuid.New()
	id := uuid.New()
	want := &domain.UserTenantRole{ID: id, TenantID: tenantID, RoleID: strPtr("operario")}
	repo := &fakeRepo{
		findByIDResult: &domain.UserTenantRole{ID: id, TenantID: tenantID},
		updateResult:   want,
	}
	uc := NewUseCase(repo, &fakeRolesRepo{})

	got, err := uc.Execute(context.Background(), id, tenantID, "operario", false)

	assert.NoError(t, err)
	assert.Equal(t, want, got)
}

func strPtr(s string) *string { return &s }
```

- [ ] **Step 3: Correr los tests**

Run: `go test ./internal/api/usecases/user_roles/update_user_role/... -v`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/api/usecases/user_roles/update_user_role/usecase_test.go
git commit -m "test(update_user_role): cubrir cloaking, EnsureAssignable sobre el rol nuevo, y el guard de identidad de plataforma"
```

---

## Task 18: `internal/api/usecases/auth_usecase.go` — `ProvisionUser` sin test

**Files:**
- Create: `internal/api/usecases/auth_usecase_test.go`

**Interfaces:**
- Consumes: `users.UserRepository` (`internal/repo/pg/users/users_repo.go`: `UpsertBySupabaseID`, `GetBySupabaseID`, `GetByID`, `SetStatus`, `SetPasswordChangeRequired`, `IsActiveMemberOfTenant`). `ProvisionUser` es un passthrough total a `UpsertBySupabaseID`.

Este archivo vive en el paquete `usecases` (mismo paquete que `create_invitation_test.go`, `activate_pending_invitations_test.go`, etc.) — **usar un nombre de fake propio** (`fakeUserRepoForAuth`) para no colisionar con `fakeUserRepoForActivation` ya definido en `activate_pending_invitations_test.go`.

- [ ] **Step 1: Crear el archivo completo**

```go
package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tu-org/embolsadora-api/internal/domain"
)

type fakeUserRepoForAuth struct {
	upsertResult *domain.User
	upsertErr    error
	upsertCalls  []struct{ supabaseUserID, email string }
}

func (f *fakeUserRepoForAuth) UpsertBySupabaseID(_ context.Context, supabaseUserID, email string) (*domain.User, error) {
	f.upsertCalls = append(f.upsertCalls, struct{ supabaseUserID, email string }{supabaseUserID, email})
	return f.upsertResult, f.upsertErr
}
func (f *fakeUserRepoForAuth) GetBySupabaseID(context.Context, string) (*domain.User, error) { return nil, nil }
func (f *fakeUserRepoForAuth) GetByID(context.Context, string) (*domain.User, error)         { return nil, nil }
func (f *fakeUserRepoForAuth) SetStatus(context.Context, string, domain.UserStatus) error    { return nil }
func (f *fakeUserRepoForAuth) SetPasswordChangeRequired(context.Context, string, bool) error { return nil }
func (f *fakeUserRepoForAuth) IsActiveMemberOfTenant(context.Context, string, string) (bool, error) {
	return false, nil
}

func TestProvisionUser_Feliz(t *testing.T) {
	want := &domain.User{ID: "u1", Email: "x@example.com"}
	repo := &fakeUserRepoForAuth{upsertResult: want}
	uc := NewAuthUsecase(repo)

	got, err := uc.ProvisionUser(context.Background(), "supa-123", "x@example.com")

	assert.NoError(t, err)
	assert.Equal(t, want, got)
	assert.Equal(t, []struct{ supabaseUserID, email string }{{"supa-123", "x@example.com"}}, repo.upsertCalls)
}

func TestProvisionUser_ErrorDeRepoPropaga(t *testing.T) {
	repo := &fakeUserRepoForAuth{upsertErr: errors.New("db down")}
	uc := NewAuthUsecase(repo)

	_, err := uc.ProvisionUser(context.Background(), "supa-123", "x@example.com")

	assert.Error(t, err)
}
```

- [ ] **Step 2: Correr los tests**

Run: `go test ./internal/api/usecases/... -run TestProvisionUser -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/api/usecases/auth_usecase_test.go
git commit -m "test(auth_usecase): cubrir ProvisionUser (no tenía ningún test)"
```

---

## Task 19: `internal/api/usecases/invitation_usecase.go` — `ResendInvitation`, `RevokeInvitation`, `ListInvitations`, `checkRateLimit` sin test

**Files:**
- Create: `internal/api/usecases/invitation_resend_revoke_list_test.go` (fakes propios, 100% unitario)
- Create: `internal/api/usecases/invitation_ratelimit_integration_test.go` (gateado por `REDIS_URL`, sigue el patrón de `internal/api/middleware/dashboard_ratelimit_test.go`)

**Interfaces:**
- Consumes: `invitations.InvitationRepository` (`GetByID`, `UpdateStatus`, `ListByTenant` — más los 3 métodos que no se usan acá), `supabase.AdminClient` (`InviteUserByEmail`).
- Reutiliza (sin modificarlos): `testTenantID` y `ctxForCreateInvitation` de `invite_metadata_test.go` / `create_invitation_test.go` (mismo paquete `usecases`) — **no** redefinir esos símbolos.

- [ ] **Step 1: Crear `invitation_resend_revoke_list_test.go` con los 2 fakes propios**

```go
package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform/supabase"
)

// fakeInvRepoForResendRevoke implementa invitations.InvitationRepository al
// completo (los métodos que Resend/Revoke/List no usan devuelven cero).
type fakeInvRepoForResendRevoke struct {
	getByIDResult *domain.UserInvitation
	getByIDErr    error

	updateStatusErr   error
	updateStatusCalls []struct {
		id     string
		status domain.InvitationStatus
	}

	listByTenantResult []domain.UserInvitation
	listByTenantErr    error
}

func (f *fakeInvRepoForResendRevoke) Create(context.Context, *domain.UserInvitation) (*domain.UserInvitation, error) {
	return nil, nil
}
func (f *fakeInvRepoForResendRevoke) GetPendingByEmailAndTenant(context.Context, string, string, bool) (*domain.UserInvitation, error) {
	return nil, nil
}
func (f *fakeInvRepoForResendRevoke) ListPendingByEmail(context.Context, string) ([]domain.UserInvitation, error) {
	return nil, nil
}
func (f *fakeInvRepoForResendRevoke) GetByID(context.Context, string, string, bool) (*domain.UserInvitation, error) {
	return f.getByIDResult, f.getByIDErr
}
func (f *fakeInvRepoForResendRevoke) ListByTenant(context.Context, string, *string, bool) ([]domain.UserInvitation, error) {
	return f.listByTenantResult, f.listByTenantErr
}
func (f *fakeInvRepoForResendRevoke) UpdateStatus(_ context.Context, id string, status domain.InvitationStatus) error {
	f.updateStatusCalls = append(f.updateStatusCalls, struct {
		id     string
		status domain.InvitationStatus
	}{id, status})
	return f.updateStatusErr
}

type fakeAdminClientForResendRevoke struct {
	inviteCalls []supabase.InviteParams
	inviteErr   error
}

func (f *fakeAdminClientForResendRevoke) InviteUserByEmail(_ context.Context, p supabase.InviteParams) error {
	f.inviteCalls = append(f.inviteCalls, p)
	return f.inviteErr
}
func (f *fakeAdminClientForResendRevoke) SendPasswordResetEmail(context.Context, string, string) error {
	return nil
}
```

- [ ] **Step 2: `ResendInvitation` — cloaking/error de `GetByID`, no-pending, `InviterName` del que reenvía (no del invitador original), error de `InviteUserByEmail`**

```go
func TestResendInvitation_ErrorDeGetByIDPropaga(t *testing.T) {
	invRepo := &fakeInvRepoForResendRevoke{getByIDErr: domain.ErrNotFound}
	uc := NewInvitationUsecase(invRepo, nil, nil, fakeTenantLookup{}, fakeRoleLookup{}, &fakeAdminClientForResendRevoke{}, nil, "https://app.example.com", 100)

	err := uc.ResendInvitation(ctxForCreateInvitation("admin"), "inv1", false)

	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestResendInvitation_NoPending(t *testing.T) {
	invRepo := &fakeInvRepoForResendRevoke{getByIDResult: &domain.UserInvitation{ID: "inv1", Status: domain.InvitationStatusAccepted}}
	uc := NewInvitationUsecase(invRepo, nil, nil, fakeTenantLookup{}, fakeRoleLookup{}, &fakeAdminClientForResendRevoke{}, nil, "https://app.example.com", 100)

	err := uc.ResendInvitation(ctxForCreateInvitation("admin"), "inv1", false)

	require.ErrorIs(t, err, domain.ErrInvitationNotPending)
}

func TestResendInvitation_InviterNameEsQuienReenviaNoElOriginal(t *testing.T) {
	invRepo := &fakeInvRepoForResendRevoke{getByIDResult: &domain.UserInvitation{
		ID: "inv1", Email: "target@example.com", Status: domain.InvitationStatusPending, RoleID: "operario", InvitedBy: "otro-user-id",
	}}
	admin := &fakeAdminClientForResendRevoke{}
	uc := NewInvitationUsecase(invRepo, nil, nil, fakeTenantLookup{}, fakeRoleLookup{role: &domain.Role{ID: "operario", Name: "Operario"}}, admin, nil, "https://app.example.com", 100)

	err := uc.ResendInvitation(ctxForCreateInvitation("admin"), "inv1", false)

	require.NoError(t, err)
	require.Len(t, admin.inviteCalls, 1)
	assert.Equal(t, "Caller", admin.inviteCalls[0].InviterName, "InviterName debe ser quien reenvía (el user del contexto), no InvitedBy")
	assert.Equal(t, "target@example.com", admin.inviteCalls[0].Email)
}

func TestResendInvitation_ErrorDeInviteUserByEmailPropaga(t *testing.T) {
	invRepo := &fakeInvRepoForResendRevoke{getByIDResult: &domain.UserInvitation{ID: "inv1", Status: domain.InvitationStatusPending}}
	admin := &fakeAdminClientForResendRevoke{inviteErr: errors.New("supabase down")}
	uc := NewInvitationUsecase(invRepo, nil, nil, fakeTenantLookup{}, fakeRoleLookup{}, admin, nil, "https://app.example.com", 100)

	err := uc.ResendInvitation(ctxForCreateInvitation("admin"), "inv1", false)

	require.Error(t, err)
}
```

- [ ] **Step 3: `RevokeInvitation` — cloaking/error de `GetByID`, error de `UpdateStatus`, feliz muta localmente el status devuelto**

```go
func TestRevokeInvitation_ErrorDeGetByIDPropaga(t *testing.T) {
	invRepo := &fakeInvRepoForResendRevoke{getByIDErr: domain.ErrNotFound}
	uc := NewInvitationUsecase(invRepo, nil, nil, nil, nil, nil, nil, "", 100)

	_, err := uc.RevokeInvitation(ctxForCreateInvitation("admin"), "inv1", false)

	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestRevokeInvitation_ErrorDeUpdateStatusPropaga(t *testing.T) {
	invRepo := &fakeInvRepoForResendRevoke{
		getByIDResult: &domain.UserInvitation{ID: "inv1", Status: domain.InvitationStatusPending},
		updateStatusErr: errors.New("db down"),
	}
	uc := NewInvitationUsecase(invRepo, nil, nil, nil, nil, nil, nil, "", 100)

	_, err := uc.RevokeInvitation(ctxForCreateInvitation("admin"), "inv1", false)

	require.Error(t, err)
}

func TestRevokeInvitation_FelizDevuelveElObjetoConStatusRevocadoAunqueElFakeDevuelvaOtro(t *testing.T) {
	invRepo := &fakeInvRepoForResendRevoke{
		getByIDResult: &domain.UserInvitation{ID: "inv1", Status: domain.InvitationStatusPending, Email: "x@example.com"},
	}
	uc := NewInvitationUsecase(invRepo, nil, nil, nil, nil, nil, nil, "", 100)

	got, err := uc.RevokeInvitation(ctxForCreateInvitation("admin"), "inv1", false)

	require.NoError(t, err)
	assert.Equal(t, domain.InvitationStatusRevoked, got.Status, "el status se muta localmente, no se vuelve a leer de la DB")
	require.Len(t, invRepo.updateStatusCalls, 1)
	assert.Equal(t, domain.InvitationStatusRevoked, invRepo.updateStatusCalls[0].status)
}
```

- [ ] **Step 4: `ListInvitations` — passthrough puro**

```go
func TestListInvitations_Feliz(t *testing.T) {
	want := []domain.UserInvitation{{ID: "inv1"}}
	invRepo := &fakeInvRepoForResendRevoke{listByTenantResult: want}
	uc := NewInvitationUsecase(invRepo, nil, nil, nil, nil, nil, nil, "", 100)

	got, err := uc.ListInvitations(ctxForCreateInvitation("admin"), nil, true)

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestListInvitations_ErrorDeRepoPropaga(t *testing.T) {
	invRepo := &fakeInvRepoForResendRevoke{listByTenantErr: errors.New("db down")}
	uc := NewInvitationUsecase(invRepo, nil, nil, nil, nil, nil, nil, "", 100)

	_, err := uc.ListInvitations(ctxForCreateInvitation("admin"), nil, false)

	require.Error(t, err)
}
```

- [ ] **Step 5: `checkRateLimit` — fail-open cuando `redis == nil` (100% unitario, sin infraestructura)**

```go
func TestCheckRateLimit_RedisNilFailaAbierto(t *testing.T) {
	uc := NewInvitationUsecase(nil, nil, nil, nil, nil, nil, nil, "", 100)

	err := uc.checkRateLimit(context.Background(), testTenantID)

	require.NoError(t, err, "sin Redis, el rate limit debe fallar abierto (deshabilitado), nunca bloquear")
}
```

- [ ] **Step 6: Correr los tests unitarios del paquete**

Run: `go test ./internal/api/usecases/... -run 'TestResendInvitation|TestRevokeInvitation|TestListInvitations|TestCheckRateLimit_RedisNil' -v`
Expected: PASS.

- [ ] **Step 7: Crear `invitation_ratelimit_integration_test.go`, gateado por `REDIS_URL`, siguiendo el patrón de `internal/api/middleware/dashboard_ratelimit_test.go`**

```go
package usecases

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"

	"github.com/tu-org/embolsadora-api/internal/domain"
)

func TestCheckRateLimit_ContraRedisReal(t *testing.T) {
	url := os.Getenv("REDIS_URL")
	if url == "" {
		t.Skip("REDIS_URL no seteada; se omite el test de rate limit de invitaciones contra Redis real")
	}
	opt, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	t.Cleanup(func() { _ = rdb.Close() })

	tenantID := fmt.Sprintf("rl-test-%d", time.Now().UnixNano())
	key := fmt.Sprintf("invitations:ratelimit:%s:%s", tenantID, time.Now().UTC().Format("2006-01-02-15"))
	t.Cleanup(func() { rdb.Del(context.Background(), key) })

	// limite bajo (2) para no tener que iterar cientos de veces
	uc := NewInvitationUsecase(nil, nil, nil, nil, nil, nil, rdb, "", 2)

	require.NoError(t, uc.checkRateLimit(context.Background(), tenantID), "1ra llamada, dentro del limite")
	require.NoError(t, uc.checkRateLimit(context.Background(), tenantID), "2da llamada, en el limite")
	err = uc.checkRateLimit(context.Background(), tenantID)
	require.ErrorIs(t, err, domain.ErrInvitationRateLimitExceeded, "3ra llamada, excede el limite de 2")
}
```

- [ ] **Step 8: Correr el test de integración (requiere Redis local: `docker compose up -d redis`)**

Run: `REDIS_URL=redis://:embolsadora_redis_pass@localhost:6379/0 go test ./internal/api/usecases/... -run TestCheckRateLimit_ContraRedisReal -v`
Expected: PASS con Redis levantado; `SKIP` sin `REDIS_URL` seteada (no debe fallar el resto de la suite).

- [ ] **Step 9: Commit**

```bash
git add internal/api/usecases/invitation_resend_revoke_list_test.go internal/api/usecases/invitation_ratelimit_integration_test.go
git commit -m "test(invitation_usecase): cubrir ResendInvitation, RevokeInvitation, ListInvitations y checkRateLimit"
```

---

## Task 20: `internal/api/usecases/password_usecase.go` — `ClearPasswordChangeRequired` sin test

**Files:**
- Create: `internal/api/usecases/password_usecase_clear_test.go`

**Interfaces:**
- Consumes: `users.UserRepository` (solo se usa `SetPasswordChangeRequired`), `platform.DomainUser(ctx)`.

Fake propio (`fakeUserRepoForClearPwd`) para no colisionar con `fakeUserRepoForActivation` ni con el `fakeUserRepoForAuth` de la Task 18 — las tres tareas tocan archivos distintos dentro del mismo paquete `usecases` y no deben compartir símbolos.

- [ ] **Step 1: Crear el archivo completo**

```go
package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tu-org/embolsadora-api/internal/domain"
	"github.com/tu-org/embolsadora-api/internal/platform"
)

type fakeUserRepoForClearPwd struct {
	setPasswordChangeRequiredErr   error
	setPasswordChangeRequiredCalls []struct {
		userID string
		value  bool
	}
}

func (f *fakeUserRepoForClearPwd) UpsertBySupabaseID(context.Context, string, string) (*domain.User, error) {
	return nil, nil
}
func (f *fakeUserRepoForClearPwd) GetBySupabaseID(context.Context, string) (*domain.User, error) {
	return nil, nil
}
func (f *fakeUserRepoForClearPwd) GetByID(context.Context, string) (*domain.User, error) { return nil, nil }
func (f *fakeUserRepoForClearPwd) SetStatus(context.Context, string, domain.UserStatus) error {
	return nil
}
func (f *fakeUserRepoForClearPwd) SetPasswordChangeRequired(_ context.Context, userID string, value bool) error {
	f.setPasswordChangeRequiredCalls = append(f.setPasswordChangeRequiredCalls, struct {
		userID string
		value  bool
	}{userID, value})
	return f.setPasswordChangeRequiredErr
}
func (f *fakeUserRepoForClearPwd) IsActiveMemberOfTenant(context.Context, string, string) (bool, error) {
	return false, nil
}

func TestClearPasswordChangeRequired_SinDomainUserEnContextoEsForbidden(t *testing.T) {
	uc := NewPasswordUsecase(&fakeUserRepoForClearPwd{}, nil, nil, "", nil)

	err := uc.ClearPasswordChangeRequired(context.Background())

	assert.ErrorIs(t, err, domain.ErrForbidden)
}

func TestClearPasswordChangeRequired_Feliz(t *testing.T) {
	repo := &fakeUserRepoForClearPwd{}
	uc := NewPasswordUsecase(repo, nil, nil, "", nil)
	ctx := platform.WithDomainUser(context.Background(), &domain.User{ID: "u1"})

	err := uc.ClearPasswordChangeRequired(ctx)

	assert.NoError(t, err)
	assert.Equal(t, []struct {
		userID string
		value  bool
	}{{"u1", false}}, repo.setPasswordChangeRequiredCalls)
}

func TestClearPasswordChangeRequired_ErrorDeRepoPropaga(t *testing.T) {
	repo := &fakeUserRepoForClearPwd{setPasswordChangeRequiredErr: errors.New("db down")}
	uc := NewPasswordUsecase(repo, nil, nil, "", nil)
	ctx := platform.WithDomainUser(context.Background(), &domain.User{ID: "u1"})

	err := uc.ClearPasswordChangeRequired(ctx)

	assert.Error(t, err)
}
```

- [ ] **Step 2: Correr los tests**

Run: `go test ./internal/api/usecases/... -run TestClearPasswordChangeRequired -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/api/usecases/password_usecase_clear_test.go
git commit -m "test(password_usecase): cubrir ClearPasswordChangeRequired (no tenía ningún test)"
```

---

## Final verification (después de que las 20 tareas estén mergeadas)

- [ ] **Build completo**

Run: `go build ./...`
Expected: sin errores.

- [ ] **Suite completa (unitarios + los gateados por infraestructura si está disponible)**

Run: `go test ./... -v`
Expected: PASS (o SKIP para los tests gateados por `DATABASE_URL`/`REDIS_URL`/`MONGO_URI` si no están seteadas — nunca FAIL).

- [ ] **Con infraestructura levantada (opcional, para no dejar ciego el `checkRateLimit` real ni los tests de integración de `users`/`assign_user_role`/`password_usecase`)**

```bash
docker compose up -d db redis mongo
export DATABASE_URL=postgres://embolsadora_user:embolsadora_password@localhost:5432/embolsadora_dev?sslmode=disable
export REDIS_URL=redis://:embolsadora_redis_pass@localhost:6379/0
export MONGO_URI=mongodb://localhost:27017
go test ./... -v
```
Expected: PASS en todos los tests, incluidos los de integración.

