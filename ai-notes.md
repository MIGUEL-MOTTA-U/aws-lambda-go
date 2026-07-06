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

## Etapa 1 — Integración funcional sin autenticación (en curso)

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

### 1.6 Registro de commits (Etapa 1)

Backend (este repo):

- _pendiente de completar al cierre de la etapa_

Frontend (`stuff/front-real-state/real-state-website`):

- _pendiente de completar al cierre de la etapa_

## Etapa 2 — Autenticación con AWS Cognito (pendiente)

Diseño previsto (no implementado aún):

1. **Cognito User Pool** + App Client (variables ya reservadas en `.env.example`: `AUTH_CLIENT_ID`, `AUTH_CLIENT_SECRET`, `REDIRECT_URL`, `USSUER_URL`).
2. **API Gateway JWT authorizer** sobre las rutas de mutación (`POST/PUT/DELETE /listings`, `PUT /users/{id}`, `POST /uploads`, `DELETE /uploads/{id}`). Las lecturas públicas (`GET /listings`, `GET /listings/{id}/media`, `GET /users`) permanecen abiertas para el sitio público.
3. Backend: leer `sub` desde `RequestContext.Authorizer.JWT.Claims` (ya implementado en uploads); eliminar `ALLOW_UNAUTHENTICATED_UPLOADS`.
4. Frontend: flujo OIDC (Authorization Code + PKCE) contra Cognito Hosted UI; `LoginPage` real; adjuntar `Authorization: Bearer <access_token>` en el cliente API (punto único: `services/api.ts`).
5. Tests: casos 401/403 en router y en el cliente API.
