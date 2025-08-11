FROM golang:1.24.5-alpine3.22 AS builder

WORKDIR /app
COPY . .

RUN GOOS=linux go build -o app .

FROM debian

COPY --from=builder /app/app /app
ENTRYPOINT ["/app"]
