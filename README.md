# rs-lambda-go

A Go-based AWS Lambda function that provides a serverless RESTful interface to manage user and listing data in a PostgreSQL database using GORM, as well as a high-performance concurrent file upload and storage system utilizing Cloudflare R2. It handles request payload routing, strict validations, metadata database persistence, and AWS API Gateway V2 binary integration.

## Description

The `rs-lambda-go` application is an AWS Lambda function designed to handle HTTP request events forwarded by Amazon API Gateway V2. It implements four core modules:
1. **User Management**: CRUD operations on a PostgreSQL database.
2. **Listing Management**: CRUD operations on real estate listing objects stored in PostgreSQL.
3. **Asset & Storage Management**: Concurrent file uploads to Cloudflare R2, metadata tracking, secure URL generation (signed GET URLs for private files, CDN caching for public files), and clean rollback mechanisms.
4. **Visit Analytics**: Anonymous visitor tracking and aggregated analytics for real estate listings.

The application automatically validates incoming HTTP request payloads, performs MIME verification using file magic numbers, and returns standardized responses with CORS-compliant headers.

- For **Users**: Generates a cryptographically secure 16-byte random hexadecimal identifier.
- For **Listings**: Normalizes numeric and string listing IDs automatically, and generates unique IDs if not provided.
- For **Assets**: Concurrently uploads batches of files directly to Cloudflare R2 via path-style endpoint signing, validates sizes and real MIME types, and guarantees storage/DB consistency via parallel transaction rollbacks.

## Features

- **AWS Lambda & API Gateway Integration**: Formatted to handle `events.APIGatewayV2HTTPRequest` inputs and return `events.APIGatewayV2HTTPResponse` outputs. Supports base64-decoded binary payload routing for file uploads.
- **GORM & PostgreSQL Persistence**: Performs schema migrations and database operations automatically.
- **Concurrent Upload Engine**:
  - Leverages Go's `golang.org/x/sync/errgroup` with a bounded semaphore (`SetLimit`) to upload multiple files to Cloudflare R2 concurrently.
  - Automatically performs concurrent best-effort rollbacks (deleting uploaded R2 objects) if database registration or any part of the batch fails.
- **Advanced File Validation**:
  - Enforces entity-specific size and count limits (e.g., maximum 30 photos per listing, 1 avatar per user).
  - Inspects file headers using magic number detection (`http.DetectContentType`) to prevent MIME-spoofing attacks (renaming `.exe` to `.jpg`).
- **Secure File URL Resolution**:
  - Public assets (e.g. listing photos, user avatars) use non-expiring CDN URLs.
  - Private assets (e.g. listing PDFs) generate short-lived presigned GET URLs dynamically.

## Architecture

The project is structured according to a layered architecture pattern:

- **Entry Point (`main.go`)**: Initializes database and storage connections, executes schema migrations, wires dependencies, and routes incoming HTTP requests to controllers.
- **Controller Layer (`internal/controller/`)**: Maps HTTP paths/methods, parses multipart/form-data payloads (decoding base64 binary inputs from API Gateway), and marshals JSON responses.
- **Service Layer (`internal/service/`)**: Handles core business logic, validations (magic-number content-type checks, limit controls), and coordinates concurrent upload execution.
- **Storage Client (`internal/storage/`)**: Abstracts Cloudflare R2 / S3 operations (file put, delete, presigning, and content header detection).
- **Repository Layer (`internal/repository/`)**: Standardizes GORM queries for PostgreSQL.
- **Data Model (`internal/model/`)**: Defines structural schemas (Users, Listings, Assets) and maps constraints (upload rules per file type).

## Technologies Used

| Category | Technology |
| :--- | :--- |
| Programming Language | Go (v1.26.4) |
| Core Framework | AWS Lambda Go (`github.com/aws/aws-lambda-go`) |
| ORM & Database Driver | GORM (`gorm.io/gorm`) & PostgreSQL Driver (`gorm.io/driver/postgres`) |
| Storage Client SDK | AWS SDK for Go V2 (`github.com/aws/aws-sdk-go-v2/service/s3`) |
| Concurrency Utils | Go Sync Library (`golang.org/x/sync/errgroup`) |

## Requirements

- Go version `1.26.4` or newer.
- A PostgreSQL database instance.
- A Cloudflare R2 bucket.

## Installation

Download the required Go module dependencies:

```bash
go mod download
```

To compile the application as a binary named `bootstrap` (the standard entrypoint file name for custom runtime environments like `provided.al2023` in AWS Lambda) and package it into a zip archive:

### On Linux/macOS
```bash
GOOS=linux GOARCH=amd64 go build -o bootstrap main.go
zip lambda-handler.zip bootstrap
```

### On Windows (PowerShell)
```powershell
$env:GOOS = "linux"
$env:GOARCH = "amd64"
$env:CGO_ENABLED = "0"
go build -o bootstrap main.go
~\Go\Bin\build-lambda-zip.exe -o lambda-handler.zip bootstrap
```

