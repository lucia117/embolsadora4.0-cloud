# Quickstart: MongoDB Infrastructure Layer

**Feature**: 006-mongo-infra  
**Date**: 2026-04-02

---

## Prerequisitos

- Docker y Docker Compose instalados
- Go **no** instalado en el host — todos los comandos Go corren vía Docker (ver CLAUDE.md)
- Acceso al repositorio en la rama `006-mongo-infra`

---

## 1. Levantar MongoDB local

```bash
# Agregar el servicio mongo al docker-compose.yml si no está aún
docker compose up -d mongo

# Verificar que el servicio está activo
docker compose ps mongo
# Expected: mongo  running  0.0.0.0:27017->27017/tcp
```

---

## 2. Configurar variables de entorno

Agregar al archivo `.env` (o exportar en la terminal):

```env
MONGO_URI=mongodb://localhost:27017
MONGO_DB=embolsadora_dev
```

> Si `MONGO_URI` no está definido, el servidor arranca normalmente con un WARN en el log y MongoDB deshabilitado.

---

## 3. Agregar la dependencia del driver

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go get go.mongodb.org/mongo-driver/v2/mongo && go mod tidy"
```

---

## 4. Compilar con la nueva dependencia

```bash
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  golang:1.24-alpine \
  sh -c "go build ./..."
```

---

## 5. Levantar el servidor completo

```bash
docker compose up -d        # levanta postgres + redis + mongo
go run ./cmd/api/main.go    # (vía Docker si se prefiere)
```

Logs esperados al arrancar con MongoDB configurado:
```
MongoDB connection established
Database connection established
Starting server on :8080
```

Logs esperados **sin** MongoDB configurado:
```
WARN mongo disabled — MONGO_URI not set
Database connection established
Starting server on :8080
```

---

## 6. Correr tests de integración de repositorios MongoDB

```bash
# Con MongoDB corriendo en localhost:27017
docker run --rm \
  -v /tmp/go-mod-cache:/go/pkg/mod \
  -v $(pwd):/app -w /app \
  -e MONGO_TEST_URI=mongodb://host.docker.internal:27017 \
  golang:1.24-alpine \
  sh -c "go test ./internal/repo/mongo/... -v -count=1"
```

> Cada test crea y destruye su propia base de datos (`test_<uuid>`). Si `MONGO_TEST_URI` no está definido, los tests se saltan automáticamente con `t.Skip()`.

---

## 7. Verificar métricas Prometheus de MongoDB

```bash
curl http://localhost:8080/metrics | grep mongo_
```

Métricas esperadas:
```
# HELP mongo_operation_duration_seconds Latency of MongoDB repository operations
# TYPE mongo_operation_duration_seconds histogram
mongo_operation_duration_seconds_bucket{collection="shells",operation="create",le="0.005"} 0
...

# HELP mongo_operation_errors_total Total number of MongoDB operation errors
# TYPE mongo_operation_errors_total counter
mongo_operation_errors_total{collection="shells",operation="create"} 0
```

---

## 8. Verificar el healthcheck

```bash
curl http://localhost:8080/ping
# Con MongoDB: devuelve información de estado incluyendo sección "mongo"
```

---

## Estructura de archivos creados por esta feature

```
internal/
├── config/config.go              — MongoConfig struct agregada (MONGO_URI, MONGO_DB)
├── platform/
│   └── mongo/
│       └── client.go             — Connect(), Disconnect() lifecycle
├── domain/
│   └── aas/
│       ├── shell.go              — AssetAdministrationShell, ShellRepository interface
│       └── submodel.go           — Submodel, SubmodelElement, SubmodelRepository interface
├── repo/
│   └── mongo/
│       ├── aas/
│       │   └── repository.go     — MongoShellRepository (implementación)
│       └── submodel/
│           └── repository.go     — MongoSubmodelRepository (implementación)
└── telemetry/
    └── mongo_metrics.go          — MongoOperationDuration, MongoOperationErrors

cmd/api/main.go                   — Wiring de MongoDB (opcional, como Redis)
docker-compose.yml                — Servicio mongo:7 agregado
```

---

## Troubleshooting

| Síntoma | Causa probable | Solución |
|---------|----------------|----------|
| `WARN mongo disabled` en startup | `MONGO_URI` no definido | Agregar `MONGO_URI` al `.env` |
| Tests de repo salteados | `MONGO_TEST_URI` no definido | Exportar `MONGO_TEST_URI=mongodb://localhost:27017` |
| `connection refused` al conectar | Servicio mongo no está corriendo | `docker compose up -d mongo` |
| Duplicate key error en Create | Shell/submodelo ya existe con ese globalAssetId/idShort | Usar Update en vez de Create |
| `mongo_operation_duration_seconds` no aparece en /metrics | Ninguna operación de repo ejecutada aún | Ejecutar al menos una operación; las métricas son lazy-initialized por `promauto` |
