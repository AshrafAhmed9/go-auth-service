run:
	go run .

test:
	go test ./... -v

build:
	go build -o bin/app .

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
