FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o lgln-citygml-proxy .

# ---

FROM alpine:3.21

RUN adduser -D -u 1000 proxy

WORKDIR /app
COPY --from=builder /build/lgln-citygml-proxy .

USER proxy

VOLUME ["/cache"]

EXPOSE 8080

ENTRYPOINT ["/app/lgln-citygml-proxy", "serve"]
CMD ["--port", "8080", "--cache-dir", "/cache"]
