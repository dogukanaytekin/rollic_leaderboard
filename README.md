# Leaderboard Service

A REST API for managing leaderboards and tracking user scores, written in Go. Each leaderboard can have an optional schedule that periodically resets all scores.

## Features

- Create and list leaderboards, with optional interval-based reset schedules
- Set/overwrite user scores (highest score ranks first; ties broken by who reached the score first)
- Fetch the top **N** scores of a board
- Fetch the scores surrounding a given user (the **N** above and **N** below)
- Automatic period reset via a lazy filter, backed by a background cleaner
- Helper endpoint to populate a board with mock data for testing

## Tech Stack

- **Go 1.22** with [Gin](https://github.com/gin-gonic/gin)
- **PostgreSQL 16** for persistence
- **golang-migrate** for schema migrations
- **Docker / Docker Compose** for local orchestration

## Quick Start (Docker)

The fastest way to run everything — Postgres, migrations, and the API — with a single command:

```bash
docker compose up
```

This starts:
1. `postgres` — waits until healthy
2. `migrate` — applies migrations, then exits
3. `api` — starts once migrations complete, listening on **http://localhost:8081**

To stop and remove containers (keeping data):

```bash
docker compose down
```

To also wipe the database volume:

```bash
docker compose down -v
```

## Local Development

Run only Postgres in Docker and the API on your host:

```bash
# 1. start postgres
docker compose up postgres -d

# 2. apply migrations (requires golang-migrate installed)
make migrate-up

# 3. run the API
go run ./cmd/api
```

The app reads configuration from a `.env` file in local development (see below).

## Configuration

Configuration comes from environment variables. Locally these are loaded from `.env`; in Docker they are injected via `docker-compose.yml`.

| Variable   | Description                          | Example                                                           |
|------------|--------------------------------------|-------------------------------------------------------------------|
| `PORT`     | Port the HTTP server listens on      | `8081`                                                            |
| `DB_ADDR`  | PostgreSQL connection string         | `postgres://rollic:rollic@localhost:3435/leaderboard?sslmode=disable` |

Both variables are **required** — the service fails fast with a clear error if either is missing.

## API Reference

Base URL: `http://localhost:8081`

### Create Board
`POST /boards`

```json
{
  "name": "Weekly Tournament",
  "description": "Global leaderboard for weekly tournament",
  "schedule": { "type": "interval", "intervalSeconds": 604800 }
}
```
`schedule` is optional. If provided, `type` must be `"interval"` and `intervalSeconds > 0`.

**201 Created**
```json
{
  "boardId": 1,
  "name": "Weekly Tournament",
  "description": "Global leaderboard for weekly tournament",
  "schedule": { "type": "interval", "intervalSeconds": 604800 }
}
```

### List Boards
`GET /boards`

**200 OK**
```json
[
  { "boardId": 1, "name": "Weekly Tournament" },
  { "boardId": 2, "name": "All-time Top Scores" }
]
```

### Get Board
`GET /boards/{boardId}`

Returns board details including the next scheduled reset (if any).

**200 OK**
```json
{
  "boardId": 1,
  "name": "Weekly Tournament",
  "description": "Global leaderboard for weekly tournament",
  "createdAt": "2026-01-01T12:00:00Z",
  "schedule": { "type": "interval", "intervalSeconds": 604800 },
  "nextResetAt": "2026-01-08T12:00:00Z"
}
```
**404** if the board does not exist.

### Set Score
`POST /boards/{boardId}/scores`

```json
{ "userId": "user_789", "score": 1500 }
```
Each call **overwrites** the user's previous score (not incremental).

**200 OK**
```json
{ "boardId": 1, "userId": "user_789", "score": 1500 }
```
**404** if the board does not exist.

### Get Top Scores
`GET /boards/{boardId}/scores?n=10`

Returns the top `n` users ranked by score (descending; ties broken by earliest to reach the score).

**200 OK**
```json
[
  { "userId": "user_1", "score": 5000 },
  { "userId": "user_789", "score": 1500 }
]
```
**400** if `n` is missing or not a positive integer. **404** if the board does not exist.

### Get Score Surroundings
`GET /boards/{boardId}/scores/{userId}/surroundings?n=5`

Returns the user's score along with the `n` users immediately above and below them.

**200 OK**
```json
{
  "user":  { "userId": "user_789", "score": 1500 },
  "above": [ { "userId": "user_above_1", "score": 1510 } ],
  "below": [ { "userId": "user_below_1", "score": 1490 } ]
}
```
**400** if `n` is invalid. **404** if the board or user is not found.

### Populate (testing helper)
`POST /boards/{boardId}/populate?n=100`

Fills a board with `n` mock users (`mock_user_1` … `mock_user_n`) with random scores.

**200 OK**
```json
{ "boardId": 1, "populated": 100 }
```

### Health
`GET /health` → `{ "status": "ok" }`

## Project Structure

```
cmd/api/            # entrypoint: wiring config, db, store, worker, server
internal/
  config/           # environment configuration loading + validation
  db/               # database connection + pool setup
  domain/           # core types (Board, Score, Schedule, ...)
  server/           # HTTP layer: handlers, routing, middleware
  store/            # repository layer: SQL queries behind interfaces
  worker/           # background cleaner goroutine
migrations/         # SQL schema migrations
```

The layering is **handler → store (repository) → database**. Repositories are defined as interfaces, so handlers and the worker are unit-tested against mocks with no database dependency.

## Design Notes

**Period reset (lazy filter + background cleaner).** Instead of deleting scores at the exact reset moment (which would require a fragile, restart-sensitive scheduler), every read query filters by `scored_at >= periodStart`, where `periodStart` is derived from the board's `created_at` and `interval`. This guarantees correctness at all times. A background goroutine then physically deletes stale rows every 2 hours purely as housekeeping — if it runs late, nothing breaks.

**Indexing.** A single composite index `(board_id, score DESC, scored_at ASC)` serves the hot paths. Top-scores reads walk it directly with no sort step; surroundings queries seek to the user's score and read neighbours via forward/backward index scans, avoiding `OFFSET` and window-function full-partition scans.

**Tie-breaking.** Equal scores are ordered by `scored_at` ascending — the user who reached the score first ranks higher.

**Error handling.** Internal errors are logged server-side and returned to clients as a generic `Internal server error`, so database details never leak. Domain errors (400/404) carry meaningful, spec-defined messages.

## Testing

```bash
go test ./...
```

The suite covers all HTTP handlers (happy paths, validation, 404/500 cases) and the background cleaner's period logic and batching — all against repository mocks, so no database is required to run them.

## Database Schema

Two tables: `boards` (with a check constraint enforcing valid schedules) and `scores` (with a `UNIQUE (board_id, user_id)` constraint enabling upserts, and the composite ranking index). See `migrations/` for the full schema.
