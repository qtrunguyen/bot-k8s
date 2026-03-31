FROM golang:1.25 AS builder
WORKDIR /app
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bot

FROM scratch
COPY --from=builder /app/bot /bot

ENTRYPOINT ["/bot"]