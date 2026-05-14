# AI-Script backend Makefile
SHELL := /bin/bash
GO    ?= go
APP   ?= ai-script
PKG   := git.myscrm.cn/ganqx01/ai-script/backend

.PHONY: help tidy run worker build test lint docker swagger

help:           ## 查看帮助
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk -F ':.*?## ' '{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

tidy:           ## go mod tidy
	$(GO) mod tidy

run:            ## 本地启动 server
	$(GO) run ./cmd/server

worker:         ## 本地启动 worker
	$(GO) run ./cmd/worker

build:          ## 构建二进制(out/)
	mkdir -p out
	$(GO) build -trimpath -ldflags="-s -w" -o out/server ./cmd/server
	$(GO) build -trimpath -ldflags="-s -w" -o out/worker ./cmd/worker

test:           ## 跑测试
	$(GO) test ./... -count=1

lint:           ## 静态检查(需 golangci-lint)
	golangci-lint run ./...

docker:         ## 构建 docker 镜像
	docker build -t $(APP):latest .

swagger:        ## 生成 Swagger 文档
	which swag > /dev/null 2>&1 || $(GO) install github.com/swaggo/swag/cmd/swag@latest
	swag init -g internal/server/server.go -o docs --parseDependency --parseInternal
