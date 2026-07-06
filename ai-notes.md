# ai-notes.md — Trazabilidad de integración Front ↔ Back

Documento de trazabilidad del procedimiento por etapas para integrar:

- **Backend**: `rs-lambda-go` (este repo) — AWS Lambda (Go) + API Gateway HTTP API (v2) + Neon Postgres + Cloudflare R2.
- **Frontend**: `stuff/front-real-state/real-state-website` (repo independiente, ignorado por git aquí) — Vite + React, desplegado en Vercel: `https://aura-urrea.vercel.app/`.

Caso de uso: CRUD de los datos del agente inmobiliario (`/users`) y de los inmuebles (`/listings`, `/uploads`), consumido por el panel del front. En la Etapa 2 esas mutaciones quedarán protegidas por AWS Cognito (JWT authorizer del API Gateway).

---

## Etapa 0 — Reconocimiento (completada)

Hallazgos relevantes:

- El backend expone: CRUD `/users`, CRUD `/listings`, `POST /uploads` (multipart), `GET /uploads/{id}/url`, `DELETE /uploads/{id}`, `GET /listings/{id}/media`. Respuestas de error normalizadas `{code, message, status}`.
- CORS se maneja en la configuración del API Gateway (no en código). El `.env` local ya contiene `ACCESS_CONTROL_ALLOW_*` como referencia de esa configuración.
- Los endpoints de uploads **ya exigen** un `sub` (JWT authorizer) → sin auth devuelven 401. Se necesita un fallback temporal para la Etapa 1.
- El front era 100% mock: sin cliente HTTP, sin variables de entorno, sin tests.
- El modelo `Listing` del backend no tenía flag `featured` (el front lo usa en tabla y sitio público).

## Etapa 1 — Integración funcional sin autenticación (✅ completada)

### 1.1 Backend

| Cambio | Motivo | Estado |
| --- | --- | --- |
| `ai-notes.md` | Trazabilidad del procedimiento | ✅ |
| `internal/localserver` — servidor HTTP local (adaptador `net/http` → evento API GW v2, carga de `.env`, CORS y preflight OPTIONS) | Permitir desarrollo/integración local con el front (`pnpm dev` ↔ `go run .`) sin desplegar | ✅ |
| Campo `featured` en `model.Listing` (columna bool, AutoMigrate) | El front lo requiere (destacados en tabla y sitio público) | ✅ |
| Fallback de `owner_id` para uploads vía env `ALLOW_UNAUTHENTICATED_UPLOADS=true` | Etapa 1 sin Cognito; el flag es **temporal** y se elimina/desactiva en Etapa 2 | ✅ |
| Tests de integración a nivel router (`main_test.go`, repos fake en memoria) | Verificar routing + controller + service end-to-end sin DB | ✅ |

### 1.2 Frontend

| Cambio | Motivo | Estado |
| --- | --- | --- |
| `src/app/services/` — cliente API tipado (`VITE_API_URL`), tipos espejo del modelo Go y mappers UI↔API | Base única de integración | ✅ |
| `ListingsTable` conectada a `GET/DELETE/PUT /listings` (listar, eliminar, archivar) | CRUD real de inmuebles | ✅ |
| `ListingFormView` crear/editar (`POST/PUT /listings`) + fotos (`GET /listings/{id}/media`, `POST /uploads`, `DELETE /uploads/{id}`) | Alta y edición real de inmuebles | ✅ |
| `PublicSite` lee inmuebles publicados del API con fallback a datos estáticos si el API no responde | El sitio público nunca se rompe | ✅ |
| `SettingsView` carga/actualiza el perfil del agente (`GET /users`, `PUT /users/{id}`, `POST /users` si no existe) | CRUD real de datos del agente | ✅ |
| `DashboardOverview` KPIs calculados desde listings reales | Coherencia del panel | ✅ |
| Tests de integración del cliente API y mappers (vitest, `fetch` mockeado) | Regresión del contrato API | ✅ |
| `.env.example` + README de integración | Configuración reproducible | ✅ |

### 1.3 Convenciones de datos acordadas entre ambos proyectos

- `publication_status`: códigos canónicos `published | draft | archived` (el front traduce a "Publicado/Borrador/Archivado").
- `operation_type`: `sale | rent | sale_rent | exchange`.
- `property_type`: `apartment | house | penthouse | farm | commercial | office | lot | warehouse`.
- `language`: `es | en` (validado por el backend).
- Moneda: `COP | USD | EUR` (validado por el backend).
- Fotos: la principal es `media.photos[0]`; las URLs provienen de R2 (`R2_PUBLIC_URL`).

### 1.4 Configuración de despliegue requerida (manual)

**API Gateway (HTTP API) — CORS** (se mantiene en el Gateway, no en código):

- `Access-Control-Allow-Origin`: `https://aura-urrea.vercel.app` y `http://localhost:5173` (dev).
- `Access-Control-Allow-Methods`: `GET,POST,PUT,DELETE,OPTIONS`.
- `Access-Control-Allow-Headers`: `content-type,authorization`.

