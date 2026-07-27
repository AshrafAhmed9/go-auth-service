# Go Auth Service

![CI](https://github.com/AshrafAhmed9/go-auth-service/actions/workflows/ci.yml/badge.svg)

A JWT auth service in Go. It does the usual login/signup stuff, but I spent most of the effort on the things that are easy to get wrong: short-lived access tokens with rotating refresh tokens, refresh-token reuse detection, and Redis-backed rate limiting that falls back to in-memory when Redis is down. There's also a gRPC endpoint for other services to validate tokens against, and it runs on Postgres in "production" or SQLite locally so you don't need any infra to try it.

31 tests · Postgres + SQLite · gRPC + REST

## Architecture

```
                          ┌─────────────────────────────────┐
                          │        Gin HTTP :8080            │
                          │                                   │
  Client ──── REST ──────►│  Rate Limiter (Redis / in-memory)│  ← /login only
                          │  → Handler                        │
                          │  (JWT Auth Middleware on /logout) │
                          └──────────┬──────────┬─────────────┘
                                     │          │
  Service ── gRPC :9090 ─► ValidateToken        │
                                     │          │
                          ┌──────────▼──┐  ┌────▼────┐
                          │  Postgres   │  │  Redis  │
                          │  (or SQLite)│  │         │
                          │             │  │ blacklist│
                          │ users       │  │ rate lim │
                          │ refresh_tkns│  └─────────┘
                          └─────────────┘
```

## Features

**Authentication & Token Management**
- JWT access tokens (HS256, 15-min TTL, `jti` claim for revocation)
- Rotating refresh tokens (7-day TTL, SHA-256 hashed, DB-stored)
- Refresh token reuse detection — replaying a consumed token revokes all sessions for that user, via a single atomic conditional UPDATE (not a check-then-write, which would race under concurrent replay)
- Token blacklist via Redis with TTL matching remaining token life (auto-expires, zero cleanup)
- bcrypt password hashing with configurable cost factor

**Security**
- Distributed rate limiting (Redis INCR + EXPIRE) on `/login`, with automatic in-memory fallback if Redis is unreachable
- `alg:none` attack prevention (explicit HMAC signing method assertion)
- Role hardcoded server-side — signup always assigns "user", never trusts client input

**Infrastructure**
- Dual-database support: Postgres (production) + SQLite (local/test) via GORM driver switching
- Versioned SQL migrations (golang-migrate) for Postgres; AutoMigrate for SQLite
- gRPC token-validation service for internal service-to-service auth
- Multi-stage Docker build running as non-root user
- Docker Compose with Postgres + Redis
- CI pipeline running the full test suite on every push

## API

### REST (HTTP :8080)

| Method | Path | Auth | Rate Limited | Description |
|--------|------|------|-------------|-------------|
| POST | /signup | No | No | Register a new user |
| POST | /login | No | Per-IP | Authenticate and receive access + refresh tokens |
| POST | /refresh | No | No | Exchange refresh token for new token pair (rotation) |
| POST | /logout | JWT | No | Revoke access token + all refresh tokens |
| GET | /health | No | No | Service health check |

### gRPC (TCP :9090)

| Service | RPC | Description |
|---------|-----|-------------|
| AuthService | ValidateToken | Validate a JWT and return claims — for internal service-to-service auth |

## Who actually uses the gRPC endpoint

The `ValidateToken` gRPC method isn't just sitting there for show — I built a second service that uses it. It's a Java/Spring Boot resource API ([springboot-resource-api](https://github.com/AshrafAhmed9/springboot-resource-api)) that checks every incoming request against this Go service over gRPC. So the two of them together are a little polyglot microservices setup, and that's also where the load-test numbers for the auth path live — see that repo's README for the two-service k6 results.

```
                 REST + JWT                       gRPC ValidateToken
  Client ─────────────────────▶ Notes API (Java) ─────────────────────▶ Auth Service (Go, this repo)
                                     │                                        │
                                PostgreSQL                            PostgreSQL + Redis
                                (notes)                               (users, revocation blacklist)
```

How it fits together:
- You log in here (`/login`) and hand the JWT to the Java service.
- The Java service calls this service's `ValidateToken` to check it, then applies its own ownership rules.
- Both sides share the same `.proto` file, so if I change the contract the Java build breaks at compile time instead of blowing up at runtime.

A couple of design choices differ between the two services on purpose, which makes for a decent thing to talk through:
- **Revocation vs. caching.** This service checks each token's `jti` against the Redis blacklist every time, so revocation is instant. The Java side caches good validations for up to 60s to avoid a gRPC hop on every request — so a revoked token can stay usable there for up to a minute. That's a conscious trade of instant revocation for latency and less load on this service.
- **Failing closed vs. failing open.** If this service is down and the token isn't cached, the Java service returns 503 — it won't serve data it can't authorize. This service's rate limiter does the opposite: if Redis dies it falls back to in-memory limits rather than locking everyone out, because rate limiting is a nice-to-have, not a security boundary. Same "dependency went down" situation, opposite call, depending on what's actually at stake.

The combined `docker compose up` that boots both services lives in the Java repo (it builds this one from a sibling folder).

## Example Requests

```bash
# Signup
curl -X POST http://localhost:8080/signup \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com","password":"secret123"}'

# Login — returns access_token + refresh_token
curl -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"secret123"}'

# Refresh tokens (rotate)
curl -X POST http://localhost:8080/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"<refresh_token>"}'

# Logout — blacklists access token, revokes all refresh tokens
curl -X POST http://localhost:8080/logout \
  -H "Authorization: Bearer <access_token>"
```

## Security Model

| Threat | Mitigation |
|--------|------------|
| Brute-force login | Per-IP rate limiting (Redis, in-memory fallback) |
| Stolen access token | 15-minute TTL limits blast radius; Redis blacklist for explicit revocation |
| Stolen refresh token | Single-use rotation; reuse detection revokes entire token chain |
| Concurrent replay of a refresh token | Rotation is a single conditional UPDATE (`WHERE revoked_at IS NULL`), so only one of two simultaneous replays can ever succeed |
| Token forgery | HS256 with explicit signing method check (blocks `alg:none` attack) |
| Privilege escalation | Role hardcoded to "user" on signup; admin created only via startup seed |
| Password leakage | bcrypt hashing (cost 12) + `json:"-"` tag (never serialized in responses) |
| Weak JWT secret | Startup fails if secret < 32 characters |
| Container privilege | Docker runs as non-root `appuser` |

## Running Locally

```bash
git clone https://github.com/AshrafAhmed9/go-auth-service.git
cd go-auth-service
cp .env.example .env
# Set JWT_SECRET to a random string of at least 32 characters
make run
```

Admin account is seeded automatically on first run: `admin@app.com` / `admin123`

### With Docker (Postgres + Redis)

```bash
docker compose up --build
```

### Running Tests

```bash
make test                # 31 tests, SQLite in-memory + miniredis (no infra required)
go test ./... -race -cover
```

### Database Migrations (Postgres)

```bash
# Install migrate CLI
go install -tags "pgx5" github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Run migrations
migrate -path migrations -database "pgx5://<DATABASE_URL>" up
```

## Configuration

All configuration via environment variables (`.env` file loaded automatically):

| Variable | Default | Description |
|----------|---------|-------------|
| `JWT_SECRET` | *(required)* | HMAC signing key (min 32 chars) |
| `PORT` | `8080` | HTTP server port |
| `GRPC_PORT` | `9090` | gRPC server port |
| `BCRYPT_COST` | `12` | bcrypt work factor |
| `ACCESS_TOKEN_MINUTES` | `15` | Access token TTL |
| `REFRESH_TOKEN_HOURS` | `168` | Refresh token TTL (7 days) |
| `REDIS_ADDR` | `localhost:6379` | Redis connection address |
| `DB_DRIVER` | `sqlite` | Database driver (`sqlite` or `postgres`) |
| `DATABASE_URL` | — | Postgres connection string |
| `RATE_LIMIT_REQUESTS` | `5` | Max requests per IP per window |
| `RATE_LIMIT_WINDOW_SECONDS` | `60` | Rate limit window duration |

## Makefile Commands

| Command | Description |
|---------|-------------|
| `make run` | Start the server |
| `make test` | Run all 31 tests |
| `make build` | Build binary to `bin/app` |
| `make fmt` | Format all Go files |
| `make lint` | Run `go vet` |
| `make docker-build` | Build Docker image |
| `make migrate-up` | Run Postgres migrations |
| `make migrate-down` | Rollback Postgres migrations |

## Design Decisions & Tradeoffs

| Decision | Why |
|----------|-----|
| Short access token (15m) + rotating refresh token (7d) | Limits blast radius of a stolen access token while keeping users logged in. Rotation + reuse detection catches token theft. |
| Refresh rotation is one conditional UPDATE, not read-then-write | Two concurrent replays of the same refresh token must not both succeed. A single `UPDATE ... WHERE revoked_at IS NULL` makes the "claim this token" step atomic — the loser's update affects zero rows and triggers reuse detection instead. |
| Refresh token reuse detection revokes entire chain | If an already-consumed refresh token is replayed, the server assumes theft and revokes all of the user's sessions — forces re-authentication. |
| Redis rate limiting with in-memory fallback | Distributed rate limiting survives across multiple instances. If Redis is down, the service degrades gracefully to per-instance in-memory limits rather than failing open or taking the login system down. |
| Postgres for production, SQLite for local/test | Tests run in ~2s on in-memory SQLite with zero infrastructure. Production uses Postgres with versioned migrations. GORM's driver abstraction makes the switch a one-line config change. |
| gRPC for internal token validation, REST for public API | Other microservices validate tokens via a typed gRPC contract (binary protobuf, HTTP/2) rather than each reimplementing JWT parsing. Public API stays REST for browser/client compatibility. |
| Blacklist by JTI, not full token string | Shorter Redis keys. Each access token gets a UUID `jti` claim; revocation stores only the 36-char ID instead of the full ~300-char JWT. |
| bcrypt cost 12 | ~340ms per hash on modern hardware. High enough to make brute-forcing impractical, low enough for acceptable login latency. Configurable via env var. |

## Limitations

- **JWT secret rotation** — not implemented; production systems would use a managed secret store (Vault, AWS KMS) with key rotation policies
- **No MFA** — single-factor authentication only
- **No account lockout** — brute-force defense is per-IP rate limiting only; a distributed attack from many IPs against one account isn't caught
- **CORS** — not configured; browser-facing deployments would need explicit origin restrictions
- **Single Redis instance** — a Redis Sentinel or Cluster setup would eliminate the SPOF
- **No OAuth2/OIDC** — no "Login with Google" or federated identity; the service is its own identity provider
- **No audit log** — security events aren't persisted anywhere queryable beyond stdout

## Project Structure

```
main.go              HTTP + gRPC server startup, route registration
config.go            Environment variable loading and validation
models.go            User and RefreshToken structs
store.go             DB connection (Postgres/SQLite), admin seeding
jwt.go                JWT generation + parsing (HS256, jti)
auth.go               Signup, Login, Refresh, Logout, requireAuth middleware
ratelimit.go          Per-IP rate limiting with in-memory fallback
redis.go              Redis client (blacklist + rate-limit counter)
grpc.go               gRPC ValidateToken implementation
*_test.go             31 tests, one file per source file it covers
helpers_test.go       shared test helpers (miniredis setup) — no tests of its own

proto/
├── auth.proto        Protobuf service definition
└── authpb/           Generated Go stubs
migrations/           Versioned SQL migrations (Postgres)
Dockerfile
docker-compose.yml    Postgres + Redis + API
.github/workflows/ci.yml
```
