FROM golang:1.23 as builder

WORKDIR /app
COPY . .

RUN go mod tidy && GOOS=linux CGO_ENABLED=0 GOARCH=amd64 go build -o proxy-service ./cmd/proxy/

FROM alpine:latest
RUN apk add --no-cache curl
WORKDIR /root/
COPY --from=builder /app/proxy-service .

CMD ["/root/proxy-service", "-v6_n=16", "-interface=eth0", "-verbose=true", "print_addrs=true"]
