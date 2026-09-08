FROM golang:1.27

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

RUN go build -o ghost-server ./cmd/server

EXPOSE 443

CMD ["./ghost-server"]
