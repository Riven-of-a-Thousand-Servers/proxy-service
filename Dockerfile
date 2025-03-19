FROM golang:1.23 as builder

WORKDIR /app
COPY . .

RUN go mod tidy && GOOS=linux GOARCH=amd64 go build -o proxy-service ./cmd/proxy/

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/proxy-service .

CMD ["/root/proxy-service"]
