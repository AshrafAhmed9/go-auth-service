# assignment-golang

A REST API built with Go featuring JWT authentication and role-based access control.

## Stack
- Go
- Gin
- GORM
- SQLite
- JWT (golang-jwt/jwt v5)
- bcrypt

## Setup

1. Clone the repository
   git clone https://github.com/AshrafAhmed9/assignment-golang.git

2. Create your .env file
   JWT_SECRET=supersecretjwtkey123456789
   PORT=8080

3. Run the server
   go run main.go

## Endpoints

| Method | Path     | Auth | Role  |
|--------|----------|------|-------|
| POST   | /signup  | No   | -     |
| POST   | /login   | No   | -     |
| GET    | /profile | JWT  | any   |
| GET    | /users   | JWT  | admin |

## Notes
- Admin is seeded automatically on first run (admin@app.com / admin123)
- Passwords are hashed with bcrypt
- JWT tokens expire after 24 hours
