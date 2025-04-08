FROM golang:1.23 as builder

WORKDIR /app
COPY . .

RUN go mod tidy && GOOS=linux CGO_ENABLED=0 GOARCH=amd64 go build -o proxy-service ./cmd/proxy/

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/proxy-service .

CMD ["/root/proxy-service", "-ipv6n=16", "-interface=eth0", "verbose=true"]
