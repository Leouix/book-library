# AGENTS.md — Book Library

> Instruction file for AI agents. Describes the stack, commands, and architectural conventions of the project.

---

## Tech Stack

| Layer | Technology | Purpose |
|---|---|---|
| Language | **Go 1.26** | Main development language |
| Database | **PostgreSQL 16** | Store books and users |
| DB containerization | **Docker Compose** | Run PostgreSQL in a container |
| Container build | **Docker (multi-stage)** | Production image on `alpine:3.21` |
| SQL driver | **pgx/v5** | Type-safe PostgreSQL driver |
| SQL code generation | **sqlc v1.31** | Generate Go code from `schema.sql` + `query.sql` |
| DB migrations | **golang-migrate v4** | Version DB schema (auto-run on startup) |
| Authentication | **golang-jwt/v5** | JWT tokens (HS256, 24h TTL) |
| Password hashing | **bcrypt** (`x/crypto`) | Hash and verify passwords |
| HTTP routing | **chi/v5** | chi-based routing |
| Swagger | **swaggo/swag + http-swagger** | Auto-generate OpenAPI spec + Swagger UI |
| S3 client | **aws-sdk-go-v2** | Backblaze B2 via S3 API |
| UUID | **google/uuid** | Generate UUID for s3_key |
| Logging | **log/slog** | Structured logging (stdout, Debug level) |
| Live-reload | **air** | Auto-rebuild on changes (`.air.toml`) |
| Configuration | **godotenv** | Load variables from `.env` file |
| Linting | **golangci-lint** | Static code analysis |

---

## Quick Start

```bash
# 1. Start PostgreSQL
docker compose up -d

# 2. Apply migrations (optional — server does this automatically on startup)
go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
  -path migrations -database "postgres://bookuser:bookpass@localhost:5432/bookdb?sslmode=disable" up

# 3. Generate code from SQL (if schema.sql / query.sql changed)
sqlc generate

# 4. Generate Swagger spec (if annotations changed)
swag init -g cmd/api/main.go -o docs/

# 5. Run server in live-reload mode
air

# 6. Or run manually
go run ./cmd/api
```

---

## Project Structure (Standard Go Layout)

```
cmd/api/main.go          # Entry point, DI, graceful shutdown + WorkerPool
internal/
  api/
    handler.go           # CRUD handlers for books
    auth.go              # Registration, login, JWT middleware
    files.go             # File upload/download
  service/
    file.go              # File business logic (S3 + DB)
    worker.go            # WorkerPool — async file upload to S3
  storage/
    db.go                # DBTX interface, Queries (sqlc generated)
    models.go            # Go table structs (sqlc generated)
    query.sql.go         # Query methods for books/files (sqlc generated)
    s3/
      s3.go              # S3 client, FileStorage interface, S3Storage
  logger/logger.go       # Global slog.Logger
docs/                    # Generated swagger spec
query.sql                # Annotated SQL queries for sqlc
migrations/              # golang-migrate migrations
sqlc.yaml                # sqlc configuration
docker-compose.yml       # PostgreSQL 16
Dockerfile               # Multi-stage production image
.air.toml                # Live-reload configuration
.env                     # Environment variables (not committed!)
```

---

## Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `DATABASE_URL` | `postgres://bookuser:bookpass@localhost:5432/bookdb` | DB connection string |
| `ADDR` | `:8080` | HTTP server address |
| `JWT_SECRET` | `changeme` | Secret for JWT signing |
| `B2_KEY_ID` | — | Backblaze B2 Application Key ID |
| `B2_APPLICATION_KEY` | — | Backblaze B2 Application Key |
| `B2_REGION` | — | B2 region (e.g. `us-west-002`) |
| `B2_ENDPOINT` | — | S3-compatible B2 endpoint |
| `B2_BUCKET` | — | B2 bucket name |
| `FILE_BASE_URL` | `http://localhost:8080` | Base URL for book file links (formed as `{FILE_BASE_URL}/{s3_key}`) |

---

## Architectural Conventions

### Dependency Injection
No global variables for DB. Connections are passed through structs:
- `pgxpool.Pool` → `storage.Queries` → `api.Handler`
- `s3.Client` → `s3.S3Storage` → `service.FileService` → `api.Handler`
- `storage.Queries` + `service.FileService` → `service.WorkerPool` → `api.Handler`

