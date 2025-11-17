# -----------------------------
# STAGE 1 — BUILD
# -----------------------------
FROM golang:1.23-bullseye AS builder
ENV CGO_ENABLED=1
ENV GOOS=linux
ENV GOTOOLCHAIN=auto

# Dependências necessárias para compilar ZeroMQ
RUN apt-get update && apt-get install -y \
    libzmq3-dev \
    libsodium-dev \
    pkg-config \
    build-essential && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o broker .


# -----------------------------
# STAGE 2 — RUNTIME
# -----------------------------
FROM debian:bullseye-slim

# Instala apenas libs necessárias para rodar o binário
RUN apt-get update && apt-get install -y \
    libzmq5 \
    libsodium23 && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/broker .

CMD ["./broker"]
