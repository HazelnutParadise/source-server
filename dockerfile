FROM golang:1.23-alpine

WORKDIR /app

COPY . .

RUN go mod init source-server

RUN go mod tidy

RUN go build -o main main.go

CMD ["./main"]
