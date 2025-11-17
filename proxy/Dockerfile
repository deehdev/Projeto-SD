FROM golang:1.23-bullseye AS builder
ENV GOTOOLCHAIN=auto

WORKDIR /app

# ZeroMQ + libs obrigatórias
RUN apt-get update && apt-get install -y \
    libzmq3-dev pkg-config build-essential && \
    rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o proxy .
