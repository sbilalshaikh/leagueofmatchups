FROM golang:1.23.0

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o server ./exec/main.go

EXPOSE 8080

CMD ["./server"]