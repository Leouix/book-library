# Book Library API

### Key Features
- routing, middleware
- Authorization
- Migration System
- sqlc for model generation
- Async tasks
- PostgreSQL
- Cloud file storage
- Technical details are in AGENTS.md

### Swagger UI
``` 
http://localhost:8080/swagger/index.html#/
```
![Screenshot From 2026-06-28 22-27-31.png](docs/assets/Screenshot%20From%202026-06-28%2022-27-31.png)


## Endpoints

### Authorization

| Method | Path         | Description          | Auth    |
|--------|--------------|----------------------|---------|
| POST   | `/register`  | Register             | —       |
| POST   | `/login`     | Login, get JWT       | —       |

### Books

| Method | Path            | Description                                    | Auth    |
|--------|-----------------|------------------------------------------------|---------|
| POST   | `/books`        | Create book + upload file (async)              | Bearer  |
| GET    | `/books/{id}`   | Get book (completed only)                      | —       |
| GET    | `/books`        | List processed books (completed only)          | —       |
| PUT    | `/books/{id}`   | Update book metadata                           | —       |
| DELETE | `/books/{id}`   | Delete book                                    | —       |

> 🔐 `POST /books` requires a JWT token in the `Authorization: Bearer <token>` header. Other book endpoints are open.

---

## Authorization

### Register

```bash
curl -X POST http://localhost:8080/register \
  -H "Content-Type: application/json" \
  -d '{"username": "alice", "password": "secret123"}'
```

**Request body:**

| Field    | Type   | Required | Description |
|----------|--------|----------|-------------|
| username | string | yes      | Login       |
| password | string | yes      | Password    |

**Response `201 Created`:**

```json
{
  "username": "alice"
}
```

**Errors:**

| Status | When                           |
|--------|--------------------------------|
| 400    | Invalid JSON or empty fields   |
| 409    | User already exists            |

---

### Login (get JWT)

```bash
curl -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"username": "alice", "password": "secret123"}'
```

**Request body:**

| Field    | Type   | Required | Description |
|----------|--------|----------|-------------|
| username | string | yes      | Login       |
| password | string | yes      | Password    |

**Response `200 OK`:**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "username": "alice"
}
```

**Errors:**

| Status | When                              |
|--------|-----------------------------------|
| 400    | Invalid JSON or empty fields      |
| 401    | Invalid username or password      |

> 💡 Token lives **24 hours**. Save the `token` value — you'll need it for protected endpoints.

---

## Books

### Create a book

```bash
TOKEN="eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."

curl -X POST http://localhost:8080/books \
  -H "Authorization: Bearer $TOKEN" \
  -F "title=War and Peace" \
  -F "author=Leo Tolstoy" \
  -F "year=1869" \
  -F "file=@book.txt"
```

**Headers:**

| Header        | Value                | Required |
|---------------|----------------------|----------|
| Authorization | `Bearer <JWT-token>` | yes      |

**Form fields (multipart/form-data):**

| Field  | Type   | Required | Description                |
|--------|--------|----------|----------------------------|
| title  | string | yes      | Book title                 |
| author | string | yes      | Author                     |
| year   | string | yes      | Year of publication        |
| file   | file   | yes      | File (.txt, max 10 MB)     |

**Response `202 Accepted`:**

```json
{
  "id": 1,
  "status": "pending"
}
```

The book will be processed asynchronously by a worker. When processing completes, `status` changes to `completed` and the book appears in `GET /books`.

**Errors:**

| Status | When                                  |
|--------|---------------------------------------|
| 400    | Missing required fields or file       |
| 401    | Missing or invalid token              |
| 413    | File exceeds 10 MB                    |
| 503    | File storage not configured           |

---

### Get a book by ID

```bash
curl http://localhost:8080/books/1
```

**Path parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| id        | int  | Book ID     |

**Response `200 OK`:**

```json
{
  "id": 1,
  "title": "War and Peace",
  "author": "Leo Tolstoy",
  "year": 1869,
  "file_url": "http://localhost:8080/books/550e8400-e29b-41d4-a716-446655440000/book.txt",
  "s3_key": "books/550e8400-e29b-41d4-a716-446655440000/book.txt",
  "file_name": "book.txt",
  "status": "completed"
}
```

Only returned for books with `completed` status. If the book is still processing (`pending`) or processing failed (`failed`) — `404`.

**Errors:**

| Status | When                             |
|--------|----------------------------------|
| 400    | Invalid id                       |
| 404    | Book not found or not processed  |

---

### List processed books

```bash
curl http://localhost:8080/books
```

Returns only books with `completed` status. Books that are still processing (`pending`) or failed (`failed`) are not shown.

**Response `200 OK`:**

```json
[
  {
    "id": 1,
    "title": "War and Peace",
    "author": "Leo Tolstoy",
    "year": 1869,
    "file_url": "http://localhost:8080/books/550e8400-e29b-41d4-a716-446655440000/book.txt",
    "s3_key": "books/550e8400-e29b-41d4-a716-446655440000/book.txt",
    "file_name": "book.txt",
    "status": "completed"
  }
]
```

If no processed books exist — an empty array `[]` is returned.

---

### Update a book

```bash
curl -X PUT http://localhost:8080/books/1 \
  -H "Content-Type: application/json" \
  -d '{"title": "War and Peace", "author": "Leo Tolstoy", "year": 1873}'
```

**Path parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| id        | int  | Book ID     |

**Request body:**

| Field  | Type   | Required | Description     |
|--------|--------|----------|-----------------|
| title  | string | yes      | New title       |
| author | string | yes      | New author      |
| year   | int    | yes      | New year        |

**Response `200 OK`:**

```json
{
  "id": 1,
  "title": "War and Peace",
  "author": "Leo Tolstoy",
  "year": 1873,
  "file_url": "http://localhost:8080/books/550e8400-e29b-41d4-a716-446655440000/book.txt",
  "s3_key": "books/550e8400-e29b-41d4-a716-446655440000/book.txt",
  "file_name": "book.txt"
}
```

**Errors:**

| Status | When                   |
|--------|------------------------|
| 400    | Invalid id or JSON     |
| 404    | Book not found          |

---

### Delete a book

```bash
curl -X DELETE http://localhost:8080/books/1
```

**Path parameters:**

| Parameter | Type | Description |
|-----------|------|-------------|
| id        | int  | Book ID     |

**Response:** `204 No Content` (empty body)

**Errors:**

| Status | When           |
|--------|----------------|
| 400    | Invalid id     |
| 404    | Book not found |
