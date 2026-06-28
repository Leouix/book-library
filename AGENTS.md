# AGENTS.md — Book Library

> Файл-инструкция для AI-агентов. Описывает стек, команды и архитектурные соглашения проекта.

---

## Стек технологий

| Слой | Технология | Назначение |
|---|---|---|
| Язык | **Go 1.26** | Основной язык разработки |
| База данных | **PostgreSQL 16** | Хранение книг и пользователей |
| Контейнеризация БД | **Docker Compose** | Поднятие PostgreSQL в контейнере |
| Сборка в контейнер | **Docker (multi-stage)** | Production-образ на `alpine:3.21` |
| SQL-драйвер | **pgx/v5** | Типобезопасный драйвер PostgreSQL |
| Генерация из SQL | **sqlc v1.31** | Генерация Go-кода из `schema.sql` + `query.sql` |
| Миграции БД | **golang-migrate v4** | Версионирование схемы БД (автозапуск при старте)
| Аутентификация | **golang-jwt/v5** | JWT-токены (HS256, 24h TTL) |
| Хеширование паролей | **bcrypt** (`x/crypto`) | Хеширование и проверка паролей |
| HTTP-роутинг | **chi/v5** | Роутинг на основе chi |
| Swagger | **swaggo/swag + http-swagger** | Автогенерация OpenAPI-спецификации + Swagger UI |
| S3-клиент | **aws-sdk-go-v2** | Backblaze B2 через S3 API |
| UUID | **google/uuid** | Генерация UUID для s3_key |
| Логирование | **log/slog** | Структурированное логирование (stdout, уровень Debug) |
| Live-reload | **air** | Автопересборка при изменениях (`.air.toml`) |
| Конфигурация | **godotenv** | Загрузка переменных из `.env`-файла |
| Линтинг | **golangci-lint** | Статический анализ кода |

---

## Быстрый старт

```bash
# 1. Поднять PostgreSQL
docker compose up -d

# 2. Применить миграции (опционально — сервер делает это автоматически при запуске)
go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
  -path migrations -database "postgres://bookuser:bookpass@localhost:5432/bookdb?sslmode=disable" up

# 3. Сгенерировать код из SQL (если менялись schema.sql / query.sql)
sqlc generate

# 4. Сгенерировать Swagger-спецификацию (если менялись аннотации)
swag init -g cmd/api/main.go -o docs/

# 5. Запустить сервер в режиме live-reload
air

# 6. Либо запустить вручную
go run ./cmd/api
```

---

## Структура проекта (Standard Go Layout)

```
cmd/api/main.go          # Точка входа, DI, graceful shutdown + WorkerPool
internal/
  api/
    handler.go           # CRUD-хэндлеры книг
    auth.go              # Регистрация, логин, JWT middleware
    files.go             # Загрузка/скачивание файлов
  service/
    file.go              # Бизнес-логика файлов (S3 + БД)
    worker.go            # WorkerPool — асинхронная загрузка файлов в S3
  storage/
    db.go                # DBTX-интерфейс, Queries (сгенерирован sqlc)
    models.go            # Go-структуры таблиц (сгенерирован sqlc)
    query.sql.go         # Методы запросов к книгам/файлам (сгенерирован sqlc)
    s3/
      s3.go              # S3-клиент, FileStorage интерфейс, S3Storage
  logger/logger.go       # Глобальный slog.Logger
docs/                    # Сгенерированная swagger-спецификация
query.sql                # Аннотированные SQL-запросы для sqlc
migrations/              # Миграции golang-migrate
sqlc.yaml                # Конфигурация sqlc
docker-compose.yml       # PostgreSQL 16
Dockerfile               # Multi-stage production-образ
.air.toml                # Конфигурация live-reload
.env                     # Переменные окружения (не коммитится!)
```

---

## Переменные окружения

| Переменная | По умолчанию | Назначение |
|---|---|---|
| `DATABASE_URL` | `postgres://bookuser:bookpass@localhost:5432/bookdb` | Строка подключения к БД |
| `ADDR` | `:8080` | Адрес HTTP-сервера |
| `JWT_SECRET` | `changeme` | Секрет для подписи JWT |
| `B2_KEY_ID` | — | Backblaze B2 Application Key ID |
| `B2_APPLICATION_KEY` | — | Backblaze B2 Application Key |
| `B2_REGION` | — | Регион B2 (например `us-west-002`) |
| `B2_ENDPOINT` | — | S3-совместимый endpoint B2 |
| `B2_BUCKET` | — | Имя корзины B2 |
| `FILE_BASE_URL` | `http://localhost:8080` | Базовый URL для ссылок на файлы книг (формируется как `{FILE_BASE_URL}/{s3_key}`) |

---

## Архитектурные соглашения

### Dependency Injection
Никаких глобальных переменных для БД. Подключение передаётся через структуры:
- `pgxpool.Pool` → `storage.Queries` → `api.Handler`
- `s3.Client` → `s3.S3Storage` → `service.FileService` → `api.Handler`
- `storage.Queries` + `service.FileService` → `service.WorkerPool` → `api.Handler`

