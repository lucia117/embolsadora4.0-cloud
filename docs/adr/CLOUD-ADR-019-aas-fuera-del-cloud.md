---
id: CLOUD-ADR-019
title: "El Asset Administration Shell no se implementa en el cloud"
status: propuesta
date: 2026-09-29
owner: Lucia Scharff
last_reviewed: 2026-09-29
supersedes: []
superseded_by: []
---

# CLOUD-ADR-019 — El Asset Administration Shell no se implementa en el cloud

> ADR retrospectivo. Registra un cambio de arquitectura que ya ocurrió y no quedó
> escrito. **La fecha exacta y la razón explícita del cambio no están documentadas.**
> La decisión se reconstruye con la evidencia de abajo.

## Contexto

La spec `004-aas-server` (2026-03-24, commit `2eef903`, rama
`origin/docs/nosql-aas-plc-specs`) especificaba un **AAS Server en el cloud**, conforme a
IDTA-01002-3-1 e IEC 63278-1:2023, con este flujo:

```text
PLC → InfluxDB → Processing API (nueva) → AAS Server en el cloud (MongoDB) → frontend
```

El PR #30, "feat(006): MongoDB infrastructure layer with AAS Shell CRUD" (abierto el
2026-04-11), implementaba parte de ese camino.

Evidencia de lo que pasó después:

| Hecho | Fuente |
|---|---|
| El historian (`proyecto-embolsadora`) implementa el AAS con **FA³ST AAS Server**, junto a InfluxDB, del lado de la planta | `proyecto-embolsadora/README.md` (2026-04-19) |
| El PR #30 se **cerró sin mergear** el 2026-09-01 | GitHub, `lucia117/embolsadora4.0-cloud#30` |
| El cloud no tiene código de AAS. Solo transporta el string `aasPath` dentro de `payload`, sin validarlo | `internal/app/ingest/validate.go`; D-7 y D-8 del diseño de la ingesta |
| Las specs `004-aas-server` y `005-plc-events` quedaron fuera del árbol versionado (en `.gitignore`) | `.gitignore` de este repo |

## Decisión

El cloud **no** aloja un AAS Server ni modela shells ni submodelos. El gemelo digital AAS
vive en el historian (FA³ST, en la planta). El cloud:

- recibe mediciones ya mapeadas contra el catálogo AAS, identificadas por `aasPath`
  dentro de `payload` (D-7 del diseño de la ingesta);
- las guarda sin validar el contenido
  ([CLOUD-ADR-017](CLOUD-ADR-017-mediciones-en-mongodb.md)), para que un cambio del
  catálogo AAS no mande datos reales a DEAD (D-8);
- expone esas mediciones al frontend por la API de métricas de dashboards.

Razón no documentada. **[I]** La evidencia sugiere que, una vez que el historian resolvió
el AAS con FA³ST cerca de la fuente de datos, un segundo AAS en el cloud duplicaba el
modelo semántico sin agregar capacidad.

## Alternativas consideradas

1. **AAS Server en el cloud** (spec `004-aas-server`, PR #30). Abandonada; ver el
   contexto.

## Consecuencias

- La spec `004-aas-server` pasa a `status: abandoned` (veredicto C): su cuerpo se
  conserva como evidencia de la decisión descartada y queda registrada en la bitácora de
  alcance.
- El catálogo canónico de `aasPath` vive fuera de este repo (historian). El cloud no
  puede detectar un `aasPath` inexistente o renombrado. Esa validación, si se hace,
  corresponde al frontend o a un control de CI entre repos.
- La interoperabilidad AAS con sistemas externos (el objetivo de la spec 004) queda a
  cargo del historian, no de esta API.