## Local Development

Outside the AWS Lambda runtime the binary starts a plain HTTP server (see `internal/localserver`) that adapts each request into an API Gateway V2 event, so the exact same code path runs locally and in AWS. It loads variables from `.env` automatically and answers CORS preflights itself:

```bash
go run .        # serves on http://localhost:8080 (PORT to override)
```

This is the backend the front end (`stuff/front-real-state/real-state-website`, `pnpm dev`) points to by default.

Run the integration tests (router + controllers + services over in-memory repositories):

```bash
go test ./...
```

## Configuration

The application is configured using the following environment variables:

| Variable | Required | Description |
| :--- | :--- | :--- |
| `DATABASE_CONNECTION` | Yes | Connection string for PostgreSQL (e.g. `postgresql://user:pass@host:5432/dbname?sslmode=disable`). |
| `R2_ENDPOINT` | Yes | S3-compatible Cloudflare R2 endpoint (e.g. `https://<account-id>.r2.cloudflarestorage.com`). |
| `R2_BUCKET_NAME` | Yes | The target R2 bucket name. |
| `R2_ACCESS_KEY_ID` | Yes | R2 API token Access Key ID (from Cloudflare dashboard). |
| `R2_SECRET_ACCESS_KEY` | Yes | R2 API token Secret Access Key. |
| `R2_PUBLIC_URL` | Yes | Custom domain/CDN endpoint mapped to the R2 bucket for serving public files (e.g. `https://assets.tudominio.com`). |
| `ALLOW_UNAUTHENTICATED_UPLOADS` | No | **Stage 1 only (pre-Cognito).** When `true`, uploads without a JWT authorizer are attributed to a fixed owner. Remove once the Cognito JWT authorizer is enabled. |
| `PORT` | No | Local dev server port (default `8080`). Ignored in AWS Lambda. |
| `ACCESS_CONTROL_ALLOW_ORIGIN` | No | CORS origin echoed by the **local** dev server (default `*`). In AWS, CORS is configured on the API Gateway. |

### Upload troubleshooting (R2 credentials)

If uploads fail with `UPLOAD_FAILED` and a message similar to:

`InvalidArgument: Credential access key has length 20, should be 32`

the Lambda is not using Cloudflare R2 credentials. Ensure both
`R2_ACCESS_KEY_ID` and `R2_SECRET_ACCESS_KEY` are set in the **active Lambda
environment** (not only in local `.env`), and that the key pair belongs to an
R2 API token for the configured bucket/account.

## Usage

### Endpoint Specifications

| Action | HTTP Method | Path | Request Body / Query Params | Status |
| :--- | :--- | :--- | :--- | :--- |
| **List Users** | `GET` | `/users` | None | `200 OK` |
| **Get User** | `GET` | `/users/{id}` | None | `200 OK` |
| **Create User** | `POST` | `/users` | User JSON | `201 Created` |
| **Update User** | `PUT` | `/users/{id}` | User JSON | `200 OK` |
| **Delete User** | `DELETE` | `/users/{id}` | None | `204 No Content` |
| **List Listings** | `GET` | `/listings` | None | `200 OK` |
| **Get Listing** | `GET` | `/listings/{id}` | None | `200 OK` |
| **Create Listing** | `POST` | `/listings` | Listing JSON | `201 Created` |
| **Update Listing** | `PUT` | `/listings/{id}` | Listing JSON | `200 OK` |
| **Delete Listing** | `DELETE` | `/listings/{id}` | None | `204 No Content` |
| **Upload Files** | `POST` | `/uploads` | `multipart/form-data` | `201 Created` |
| **Get Private Asset URL** | `GET` | `/uploads/{id}/url` | None | `200 OK` |
| **Delete Asset** | `DELETE` | `/uploads/{id}` | None | `204 No Content` |
| **List Listing Media** | `GET` | `/listings/{id}/media` | None | `200 OK` |
| **Record Visit Event** | `POST` | `/visits` | Visit JSON (public, no auth) | `201 Created` |
| **Top Listings Analytics** | `GET` | `/analytics/listings` | `?window=7d\|30d` (JWT required) | `200 OK` |

### Asset Management Upload Format (`POST /uploads`)

The files must be uploaded as `multipart/form-data`.

**Form Fields:**
- `entity_type`: Category of asset. Allowed options:
  - `user_avatar` (Public — shown on the public site, max 5MB, JPG/PNG/WebP, limit 1)
  - `listing_photo` (Public, max 10MB, JPG/PNG/WebP, limit 30)
  - `listing_pdf` (Private, max 20MB, PDF, limit 1)

Single-asset entity types (limit 1, e.g. `user_avatar`) use **replace semantics**: uploading a new file soft-deletes the previous asset and removes its R2 object instead of failing against the limit. Multi-asset types (e.g. `listing_photo`) keep the hard limit.
- `entity_id`: The ID of the owner user or listing.
- `file` (or `files`): File payload(s) to upload. Up to 10 files per request.