### Обработка ошибок
Ошибки проверяются явно (`if err != nil`), клиенту возвращаются осмысленные HTTP-статусы (400, 401, 404, 409, 500). Паниковать нельзя.

### API-роуты

| Метод | Путь | Auth | Описание |
|---|---|---|---|
| `POST` | `/register` | — | Регистрация пользователя |
| `POST` | `/login` | — | Вход, возвращает JWT |
| `POST` | `/books` | Bearer | Создать книгу + загрузить .txt-файл (multipart: title*, author*, year*, file*). Возвращает 202, обработка асинхронная |
| `GET` | `/books` | — | Список обработанных книг (только status=completed) |
| `GET` | `/books/{id}` | — | Получить книгу по ID (только если status=completed) |
| `PUT` | `/books/{id}` | Bearer | Обновить метаданные книги |
| `DELETE` | `/books/{id}` | Bearer | Удалить книгу |

### Swagger UI

URL: `http://localhost:8080/swagger/index.html`

```bash
# Генерация после изменения аннотаций
swag init -g cmd/api/main.go -o docs/
```

> Файлы `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` лежат в репозитории, чтобы проект компилировался без `swag`.

### sqlc: когда перегенерировать
- Изменился `schema.sql` (таблицы/колонки)
- Изменился `query.sql` (запросы)
- Изменился `sqlc.yaml`

```bash
sqlc generate          # или: go run github.com/sqlc-dev/sqlc/cmd/sqlc@latest generate
```

> Файлы `db.go`, `models.go`, `query.sql.go` генерируются автоматически. Не редактировать вручную.

### Сгенерированные файлы (в .git)
`internal/storage/db.go`, `models.go`, `query.sql.go` — несмотря на авто-генерацию, лежат в репозитории, чтобы проект компилировался без запуска sqlc.
`docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml` — аналогично для swagger.

### File Storage (S3 / Backblaze B2)

Пакет `internal/storage/s3` предоставляет:
- `s3.FileStorage` — интерфейс с методами `Upload` / `Download`
- `s3.S3Storage` — имплементация через aws-sdk-go-v2 (`PutObject` / `GetObject`)
- `s3.NewClient` — инициализация S3-клиента с `BaseEndpoint` и `UsePathStyle = true`

Пакет `internal/service` реализует:
- `FileService.UploadBookFile` — загрузка файла книги в S3, возвращает `(s3Key, fileURL)`
- S3-ключ: `books/{uuid}/{original_name}`
- `fileURL` = `{FILE_BASE_URL}/{s3_key}`
- Координация S3 + БД в `main.go` (вызывается из `CreateBook` хендлера)

БД: таблица `books` хранит `file_url`, `s3_key`, `file_name` напрямую.
Таблица `files` удалена (миграция 000003).

Если B2 не настроен (переменные пусты), сервер запускается без файлового хранилища — ручка `POST /books` возвращает 503.

### Async Worker Pool (загрузка файлов)

Пакет `internal/service/worker.go` реализует асинхронную загрузку файлов:

- `WorkerPool` — пул воркеров (3 goroutines), слушающих буферизированный канал `chan Job` (100 задач)
- При `POST /books`: файл сохраняется в `/tmp`, книга создаётся со статусом `pending`, возвращается 202
- Воркер подхватывает задачу: читает temp-файл → upload в S3 → обновляет статус на `completed` → удаляет temp-файл
- При ошибке: статус `failed`, temp-файл удаляется
- **Graceful shutdown**: последовательно HTTP → WorkerPool (30s timeout на дожидание текущих задач)
- **Ретрей при старте**: все книги со статусом `pending`/`processing` помечаются как `failed` (temp-файлы потеряны при рестарте)

**Статусы книги**: `pending` → `completed` | `failed`
- `GET /books` — только `completed`
- `GET /books/{id}` — только `completed`

### Миграции (golang-migrate v4)

Миграции находятся в `migrations/` и применяются автоматически при старте сервера в `cmd/api/main.go`.
Драйвер: `database/postgres` (использует `postgres://`-схему URL, единую с `pgxpool`).

**Создание новой миграции:**

```bash
migrate create -ext sql -dir migrations -seq <описание_изменения>
```

После создания заполните файлы `up.sql` и `down.sql`.

**Ручное применение миграций:**

```bash
# Накатить все неприменённые миграции
go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
  -path migrations -database "postgres://bookuser:bookpass@localhost:5432/bookdb?sslmode=disable" up

# Откатить последнюю миграцию
go run github.com/golang-migrate/migrate/v4/cmd/migrate@latest \
  -path migrations -database "postgres://bookuser:bookpass@localhost:5432/bookdb?sslmode=disable" down 1
```

**Важно:** 

После изменения схемы БД через миграцию необходимо перезапустить `sqlc generate` (схема читается из `migrations/`). Каждая новая миграция — это инкрементальное изменение; весь код миграций должен быть идемпотентным насколько возможно (используйте `IF EXISTS`, `IF NOT EXISTS`).

Никогда не редактируй файл .env, веди .env.example.

Роуты сохраняй актуальными в файле README.md.

Дописывай важные изменения в AGENTS.md.