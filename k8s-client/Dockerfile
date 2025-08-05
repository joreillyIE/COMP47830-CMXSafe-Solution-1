FROM golang:1.24 AS builder
WORKDIR /app
COPY . .

RUN GOOS=linux go build -o app .

FROM debian

COPY --from=builder /app/app /app
ENTRYPOINT ["/app"]
