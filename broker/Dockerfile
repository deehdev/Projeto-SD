FROM golang:1.25 AS builder

RUN apt-get update && apt-get install -y \
    pkg-config \
    libzmq3-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o broker .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y libzmq5 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/broker .

CMD ["./broker"]