**Example Response (201 Created):**
```json
[
  {
    "id": "e4f8d27a-5b6c-7d8e-9f0a-1b2c3d4e5f6g",
    "entity_type": "listing_photo",
    "entity_id": "c21-apartment-bogota",
    "content_type": "image/jpeg",
    "file_size": 245312,
    "status": "confirmed",
    "is_public": true,
    "owner_id": "user-auth-id",
    "created_at": "2026-06-23T23:00:00Z",
    "url": "https://assets.tudominio.com/listings/c21-apartment-bogota/photos/e4f8d27a-5b6c-7d8e-9f0a-1b2c3d4e5f6g.jpg"
  }
]
```

### Visit Analytics

#### `POST /visits` — Record a visit event (public, no authentication)

The front end emits a `view_start` event when a listing detail page is opened and a `view_end` when the user navigates away. Events are anonymous: the visitor is identified by a UUID v4 stored in `localStorage` (DEC-008).

**Request Body:**
```json
{
  "visitor_id": "550e8400-e29b-41d4-a716-446655440000",
  "listing_id": "c21-apartment-bogota",
  "event_type": "view_start",
  "source": "public",
  "duration_ms": 0
}
```

| Field | Required | Description |
| :--- | :--- | :--- |
| `visitor_id` | Yes | UUID v4 generated by the front end. |
| `listing_id` | Yes | Listing being viewed. |
| `event_type` | Yes | `view_start` or `view_end`. |
| `source` | No | `public` (default) or `admin`. |
| `duration_ms` | No | Client-measured duration in ms. Used as fallback when server-side pairing fails (DEC-009). |

Rate-limited to 1 event per second per `visitor_id` (start events only).

#### `GET /analytics/listings` — Aggregated listing analytics (JWT required)

Returns the top listings ranked by total views within the requested time window.

**Query Parameters:**
- `window`: `7d` (default) or `30d`.

**Response (200 OK):**
```json
{
  "items": [
    {
      "listing_id": "c21-apartment-bogota",
      "unique_visitors": 42,
      "total_views": 87,
      "avg_duration_ms": 15230.5,
      "p50_duration_ms": 12000,
      "p95_duration_ms": 45000
    }
  ]
}
```

Duration is computed server-side by pairing `view_start`/`view_end` events within a 30-minute window per visitor (DEC-009). Events older than 90 days are excluded (DEC-007).

## How Upload Concurrency & Consistency Work

When uploading a batch of files:
1. **Validation**: All files in the batch are inspected in-memory first. Size, extension, and headers (magic-numbers) are verified.
2. **Parallel Dispatch**: The application creates a scoped context with `errgroup.WithContext`. It spawns a concurrent worker goroutine for each file.
3. **Execution**: Workers upload the bytes to R2. Upon success, they register the asset metadata to PostgreSQL.
4. **Failure & Rollback**:
   - If *any* worker fails (e.g. database disconnect, connection loss to R2), the context is cancelled.
   - Sibling workers exit immediately.
   - The application starts a concurrent rollback process, deleting any objects successfully written during that transaction from R2 to avoid orphaned assets.

## Project Structure

```text
rs-lambda-go/
├── internal/
│   ├── auth/
│   │   └── jwt.go
│   ├── controller/
│   │   ├── listing_controller.go
│   │   ├── upload_controller.go
│   │   ├── user_controller.go
│   │   └── visit_controller.go
│   ├── model/
│   │   ├── asset.go
│   │   ├── listing.go
│   │   ├── user.go
│   │   └── visit.go
│   ├── repository/
│   │   ├── asset_repository.go
│   │   ├── gorm_asset_repository.go
│   │   ├── gorm_listing_repository.go
│   │   ├── gorm_user_repository.go
│   │   ├── gorm_visit_repository.go
│   │   ├── listing_repository.go
│   │   ├── user_repository.go
│   │   └── visit_repository.go
│   ├── service/
│   │   ├── analytics_service.go
│   │   ├── listing_service.go
│   │   ├── upload_service.go
│   │   ├── user_service.go
│   │   └── visit_service.go
│   └── storage/
│       ├── r2_client.go
│       └── storage.go
├── .env
├── .env.example
├── .gitignore
├── go.mod
├── go.sum
└── main.go
```

## Security & Error Handling

- **Authentication Guard**: Asset actions require a valid `owner_id` (sub claim extracted from API Gateway V2 context / JWT authorizer).
- **Path Sanitization**: `entity_id` is sanitized using regex patterns (`[^a-zA-Z0-9\-]`) to prevent directory traversal attacks during object key generation in R2.
- **Magic Number Detection**: Re-analyzes headers via `http.DetectContentType` to enforce real file types, preventing users from uploading executables masquerading as images.
- **CORS Config**: CORS configuration restricts responses to authorized origins, allowing seamless client-to-gateway actions.
