### ------------ STAGE 1 — BUILD ------------- ###
FROM golang:1.23-bullseye AS builder

# Instalar dependências do ZeroMQ
RUN apt-get update && \
    apt-get install -y libzmq3-dev pkg-config && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compilar com CGO HABILITADO
RUN CGO_ENABLED=1 GOOS=linux go build -o server_sync .


### ------------ STAGE 2 — RUNTIME ------------- ###
FROM debian:bullseye-slim

# Instalar libzmq (runtime)
RUN apt-get update && \
    apt-get install -y libzmq5 && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /app/server_sync .

CMD ["./server_sync"]