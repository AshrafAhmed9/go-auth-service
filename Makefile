run:
	go run main.go

test:
	go test ./... -v

build:
	go build -o bin/app main.go

fmt:
	go fmt ./...

lint:
	go vet ./...

docker-build:
	docker build -t go-auth-service .

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down

migrate-create:
	migrate create -ext sql -dir migrations -seq $(name)

loadtest-login:
	k6 run loadtest/login.js

loadtest-profile:
	k6 run loadtest/profile.js

loadtest-refresh:
	k6 run loadtest/refresh.js
