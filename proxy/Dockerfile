FROM golang:1.25 AS builder

# 📌 INSTALA DEPENDÊNCIAS NECESSÁRIAS PARA ZMQ4
RUN apt-get update && apt-get install -y \
    pkg-config \
    libzmq3-dev \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# 📌 IMPORTANTE: CGO habilitado para libzmq
RUN CGO_ENABLED=1 go build -o proxy .

# -------------------------------
# SEGUNDA IMAGEM FINAL (menor)
# -------------------------------
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y \
    libzmq5 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/proxy .

CMD ["./proxy"]
