# ----------------------------
# STAGE 1 — Build
# ----------------------------
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Instalar dependências necessárias para compilação com CGO e ZeroMQ
RUN apk add --no-cache gcc g++ make pkgconfig zeromq zeromq-dev

# Copiar arquivos de dependências do Go
COPY go.mod go.sum ./

# Baixar dependências do Go
RUN go mod download

# Copiar todo o código
COPY . .

# Compilar binário (CGO_ENABLED=1 para ZeroMQ)
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o ref .

# ----------------------------
# STAGE 2 — Runtime
# ----------------------------
FROM alpine:3.19

WORKDIR /app

# Instalar biblioteca runtime do ZeroMQ
RUN apk add --no-cache zeromq

# Copiar binário compilado
COPY --from=builder /app/ref /usr/local/bin/ref

# Expor porta do serviço
EXPOSE 6000

# Entrypoint
ENTRYPOINT ["ref"]
