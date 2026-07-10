# Authentication & Authorization Service in Go

![CI](https://github.com/AshrafAhmed9/go-auth-service/actions/workflows/ci.yml/badge.svg)

A production-grade JWT authentication, RBAC, and token-management service built with Go. Features short-lived access tokens with rotating refresh tokens, Redis-backed distributed rate limiting with in-memory fallback, account lockout, per-user throttling, audit logging, gRPC token validation, and dual-database support (Postgres for production, SQLite for zero-dependency local development).

**31 tests** | **Postgres + SQLite dual-driver** | **gRPC + REST** | **k6 load-tested**

## Architecture

```
                          ┌─────────────────────────────────────┐
                          │           Gin HTTP :8080            │
                          │                                     │
  Client ──── REST ──────►│  Security Headers                   │
                          │  → Request ID (X-Request-ID)        │
                          │  → Structured JSON Logger (slog)    │
                          │  → Rate Limiter (Redis / in-memory) │
                          │  → JWT Auth Middleware               │
                          │  → Per-User Rate Limiter             │
                          │  → RBAC Middleware                   │
                          │  → Handler                           │
                          └──────────┬──────────┬───────────────┘
                                     │          │
  Service ── gRPC :9090 ─► ValidateToken        │
                                     │          │
                          ┌──────────▼──┐  ┌────▼────┐
                          │  Postgres   │  │  Redis  │
                          │  (or SQLite)│  │         │
                          │             │  │ blacklist│
                          │ users       │  │ lockout  │
                          │ refresh_tkns│  │ rate lim │
                          │ audit_events│  └─────────┘
                          └─────────────┘
```

## Features

**Authentication & Token Management**
- JWT access tokens (HS256, 15-min TTL, `jti` claim for revocation)
- Rotating refresh tokens (7-day TTL, SHA-256 hashed, DB-stored)
- Refresh token reuse detection — replaying a consumed token revokes all sessions for that user
- Token blacklist via Redis with TTL matching remaining token life (auto-expires, zero cleanup)
- bcrypt password hashing with configurable cost factor

**Authorization & Security**
- Role-based access control (admin / user) with middleware separation
- Account lockout after N failed login attempts (Redis, configurable threshold + duration)
- Distributed rate limiting (Redis INCR + EXPIRE) with automatic in-memory fallback
- Per-user rate limiting on authenticated routes
- `alg:none` attack prevention (explicit HMAC signing method assertion)
- Role hardcoded server-side — signup always assigns "user", never trusts client input
- Security headers (X-Content-Type-Options, X-Frame-Options)

**Observability & Operations**
- Audit event log (DB table) for signup, login success/failure, lockout, refresh, logout
- Structured JSON request logging via `log/slog` with request ID correlation
- Health endpoint with DB latency and server uptime
- Graceful shutdown for both HTTP and gRPC servers
- HTTP server timeouts (read: 10s, write: 10s, idle: 30s)

**Infrastructure**
- Dual-database support: Postgres (production) + SQLite (local/test) via GORM driver switching
- Versioned SQL migrations (golang-migrate) for Postgres; AutoMigrate for SQLite
- gRPC token-validation service for internal service-to-service auth
- Multi-stage Docker build running as non-root user
- Docker Compose with Postgres + Redis
- CI pipeline with Postgres + Redis services, migration step, and test execution

## Load Test Results

Tested with k6 (10 concurrent VUs, 40s run, SQLite, single instance):

| Endpoint | Req/s | p50 | p95 | Bottleneck |
|----------|-------|-----|-----|------------|
| `POST /login` | ~17 | 345ms | 409ms | bcrypt cost 12 (~340ms/hash) |
| `GET /profile` | ~72 | 4ms | 10ms | JWT parse + DB read |

Login throughput plateaus at ~17 req/s regardless of concurrency because each request blocks on `bcrypt.CompareHashAndPassword` for ~340ms. This is a feature, not a bug — the same property that limits throughput is what makes brute-forcing a leaked password database impractical. The authenticated read path (`/profile`) is 18x faster because it only validates the JWT signature (a fast HMAC check) and performs a single DB lookup.

Scripts: `loadtest/login.js`, `loadtest/profile.js`, `loadtest/refresh.js`

## API

### REST (HTTP :8080)

| Method | Path | Auth | Rate Limited | Description |
|--------|------|------|-------------|-------------|
| POST | /signup | No | No | Register a new user |
| POST | /login | No | Per-IP | Authenticate and receive access + refresh tokens |
| POST | /refresh | No | No | Exchange refresh token for new token pair (rotation) |
| POST | /logout | JWT | Per-user | Revoke access token + all refresh tokens |
| GET | /profile | JWT | Per-user | Get authenticated user's profile |
| GET | /users | JWT + Admin | Per-user | List all users (admin only) |
| GET | /health | No | No | Service health with DB latency and uptime |

### gRPC (TCP :9090)

| Service | RPC | Description |
|---------|-----|-------------|
| AuthService | ValidateToken | Validate a JWT and return claims — for internal service-to-service auth |

## Consumers / Polyglot Architecture

The gRPC `ValidateToken` endpoint isn't just a design flourish — it has a real consumer: a separate **Java/Spring Boot resource service** ([springboot-resource-api](https://github.com/AshrafAhmed9/springboot-resource-api)) that authenticates every request against this Go service over gRPC. Together they form a polyglot microservices identity system.

```
                 REST + JWT                       gRPC ValidateToken
  Client ─────────────────────▶ Notes API (Java) ─────────────────────▶ Auth Service (Go, this repo)
                                     │                                        │
                                PostgreSQL                            PostgreSQL + Redis
                                (notes)                               (users, revocation blacklist)
```

- Clients log in here (`/login`) and send the JWT to the Java service.
- The Java service validates it via this service's gRPC `ValidateToken` and enforces its own resource ownership + `ROLE_ADMIN` rules.
- The `.proto` contract is shared verbatim, so a contract drift is a compile error in the Java build, not a runtime surprise.

**Cross-service design contrasts worth knowing:**
- **Revocation vs. cache latency:** this service checks each token's `jti` against a Redis blacklist for instant revocation. The Java consumer caches successful validations for ≤60s to cut gRPC round-trips — deliberately trading up-to-60s revocation latency for availability and lower load on this service.
- **Fail-closed vs. fail-open:** the Java service *fails closed* (503) when this service is unreachable and the token isn't cached — a resource API must not serve data it can't authorize. This service's rate limiter *fails open* (falls back to in-memory) when Redis dies — rate limiting is a QoS concern, not a security gate. Same failure pattern, opposite policy, driven by what each dependency protects.

A combined `docker compose up` that runs **both** services lives in the Java repo (it builds this service from a sibling checkout).

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

# Access a protected route
curl http://localhost:8080/profile \
  -H "Authorization: Bearer <access_token>"

# Refresh tokens (rotate)
curl -X POST http://localhost:8080/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"<refresh_token>"}'

# Logout — blacklists access token, revokes all refresh tokens
curl -X POST http://localhost:8080/logout \
  -H "Authorization: Bearer <access_token>"

# Admin-only: list all users
curl http://localhost:8080/users \
  -H "Authorization: Bearer <admin_access_token>"
```

## Security Model

| Threat | Mitigation |
|--------|------------|
| Brute-force login | Per-IP rate limiting (Redis, in-memory fallback) + account lockout after N failures |
| Stolen access token | 15-minute TTL limits blast radius; Redis blacklist for explicit revocation |
| Stolen refresh token | Single-use rotation; reuse detection revokes entire token chain |
| Token forgery | HS256 with explicit signing method check (blocks `alg:none` attack) |
| Privilege escalation | Role hardcoded to "user" on signup; admin created only via startup seed |
| Password leakage | bcrypt hashing (cost 12) + `json:"-"` tag (never serialized in responses) |
| Weak JWT secret | Startup panics if secret < 32 characters |
| Container privilege | Docker runs as non-root `appuser` |
| Distributed brute-force | Per-account lockout (Redis) survives across IPs — complements per-IP rate limiting |

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
make test                # 31 tests, SQLite in-memory (no infra required)
go test ./... -cover     # with coverage
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
| `PORT` | *(required)* | HTTP server port |
| `GRPC_PORT` | `9090` | gRPC server port |
| `BCRYPT_COST` | `12` | bcrypt work factor |
| `ACCESS_TOKEN_MINUTES` | `15` | Access token TTL |
| `REFRESH_TOKEN_HOURS` | `168` | Refresh token TTL (7 days) |
| `REDIS_ADDR` | `localhost:6379` | Redis connection address |
| `DB_DRIVER` | `sqlite` | Database driver (`sqlite` or `postgres`) |
| `DATABASE_URL` | — | Postgres connection string |
| `RATE_LIMIT_REQUESTS` | `5` | Max requests per IP per window |
| `RATE_LIMIT_WINDOW_SECONDS` | `60` | Rate limit window duration |
| `PER_USER_LIMIT_REQUESTS` | `30` | Max requests per authenticated user per window |
| `PER_USER_LIMIT_WINDOW_SECONDS` | `60` | Per-user rate limit window |
| `LOCKOUT_MAX_ATTEMPTS` | `5` | Failed logins before account lockout |
| `LOCKOUT_DURATION_MINUTES` | `15` | Account lockout duration |

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
| `make loadtest-login` | k6 load test on `/login` |
| `make loadtest-profile` | k6 load test on `/profile` |

## Design Decisions & Tradeoffs

| Decision | Why |
|----------|-----|
| Short access token (15m) + rotating refresh token (7d) | Limits blast radius of a stolen access token while keeping users logged in. Rotation + reuse detection catches token theft. |
| Refresh token reuse detection revokes entire chain | If an already-consumed refresh token is replayed, the server assumes theft and revokes all of the user's sessions — forces re-authentication. |
| Redis rate limiting with in-memory fallback | Distributed rate limiting survives across multiple instances. If Redis is down, the service degrades gracefully to per-instance in-memory limits rather than failing open. |
| Account lockout keyed by email, not IP | A slow distributed brute-force from many IPs still triggers lockout on the target account. Complements per-IP rate limiting. |
| Postgres for production, SQLite for local/test | Tests run in ~3s on in-memory SQLite with zero infrastructure. Production uses Postgres with versioned migrations. GORM's driver abstraction makes the switch a one-line config change. |
| gRPC for internal token validation, REST for public API | Other microservices validate tokens via a typed gRPC contract (binary protobuf, HTTP/2) rather than each reimplementing JWT parsing. Public API stays REST for browser/client compatibility. |
| Blacklist by JTI, not full token string | Shorter Redis keys. Each access token gets a UUID `jti` claim; revocation stores only the 36-char ID instead of the full ~300-char JWT. |
| bcrypt cost 12 | ~340ms per hash on modern hardware. High enough to make brute-forcing impractical, low enough for acceptable login latency. Configurable via env var. |
| Audit events in DB, not just logs | Logs are ephemeral (stdout). The `audit_events` table is queryable, durable, and survives log rotation — supports compliance, incident investigation, and analytics. |
| Fixed-window rate limiting (INCR + EXPIRE) | Simple, atomic, well-understood. Trades a burst-at-boundary edge case for implementation clarity. A sliding window (Lua script) would eliminate the edge case at the cost of complexity. |

## Limitations

- **JWT secret rotation** — not implemented; production systems would use a managed secret store (Vault, AWS KMS) with key rotation policies
- **No MFA** — single-factor authentication only
- **CORS** — not configured; browser-facing deployments would need explicit origin restrictions
- **Single Redis instance** — a Redis Sentinel or Cluster setup would eliminate the SPOF
- **No OAuth2/OIDC** — no "Login with Google" or federated identity; the service is its own identity provider

## Project Structure

```
├── main.go                  # HTTP + gRPC server startup, route registration
├── config/config.go         # Environment variable loading and validation
├── database/database.go     # Dual-driver DB connection (Postgres/SQLite)
├── cache/redis.go           # Redis client (blacklist, rate limit, lockout)
├── handlers/
│   ├── auth.go              # Signup, Login, Refresh, Logout
│   ├── audit.go             # Audit event writer
│   ├── user.go              # Profile, GetAllUsers
│   └── health.go            # Health check
├── middleware/
│   ├── auth.go              # JWT validation + RBAC
│   ├── ratelimit.go         # Per-IP + per-user rate limiting
│   ├── requestid.go         # X-Request-ID propagation
│   └── security.go          # Security response headers
├── models/
│   ├── user.go              # User model
│   ├── refresh_token.go     # Refresh token model
│   └── audit_event.go       # Audit event model
├── utils/
│   ├── jwt.go               # JWT generation + parsing (HS256, jti)
│   └── token.go             # Refresh token generation + SHA-256 hashing
├── proto/
│   ├── auth.proto           # Protobuf service definition
│   └── authpb/              # Generated Go stubs
├── grpcserver/server.go     # gRPC ValidateToken implementation
├── migrations/              # Versioned SQL migrations (Postgres)
├── loadtest/                # k6 load test scripts
├── tests/                   # 31 test cases
├── Dockerfile               # Multi-stage build, non-root user
├── docker-compose.yml       # Postgres + Redis + API
└── .github/workflows/ci.yml # CI with Postgres + Redis services
```
