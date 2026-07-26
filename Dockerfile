FROM golang:1.26-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o app .

FROM alpine:latest
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/app .
COPY --from=builder /app/migrations ./migrations
RUN mkdir -p data && touch .env && chown -R appuser:appgroup /app
USER appuser
EXPOSE 8080
CMD ["./app"]
