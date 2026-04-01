FROM golang:1.25 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/worker .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/worker /worker
USER nonroot:nonroot
ENTRYPOINT ["/worker"]