# syntax=docker/dockerfile:1.7
# ---------- builder ----------
FROM golang:1.24-alpine AS builder
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux GOPROXY=https://goproxy.cn,direct
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server \
 && go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

# ---------- runtime ----------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata ffmpeg && update-ca-certificates
ENV TZ=UTC

# 修复 P0 F5 — 以非 root 用户运行,遵循最小权限原则
RUN addgroup -g 1000 appgroup && adduser -u 1000 -G appgroup -s /bin/sh -D appuser

WORKDIR /app
COPY --from=builder /out/server /app/server
COPY --from=builder /out/worker /app/worker
COPY configs /app/configs
RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

# 修复 P0 F5 — 容器健康检查,供编排系统(compose/k8s)使用
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://127.0.0.1:8080/healthz/live || exit 1

ENTRYPOINT ["/app/server"]
