FROM golang:1.23-bullseye

ENV GOTOOLCHAIN=auto

WORKDIR /app

# instalar dependências do ZeroMQ
RUN apt-get update && apt-get install -y \
    libzmq3-dev pkg-config && \
    rm -rf /var/lib/apt/lists/*

# criar pasta data antes do volume do compose
RUN mkdir -p /app/data

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -o server .

CMD ["./server"]