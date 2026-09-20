# Pigeongram Chat

[Русская версия](README_RU.md) | [English version](README_EN.md)

## Purpose

`pigeongram-chat` is the Go application in Pigeongram. It provides an HTTP chat interface, WebSocket connections for real-time message exchange, and file operations backed by MinIO. The application uses PostgreSQL for persistent users, messages, and sessions, Redis for caching and event exchange between server instances, and Prometheus for metrics.

This document is based on the current source code and configuration in the project. Capabilities that are not confirmed by the code are not assumed here.

## Entry point and startup flow

Execution starts in `cmd/server/main.go`, in the `main` function.

At startup, the application:

1. reads command-line flags and creates a `serverID`;
2. opens a PostgreSQL connection and, when needed, creates tables, indexes, the `get_recent_messages` function, and the `active_users` view;
3. attempts to connect to Redis; if Redis is unavailable, it falls back to the in-memory cache;
4. attempts to connect to MinIO; if MinIO is unavailable, file routes are not registered;
5. creates the PostgreSQL repositories, WebSocket manager, and cache;
6. starts the WebSocket manager in a separate goroutine;
7. registers HTTP routes and wraps them with middleware for HTTP metrics;
8. starts serving HTTP requests.

## Technologies

| Area | Implementation in the project |
|---|---|
| Language | Go, version from `go.mod`: `1.26` |
| HTTP server | `net/http` |
| HTML | `html/template` and templates in `web/templates` |
| WebSocket | `github.com/gorilla/websocket` |
| Database | PostgreSQL through GORM |
| Cache and Pub/Sub | Redis through `go-redis/redis/v8`; fallback: `MemoryCache` |
| Files | MinIO through `minio-go/v7` |
| Passwords | `bcrypt` through `golang.org/x/crypto` |
| Metrics | `prometheus/client_golang` |
| Container | Multi-stage build in `Dockerfile` |

## Project structure

```text
cmd/server/main.go              entry point, dependency initialization, and routes
config/                         configuration loading and PostgreSQL connection
internal/
  metrics/                       application Prometheus metrics
  middleware/                    HTTP metrics middleware
  models/                       entity models used by repositories
  storage/                      MinIO client and object operations
  websocket/                    connection client, manager, and server registry
models/                         separate User, Message, and Session models
pkg/
  crypto/                       password hashing and verification
  models/                       DTOs for messages, users, and sessions
repository/
  postgres/                     user, message, and session repositories
  cache/                        cache interface, Redis and in-memory implementations
web/
  handlers.go                   pages, registration, login, logout, and auth middleware
  websocket.go                  HTTP-to-WebSocket adapter
  file_handlers.go              file pages and HTTP API
  render.go                     HTML template rendering
  debug.go                      debugging and diagnostic handlers
  templates/                    interface HTML templates
  static/                       static files
migrations/                     initial SQL schema file
Dockerfile                      server container build

docker_local/                   local PostgreSQL and monitoring configuration
scripts/minio_token/             systemd files and documentation for a MinIO token
tests/                          shell scripts for PostgreSQL and Redis checks
```

The project contains two sets of models. `internal/models` contains entities used by the PostgreSQL repositories, while `pkg/models` contains DTOs passed between WebSocket code, handlers, the cache, and repositories. The `models` directory also contains `User`, `Message`, and `Session` types; based on the source layout, it is a separate set of types.

## Architecture and component responsibilities

### HTTP and presentation

The `web` package serves HTML pages and the HTTP API. `renderTemplate` loads templates from `web/templates`. The main pages are login, registration, chat, and file storage.

### Authentication and sessions

A session is stored in PostgreSQL and its representation can be cached. The session ID is sent in the `session_id` cookie. `getUserFromSession` checks the cache first, then uses `SessionRepository`, verifies expiration, and returns the username when the session is valid. `AuthMiddleware` redirects requests without a valid session to the login page.

During registration, the password is hashed with bcrypt in `UserRepository`. During login, the hash is compared with the supplied password.

### WebSocket

`websocket.WebSocketHandler` checks the session, upgrades the HTTP connection to WebSocket, creates a `Client`, and passes it to the manager. Each client gets a `ReadPump` and a `WritePump` goroutine.

`Manager` stores connected clients, online users, and event channels. It is responsible for:

- registering and removing clients;
- sending message history to a new client;
- broadcasting messages locally;
- broadcasting online-user status;
- processing file events;
- processing message edits;
- saving messages and updating the cache;
- exchanging messages and file events through Redis Pub/Sub;
- periodically collecting part of the database statistics.

### Data storage

PostgreSQL repositories separate database operations from HTTP and WebSocket code. The cache implements the common `cache.Cache` interface, so the manager can use either Redis or the in-memory cache.

MinIO is used separately from PostgreSQL. File metadata is represented by `FileInfo`, while the object itself is stored in MinIO.

## Data model

### PostgreSQL

The application creates these tables:

- `users` — ID, unique username, password hash, last-seen time, and timestamp fields;
- `messages` — author, text, message time, timestamp fields, and edit fields;
- `sessions` — session ID, user, expiration time, and timestamp fields.

`messages.user_id` and `sessions.user_id` reference `users.id`. `config/database.go` also creates indexes, the `get_recent_messages` SQL function, and the `active_users` view.

`MessageRepository.GetRecent` selects recent messages with their users and returns them in oldest-to-newest order. When a client connects, the manager first tries to obtain recent messages from the cache and then queries the database if the cache has no data.

