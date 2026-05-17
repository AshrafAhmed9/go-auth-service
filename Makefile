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
