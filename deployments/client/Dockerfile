FROM golang:1.27

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN go build -o ghost-client ./cmd/client

EXPOSE 1080

CMD ["./ghost-client"]
