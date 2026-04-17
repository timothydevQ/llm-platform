FROM golang:1.22-alpine AS builder
RUN apk add --no-cache ca-certificates gcc musl-dev
WORKDIR /app
COPY go.mod ./
RUN go mod download || true
COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /service ./cmd/

FROM alpine:3.20
RUN apk add --no-cache ca-certificates sqlite
COPY --from=builder /service /service
RUN mkdir -p /data
ENTRYPOINT ["/service"]