**Lambda — variables de entorno** (además de las existentes):

- `ALLOW_UNAUTHENTICATED_UPLOADS=true` → **solo durante la Etapa 1**. Quitar al activar Cognito.

**Vercel (front)**:

- `VITE_API_URL=https://<api-id>.execute-api.<region>.amazonaws.com` (URL del API Gateway HTTP API). Sin esta variable el front usa `http://localhost:8080` (backend local).

### 1.5 Cómo correr la integración en local

```bash
# Backend (lee .env automáticamente en modo local)
go run .            # sirve en http://localhost:8080

# Frontend
cd stuff/front-real-state/real-state-website
pnpm install
pnpm dev            # http://localhost:5173 apunta a http://localhost:8080 por defecto
```

### 1.6 Bugs preexistentes encontrados y corregidos durante la etapa

1. **Pánico por puntero nil en uploads** (`089b862`): `ownerIDFromRequest` desreferenciaba `RequestContext.Authorizer.JWT` sin verificar nil; cualquier `POST /uploads` sin authorizer (el estado actual del Gateway) tumbaba la Lambda. Detectado por los tests de router.
2. **Pánico de AutoMigrate en el arranque** (`d7c7336`): `UserMetadata` tenía campos `interface{}` sin mapeo GORM → `AutoMigrate(&User{})` fallaba (`unsupported data type`) y el proceso entraba en pánico. Corregido con `driver.Valuer`/`sql.Scanner` + columna `jsonb` (mismo patrón que `Listing`). Detectado al levantar el servidor local contra Neon.

### 1.7 Verificación end-to-end ejecutada (2026-07-06)

Contra el servidor local (`go run .`) conectado a la base de datos real de Neon:

- `POST /listings` → 201 con ID generado, `featured` persistido, `metadata.updated_at` y `source_system` por defecto.
- `GET /listings/{id}` → 200 · `PUT` (publicar) → 200 · `GET /listings/{id}/media` → 200 · `DELETE` → 204 · `GET` posterior → 404.
- `OPTIONS /listings` (preflight) → 204 con headers `Access-Control-*`.
- `POST /users` con `metadata.stats` jsonb → 201 · `PUT` parcial (solo teléfono) preserva nombre y metadata → 200 · `DELETE` → 204.
- Suite Go: `go test ./...` OK (9 tests de integración de router). Suite front: `pnpm test` OK (19 tests) y `pnpm build` OK.

### 1.8 Registro de commits (Etapa 1)

Backend (este repo):

- `007e819` docs: add ai-notes.md with staged front-back integration plan
- `40e8a21` feat(localserver): run API as plain HTTP server for local development
- `facf8e9` feat(model): add featured flag to listings
- `ee75b7a` feat(uploads): allow anonymous owner via ALLOW_UNAUTHENTICATED_UPLOADS (stage 1)
- `089b862` fix(uploads): avoid nil pointer panic when request has no authorizer
- `4dbb748` test: add router-level integration tests for users, listings and uploads
- `ca2e8ce` docs: document local dev server, tests and new env vars
- `d7c7336` fix(model): store user metadata as jsonb to unblock AutoMigrate

Frontend (`stuff/front-real-state/real-state-website`):

- `3d54eda` feat(api): add typed API client, backend model types and UI mappers
- `24068e3` feat(listings): connect listings table to backend CRUD
- `acae4f8` feat(settings): back agent profile with /users API
- `24b72dd` feat(form): wire listing form to create/update and photo uploads
- `3014e86` feat(public,dashboard): read real listings for public site and KPIs
- `d1d6477` test(api): add integration tests for API client and mappers
- `47bb8a2` docs: document backend integration, VITE_API_URL and test command

## Etapa 2 — Autenticación con AWS Cognito (pendiente)

Diseño previsto (no implementado aún):

1. **Cognito User Pool** + App Client (variables ya reservadas en `.env.example`: `AUTH_CLIENT_ID`, `AUTH_CLIENT_SECRET`, `REDIRECT_URL`, `USSUER_URL`).
2. **API Gateway JWT authorizer** sobre las rutas de mutación (`POST/PUT/DELETE /listings`, `PUT /users/{id}`, `POST /uploads`, `DELETE /uploads/{id}`). Las lecturas públicas (`GET /listings`, `GET /listings/{id}/media`, `GET /users`) permanecen abiertas para el sitio público.
3. Backend: leer `sub` desde `RequestContext.Authorizer.JWT.Claims` (ya implementado en uploads); eliminar `ALLOW_UNAUTHENTICATED_UPLOADS`.
4. Frontend: flujo OIDC (Authorization Code + PKCE) contra Cognito Hosted UI; `LoginPage` real; adjuntar `Authorization: Bearer <access_token>` en el cliente API (punto único: `services/api.ts`).
5. Tests: casos 401/403 en router y en el cliente API.
