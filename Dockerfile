# Build stage
FROM golang:1.22 as builder

WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o speedtest-exporter

# Create an image
FROM alpine:latest
RUN apk add --no-cache speedtest-cli
WORKDIR /app/
COPY --from=builder /app/speedtest-exporter .
EXPOSE 8080

ENTRYPOINT ["/app/speedtest-exporter"]
