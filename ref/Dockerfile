FROM golang:1.23-bullseye AS builder

RUN apt-get update && \
    apt-get install -y libzmq3-dev pkg-config && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Compile ref.go
RUN CGO_ENABLED=1 GOOS=linux go build -o ref ref.go


FROM debian:bullseye-slim
RUN apt-get update && apt-get install -y libzmq5 && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/ref .

CMD ["./ref"]