FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.20

RUN adduser -D -u 10001 app
WORKDIR /app

COPY --from=builder /out/server /app/server

USER app
EXPOSE 8080

ENTRYPOINT ["/app/server"]
