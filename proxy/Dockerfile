FROM golang:latest

RUN apt-get update && apt-get install -y libzmq3-dev

WORKDIR /app

# Copiar dependências
COPY go.mod go.sum ./

# Baixar módulos (agora incluindo msgpack)
RUN go mod download

# Copiar o código
COPY . .

RUN go build -o proxy .

CMD ["./proxy"]
