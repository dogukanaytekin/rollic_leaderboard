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

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/boards` | Create a board (optional `interval` schedule) |
| `GET`  | `/boards` | List all boards |
| `GET`  | `/boards/{boardId}` | Board details, including `nextResetAt` if scheduled |
| `POST` | `/boards/{boardId}/scores` | Set/overwrite a user's score |
| `GET`  | `/boards/{boardId}/scores?n=10` | Top `n` scores |
| `GET`  | `/boards/{boardId}/scores/{userId}/surroundings?n=5` | A user's score with the `n` above and below |
| `POST` | `/boards/{boardId}/populate?n=100` | Fill a board with `n` mock users (testing helper) |
| `GET`  | `/health` | Liveness check |

Request/response payloads follow the case-study specification. A few behavioural notes:

- **Set Score** overwrites the previous score (not incremental); ranking is descending, ties broken by who reached the score first.
- **`n`** must be a positive integer (otherwise `400`).
- Missing boards/users return `404`.

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

## Future Improvements

**Redis sorted sets as a leaderboard backend.** Real-time game leaderboards are often built on Redis sorted sets (ZSET) rather than a relational store. I evaluated this approach first, but chose PostgreSQL to stay aligned with the spec's explicit requirements — **persistence** as the source of truth and a proper **index** for ranking — without depending on a cache layer's durability settings. With a deliberate design, Redis ZSETs would be a strong addition:

- **O(log N) everywhere.** `ZADD` (set score), `ZREVRANGE` (top-N), and `ZREVRANK` (a user's rank) are all logarithmic, and rank — which we currently don't expose cheaply — comes essentially for free.
- **Periodic resets via key + TTL.** Modelling each period as its own key (`board:{id}:period:{periodStart}`) makes a reset just a switch to a new key, with the old one expiring automatically via TTL — replacing the background cleaner entirely.
- **Naturally sorted.** The data structure keeps members ranked by score at all times, so reads never sort.

The migration would be low-risk thanks to the **repository pattern** already in place: scores live behind the `ScoreRepository` interface, so a `RedisScoreRepository` could be added as a second implementation and selected via configuration — without touching handlers, routing, middleware, or response types. Board metadata would remain in PostgreSQL.

The main subtlety to design around is **tie-breaking**: the spec ranks earlier-achieved scores higher, but a ZSET orders equal scores lexicographically by member. This requires either encoding score+timestamp into the ZSET score (bounded by float64's 53-bit precision) or into the member key — a trade-off worth making explicit before implementing.

A production-grade evolution would keep PostgreSQL as the durable source of truth with Redis as a write-through read layer, combining durability with Redis's read performance.