### Cache

The cache stores:

- recent messages;
- users;
- sessions.

The message cache is limited to 100 items in both implementations. Redis stores values with the configured TTL and prefix. The memory cache removes expired entries in a separate goroutine.

### Files

A MinIO object key is built using this pattern:

```text
chat-<chat_id>/<user_id>/<timestamp>-<filename>
```

`FileInfo` contains the name, size, upload time, content type, owner, chat ID, and object key.

## Main data flows

### Registration and login

```text
HTTP form
  → RegisterHandler / LoginHandler
  → UserRepository and bcrypt
  → SessionRepository
  → session cache
  → session_id cookie
  → /chat
```

On logout, `LogoutHandler` deletes the session from PostgreSQL, invalidates it in the cache, and removes the cookie.

### Connecting to the chat

```text
GET /ws
  → AuthMiddleware
  → getUserFromSession
  → WebSocketHandler
  → NewClient
  → Manager.Register
  → sendMessageHistory
  → Client.Send
  → WebSocket WritePump
```

History is loaded from the cache, or from PostgreSQL on a cache miss. It is filtered and sorted by time before being sent.

### Sending a message

`Client.ReadPump` accepts JSON in several supported forms: a string, an object with `text`, an object with `type` and `data`, and an object with `username` and `text`. Empty messages are ignored, surrounding whitespace is trimmed, and text longer than 10,000 bytes is truncated.

```text
WebSocket ReadPump
  → parse and validate text
  → Manager.Broadcast
  → save to PostgreSQL
  → update cache
  → Redis Pub/Sub (when Redis is available)
  → local WebSocket clients
```

Saving to PostgreSQL and updating the cache are started in separate goroutines.

### Editing a message

A message with `type: "edit"` is converted to an `EditRequest` and sent to `Manager.EditMessage`. The repository checks message ownership, updates the text and edit fields, and the manager broadcasts the updated DTO to local clients.

### File operations

```text
HTTP request with session_id
  → FileHandler
  → validate chat_id, filename, or key
  → MinIOClient
  → temporary URL / list / delete / proxy file
  → FileEvents
  → WebSocket clients and Redis Pub/Sub
```

For deletion, the code additionally checks that the key belongs to the requested chat and that the owner name contained in the key matches the current user.

### Exchange between server instances

When Redis is available, the manager publishes messages to `chat:messages` and file events to `chat:files`. Events received from another `serverID` are broadcast to local clients. The server registry is stored under `server:<serverID>` keys with a TTL and heartbeat.

## HTTP routes

| Route | Purpose in the code |
|---|---|
| `/` | login page |
| `/login` | login handler |
| `/register` | registration page |
| `/register-handler` | registration handler |
| `/chat` | chat page, session required |
| `/logout` | logout and session deletion |
| `/ws` | chat WebSocket, session required |
| `/files` | file page, when MinIO is active |
| `/api/files/upload-url` | obtain upload data |
| `/api/files/upload-complete` | notify that an upload is complete |
| `/api/files/list` | list chat files |
| `/api/files/download-url` | obtain a download URL |
| `/api/files/delete` | delete the current user's file |
| `/api/files/download` | proxy a file download through the application |
| `/metrics` | Prometheus endpoint |
| `/debug/...` | development debug routes when MinIO is available |

## Configuration

### Command-line flags

`main` declares the `reset-db`, `use-redis`, `port`, and `server-id` flags. They control database reset, Redis selection, HTTP port, and instance ID respectively.

### PostgreSQL

The configuration reads `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`, `PIGEONGRAM_RESET_DB`, and `PIGEONGRAM_FORCE`.

### Redis

The configuration reads `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB`, `REDIS_TTL`, `REDIS_PREFIX`, and `REDIS_ENABLED`.

### MinIO

The configuration reads `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, `MINIO_USE_SSL`, `MINIO_BUCKET`, `MINIO_REGION`, `MINIO_UPLOAD_EXPIRY`, `MINIO_DOWNLOAD_EXPIRY`, `MINIO_MAX_FILE_SIZE`, and `MINIO_PUBLIC_URL`.

### General settings

`SERVER_ENVIRONMENT` selects the environment. `DOMAIN` and `NGINX_PORT` are used by the public-URL helper.

## Observability

`internal/metrics` defines metrics for HTTP requests, WebSocket connections and errors, messages, files, PostgreSQL operations, operation duration, cache operations, and user counts. HTTP middleware collects metrics for ordinary routes; `/ws` is excluded from that middleware.

Local Prometheus and Grafana configuration is located in `docker_local/monitoring`.

## Boundaries and confirmed characteristics of the current code

- If Redis is unavailable, the application continues with `MemoryCache`, but cross-server exchange through Redis is unavailable.
- If MinIO is unavailable, the application continues to start, but file routes are not registered.
- The `Dockerfile` healthcheck requests `/health`, but `main` does not register this route. `HealthCheckHandler` exists in `web/debug.go`, but it is not connected to the router in the current `main` function.
- `MessageRepository.DeleteOlderThan` still contains `TODO` and does not delete anything.
- The WebSocket upgrader accepts any origin (`CheckOrigin` returns `true`).
- Debug routes are registered only when `SERVER_ENVIRONMENT == "development"` and MinIO is available; separate diagnostic handlers exist, but not all of them are connected to the router.
- Kubernetes manifests and local monitoring configuration are present in this repository, but their application belongs to the infrastructure project rather than this README.

## Running

TODO: Add build and run instructions for the application.
