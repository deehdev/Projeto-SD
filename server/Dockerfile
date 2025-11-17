FROM golang:latest

RUN apt-get update && apt-get install -y libzmq3-dev

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o server .

CMD ["./server"]