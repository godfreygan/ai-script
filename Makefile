.PHONY: build build-frontend build-all test test-frontend run server worker migrate lint lint-frontend fmt clean swagger docker up down

# 默认目标
.DEFAULT_GOAL := build

# ---------- Backend ----------

build:
	cd backend && go build -o bin/server ./cmd/server
	cd backend && go build -o bin/worker ./cmd/worker

test:
	cd backend && go test ./... -race -count=1

run:
	cd backend && go run ./cmd/server

server:
	cd backend && go run ./cmd/server

worker:
	cd backend && go run ./cmd/worker

migrate:
	cd backend && go run ./cmd/server -migrate

lint:
	cd backend && golangci-lint run ./...

fmt:
	cd backend && gofmt -w .

swagger:
	cd backend && swag init -g internal/server/server.go

# ---------- Frontend ----------

build-frontend:
	cd frontend && npm ci && npm run build

lint-frontend:
	cd frontend && npm run lint

test-frontend:
	cd frontend && npm run test

# ---------- Combined ----------

build-all: build build-frontend

# ---------- Misc ----------

clean:
	rm -rf backend/bin/

docker:
	docker compose build

up:
	docker compose up -d

down:
	docker compose down
