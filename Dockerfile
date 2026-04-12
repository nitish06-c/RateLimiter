FROM golang:1.20-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /ratelimiter ./cmd/limiter

FROM alpine:3.18
RUN apk --no-cache add ca-certificates
COPY --from=builder /ratelimiter /ratelimiter
COPY configs/rules.yaml /etc/ratelimiter/rules.yaml

EXPOSE 8080
ENTRYPOINT ["/ratelimiter"]
CMD ["-config", "/etc/ratelimiter/rules.yaml"]
