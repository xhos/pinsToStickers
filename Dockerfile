FROM golang:1.21-alpine AS builder

RUN apk add --no-cache git gcc musl-dev sqlite-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o itanoru-bot cmd/bot/main.go

FROM alpine:latest

RUN apk add --no-cache \
    python3 \
    py3-pip \
    ffmpeg \
    imagemagick \
    ca-certificates \
    tzdata \
    sqlite

# Install gallery-dl
RUN pip3 install --no-cache-dir gallery-dl

WORKDIR /app

COPY --from=builder /app/itanoru-bot .
COPY config/gallery-dl.conf /etc/gallery-dl.conf

# Create directories
RUN mkdir -p /app/data /app/temp /app/logs

# Create non-root user
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -s /bin/sh -D appuser && \
    chown -R appuser:appgroup /app

USER appuser

VOLUME ["/app/data", "/app/logs"]

ENV TZ=UTC
ENV BOT_TOKEN=""
ENV DB_PATH="/app/data/bot.db"
ENV TEMP_DIR="/app/temp"
ENV LOG_LEVEL="info"
ENV SYNC_INTERVAL="@hourly"
ENV MAX_CONCURRENT_SYNCS="3"
ENV GALLERY_DL_RATE_LIMIT="1.0-2.0"
ENV MAX_STICKERS="120"

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD pgrep itanoru-bot || exit 1

CMD ["./itanoru-bot"]