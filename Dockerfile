FROM golang:1.24 AS builder
WORKDIR /app

COPY . .
RUN ls -lha
RUN CGO_ENABLED=0 go build -o deploy-watcher cmd/main.go

FROM gcr.io/distroless/static
COPY --from=builder /app/deploy-watcher /
CMD ["/deploy-watcher"]
