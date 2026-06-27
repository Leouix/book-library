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
| HTTP-роутинг | **net/http** (Go 1.22+) | Роутинг без сторонних библиотек |
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

# 4. Запустить сервер в режиме live-reload
air

# 5. Либо запустить вручную
go run ./cmd/api
```

---

## Структура проекта (Standard Go Layout)

```
cmd/api/main.go          # Точка входа, DI, graceful shutdown
internal/
  api/
    handler.go           # CRUD-хэндлеры книг
    auth.go              # Регистрация, логин, JWT middleware
  storage/
    db.go                # DBTX-интерфейс, Queries (сгенерирован sqlc)
    models.go            # Go-структуры таблиц (сгенерирован sqlc)
    query.sql.go         # Методы запросов к книгам (сгенерирован sqlc)
  logger/logger.go       # Глобальный slog.Logger
schema.sql               # DDL (таблицы books, users)
query.sql                # Аннотированные SQL-запросы для sqlc
migrations/              # Миграции golang-migrate (000001_init.up.sql, .down.sql)
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

---

## Архитектурные соглашения

### Dependency Injection
Никаких глобальных переменных для БД. Подключение передаётся через структуры:
- `pgxpool.Pool` → `storage.Queries` → `api.Handler`

### Обработка ошибок
Ошибки проверяются явно (`if err != nil`), клиенту возвращаются осмысленные HTTP-статусы (400, 401, 404, 409, 500). Паниковать нельзя.

### API-роуты

| Метод | Путь | Auth | Описание |
|---|---|---|---|
| `POST` | `/register` | — | Регистрация пользователя |
| `POST` | `/login` | — | Вход, возвращает JWT |
| `POST` | `/books` | Bearer | Создать книгу |
| `GET` | `/books` | — | Список всех книг |
| `GET` | `/books/{id}` | — | Получить книгу по ID |
| `PUT` | `/books/{id}` | — | Обновить книгу |
| `DELETE` | `/books/{id}` | — | Удалить книгу |

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

**Важно:** после изменения схемы БД через миграцию необходимо обновить `schema.sql` (для sqlc-генерации типов) и перезапустить `sqlc generate`. Каждая новая миграция — это инкрементальное изменение; весь код миграций должен быть идемпотентным насколько возможно (используйте `IF EXISTS`, `IF NOT EXISTS`).

Никогда не радактируй файл .env
Дописывай важные изменения в AGENTS.md 