### Error Handling
Errors are checked explicitly (`if err != nil`), meaningful HTTP statuses are returned to the client (400, 401, 404, 409, 500). No panicking.

### API Routes

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/register` | — | Register a user |
| `POST` | `/login` | — | Login, returns JWT |
| `POST` | `/books` | Bearer | Create book + upload .txt file (multipart: title*, author*, year*, file*). Returns 202, async processing |
| `GET` | `/books` | — | List processed books (status=completed only) |
| `GET` | `/books/{id}` | — | Get book by ID (only if status=completed) |
| `PUT` | `/books/{id}` | Bearer | Update book metadata |
| `DELETE` | `/books/{id}` | Bearer | Delete book |

### Swagger UI

URL: `http://localhost:8080/swagger/index.html`

```bash
# Generate after changing annotations
swag init -g cmd/api/main.go -o docs/
```

> Files `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` are in the repository so the project compiles without `swag`.

### sqlc: when to regenerate
- `schema.sql` changed (tables/columns)
- `query.sql` changed (queries)
- `sqlc.yaml` changed

```bash
sqlc generate          # or: go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate
```

> Files `db.go`, `models.go`, `query.sql.go` are auto-generated. Do not edit manually.

### Generated files (in .git)
`internal/storage/db.go`, `models.go`, `query.sql.go` — despite being auto-generated, they live in the repository so the project compiles without running sqlc.
`docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` — same for swagger.

### File Storage (S3 / Backblaze B2)

Package `internal/storage/s3` provides:
- `s3.FileStorage` — interface with `Upload` / `Download` methods
- `s3.S3Storage` — implementation via aws-sdk-go-v2 (`PutObject` / `GetObject`)
- `s3.NewClient` — initialize S3 client with `BaseEndpoint` and `UsePathStyle = true`

Package `internal/service` implements:
- `FileService.UploadBookFile` — upload book file to S3, returns `(s3Key, fileURL)`
- S3 key: `books/{uuid}/{original_name}`
- `fileURL` = `{FILE_BASE_URL}/{s3_key}`
- S3 + DB coordination in `main.go` (called from `CreateBook` handler)

DB: the `books` table stores `file_url`, `s3_key`, `file_name` directly.
The `files` table was removed (migration 000003).

If B2 is not configured (variables are empty), the server starts without file storage — `POST /books` returns 503.

### Async Worker Pool (file upload)

Package `internal/service/worker.go` implements async file upload:

- `WorkerPool` — pool of workers (3 goroutines), listening on a buffered channel `chan Job` (100 tasks)
- On `POST /books`: file is saved to `/tmp`, book is created with `pending` status, returns 202
- Worker picks up the task: reads temp file → upload to S3 → updates status to `completed` → deletes temp file
- On error: status set to `failed`, temp file is deleted
- **Graceful shutdown**: HTTP → WorkerPool sequentially (30s timeout to finish current tasks)
- **Retry on startup**: all books with `pending`/`processing` status are marked as `failed` (temp files lost on restart)

**Book statuses**: `pending` → `completed` | `failed`
- `GET /books` — only `completed`
- `GET /books/{id}` — only `completed`

### Migrations (golang-migrate v4)

Migrations are in `migrations/` and are applied automatically on server startup in `cmd/api/main.go`.
Driver: `database/postgres` (uses `postgres://` URL scheme, shared with `pgxpool`).

**Creating a new migration:**

```bash
migrate create -ext sql -dir migrations -seq <change_description>
```

After creation, fill in the `up.sql` and `down.sql` files.

**Applying migrations manually:**

```bash
# Apply all pending migrations
go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
  -path migrations -database "postgres://bookuser:bookpass@localhost:5432/bookdb?sslmode=disable" up

# Roll back the last migration
go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
  -path migrations -database "postgres://bookuser:bookpass@localhost:5432/bookdb?sslmode=disable" down 1
```

**Important:** 

After changing the DB schema via migration, you must re-run `sqlc generate` (the schema is read from `migrations/`). Each new migration is an incremental change; all migration code should be as idempotent as possible (use `IF EXISTS`, `IF NOT EXISTS`).

Never edit the .env file, maintain .env.example instead.

Keep routes up to date in README.md.

Write important changes into AGENTS.md.
