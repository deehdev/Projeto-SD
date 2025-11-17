FROM golang:1.25 AS builder

RUN apt update && apt install -y \
    libzmq3-dev \
    libsodium-dev \
    pkg-config \
    git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY broker.go ./

RUN go build -o broker broker.go


FROM debian:stable-slim

RUN apt update && apt install -y \
    libzmq5 \
    libsodium23

WORKDIR /app

COPY --from=builder /app/broker .

CMD ["./broker"]