# syntax=docker/dockerfile:1.7
# ---------- builder ----------
FROM golang:1.22-alpine AS builder
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
WORKDIR /app
COPY --from=builder /out/server /app/server
COPY --from=builder /out/worker /app/worker
COPY configs /app/configs
EXPOSE 8080
ENTRYPOINT ["/app/server"]
