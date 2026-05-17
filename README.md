# Authentication & RBAC API in Go

![CI](https://github.com/AshrafAhmed9/go-auth-service/actions/workflows/ci.yml/badge.svg)

A production-inspired JWT authentication and role-based access control REST API built with Go.

## Architecture

```
Client
  → Gin Router
  → Security Headers Middleware
  → Request ID Middleware
  → Structured JSON Logger
  → JWT Auth Middleware
  → RBAC Middleware
  → Handler
  → SQLite (GORM)
```

## Features

- JWT authentication with HS256 signing and issuer validation
- Role-based access control (admin / user)
- bcrypt password hashing (configurable cost factor)
- In-memory rate limiting on login (5 req/min per IP) with cleanup goroutine
- Structured JSON request logging via log/slog
- Request ID propagation (X-Request-ID) across logs and error responses
- Security headers (X-Content-Type-Options, X-Frame-Options)
- Health endpoint with DB latency and server uptime
- Graceful shutdown with configurable timeout
- HTTP server timeouts (read, write, idle)
- Admin seeded at startup — no public admin registration
- SQLite with GORM AutoMigrate

## Request Flow

```
POST /login
  → validate input (required fields, email format)
  → normalize email (lowercase + trim)
  → lookup user by email
  → bcrypt.CompareHashAndPassword
  → GenerateToken (HS256, configurable expiry, issuer: go-auth-service)
  → return { "token": "..." }
```

## Security

| Concern | Mitigation |
|---------|------------|
| Brute-force login | Rate limiter — 5 req/min per IP |
| Token forgery | HS256 with explicit algorithm check (blocks alg:none attack) |
| Privilege escalation | Role hardcoded server-side, never from request body |
| Password leakage | bcrypt hashing + json:"-" tag (never serialized) |
| Weak JWT secret | Startup panic if secret < 32 characters |
| Container privilege | Docker runs as non-root appuser |

## Threat Model

- **Brute-force login attempts** — mitigated by rate limiting per IP
- **Token forgery** — mitigated by HS256 + explicit signing method validation
- **Privilege escalation** — role is hardcoded to "user" on signup; admin exists only via startup seed
- **Password leakage** — passwords are bcrypt-hashed and never appear in any API response

## API

| Method | Path | Auth | Role | Description |
|--------|------|------|------|-------------|
| POST | /signup | No | - | Register a new user |
| POST | /login | No | - | Login and receive JWT |
| GET | /profile | JWT | any | Get own profile |
| GET | /users | JWT | admin | List all users |
| GET | /health | No | - | Service health check |

## Example Requests

```bash
# Signup
curl -X POST http://localhost:8080/signup \
  -H "Content-Type: application/json" \
  -d '{"name":"Alice","email":"alice@example.com","password":"secret123"}'

# Login
curl -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"email":"alice@example.com","password":"secret123"}'

# Profile (replace TOKEN)
curl http://localhost:8080/profile \
  -H "Authorization: Bearer TOKEN"

# All users — admin only
curl http://localhost:8080/users \
  -H "Authorization: Bearer ADMIN_TOKEN"
```

## Running Locally

1. Clone the repository

```bash
git clone https://github.com/AshrafAhmed9/go-auth-service.git
cd go-auth-service
```

2. Create .env from example

```bash
cp .env.example .env
```

Set JWT_SECRET to a random string of at least 32 characters.

3. Run

```bash
make run
```

Admin account is seeded automatically on first run: admin@app.com / admin123

## Running Tests

```bash
make test
```

With coverage:

```bash
go test ./... -cover
```

## Docker

```bash
docker compose up --build
```

## Makefile Commands

| Command | Description |
|---------|-------------|
| make run | Run the server |
| make test | Run all tests |
| make build | Build binary to bin/app |
| make fmt | Format all Go files |
| make lint | Run go vet |
| make docker-build | Build Docker image |

## Tradeoffs

| Decision | Why |
|----------|-----|
| SQLite over PostgreSQL | Zero-dependency local setup; swap the GORM driver for production |
| JWT over sessions | Stateless auth — no session store needed |
| Gin over net/http | Built-in middleware chaining, route groups, and JSON binding |
| bcrypt over SHA | Adaptive hashing — cost factor increases with hardware |
| In-memory rate limiter | Simple for single-instance; use Redis for distributed deployments |

## Limitations

- **No token revocation** — JWTs remain valid until expiry; no blacklist or revocation store is implemented
- **No refresh tokens** — chosen for simplicity and statelessness; production systems pair access tokens with refresh tokens for session continuity
- **JWT secret rotation** — not implemented; production systems would use managed secret stores with rotation policies
- **SQLite concurrency** — not optimized for high-concurrency writes; replace with PostgreSQL for production
- **Rate limiter** — in-memory only, resets on restart; use Redis for multi-instance deployments
- **No MFA** — single-factor authentication only
- **CORS** — not configured; this is a backend-only service; browser-facing deployments would restrict allowed origins explicitly

## Notes

- Authentication verifies **who you are** (JWT). Authorization verifies **what you can do** (RBAC middleware). These are intentionally separated.
- Future versions would expose routes under /api/v1 for backwards compatibility.
- Dependencies are pinned via go.mod and go.sum for reproducible builds.
