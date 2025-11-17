# --------------------------------------------------------
# STAGE 1 — BUILD
# --------------------------------------------------------
FROM golang:1.24-bullseye AS builder

WORKDIR /app

# Dependências necessárias para ZMQ + CGO
RUN apt-get update && apt-get install -y \
    gcc \
    g++ \
    make \
    pkg-config \
    libzmq3-dev \
    libsodium-dev

# Copiar arquivos de módulos do Go e baixar dependências
COPY go.mod go.sum ./
RUN go mod download

# Copiar código do servidor
COPY . .

# Compilar binário
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o server_unified .

# --------------------------------------------------------
# STAGE 2 — RUNTIME
# --------------------------------------------------------
FROM debian:bullseye-slim

WORKDIR /app

# Bibliotecas runtime do ZeroMQ
RUN apt-get update && apt-get install -y \
    libzmq5 \
    libsodium23 && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

COPY --from=builder /app/server_unified /usr/local/bin/server_unified

RUN mkdir -p /app/data

EXPOSE 7000 7001 7002

ENTRYPOINT ["server_unified"]
