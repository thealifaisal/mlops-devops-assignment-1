# backend/mlops-go-api/Dockerfile
# Build the Go binary
FROM golang:1.21-alpine AS build
WORKDIR /src
COPY ./backend/mlops-go-api/go.mod ./backend/mlops-go-api/go.sum ./backend/mlops-go-api/
RUN cd backend/mlops-go-api && go mod download
COPY ./backend/mlops-go-api ./backend/mlops-go-api
RUN cd backend/mlops-go-api && CGO_ENABLED=0 GOOS=linux go build -o /app ./cmd/server

# Final image
FROM alpine:3.18
RUN apk add --no-cache ca-certificates
COPY --from=build /app /app
COPY ./backend/mlops-go-api/internal/db/migrations /internal/db/migrations
EXPOSE 8080
ENTRYPOINT ["/app"]
